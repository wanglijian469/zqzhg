package main

import (
	"embed"
	"errors"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed fonts/NotoSansSC-Regular.otf
var watermarkFontFiles embed.FS
var watermarkFontOnce sync.Once
var watermarkFont *opentype.Font
var watermarkFontError error

type WatermarkConfig struct {
	Enabled      bool   `json:"enabled"`
	Text         string `json:"text"`
	Opacity      int    `json:"opacity"`
	WidthPercent int    `json:"width_percent"`
	Position     string `json:"position"`
}

func defaultWatermarkConfig() WatermarkConfig {
	return WatermarkConfig{true, "中企展馆", 15, 22, "bottom_right"}
}
func parsedWatermarkFont() (*opentype.Font, error) {
	watermarkFontOnce.Do(func() {
		raw, e := watermarkFontFiles.ReadFile("fonts/NotoSansSC-Regular.otf")
		if e != nil {
			watermarkFontError = e
			return
		}
		watermarkFont, watermarkFontError = opentype.Parse(raw)
	})
	return watermarkFont, watermarkFontError
}
func normalizeWatermark(c *WatermarkConfig) { c.Text = strings.TrimSpace(c.Text) }
func validateWatermark(c WatermarkConfig) error {
	if utf8.RuneCountInString(c.Text) < 1 || utf8.RuneCountInString(c.Text) > 20 || strings.ContainsAny(c.Text, "\r\n\t") || c.Opacity < 5 || c.Opacity > 40 || c.WidthPercent < 10 || c.WidthPercent > 30 {
		return fieldFailure("invalid_watermark", "watermark", "invalid")
	}
	switch c.Position {
	case "top_left", "top_right", "bottom_left", "bottom_right":
	default:
		return fieldFailure("invalid_watermark", "watermark.position", "invalid")
	}
	f, e := parsedWatermarkFont()
	if e != nil {
		return e
	}
	for _, r := range c.Text {
		i, e := f.GlyphIndex(nil, r)
		if e != nil || i == 0 {
			return fieldFailure("invalid_watermark", "watermark.text", "unsupported_character")
		}
	}
	return nil
}
func watermarkText(c WatermarkConfig, vendor string) string {
	r := []rune(strings.Join(strings.Fields(vendor), " "))
	if len(r) > 18 {
		r = append(r[:18], '…')
	}
	return c.Text + " · " + string(r)
}

// Blend only RGB: the input alpha channel is preserved, including transparent PNG backgrounds.
func renderWatermark(src image.Image, c WatermarkConfig, vendor string) (image.Image, error) {
	if !c.Enabled {
		return src, nil
	}
	if strings.TrimSpace(vendor) == "" {
		return nil, errors.New("watermark_vendor_required")
	}
	f, e := parsedWatermarkFont()
	if e != nil {
		return nil, e
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return nil, errors.New("invalid_image")
	}
	text := watermarkText(c, vendor)
	face, e := opentype.NewFace(f, &opentype.FaceOptions{Size: 100, DPI: 72, Hinting: font.HintingNone})
	if e != nil {
		return nil, e
	}
	measured := float64(font.MeasureString(face, text)) / 64
	face.Close()
	if measured <= 0 {
		return nil, errors.New("invalid_watermark")
	}
	margin := math.Max(1, math.Round(float64(min(w, h))*.02))
	target := math.Min(float64(w)*float64(c.WidthPercent)/100, float64(w)-margin*2)
	size := math.Max(.25, 100*target/measured)
	size = math.Min(size, float64(min(w, h))*.035)
	face, e = opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if e != nil {
		return nil, e
	}
	defer face.Close()
	metrics := face.Metrics()
	tw := font.MeasureString(face, text).Ceil()
	th := (metrics.Ascent + metrics.Descent).Ceil()
	x, y := int(margin), int(margin)
	if strings.HasSuffix(c.Position, "right") {
		x = max(0, w-int(margin)-tw)
	}
	if strings.HasPrefix(c.Position, "bottom") {
		y = max(0, h-int(margin)-th)
	}
	rect := image.Rect(x, y, x+tw, y+th).Intersect(image.Rect(0, 0, w, h))
	var lum, weight float64
	for py := rect.Min.Y; py < rect.Max.Y; py += max(1, rect.Dy()/12) {
		for px := rect.Min.X; px < rect.Max.X; px += max(1, rect.Dx()/32) {
			v := color.NRGBAModel.Convert(src.At(b.Min.X+px, b.Min.Y+py)).(color.NRGBA)
			a := float64(v.A) / 255
			lum += (.2126*float64(v.R) + .7152*float64(v.G) + .0722*float64(v.B)) * a
			weight += a
		}
	}
	ink := float64(255)
	if weight > 0 && lum/weight > 132 {
		ink = 0
	}
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	d := font.Drawer{Dst: mask, Src: image.NewUniform(color.Alpha{255}), Face: face, Dot: fixed.P(x, y+metrics.Ascent.Ceil())}
	d.DrawString(text)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			v := color.NRGBAModel.Convert(src.At(b.Min.X+px, b.Min.Y+py)).(color.NRGBA)
			a := float64(mask.AlphaAt(px, py).A) * float64(c.Opacity) / 25500
			if v.A != 0 && a > 0 {
				v.R = uint8(math.Round(float64(v.R)*(1-a) + ink*a))
				v.G = uint8(math.Round(float64(v.G)*(1-a) + ink*a))
				v.B = uint8(math.Round(float64(v.B)*(1-a) + ink*a))
			}
			dst.SetNRGBA(px, py, v)
		}
	}
	return dst, nil
}
