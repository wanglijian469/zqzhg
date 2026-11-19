package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// New cloud videos are never transcoded. Incompatible codecs are actionable errors.
func (a *App) processCloudVideo(ctx context.Context, s FileUploadSession, input, sha string) error {
	output, e := exec.CommandContext(ctx, videoTool("ffprobe"), "-v", "error", "-protocol_whitelist", "file,pipe", "-f", "mov", "-show_entries", "format=duration:stream=codec_type,codec_name,pix_fmt,width,height", "-of", "json", input).Output()
	if e != nil {
		return errors.New("video_probe_failed")
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type   string `json:"codec_type"`
			Codec  string `json:"codec_name"`
			Pixel  string `json:"pix_fmt"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &info) != nil {
		return errors.New("video_probe_failed")
	}
	duration, _ := strconv.ParseFloat(info.Format.Duration, 64)
	w, h := 0, 0
	videos := 0
	for _, stream := range info.Streams {
		if stream.Type == "video" {
			videos++
			w, h = stream.Width, stream.Height
			if stream.Codec != "h264" || stream.Pixel != "yuv420p" {
				return errors.New("unsupported_video_codec")
			}
		}
		if stream.Type == "audio" && stream.Codec != "aac" {
			return errors.New("unsupported_video_codec")
		}
	}
	if videos != 1 || duration <= 0 || duration > float64(settingInt("MAX_VIDEO_SECONDS", 180)) || w < 1 || h < 1 || w > 4096 || h > 4096 {
		return errors.New("video_limit_exceeded")
	}
	key := s.ObjectKey
	ready := input
	remux := filepath.Ext(s.OriginalFilename) != ".mp4" || !mp4FastStart(input)
	if remux {
		ready = a.workPath("remux-" + s.ID + ".mp4")
		defer os.Remove(ready)
		if e = exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-protocol_whitelist", "file,pipe", "-f", "mov", "-i", input, "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+faststart", "-map_metadata", "-1", ready).Run(); e != nil {
			return errors.New("video_processing_failed")
		}
		key = a.mediaObjectKey(s.OwnerID, "video", randomToken(), ".mp4")
	}
	cover := a.workPath("cover-" + s.ID + ".jpg")
	defer os.Remove(cover)
	if e = exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-protocol_whitelist", "file,pipe", "-f", "mov", "-i", ready, "-frames:v", "1", "-vf", "scale=480:480:force_original_aspect_ratio=decrease", cover).Run(); e != nil {
		return errors.New("video_processing_failed")
	}
	stat, e := os.Stat(ready)
	if e != nil {
		return e
	}
	coverStat, e := os.Stat(cover)
	if e != nil {
		return e
	}
	extra := coverStat.Size()
	if remux {
		extra += stat.Size()
	}
	lease, e := a.reserveUpload(s.OwnerID, extra)
	if e != nil {
		return e
	}
	defer lease.Release()
	coverKey := a.mediaObjectKey(s.OwnerID, "video", randomToken(), "-cover.jpg")
	keys := []string{coverKey}
	if remux {
		keys = append(keys, key)
	}
	success := false
	defer func() {
		if !success {
			a.cleanupNewObjects(keys...)
		}
	}()
	if remux {
		if e = a.storeWorkFile(ctx, key, ready); e != nil {
			return e
		}
	}
	if e = a.storeWorkFile(ctx, coverKey, cover); e != nil {
		return e
	}
	if e = lease.Commit(keys...); e != nil {
		return e
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, s.OwnerID).Error; e != nil {
			return e
		}
		var current FileUploadSession
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id=?", s.ID).Error; e != nil {
			return e
		}
		if current.UploadState != "validating" || current.LeaseToken != s.LeaseToken || current.MediaID != 0 {
			return errors.New("file_upload_conflict")
		}
		if !remux {
			obj := StoredObject{Key: s.ObjectKey, StoreID: s.StoreID, Size: s.ExpectedBytes, SHA256: sha, UpdatedAt: time.Now().UTC()}
			if e := tx.Create(&obj).Error; e != nil {
				return e
			}
			if e := tx.Create(&MediaObjectOwner{ObjectKey: s.ObjectKey, OwnerID: s.OwnerID, Size: s.ExpectedBytes}).Error; e != nil {
				return e
			}
			if e := tx.Where("id=?", s.ID).Delete(&MediaUploadReservation{}).Error; e != nil {
				return e
			}
		}
		m := Media{OwnerID: s.OwnerID, Path: key, ThumbPath: coverKey}
		if e := tx.Create(&m).Error; e != nil {
			return e
		}
		meta := MediaMetadata{MediaID: m.ID, Kind: "video", State: "ready", Size: stat.Size(), Duration: duration, Width: w, Height: h}
		if e := tx.Create(&meta).Error; e != nil {
			return e
		}
		if e := a.registerEnterpriseFile(tx, m, "video", s.OriginalFilename, "video/mp4", s.BusinessID); e != nil {
			return e
		}
		return tx.Model(&current).Updates(map[string]any{"media_id": m.ID, "upload_state": "completed", "lease_token": "", "lease_until": nil, "error": "", "updated_at": time.Now().UTC()}).Error
	})
	success = e == nil
	return e
}

func mp4FastStart(path string) bool {
	f, e := os.Open(path)
	if e != nil {
		return false
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return false
	}
	for offset := int64(0); offset+8 <= st.Size(); {
		var header [16]byte
		if _, e = f.ReadAt(header[:8], offset); e != nil {
			return false
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		minimum := int64(8)
		if size == 1 {
			if _, e = f.ReadAt(header[8:], offset+8); e != nil {
				return false
			}
			size = int64(binary.BigEndian.Uint64(header[8:]))
			minimum = 16
		}
		if string(header[4:8]) == "moov" {
			return size >= minimum && size <= st.Size()-offset
		}
		if string(header[4:8]) == "mdat" {
			return false
		}
		if size < minimum || size > st.Size()-offset {
			return false
		}
		offset += size
	}
	return false
}
