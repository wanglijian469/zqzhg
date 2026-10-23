package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"
)

func (cfg *SystemConfig) UnmarshalJSON(raw []byte) error {
	type plain SystemConfig
	if e := json.Unmarshal(raw, (*plain)(cfg)); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return e
	}
	_, cfg.watermarkPresent = fields["watermark"]
	return nil
}
func (a *App) watermarkRoutes(admin *gin.RouterGroup) {
	admin.GET("/settings/watermark/vendors", func(c *gin.Context) {
		type option struct {
			ID        uint64 `json:"id"`
			Name      string `json:"name"`
			ShortName string `json:"short_name"`
		}
		var vendors []Content
		q := a.DB.Model(&Content{}).Joins("JOIN revisions r ON r.id=CASE WHEN contents.published_revision_id>0 THEN contents.published_revision_id ELSE contents.draft_revision_id END").Where("contents.kind='vendor'")
		if term := strings.TrimSpace(c.Query("q")); term != "" {
			if len([]rune(term)) > 100 {
				fail(c, 400, "invalid_request")
				return
			}
			s := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(term) + "%"
			q = q.Where("r.name LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.company_profile.short_name')) LIKE ?", s, s)
		}
		if e := q.Select("contents.*").Order("contents.id DESC").Limit(20).Find(&vendors).Error; e != nil {
			dbFail(c, e)
			return
		}
		items := []option{}
		for _, v := range vendors {
			rid := v.PublishedRevisionID
			if rid == 0 {
				rid = v.DraftRevisionID
			}
			var r Revision
			if e := a.DB.First(&r, rid).Error; e != nil {
				dbFail(c, e)
				return
			}
			var p Payload
			if e := json.Unmarshal([]byte(r.Payload), &p); e != nil {
				dbFail(c, e)
				return
			}
			short := ""
			if p.CompanyProfile != nil {
				short = p.CompanyProfile.ShortName
			}
			items = append(items, option{v.ID, p.Name, short})
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"items": items})
	})
	admin.POST("/settings/watermark/preview", func(c *gin.Context) {
		if !permit(fmt.Sprintf("watermark-preview:%d", actor(c).ID), 180, time.Minute) {
			fail(c, 429, "rate_limited")
			return
		}
		var req struct {
			Watermark WatermarkConfig `json:"watermark"`
			VendorID  uint64          `json:"vendor_id"`
			Dark      bool            `json:"dark"`
		}
		c.Request.Body = httpMaxBody(c, 16384)
		if c.ShouldBindJSON(&req) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		normalizeWatermark(&req.Watermark)
		if e := validateWatermark(req.Watermark); e != nil {
			dbFail(c, e)
			return
		}
		name := "示例农机厂"
		if req.VendorID > 0 {
			var v Content
			if e := a.DB.Where("kind='vendor'").First(&v, req.VendorID).Error; e != nil {
				dbFail(c, e)
				return
			}
			_, label, e := watermarkVendor(a.DB, v.OwnerID, nil, Payload{})
			if e != nil {
				dbFail(c, e)
				return
			}
			name = label
		}
		img := watermarkSample(req.Dark, 960, 600)
		marked, e := renderWatermark(img, req.Watermark, name)
		if e != nil {
			dbFail(c, e)
			return
		}
		var out bytes.Buffer
		if e = png.Encode(&out, marked); e != nil {
			dbFail(c, e)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(200, "image/png", out.Bytes())
	})
}
func watermarkSample(dark bool, w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(246 - y*24/max(1, h))
			if dark {
				v = uint8(28 + y*26/max(1, h))
			}
			img.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
		}
	}
	// A code-drawn machine silhouette provides a neutral product-like test surface.
	ink := color.NRGBA{130, 146, 157, 255}
	if dark {
		ink = color.NRGBA{75, 91, 105, 255}
	}
	for y := h / 3; y < h*2/3; y++ {
		for x := w / 4; x < w*3/4; x++ {
			if x < w/2 || y > h*5/12 {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	for _, cx := range []int{w * 3 / 8, w * 5 / 8} {
		r := h / 11
		for y := h*2/3 - r; y < h*2/3+r; y++ {
			for x := cx - r; x < cx+r; x++ {
				dx, dy := x-cx, y-h*2/3
				if dx*dx+dy*dy < r*r {
					img.SetNRGBA(x, y, color.NRGBA{66, 78, 91, 255})
				}
			}
		}
	}
	return img
}
