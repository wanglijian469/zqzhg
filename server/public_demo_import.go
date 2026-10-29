package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const publicDemoNote = "开发演示资料迁移 "

type publicDemoMedia struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type publicDemoBundle struct {
	Format     int                        `json:"format"`
	Usage      string                     `json:"usage"`
	Source     string                     `json:"source"`
	Vendors    []View                     `json:"vendors"`
	Products   []View                     `json:"products"`
	Categories []Category                 `json:"categories"`
	Media      map[string]publicDemoMedia `json:"media"`
}
type publicDemoImportResult struct{ Vendors, Products, Images, PreservedVendors, PreservedProducts, Ambiguous int }

// Host-operator command for the user-requested demonstration catalogue. Only
// published public snapshots are accepted; existing content and URLs are kept.
func (a *App) importPublicDemo(path string) (result publicDemoImportResult, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	var bundle publicDemoBundle
	if json.Unmarshal(raw, &bundle) != nil || bundle.Format != 1 || bundle.Usage != mapDemoUsage || len(bundle.Vendors) > 100 || len(bundle.Products) > 3000 || len(bundle.Media) > 2000 {
		return result, fmt.Errorf("invalid_public_demo_bundle")
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return result, err
	}
	note := publicDemoNote + previewDigest(raw)
	sources := map[uint64]Category{}
	for _, c := range bundle.Categories {
		sources[c.ID] = c
	}
	written := []string{}
	defer func() {
		if err != nil {
			for _, p := range written {
				_ = os.Remove(p)
			}
		}
	}()
	err = a.watermarkTransaction(func(tx *gorm.DB) error {
		var admin Account
		if e := tx.Where("role='admin'").Order("id").First(&admin).Error; e != nil {
			return e
		}
		category := func(source uint64) uint64 {
			if source == 0 {
				return 0
			}
			ref, ok := sources[source]
			if !ok {
				return 0
			}
			var rows []Category
			q := tx.Where("hall_id=? AND category_kind=? AND deleted_at IS NULL", ref.HallID, ref.CategoryKind)
			if ref.Code != nil {
				q = q.Where("code=?", *ref.Code)
			} else {
				q = q.Where("name=?", ref.Name)
			}
			if q.Find(&rows).Error != nil || len(rows) != 1 {
				return 0
			}
			return canonicalCategory(tx, rows[0].ID)
		}
		var existing []Content
		if e := tx.Where("kind='vendor'").Find(&existing).Error; e != nil {
			return e
		}
		names := map[string][]Content{}
		for _, v := range existing {
			var rev Revision
			revisionID := v.PublishedRevisionID
			if revisionID == 0 {
				revisionID = v.DraftRevisionID
			}
			if revisionID == 0 {
				continue
			}
			if e := tx.First(&rev, revisionID).Error; e != nil {
				return e
			}
			names[strings.TrimSpace(rev.Name)] = append(names[strings.TrimSpace(rev.Name)], v)
		}
		mediaMap := map[string]uint64{}
		importImage := func(source uint64, owner uint64) (uint64, error) {
			key := fmt.Sprintf("%d:%d", owner, source)
			if id := mediaMap[key]; id != 0 {
				return id, nil
			}
			item, ok := bundle.Media[strconv.FormatUint(source, 10)]
			if !ok {
				return 0, fmt.Errorf("public_demo_image_missing")
			}
			if filepath.IsAbs(item.Path) {
				return 0, fmt.Errorf("invalid_public_demo_path")
			}
			sourcePath := filepath.Join(root, filepath.FromSlash(item.Path))
			rel, e := filepath.Rel(root, sourcePath)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return 0, fmt.Errorf("invalid_public_demo_path")
			}
			info, e := os.Lstat(sourcePath)
			if e != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 10<<20 {
				return 0, fmt.Errorf("invalid_public_demo_image")
			}
			bytes, e := os.ReadFile(sourcePath)
			if e != nil || previewDigest(bytes) != item.SHA256 {
				return 0, fmt.Errorf("public_demo_integrity_failed")
			}
			f, e := os.Open(sourcePath)
			if e != nil {
				return 0, e
			}
			config, format, e := image.DecodeConfig(f)
			_ = f.Close()
			if e != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 36000000 || (format != "jpeg" && format != "png" && format != "webp") {
				return 0, fmt.Errorf("invalid_public_demo_image")
			}
			f, e = os.Open(sourcePath)
			if e != nil {
				return 0, e
			}
			img, _, e := image.Decode(f)
			_ = f.Close()
			if e != nil {
				return 0, e
			}
			extension, outputFormat := ".jpg", "jpeg"
			if format == "png" {
				extension, outputFormat = ".png", "png"
			}
			key = randomToken()
			media := Media{OwnerID: owner, Path: key + extension, ThumbPath: key + "-thumb" + extension}
			for _, target := range []struct {
				name string
				size int
			}{{media.Path, 2048}, {media.ThumbPath, 480}} {
				targetPath := filepath.Join(a.Config.MediaDir, target.name)
				written = append(written, targetPath)
				if e = writeImage(targetPath, scaled(img, target.size), outputFormat); e != nil {
					return 0, e
				}
			}
			if e = tx.Create(&media).Error; e != nil {
				return 0, e
			}
			// These are already rendered public images, like pre-watermark legacy
			// assets. Preserve their appearance rather than watermarking them again.
			mediaMap[fmt.Sprintf("%d:%d", owner, source)] = media.ID
			result.Images++
			return media.ID, nil
		}
		prepare := func(p Payload, owner uint64) (Payload, error) {
			p.PrimaryCategoryID = category(p.PrimaryCategoryID)
			p.ProductCategoryID = category(p.ProductCategoryID)
			p.CategoryID = category(p.CategoryID)
			second := []uint64{}
			for _, id := range p.SecondaryCategoryIDs {
				if mapped := category(id); mapped != 0 && mapped != p.PrimaryCategoryID && len(second) < 2 {
					second = append(second, mapped)
				}
			}
			p.SecondaryCategoryIDs = second
			p.RegionID = 0
			if p.RegionCode != "" {
				if e := resolveAdministrativeRegion(tx, &p); e != nil {
					return p, e
				}
			} else if p.RegionName != "" {
				region := Region{Name: p.RegionName, Enabled: true}
				if e := tx.Where("name=?", p.RegionName).FirstOrCreate(&region).Error; e != nil {
					return p, e
				}
				p.RegionID = region.ID
			}
			raw, _ := json.Marshal(p)
			var tree map[string]any
			_ = json.Unmarshal(raw, &tree)
			var remap func(any, string) (any, error)
			remap = func(value any, key string) (any, error) {
				switch v := value.(type) {
				case map[string]any:
					for k, item := range v {
						next, e := remap(item, k)
						if e != nil {
							return nil, e
						}
						v[k] = next
					}
					return v, nil
				case []any:
					for i, item := range v {
						childKey := key
						if key == "media_ids" {
							childKey = "media_id"
						}
						next, e := remap(item, childKey)
						if e != nil {
							return nil, e
						}
						v[i] = next
					}
					return v, nil
				case float64:
					if (key == "media_id" || key == "logo_media_id" || key == "cover_media_id") && v > 0 {
						id, e := importImage(uint64(v), owner)
						return id, e
					}
				}
				return value, nil
			}
			value, e := remap(tree, "")
			if e != nil {
				return p, e
			}
			raw, _ = json.Marshal(value)
			e = json.Unmarshal(raw, &p)
			return p, e
		}
		publish := func(content *Content, p Payload) error {
			owner := Account{ID: content.OwnerID, Role: "merchant"}
			if e := a.validatePayload(tx, owner, content.Kind, &p); e != nil {
				return e
			}
			if e := tx.Create(content).Error; e != nil {
				return e
			}
			if e := makeDraft(tx, content, p); e != nil {
				return e
			}
			now := time.Now().UTC()
			if e := tx.Model(&Revision{}).Where("id=?", content.DraftRevisionID).Updates(map[string]any{"status": "approved", "review_note": note, "reviewer_id": admin.ID, "reviewed_at": now}).Error; e != nil {
				return e
			}
			if e := tx.Model(content).Updates(map[string]any{"published_revision_id": content.DraftRevisionID, "visible": true, "epoch": 1}).Error; e != nil {
				return e
			}
			return recordOperation(tx, admin.ID, "content", content.ID, "public_demo_import", note)
		}
		for _, source := range bundle.Vendors {
			if source.Kind != "vendor" || source.PublishedRevisionID == 0 || !source.Visible {
				return fmt.Errorf("non_public_demo_content")
			}
			rows := names[strings.TrimSpace(source.Data.Name)]
			var vendor Content
			if len(rows) > 1 {
				result.Ambiguous++
				continue
			}
			if len(rows) == 1 {
				if rows[0].PublishedRevisionID == 0 || !rows[0].Visible {
					result.Ambiguous++
					continue
				}
				vendor = rows[0]
				result.PreservedVendors++
			} else {
				owner := Account{Username: "demo:" + digest(source.Data.Name)[:24], Role: "merchant"}
				if e := tx.Where("username=?", owner.Username).FirstOrCreate(&owner).Error; e != nil {
					return e
				}
				if owner.Role != "merchant" {
					return fmt.Errorf("public_demo_owner_conflict")
				}
				data, e := prepare(source.Data, owner.ID)
				if e != nil {
					return e
				}
				vendor = Content{Kind: "vendor", HallID: 1, OwnerID: owner.ID, VendorOwnerKey: &owner.ID}
				slug := source.ShowroomSlug
				if regexp.MustCompile(`^[a-z][a-z0-9-]{2,79}$`).MatchString(slug) && !strings.HasPrefix(slug, "v-") {
					var n int64
					if e := tx.Model(&Content{}).Where("showroom_slug=?", slug).Count(&n).Error; e != nil {
						return e
					}
					if n == 0 {
						vendor.ShowroomSlug = &slug
					}
				}
				if e = publish(&vendor, data); e != nil {
					return e
				}
				names[strings.TrimSpace(data.Name)] = []Content{vendor}
				result.Vendors++
			}
			var products []Content
			if e := tx.Where("kind='product' AND vendor_id=?", vendor.ID).Find(&products).Error; e != nil {
				return e
			}
			productNames := map[string]bool{}
			for _, p := range products {
				var revision Revision
				revisionID := p.PublishedRevisionID
				if revisionID == 0 {
					revisionID = p.DraftRevisionID
				}
				if revisionID == 0 {
					continue
				}
				if e := tx.First(&revision, revisionID).Error; e != nil {
					return e
				}
				productNames[strings.TrimSpace(revision.Name)] = true
			}
			for _, sourceProduct := range bundle.Products {
				if sourceProduct.VendorID != source.ID {
					continue
				}
				if sourceProduct.Kind != "product" || sourceProduct.PublishedRevisionID == 0 || !sourceProduct.Visible {
					return fmt.Errorf("non_public_demo_content")
				}
				name := strings.TrimSpace(sourceProduct.Data.Name)
				if productNames[name] {
					result.PreservedProducts++
					continue
				}
				if len(productNames) >= maxVendorProducts {
					return fmt.Errorf("product_limit_reached")
				}
				data, e := prepare(sourceProduct.Data, vendor.OwnerID)
				if e != nil {
					return e
				}
				product := Content{Kind: "product", HallID: vendor.HallID, OwnerID: vendor.OwnerID, VendorID: vendor.ID}
				if e = publish(&product, data); e != nil {
					return e
				}
				productNames[name] = true
				result.Products++
			}
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, fmt.Errorf("public_demo_reference_missing")
	}
	return result, err
}
