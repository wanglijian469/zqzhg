package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func assertCloudVideoConversionAccounting(t *testing.T, a *App, tc testClient, token string, owner uint64) {
	t.Helper()
	input := conversionFixture(t, "private-original.mov", "mpeg4", "pcm_s16le", false)
	data, _ := os.ReadFile(input)
	before, reserved, _, e := a.ownerUsage(a.DB, owner)
	if e != nil {
		t.Fatal(e)
	}
	created := tc.call("POST", "/merchant/files/uploads", token, gin.H{"kind": "video", "filename": "private-original.mov", "size": len(data), "idempotency_key": "convert-video-accounting"}, 201)
	id := created["session"].(map[string]any)["id"].(string)
	var s FileUploadSession
	a.DB.First(&s, "id=?", id)
	store, _ := a.store(s.StoreID)
	if e = store.Put(context.Background(), s.ObjectKey, bytes.NewReader(data), int64(len(data))); e != nil {
		t.Fatal(e)
	}
	tc.call("POST", "/merchant/files/uploads/"+id+"/complete", token, gin.H{}, 202)
	if !a.processNextCloudUpload(context.Background()) {
		t.Fatal("video worker did not claim")
	}
	a.DB.First(&s, "id=?", id)
	if s.UploadState != "completed" {
		t.Fatal("video conversion did not complete", s.Error)
	}
	var m Media
	var meta MediaMetadata
	a.DB.First(&m, s.MediaID)
	a.DB.First(&meta, "media_id=?", m.ID)
	if meta.SourcePath != s.ObjectKey || m.Path == s.ObjectKey || meta.State != "ready" {
		t.Fatal("private original or display mapping lost")
	}
	var total int64
	for _, key := range []string{s.ObjectKey, m.Path, m.ThumbPath} {
		var o StoredObject
		if a.DB.First(&o, "object_key=?", key).Error != nil {
			t.Fatal("object not registered")
		}
		total += o.Size
	}
	used, afterReserved, _, e := a.ownerUsage(a.DB, owner)
	if e != nil || used-before != total || afterReserved != reserved {
		t.Fatal("conversion accounting mismatch", used-before, total, afterReserved, reserved, e)
	}
	var source EnterpriseFileObject
	if a.DB.First(&source, "object_key=? AND object_role='original'", s.ObjectKey).Error != nil {
		t.Fatal("original not associated with file")
	}
	status := tc.call("GET", "/merchant/files/uploads/"+id, token, nil, 200)
	if status["session"].(map[string]any)["processing_percent"] != float64(100) {
		t.Fatal("completed progress absent")
	}
	a.processNextCloudUpload(context.Background())
	var count int64
	a.DB.Model(&Media{}).Where("id=?", m.ID).Count(&count)
	if count != 1 {
		t.Fatal("retry duplicated media")
	}
	a.DB.Model(&FileUploadSession{}).Where("id=?", id).Update("signature_expires_at", time.Now().Add(-time.Hour))
	tc.call("DELETE", fmt.Sprintf("/merchant/files/%d", m.ID), token, gin.H{"confirm": true}, 200)
	used, _, _, _ = a.ownerUsage(a.DB, owner)
	if used != before {
		t.Fatal("deleting unused video did not reclaim original and derivatives", used, before)
	}
}

