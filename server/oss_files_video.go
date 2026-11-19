package main

import (
	"context"
	"encoding/binary"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"os"
	"os/exec"
	"time"
)

func (a *App) processCloudVideo(ctx context.Context, s FileUploadSession, input, sha string) error {
	converted := a.workPath("compatible-" + s.ID + ".mp4")
	defer os.Remove(converted)
	capBytes := int64(a.uploadConfig().VideoMiB) << 20
	tempBudget := int64(settingInt("OSS_TEMP_BUDGET_MIB", 512)) << 20
	capBytes = min(capBytes, tempBudget-s.ExpectedBytes-(16<<20))
	if capBytes < 1 {
		return errors.New("file_too_large")
	}
	if e := a.checkUploadDisks(uint64(capBytes + (16 << 20))); e != nil {
		return e
	}
	prepared, e := prepareCloudVideo(ctx, input, converted, capBytes, func(stage string, percent *int) { a.setCloudUploadProgress(s.ID, stage, percent) })
	if e != nil {
		return e
	}
	changed := prepared.Path != input
	key := s.ObjectKey
	if changed {
		key = a.mediaObjectKey(s.OwnerID, "video", randomToken(), ".mp4")
	}
	a.setCloudUploadProgress(s.ID, "creating_video_cover", nil)
	cover := a.workPath("cover-" + s.ID + ".jpg")
	defer os.Remove(cover)
	if e = exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-protocol_whitelist", "file,pipe", "-threads", "1", "-filter_threads", "1", "-f", "mov", "-i", prepared.Path, "-frames:v", "1", "-vf", "scale=480:480:force_original_aspect_ratio=decrease", cover).Run(); e != nil {
		return errors.New("video_processing_failed")
	}
	st, e := os.Stat(prepared.Path)
	if e != nil {
		return e
	}
	coverStat, e := os.Stat(cover)
	if e != nil {
		return e
	}
	extra := coverStat.Size()
	if changed {
		extra += st.Size()
	}
	lease, e := a.reserveUpload(s.OwnerID, extra)
	if e != nil {
		return e
	}
	defer lease.Release()
	coverKey := a.mediaObjectKey(s.OwnerID, "video", randomToken(), "-cover.jpg")
	keys := []string{coverKey}
	if changed {
		keys = append(keys, key)
	}
	success := false
	defer func() {
		if !success {
			a.cleanupNewObjects(keys...)
		}
	}()
	a.setCloudUploadProgress(s.ID, "saving_video", nil)
	if changed {
		if e = a.storeWorkFile(ctx, key, prepared.Path); e != nil {
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
		// Keep the uploaded original private. Moving its reservation into actual
		// ownership in the same transaction never double-counts or releases it early.
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
		m := Media{OwnerID: s.OwnerID, Path: key, ThumbPath: coverKey}
		if e := tx.Create(&m).Error; e != nil {
			return e
		}
		meta := MediaMetadata{MediaID: m.ID, Kind: "video", State: "ready", Size: st.Size(), Duration: prepared.Duration, Width: prepared.Video.Width, Height: prepared.Video.Height}
		if changed {
			meta.SourcePath = s.ObjectKey
		}
		if e := tx.Create(&meta).Error; e != nil {
			return e
		}
		if e := a.registerEnterpriseFile(tx, m, "video", s.OriginalFilename, "video/mp4", s.BusinessID); e != nil {
			return e
		}
		if changed {
			var f EnterpriseFile
			if e := tx.First(&f, "media_id=?", m.ID).Error; e != nil {
				return e
			}
			var profile StorageProfile
			if e := tx.First(&profile, s.StoreID).Error; e != nil {
				return e
			}
			original := EnterpriseFileObject{ObjectKey: s.ObjectKey, PhysicalKey: s.ObjectKey, MediaID: m.ID, OwnerID: s.OwnerID, EnterpriseID: f.EnterpriseID, StoreID: s.StoreID, Bucket: profile.Bucket, ObjectRole: "original", BusinessType: "video", FileSize: s.ExpectedBytes, SHA256: sha, ObjectState: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			if e := tx.Create(&original).Error; e != nil {
				return e
			}
		}
		if e := tx.Model(&current).Updates(map[string]any{"media_id": m.ID, "upload_state": "completed", "lease_token": "", "lease_until": nil, "error": "", "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, s.ActorID, "media", m.ID, "oss_video_processed", prepared.Method)
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
