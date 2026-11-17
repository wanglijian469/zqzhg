package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"gorm.io/gorm"
)

func aiTool(name string) string {
	if p := os.Getenv(strings.ToUpper(name) + "_PATH"); p != "" {
		return p
	}
	p, e := exec.LookPath(name)
	if e == nil {
		return p
	}
	return ""
}
func aiToolsReady() bool { return aiTool("pdftoppm") != "" && aiTool("pdftotext") != "" }
func aiPDFCount(ctx context.Context, b []byte) (int, error) {
	doc, e := pdfapi.ReadContext(ctx, bytes.NewReader(b), pdfmodel.NewDefaultConfiguration())
	if e != nil || doc.Encrypt != nil {
		return 0, errors.New("invalid_pdf")
	}
	if e = pdfapi.ValidateContext(ctx, doc); e != nil || doc.PageCount < 1 || doc.PageCount > settingInt("AI_IMPORT_MAX_PAGES", 100) {
		return 0, errors.New("invalid_pdf")
	}
	return doc.PageCount, nil
}
func aiImageDimensions(b []byte) (int, int, error) {
	c, f, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (f != "jpeg" && f != "png") || c.Width < 1 || c.Height < 1 || int64(c.Width)*int64(c.Height) > 36000000 {
		return 0, 0, errors.New("invalid_image")
	}
	return c.Width, c.Height, nil
}
func (a *App) aiSaveAsset(ctx context.Context, owner, task, source uint64, page int, kind, ext string, b []byte, region *AIRegion) (asset AIImportAsset, err error) {
	lease, e := a.reserveUpload(owner, int64(len(b)))
	if e != nil {
		return asset, e
	}
	defer lease.Release()
	key := "ai-" + randomToken() + ext
	temp := a.workPath(key)
	if e = os.WriteFile(temp, b, 0600); e != nil {
		return asset, e
	}
	defer os.Remove(temp)
	committed := false
	defer func() {
		if !committed {
			a.cleanupNewObjects(key)
		}
	}()
	if e = a.storeWorkFile(ctx, key, temp); e != nil {
		return asset, errors.New("storage_unavailable")
	}
	if e = lease.Commit(key); e != nil {
		return asset, e
	}
	sum := sha256.Sum256(b)
	asset = AIImportAsset{TaskID: task, OwnerID: owner, SourceAssetID: source, Page: page, ImageType: kind, CropRegion: "null", SHA256: hex.EncodeToString(sum[:])}
	mediaKind := "image"
	if ext == ".pdf" {
		mediaKind = "pdf"
	} else {
		asset.Width, asset.Height, e = aiImageDimensions(b)
		if e != nil {
			return asset, e
		}
	}
	if region != nil {
		asset.Panel = region.Panel
		asset.PrintedPage = region.PrintedPage
		asset.CropRegion = aiJSON(region.Box)
	}
	e = a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := aiGuardWorkerWrite(tx, ctx, task, owner); e != nil {
			return e
		}
		m := Media{OwnerID: owner, Path: key}
		if mediaKind == "image" {
			m.ThumbPath = key
		}
		if e := tx.Create(&m).Error; e != nil {
			return e
		}
		asset.MediaID = m.ID
		meta := MediaMetadata{MediaID: m.ID, Kind: mediaKind, State: "ready", Size: int64(len(b)), Width: asset.Width, Height: asset.Height}
		if e := tx.Create(&meta).Error; e != nil {
			return e
		}
		// Only photographs enter the existing watermark pipeline. Diagrams are immutable PNGs.
		if kind == "product_main" || kind == "product_photo" {
			if e := stampWatermarkSnapshot(tx, m, "product_image"); e != nil {
				return e
			}
		}
		return tx.Create(&asset).Error
	})
	if e != nil {
		return asset, e
	}
	committed = true
	return asset, nil
}
func (a *App) aiAssetBytes(ctx context.Context, asset AIImportAsset) ([]byte, error) {
	var media Media
	if e := a.DB.First(&media, "id=? AND owner_id=?", asset.MediaID, asset.OwnerID).Error; e != nil {
		return nil, e
	}
	_, store, e := a.resolveObject(ctx, media.Path)
	if e != nil {
		return nil, e
	}
	f, e := store.Open(ctx, media.Path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(settingInt("AI_IMPORT_MAX_FILE_BYTES", 30000000))+1))
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != asset.SHA256 {
		return nil, errors.New("checksum_mismatch")
	}
	return b, nil
}
func (a *App) aiCrop(ctx context.Context, task AIImportTask, parent AIImportAsset, r AIRegion) (AIImportAsset, error) {
	if !aiRegionValid(r, task.Total) || r.SourcePage != parent.Page || parent.OwnerID != task.OwnerID || parent.TaskID != task.ID || parent.ImageType != "page" {
		return AIImportAsset{}, serviceFailure{"ai_invalid_crop", 400}
	}
	if (r.ImageType == "product_main" || r.ImageType == "product_photo") && (r.Box[2]-r.Box[0])*(r.Box[3]-r.Box[1]) > .85 {
		return AIImportAsset{}, serviceFailure{"ai_whole_page_image", 400}
	}
	// Reuse identical deterministic crops on retries.
	var old AIImportAsset
	if a.DB.Where("task_id=? AND source_asset_id=? AND image_type=? AND crop_region=?", task.ID, parent.ID, r.ImageType, aiJSON(r.Box)).First(&old).Error == nil {
		return old, nil
	}
	b, e := a.aiAssetBytes(ctx, parent)
	if e != nil {
		return old, e
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return old, e
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	box := image.Rect(int(math.Floor(r.Box[0]*float64(w))), int(math.Floor(r.Box[1]*float64(h))), int(math.Ceil(r.Box[2]*float64(w))), int(math.Ceil(r.Box[3]*float64(h)))).Intersect(img.Bounds())
	if box.Dx() < 8 || box.Dy() < 8 {
		return old, serviceFailure{"ai_invalid_crop", 400}
	}
	dst := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	draw.Draw(dst, dst.Bounds(), img, box.Min, draw.Src)
	var out bytes.Buffer
	if e = png.Encode(&out, dst); e != nil {
		return old, e
	}
	return a.aiSaveAsset(ctx, task.OwnerID, task.ID, parent.ID, parent.Page, r.ImageType, ".png", out.Bytes(), &r)
}
func (a *App) aiParsePDF(ctx context.Context, task AIImportTask, source AIImportAsset) ([]AIImportAsset, error) {
	if !aiToolsReady() {
		return nil, errors.New("ai_pdf_tools_required")
	}
	b, e := a.aiAssetBytes(ctx, source)
	if e != nil {
		return nil, e
	}
	dir, e := os.MkdirTemp(a.mediaTempDir(), "ai-pdf-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.pdf")
	if e = os.WriteFile(input, b, 0600); e != nil {
		return nil, e
	}
	textCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	cmd := exec.CommandContext(textCtx, aiTool("pdftotext"), "-layout", "-enc", "UTF-8", input, filepath.Join(dir, "text.txt"))
	e = cmd.Run()
	cancel()
	if e != nil {
		return nil, errors.New("ai_pdf_text_failed")
	}
	textBytes, readErr := os.ReadFile(filepath.Join(dir, "text.txt"))
	if readErr != nil {
		return nil, errors.New("ai_pdf_text_failed")
	}
	checkpoint := a.aiReadCheckpoint(task.ID)
	checkpoint.TextPages = map[string]string{}
	for i, text := range strings.Split(string(textBytes), "\f") {
		if i >= task.Total {
			break
		}
		checkpoint.TextPages[fmt.Sprint(i+1)] = text
	}
	if e = a.DB.Model(&AIImportTask{}).Where("id=? AND lease=?", task.ID, task.Lease).Update("checkpoint", aiJSON(checkpoint)).Error; e != nil {
		return nil, e
	}
	pages := []AIImportAsset{}
	for p := 1; p <= task.Total; p++ {
		if e := a.aiWork(&task, "document_parsing", 10*(p-1)/task.Total, AIImportWork{SourcePage: p, Done: p - 1, Total: task.Total, Message: fmt.Sprintf("正在解析PDF第%d页，共%d页", p, task.Total)}); e != nil {
			return nil, e
		}
		var existing AIImportAsset
		if a.DB.Where("task_id=? AND image_type='page' AND page=?", task.ID, p).First(&existing).Error == nil {
			pages = append(pages, existing)
			continue
		}
		// Preserve embedded images individually, with page provenance. Full-page scans
		// are reused without resampling; mixed/vector pages use a bounded raster view.
		imagesDir := filepath.Join(dir, fmt.Sprintf("images-%d", p))
		os.Mkdir(imagesDir, 0700)
		extractCtx, stop := context.WithTimeout(ctx, 60*time.Second)
		e = pdfapi.ExtractImagesFile(extractCtx, input, imagesDir, []string{fmt.Sprint(p)}, pdfmodel.NewDefaultConfiguration())
		stop()
		files, _ := os.ReadDir(imagesDir)
		var pageBytes []byte
		var pageExt string
		valid := 0
		if e == nil {
			for _, f := range files {
				raw, err := os.ReadFile(filepath.Join(imagesDir, f.Name()))
				if err != nil {
					continue
				}
				w, h, err := aiImageDimensions(raw)
				if err != nil {
					continue
				}
				valid++
				ext := ".jpg"
				if http.DetectContentType(raw) == "image/png" {
					ext = ".png"
				}
				var emb AIImportAsset
				if a.DB.Where("task_id=? AND page=? AND image_type='embedded' AND sha256=?", task.ID, p, fmt.Sprintf("%x", sha256.Sum256(raw))).First(&emb).Error != nil {
					emb, err = a.aiSaveAsset(ctx, task.OwnerID, task.ID, source.ID, p, "embedded", ext, raw, nil)
					if err != nil {
						return nil, err
					}
				}
				if w >= 1000 && h >= 1000 {
					pageBytes = raw
					pageExt = ext
				}
			}
		}
		if valid != 1 || pageBytes == nil || strings.TrimSpace(checkpoint.TextPages[fmt.Sprint(p)]) != "" {
			prefix := filepath.Join(dir, fmt.Sprintf("render-%d", p))
			renderCtx, stop := context.WithTimeout(ctx, 60*time.Second)
			cmd = exec.CommandContext(renderCtx, aiTool("pdftoppm"), "-f", fmt.Sprint(p), "-l", fmt.Sprint(p), "-singlefile", "-scale-to", "4096", "-png", input, prefix)
			e = cmd.Run()
			stop()
			if e != nil {
				return nil, errors.New("ai_pdf_render_failed")
			}
			pageBytes, e = os.ReadFile(prefix + ".png")
			pageExt = ".png"
			if e != nil {
				return nil, e
			}
		}
		asset, e := a.aiSaveAsset(ctx, task.OwnerID, task.ID, source.ID, p, "page", pageExt, pageBytes, nil)
		if e != nil {
			return nil, e
		}
		pages = append(pages, asset)
	}
	return pages, nil
}
func validateAIPayloadAssets(tx *gorm.DB, p Payload) error {
	if !tx.Migrator().HasTable(&AIImportAsset{}) {
		return nil
	}
	ids := payloadMedia(p)
	if len(ids) == 0 {
		return nil
	}
	var rows []AIImportAsset
	if e := tx.Where("media_id IN ?", ids).Find(&rows).Error; e != nil {
		return e
	}
	photos := map[uint64]bool{}
	for _, mid := range p.MediaIDs {
		photos[mid] = true
	}
	for _, r := range rows {
		if photos[r.MediaID] && r.ImageType != "product_main" && r.ImageType != "product_photo" {
			return fieldFailure("invalid_media", "media_ids", "ai_photo_crop_required")
		}
		if r.ImageType == "page" || r.ImageType == "embedded" || r.ImageType == "original" {
			return fieldFailure("invalid_media", "media_ids", "ai_select_crop_required")
		}
	}
	return nil
}
func (a *App) aiReadAsset(c *gin.Context) {
	var asset AIImportAsset
	if a.DB.Where("id=? AND owner_id=?", id(c), actor(c).ID).First(&asset).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	var media Media
	if a.DB.First(&media, asset.MediaID).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	c.Header("Cache-Control", "no-store")
	a.serveObject(c, media.Path)
}
func (a *App) aiCropAsset(c *gin.Context) {
	var asset AIImportAsset
	if a.DB.Where("id=? AND owner_id=?", id(c), actor(c).ID).First(&asset).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	var task AIImportTask
	var r AIRegion
	if c.ShouldBindJSON(&r) != nil || a.DB.First(&task, "id=? AND owner_id=?", asset.TaskID, actor(c).ID).Error != nil {
		fail(c, 400, "invalid_request")
		return
	}
	if task.Status != "review_pending" {
		fail(c, 409, "ai_task_locked")
		return
	}
	next, e := a.aiCrop(c.Request.Context(), task, asset, r)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(201, next)
}
