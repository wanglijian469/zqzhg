package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const imageEncodingVersion = "jpeg80-png-v1"
const maxDisplayImageBytes = 64 << 20

type imageFlight struct {
	done chan struct{}
	data []byte
	err  error
}

// Rebuildable display cache, deliberately separate from media inventory/backups.
type imageVariantCache struct {
	directory string
	limit     int64
	ttl       time.Duration
	mu        sync.Mutex
	flights   map[string]*imageFlight
	slots     chan struct{}
	lastPrune time.Time
}

func imageDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func pathWithin(parent, child string) bool {
	p, e1 := filepath.Abs(parent)
	c, e2 := filepath.Abs(child)
	if e1 != nil || e2 != nil {
		return true
	}
	rel, err := filepath.Rel(p, c)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func (a *App) displayImageCache() *imageVariantCache {
	a.imageCacheOnce.Do(func() {
		dir := env("IMAGE_CACHE_DIR", filepath.Join(a.operations().dir, "image-cache"))
		// Never create a cache that can bypass media authorization or be backed up as originals.
		if a.Config.WebDir != "" && pathWithin(a.Config.WebDir, dir) || pathWithin(a.Config.MediaDir, dir) {
			return
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return
		}
		limit := int64(settingInt("IMAGE_CACHE_MIB", 512)) << 20
		if limit <= 0 {
			return
		}
		a.imageCache = &imageVariantCache{directory: dir, limit: limit, ttl: 7 * 24 * time.Hour, flights: map[string]*imageFlight{}, slots: make(chan struct{}, 2)}
	})
	return a.imageCache
}

func (cache *imageVariantCache) read(key string) ([]byte, bool) {
	file := filepath.Join(cache.directory, key+".img")
	stat, err := os.Stat(file)
	if err != nil {
		return nil, false
	}
	if stat.Size() > maxDisplayImageBytes || time.Since(stat.ModTime()) > cache.ttl {
		_ = os.Remove(file)
		return nil, false
	}
	record, err := os.ReadFile(file)
	if err != nil || len(record) < 66 || record[64] != '\n' || string(record[:64]) != imageDigest(record[65:]) {
		_ = os.Remove(file)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(file, now, now)
	return record[65:], true
}

// Caller holds mu, so reservations and cache writes cannot exceed the byte cap.
func (cache *imageVariantCache) prune(required int64) error {
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return err
	}
	type cachedFile struct {
		path string
		size int64
		used time.Time
	}
	files := []cachedFile{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".img") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		file := filepath.Join(cache.directory, entry.Name())
		if time.Since(info.ModTime()) > cache.ttl {
			if err = os.Remove(file); err == nil {
				continue
			}
		}
		total += info.Size()
		files = append(files, cachedFile{file, info.Size(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.Before(files[j].used) })
	for _, file := range files {
		if total+required <= cache.limit {
			break
		}
		if err := os.Remove(file.path); err == nil {
			total -= file.size
		}
	}
	cache.lastPrune = time.Now()
	if total+required > cache.limit {
		return errors.New("image_cache_full")
	}
	return nil
}

func (cache *imageVariantCache) variant(ctx context.Context, key string, generate func(context.Context) ([]byte, error)) ([]byte, error) {
	cache.mu.Lock()
	if time.Since(cache.lastPrune) > 10*time.Minute {
		_ = cache.prune(0)
	}
	if data, ok := cache.read(key); ok {
		cache.mu.Unlock()
		return data, nil
	}
	if existing := cache.flights[key]; existing != nil {
		cache.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-existing.done:
			return existing.data, existing.err
		}
	}
	flight := &imageFlight{done: make(chan struct{})}
	cache.flights[key] = flight
	cache.mu.Unlock()
	defer func() {
		cache.mu.Lock()
		delete(cache.flights, key)
		close(flight.done)
		cache.mu.Unlock()
	}()
	select {
	case cache.slots <- struct{}{}:
	case <-ctx.Done():
		flight.err = ctx.Err()
		return nil, flight.err
	}
	defer func() { <-cache.slots }()
	flight.data, flight.err = generate(ctx)
	if flight.err != nil {
		return nil, flight.err
	}
	if err := ctx.Err(); err != nil {
		flight.err = err
		return nil, err
	}
	record := append([]byte(imageDigest(flight.data)+"\n"), flight.data...)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := cache.prune(int64(len(record))); err != nil {
		flight.err = err
		return nil, err
	}
	file, err := os.CreateTemp(cache.directory, ".image-*")
	if err != nil {
		flight.err = err
		return nil, err
	}
	temp := file.Name()
	defer os.Remove(temp)
	_, err = file.Write(record)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp, filepath.Join(cache.directory, key+".img"))
	}
	flight.err = err
	return flight.data, err
}

