package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func scaled(src image.Image, maxSize int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSize && h <= maxSize {
		return src
	}
	if w >= h {
		h = h * maxSize / w
		w = maxSize
	} else {
		w = w * maxSize / h
		h = maxSize
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
func writeImage(path string, img image.Image, format string) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	if format == "png" {
		e = png.Encode(f, img)
	} else {
		e = jpeg.Encode(f, img, &jpeg.Options{Quality: 88})
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func (a *App) upload(c *gin.Context) {
	c.Request.Body = io.NopCloser(c.Request.Body)
	c.Request.Body = httpMaxBody(c, 11<<20)
	file, e := c.FormFile("file")
	if e != nil || file.Size > 10<<20 || file.Size == 0 {
		fail(c, 400, "invalid_image")
		return
	}
	f, e := file.Open()
	if e != nil {
		fail(c, 400, "invalid_image")
		return
	}
	defer f.Close()
	config, format, e := image.DecodeConfig(f)
	if e != nil || (format != "jpeg" && format != "png" && format != "webp") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 36000000 {
		fail(c, 400, "invalid_image")
		return
	}
	if _, e = f.Seek(0, 0); e != nil {
		fail(c, 400, "invalid_image")
		return
	}
	img, _, e := image.Decode(f)
	if e != nil {
		fail(c, 400, "invalid_image")
		return
	}
	key := randomToken()
	extension := ".jpg"
	outputFormat := "jpeg"
	if format == "png" {
		extension = ".png"
		outputFormat = "png"
	}
	path := key + extension
	thumb := key + "-thumb" + extension
	fullPath := filepath.Join(a.Config.MediaDir, path)
	thumbPath := filepath.Join(a.Config.MediaDir, thumb)
	if e = writeImage(fullPath, scaled(img, 2048), outputFormat); e != nil {
		fail(c, 500, "internal_error")
		return
	}
	if e = writeImage(thumbPath, scaled(img, 480), outputFormat); e != nil {
		os.Remove(fullPath)
		os.Remove(thumbPath)
		fail(c, 500, "internal_error")
		return
	}
	media := Media{OwnerID: actor(c).ID, Path: path, ThumbPath: thumb}
	if owner, ok := c.Get("upload_owner_id"); ok {
		media.OwnerID = owner.(uint64)
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&media).Error; e != nil {
			return e
		}
		if _, ok := c.Get("upload_home_display"); ok {
			if e := tx.Create(&HomeDisplayMedia{MediaID: media.ID, CreatedAt: media.CreatedAt}).Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "media", media.ID, "home_display_image_uploaded", "")
		}
		if target, ok := c.Get("upload_content_id"); ok {
			return recordOperation(tx, actor(c).ID, "media", media.ID, "media_uploaded", fmt.Sprintf("content=%d", target.(uint64)))
		}
		return nil
	})
	if e != nil {
		os.Remove(fullPath)
		os.Remove(thumbPath)
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(201, gin.H{"id": media.ID, "url": fmt.Sprintf("/api/v1/media/%d", media.ID), "thumb_url": fmt.Sprintf("/api/v1/media/%d?thumb=1", media.ID)})
}
func (a *App) mediaRead(c *gin.Context) {
	mediaID, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		fail(c, 404, "not_found")
		return
	}
	var media Media
	if a.DB.First(&media, mediaID).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	var count int64
	e = a.DB.Raw("SELECT COUNT(*) FROM media_references mr JOIN contents c ON c.published_revision_id=mr.revision_id JOIN revisions r ON r.id=mr.revision_id AND r.status='approved' LEFT JOIN contents v ON v.id=c.vendor_id LEFT JOIN revisions vr ON vr.id=v.published_revision_id WHERE mr.media_id=? AND c.visible=1 AND (c.kind='vendor' OR (v.visible=1 AND vr.status='approved'))", mediaID).Scan(&count).Error
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	if count == 0 {
		if e := a.DB.Table("home_display_settings hs").Joins("JOIN home_display_media hm ON hm.media_id=hs.hero_media_id").Where("hs.id=1 AND hm.media_id=?", mediaID).Count(&count).Error; e != nil {
			fail(c, 500, "internal_error")
			return
		}
	}
	if count == 0 {
		public, err := a.resourceMediaIsPublic(mediaID)
		if err != nil {
			fail(c, 500, "internal_error")
			return
		}
		if public {
			count = 1
		}
	}
	if count == 0 {
		account, e := a.readAccount(c)
		if e != nil || (account.Role != "admin" && account.ID != media.OwnerID) {
			fail(c, 404, "not_found")
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	var meta MediaMetadata
	if a.DB.First(&meta, "media_id=?", mediaID).Error == nil && meta.State != "ready" {
		fail(c, 404, "media_not_ready")
		return
	}
	path := media.Path
	if c.Query("thumb") == "1" {
		path = media.ThumbPath
	}
	if path == "" {
		fail(c, 404, "not_found")
		return
	}
	if c.Query("download") == "1" {
		c.Header("Content-Disposition", "attachment; filename=\"catalog"+filepath.Ext(path)+"\"")
	}
	c.File(filepath.Join(a.Config.MediaDir, path))
}
