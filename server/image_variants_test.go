package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func imageTestPNG(t *testing.T, transparent bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			alpha := uint8(255)
			if transparent && x < 100 {
				alpha = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(x % 255), uint8(y % 255), 100, alpha})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func newTestImageCache(t *testing.T) *imageVariantCache {
	t.Helper()
	return &imageVariantCache{directory: t.TempDir(), limit: 1 << 20, ttl: 7 * 24 * time.Hour, flights: map[string]*imageFlight{}, slots: make(chan struct{}, 2)}
}

func TestDisplayImageEncodingAndTransparency(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		original := imageTestPNG(t, transparent)
		for _, width := range []int{240, 480, 960, 1440} {
			data, err := encodeDisplayImage(original, width)
			if err != nil {
				t.Fatal(err)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			expected := width
			if expected > 800 {
				expected = 800
			}
			if cfg.Width != expected || cfg.Height != expected/2 {
				t.Fatal(cfg)
			}
			if transparent && format != "png" || !transparent && format != "jpeg" {
				t.Fatal("wrong display format", format)
			}
			if transparent {
				img, _, _ := image.Decode(bytes.NewReader(data))
				_, _, _, a := img.At(0, 0).RGBA()
				if a != 0 {
					t.Fatal("transparency lost")
				}
			}
		}
	}
}

func TestImageVariantCacheMergeIntegrityEvictionAndCancellation(t *testing.T) {
	cache := newTestImageCache(t)
	var calls atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			data, err := cache.variant(context.Background(), "same", func(context.Context) ([]byte, error) {
				calls.Add(1)
				time.Sleep(30 * time.Millisecond)
				return []byte("pixels"), nil
			})
			if err != nil || string(data) != "pixels" {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate generation", calls.Load())
	}
	file := filepath.Join(cache.directory, "same.img")
	os.WriteFile(file, []byte("corrupt"), 0600)
	_, err := cache.variant(context.Background(), "same", func(context.Context) ([]byte, error) { calls.Add(1); return []byte("repaired"), nil })
	if err != nil || calls.Load() != 2 {
		t.Fatal("cache not repaired", err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(file, old, old)
	cache.mu.Lock()
	err = cache.prune(0)
	cache.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("expired cache retained")
	}
	cache.limit = 100
	_, err = cache.variant(context.Background(), "oversized", func(context.Context) ([]byte, error) { return make([]byte, 101), nil })
	if err == nil {
		t.Fatal("capacity not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cache.slots <- struct{}{}
	cache.slots <- struct{}{}
	_, err = cache.variant(ctx, "cancelled", func(context.Context) ([]byte, error) { t.Error("cancelled generation ran"); return nil, nil })
	if err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestImageVariantCacheLimitsConcurrency(t *testing.T) {
	cache := newTestImageCache(t)
	var active, maximum atomic.Int32
	var wg sync.WaitGroup
	for _, key := range []string{"a", "b", "c", "d", "e", "f"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := cache.variant(context.Background(), key, func(context.Context) ([]byte, error) {
				v := active.Add(1)
				for prev := maximum.Load(); v > prev; prev = maximum.Load() {
					if maximum.CompareAndSwap(prev, v) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				active.Add(-1)
				return []byte(key), nil
			})
			if err != nil {
				t.Error(err)
			}
		}(key)
	}
	wg.Wait()
	if maximum.Load() > 2 {
		t.Fatal("too many encoders", maximum.Load())
	}
}

func TestPublicImageETagChecksPublicationBefore304(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("IMAGE_CACHE_DIR", t.TempDir())
	original := imageTestPNG(t, false)
	media := Media{Path: "display.png", ThumbPath: "display-thumb.png"}
	a.DB.Create(&media)
	os.WriteFile(filepath.Join(a.Config.MediaDir, media.Path), original, 0600)
	os.WriteFile(filepath.Join(a.Config.MediaDir, media.ThumbPath), original, 0600)
	vendor := industryFixture(t, a, Payload{Name: "图片缓存隔离测试", MediaIDs: []uint64{media.ID}}, 1)
	router := a.router()
	url := "/api/v1/media/" + fmtUint(media.ID) + "?w=240"
	read := func(path, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	first := read(url, "")
	if first.Code != 200 || first.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatal(first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing etag")
	}
	again := read(url, etag)
	if again.Code != 304 || again.Body.Len() != 0 {
		t.Fatal("not revalidated", again.Code)
	}
	for _, query := range []string{"?w=12", "?w=240&thumb=1", "?w=garbage"} {
		out := read(strings.Split(url, "?")[0]+query, etag)
		if out.Code != 400 || out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid width", out.Code)
		}
	}
	a.DB.Model(&Content{}).Where("id=?", vendor.ID).Update("visible", false)
	hidden := read(url, etag)
	if hidden.Code != 404 || hidden.Header().Get("ETag") != "" || hidden.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("withdrawn image served from cache", hidden.Code)
	}
	unchanged, _ := os.ReadFile(filepath.Join(a.Config.MediaDir, media.Path))
	if !bytes.Equal(unchanged, original) {
		t.Fatal("original modified")
	}
}

func fmtUint(v uint64) string { return fmt.Sprint(v) }

func TestPrivateImageCacheCannotCrossAccountsAndFallsBack(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("IMAGE_CACHE_DIR", t.TempDir())
	owner := Account{Username: "image-owner-" + randomToken()[:12], Role: "merchant"}
	other := Account{Username: "image-other-" + randomToken()[:12], Role: "merchant"}
	a.DB.Create(&owner)
	a.DB.Create(&other)
	first, _ := a.newSession(owner)
	second, _ := a.newSession(other)
	data := imageTestPNG(t, true)
	media := Media{OwnerID: owner.ID, Path: "private.png", ThumbPath: "private-thumb.png"}
	a.DB.Create(&media)
	os.WriteFile(filepath.Join(a.Config.MediaDir, media.Path), data, 0600)
	os.WriteFile(filepath.Join(a.Config.MediaDir, media.ThumbPath), data, 0600)
	router := a.router()
	url := "/api/v1/media/" + fmtUint(media.ID) + "?w=240"
	read := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	allowed := read(first["token"].(string))
	if allowed.Code != 200 || allowed.Header().Get("ETag") != "" || allowed.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private image cache headers", allowed.Code, allowed.Header())
	}
	for _, token := range []string{"", second["token"].(string)} {
		denied := read(token)
		if denied.Code != 404 || denied.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cached image escaped authorization", denied.Code)
		}
	}
	cache := a.displayImageCache()
	cache.limit = 1
	// A different size cannot fit, and must fall back to the existing authorized thumbnail.
	r := httptest.NewRequest("GET", strings.Replace(url, "240", "480", 1), nil)
	r.Header.Set("Authorization", "Bearer "+first["token"].(string))
	out := httptest.NewRecorder()
	router.ServeHTTP(out, r)
	if out.Code != 200 || !bytes.Equal(out.Body.Bytes(), data) {
		t.Fatal("cache capacity failure broke display", out.Code)
	}
}