func encodeDisplayImage(data []byte, width int) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 36000000 {
		return nil, errors.New("invalid_image")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img := scaled(source, width)
	var output bytes.Buffer
	opaque := false
	if v, ok := img.(interface{ Opaque() bool }); ok {
		opaque = v.Opaque()
	}
	if opaque {
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 80})
	} else {
		err = png.Encode(&output, img)
	}
	return output.Bytes(), err
}

func displayImageObject(key string) bool {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	}
	return false
}

func loadDisplayImage(ctx context.Context, store ObjectStore, key, expectedHash string, width int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader, err := store.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, maxDisplayImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxDisplayImageBytes || imageDigest(raw) != expectedHash {
		return nil, errors.New("image_integrity_failed")
	}
	data, err := encodeDisplayImage(raw, width)
	if err != nil {
		return nil, err
	}
	return data, ctx.Err()
}

func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

// Called only AFTER mediaRead checks publication, ownership, readiness and watermarks.
func (a *App) serveImageDisplay(c *gin.Context, media Media, key string, public bool) bool {
	if c.Query("download") == "1" {
		return false
	}
	width := 0
	if raw, exists := c.GetQuery("w"); exists {
		width, _ = strconv.Atoi(raw)
		if width != 240 && width != 480 && width != 960 && width != 1440 || c.Query("thumb") != "" {
			fail(c, 400, "invalid_image_size")
			return true
		}
		if !displayImageObject(key) {
			key = media.ThumbPath
		}
	}
	if !displayImageObject(key) {
		if width > 0 {
			fail(c, 404, "not_found")
			return true
		}
		return false
	}
	if width > 0 {
		if cache := a.displayImageCache(); cache != nil {
			object, store, err := a.resolveObject(c.Request.Context(), key)
			if err == nil {
				cacheKey := imageDigest([]byte(object.SHA256 + fmt.Sprint(":", width, ":", imageEncodingVersion)))
				ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
				data, err := cache.variant(ctx, cacheKey, func(ctx context.Context) ([]byte, error) {
					return loadDisplayImage(ctx, store, key, object.SHA256, width)
				})
				cancel()
				if err == nil {
					if public {
						c.Header("Cache-Control", "private, no-cache")
						c.Header("ETag", `"`+imageDigest(data)+`"`)
					}
					http.ServeContent(c.Writer, c.Request, "display"+imageExtension(data), time.Time{}, bytes.NewReader(data))
					return true
				}
			}
		}
		// Existing authorized thumbnail/full display remains available on cache failure.
		if width <= 480 && displayImageObject(media.ThumbPath) {
			key = media.ThumbPath
		}
	}
	if public {
		object, store, err := a.resolveObject(c.Request.Context(), key)
		if err == nil {
			if _, err = store.Stat(c.Request.Context(), key); err == nil {
				etag := `"` + object.SHA256 + `"`
				c.Header("Cache-Control", "private, no-cache")
				c.Header("ETag", etag)
				if etagMatches(c.GetHeader("If-None-Match"), etag) {
					c.Status(http.StatusNotModified)
					return true
				}
			}
		}
	}
	a.serveObject(c, key)
	return true
}

func imageExtension(data []byte) string {
	if bytes.HasPrefix(data, []byte("\x89PNG")) {
		return ".png"
	}
	return ".jpg"
}