func conversionFixture(t *testing.T, name string, codec, audio string, fast bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	args := []string{"-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=96x64:rate=12"}
	if audio != "" {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100")
	}
	args = append(args, "-t", "1", "-c:v", codec, "-threads", "1", "-pix_fmt", "yuv420p")
	if audio != "" {
		args = append(args, "-c:a", audio)
	} else {
		args = append(args, "-an")
	}
	if fast {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, path)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if data, e := exec.CommandContext(ctx, videoTool("ffmpeg"), args...).CombinedOutput(); e != nil {
		t.Fatalf("fixture failed: %v: %s", e, data)
	}
	return path
}
func TestCloudVideoAutomaticConversion(t *testing.T) {
	for _, v := range []struct {
		name, codec, audio, method string
		fast                       bool
	}{
		{"compatible.mp4", "libx264", "aac", "copy", true},
		{"video.mp4", "mpeg4", "aac", "convert", true},
		{"audio.mov", "libx264", "pcm_s16le", "convert", false},
		{"both.mov", "mpeg4", "pcm_s16le", "convert", false},
		{"container.mov", "libx264", "aac", "remux", false},
		{"silent.mp4", "mpeg4", "", "convert", false},
	} {
		t.Run(v.name, func(t *testing.T) {
			input := conversionFixture(t, v.name, v.codec, v.audio, v.fast)
			original, _ := os.ReadFile(input)
			output := filepath.Join(t.TempDir(), "result.mp4")
			updates := 0
			prepared, e := prepareCloudVideo(context.Background(), input, output, 8<<20, func(_ string, _ *int) { updates++ })
			if e != nil {
				t.Fatal(e)
			}
			if prepared.Method != v.method {
				t.Fatalf("method %s", prepared.Method)
			}
			if prepared.Video.Width != 96 || prepared.Video.Height != 64 || prepared.Duration < .9 || prepared.Duration > 1.2 {
				t.Fatalf("output metadata: %+v", prepared)
			}
			after, _ := os.ReadFile(input)
			if sha256.Sum256(original) != sha256.Sum256(after) {
				t.Fatal("original modified")
			}
			if v.method == "copy" && prepared.Path != input {
				t.Fatal("compatible video was needlessly rewritten")
			}
			if !mp4FastStart(prepared.Path) {
				t.Fatal("output not streamable")
			}
			if v.method != "copy" && updates == 0 {
				t.Fatal("processing progress absent")
			}
		})
	}
}
func TestCloudVideoConversionBoundsAndFailures(t *testing.T) {
	input := conversionFixture(t, "large.mp4", "mpeg4", "", false)
	original, _ := os.ReadFile(input)
	if _, e := prepareCloudVideo(context.Background(), input, filepath.Join(t.TempDir(), "small.mp4"), 32, func(string, *int) {}); e == nil || e.Error() != "file_too_large" {
		t.Fatal("output budget ignored", e)
	}
	t.Setenv("OSS_VIDEO_AUTO_CONVERT_ENABLED", "false")
	if _, e := prepareCloudVideo(context.Background(), input, filepath.Join(t.TempDir(), "disabled.mp4"), 8<<20, func(string, *int) {}); e == nil || e.Error() != "video_conversion_unavailable" {
		t.Fatal("switch ignored", e)
	}
	broken := filepath.Join(t.TempDir(), "broken.mp4")
	os.WriteFile(broken, []byte("not a video"), 0600)
	if _, e := prepareCloudVideo(context.Background(), broken, filepath.Join(t.TempDir(), "bad.mp4"), 8<<20, func(string, *int) {}); e == nil {
		t.Fatal("damaged file accepted")
	}
	after, _ := os.ReadFile(input)
	if sha256.Sum256(original) != sha256.Sum256(after) {
		t.Fatal("failure changed original")
	}
}
func TestCloudVideoProgressIsScopedAndCleared(t *testing.T) {
	a := &App{}
	p := 37
	a.setCloudUploadProgress("one", "processing_video", &p)
	s := FileUploadSession{ID: "one", UploadState: "validating"}
	a.cloudUploadProgressView(&s)
	if s.ProcessingPercent == nil || *s.ProcessingPercent != 37 {
		t.Fatal("progress absent")
	}
	other := FileUploadSession{ID: "two", UploadState: "validating"}
	a.cloudUploadProgressView(&other)
	if other.ProcessingPercent != nil {
		t.Fatal("progress leaked across uploads")
	}
	a.clearCloudUploadProgress("one")
	if len(a.cloudFiles().uploadProgress) != 0 {
		t.Fatal("progress cache leaked")
	}
}

func TestCloudVideoFiveMinuteBoundary(t *testing.T) {
	t.Setenv("MAX_VIDEO_SECONDS", "300")
	for _, v := range []struct {
		duration string
		valid    bool
	}{{"197.96", true}, {"300", true}, {"300.01", false}, {"NaN", false}} {
		info := videoInfo{Streams: []videoStreamInfo{{Type: "video", Codec: "hevc", Width: 960, Height: 544}}}
		info.Format.Duration = v.duration
		_, _, _, e := info.tracks()
		if (e == nil) != v.valid {
			t.Fatalf("duration %s: %v", v.duration, e)
		}
	}
}
