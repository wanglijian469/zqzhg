package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type videoStreamInfo struct {
	Index       int    `json:"index"`
	Type        string `json:"codec_type"`
	Codec       string `json:"codec_name"`
	Pixel       string `json:"pix_fmt"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Transfer    string `json:"color_transfer"`
	Disposition struct {
		Attached int `json:"attached_pic"`
	} `json:"disposition"`
}
type videoInfo struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []videoStreamInfo `json:"streams"`
}

func probeCloudVideo(ctx context.Context, input string) (videoInfo, error) {
	var info videoInfo
	data, e := exec.CommandContext(ctx, videoTool("ffprobe"), "-v", "error", "-protocol_whitelist", "file,pipe", "-f", "mov", "-show_entries", "format=duration:stream=index,codec_type,codec_name,pix_fmt,width,height,color_transfer:stream_disposition=attached_pic", "-of", "json", input).Output()
	if e != nil || json.Unmarshal(data, &info) != nil {
		return info, errors.New("video_probe_failed")
	}
	return info, nil
}
func (info videoInfo) tracks() (videoStreamInfo, *videoStreamInfo, float64, error) {
	return info.tracksWithLimit(float64(settingInt("MAX_VIDEO_SECONDS", 300)))
}
func (info videoInfo) tracksWithLimit(maximum float64) (videoStreamInfo, *videoStreamInfo, float64, error) {
	duration, e := strconv.ParseFloat(info.Format.Duration, 64)
	var video videoStreamInfo
	var audio *videoStreamInfo
	count := 0
	for _, s := range info.Streams {
		if s.Type == "video" && s.Disposition.Attached == 0 {
			video = s
			count++
		}
		if s.Type == "audio" && audio == nil {
			copy := s
			audio = &copy
		}
	}
	if e != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || duration > maximum || count != 1 || video.Width < 1 || video.Height < 1 || video.Width > 4096 || video.Height > 4096 {
		return video, audio, 0, errors.New("video_limit_exceeded")
	}
	return video, audio, duration, nil
}

type preparedVideo struct {
	Path     string
	Method   string
	Video    videoStreamInfo
	Duration float64
}

// Only incompatible tracks are encoded. Existing compatible MP4 bytes are reused.
// Originals remain private; limits abort the job instead of publishing a truncated video.
func prepareCloudVideo(ctx context.Context, input, output string, maxBytes int64, update func(string, *int)) (preparedVideo, error) {
	var result preparedVideo
	info, e := probeCloudVideo(ctx, input)
	if e != nil {
		return result, e
	}
	video, audio, duration, e := info.tracks()
	if e != nil {
		return result, e
	}
	encodeVideo := video.Codec != "h264" || video.Pixel != "yuv420p"
	encodeAudio := audio != nil && audio.Codec != "aac"
	if !encodeVideo && !encodeAudio && strings.EqualFold(filepath.Ext(input), ".mp4") && mp4FastStart(input) {
		return preparedVideo{input, "copy", video, duration}, nil
	}
	if (encodeVideo || encodeAudio) && env("OSS_VIDEO_AUTO_CONVERT_ENABLED", "true") != "true" {
		return result, errors.New("video_conversion_unavailable")
	}
	method := "remux"
	if encodeVideo || encodeAudio {
		method = "convert"
	}
	update("preparing_video", nil)
	args := []string{"-nostdin", "-y", "-v", "error", "-protocol_whitelist", "file,pipe", "-filter_threads", "1", "-threads", "1", "-f", "mov", "-i", input, "-map", fmt.Sprintf("0:%d", video.Index)}
	if audio != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", audio.Index))
	}
	if encodeVideo {
		filter := "pad=ceil(iw/2)*2:ceil(ih/2)*2,setsar=1"
		// Bound display encoding on the small ECS instance; the private original
		// retains its full resolution. Compatible videos are never resized.
		edge := max(640, min(1920, settingInt("OSS_VIDEO_MAX_OUTPUT_EDGE", 1920)))
		scale := fmt.Sprintf("scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease:force_divisible_by=2,", edge, edge)
		if video.Transfer == "smpte2084" || video.Transfer == "arib-std-b67" {
			filter = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p," + filter
		}
		filter = scale + filter
		args = append(args, "-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p", "-threads", "1")
	} else {
		args = append(args, "-c:v", "copy")
	}
	if encodeAudio {
		args = append(args, "-c:a", "aac", "-b:a", "160k")
	} else {
		args = append(args, "-c:a", "copy")
	}
	args = append(args, "-sn", "-dn", "-movflags", "+faststart", "-map_metadata", "-1", "-progress", "pipe:1", "-nostats", output)
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(child, videoTool("ffmpeg"), args...)
	pipe, e := command.StdoutPipe()
	if e != nil {
		return result, errors.New("video_processing_failed")
	}
	if e = command.Start(); e != nil {
		return result, errors.New("video_processing_failed")
	}
	tooLarge := false
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		if st, er := os.Stat(output); er == nil && st.Size() > maxBytes {
			tooLarge = true
			cancel()
		}
		if key == "out_time_us" {
			us, _ := strconv.ParseInt(value, 10, 64)
			percent := max(0, min(99, int(float64(us)/1e6/duration*100)))
			update("processing_video", &percent)
		}
	}
	e = command.Wait()
	if tooLarge {
		return result, errors.New("file_too_large")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, errors.New("video_processing_timeout")
	}
	if e != nil || scanner.Err() != nil {
		return result, errors.New("video_processing_failed")
	}
	st, e := os.Stat(output)
	if e != nil || st.Size() < 1 || st.Size() > maxBytes {
		return result, errors.New("file_too_large")
	}
	out, e := probeCloudVideo(ctx, output)
	if e != nil {
		return result, e
	}
	// AAC priming can add a small timestamp tail; input limits remain strict.
	v, a, d, e := out.tracksWithLimit(duration + 1)
	if e != nil || v.Codec != "h264" || v.Pixel != "yuv420p" || (a != nil && a.Codec != "aac") || math.Abs(d-duration) > 1 {
		return result, errors.New("video_processing_failed")
	}
	return preparedVideo{output, method, v, d}, nil
}
