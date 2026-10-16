package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type importItem struct {
	Data       Payload  `json:"data"`
	Category   string   `json:"category"`
	MediaPaths []string `json:"media_paths"`
}
type importManifest struct {
	SourceKey  string       `json:"source_key"`
	SourceHash string       `json:"source_hash"`
	SourceRoot string       `json:"source_root"`
	Vendor     importItem   `json:"vendor"`
	Products   []importItem `json:"products"`
}
type importResult struct {
	VendorID        uint64 `json:"vendor_id"`
	Products        int    `json:"products"`
	Images          int    `json:"images"`
	AlreadyImported bool   `json:"already_imported"`
}

// An explicit local administrator command. Imported booths belong to a reserved
// account, so a supplied business phone is never treated as a verified login.
// The entire batch is atomic; rerunning the same source is a no-op.
func (a *App) importVendor(manifestPath string) (result importResult, err error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return result, err
	}
	var m importManifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return result, err
	}
	if !regexp.MustCompile(`^V[0-9]{4,12}$`).MatchString(m.SourceKey) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(m.SourceHash) || len(m.Products) > maxVendorProducts {
		return result, fmt.Errorf("invalid import manifest")
	}
	root, err := filepath.Abs(m.SourceRoot)
	if err != nil {
		return result, err
	}
	note := "本地采集导入 " + m.SourceKey + " SHA256:" + m.SourceHash
	written := []string{}
	defer func() {
		if err != nil {
			for _, p := range written {
				os.Remove(p)
			}
		}
	}()
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		var admin Account
		if e := tx.Where("role = 'admin'").Order("id").First(&admin).Error; e != nil {
			return e
		}
		account := Account{Username: "import:" + m.SourceKey, Role: "merchant"}
		if e := tx.Where("username = ?", account.Username).FirstOrCreate(&account).Error; e != nil {
			return e
		}
		if account.Role != "merchant" {
			return fmt.Errorf("import account conflict")
		}
		var existing Content
		e := tx.Where("kind = 'vendor' AND owner_id = ?", account.ID).First(&existing).Error
		if e == nil {
			var rev Revision
			if e = tx.First(&rev, existing.PublishedRevisionID).Error; e != nil {
				return e
			}
			if rev.ReviewNote != note {
				return fmt.Errorf("source changed: existing booth preserved; review updates in the administrator console")
			}
			result = importResult{VendorID: existing.ID, Products: len(m.Products), AlreadyImported: true}
			for _, item := range append([]importItem{m.Vendor}, m.Products...) {
				result.Images += len(item.MediaPaths)
			}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		region := Region{Name: m.Vendor.Data.RegionName, Enabled: true}
		if e = tx.Where("name = ?", region.Name).FirstOrCreate(&region).Error; e != nil {
			return e
		}
		parent := Category{HallID: 1, ParentID: 1, Name: "耕整地机械", NameEN: "Tillage machinery", Enabled: true}
		if e = tx.Where("name = ? AND parent_id = ?", parent.Name, parent.ParentID).FirstOrCreate(&parent).Error; e != nil {
			return e
		}
		categories := map[string]uint64{}
		for _, c := range []Category{{HallID: 1, ParentID: parent.ID, Name: "翻转犁", NameEN: "Reversible ploughs", Enabled: true}, {HallID: 1, ParentID: parent.ID, Name: "驱动耙", NameEN: "Power harrows", Enabled: true}} {
			if e = tx.Where("name = ? AND parent_id = ?", c.Name, c.ParentID).FirstOrCreate(&c).Error; e != nil {
				return e
			}
			categories[c.Name] = c.ID
		}
		seenNames := map[string]bool{}
		items := append([]importItem{m.Vendor}, m.Products...)
		for i, item := range items {
			kind := "product"
			content := Content{Kind: kind, OwnerID: account.ID, Featured: i < 9, SortOrder: i}
			item.Data.RegionID = region.ID
			if i == 0 {
				kind = "vendor"
				content.Kind = kind
				content.VendorOwnerKey = &account.ID
				item.Data.CategoryID = parent.ID
			} else {
				if seenNames[item.Data.Name] {
					return fmt.Errorf("duplicate product name: %s", item.Data.Name)
				}
				seenNames[item.Data.Name] = true
				content.VendorID, e = productSlot(tx, account.ID)
				if e != nil {
					return e
				}
				item.Data.CategoryID = categories[item.Category]
				if item.Data.CategoryID == 0 {
					return fmt.Errorf("invalid import category")
				}
			}
			item.Data.MediaIDs = nil
			for _, relative := range item.MediaPaths {
				if filepath.IsAbs(relative) {
					return fmt.Errorf("image must be relative to source root")
				}
				sourcePath := filepath.Join(root, filepath.FromSlash(relative))
				rel, e := filepath.Rel(root, sourcePath)
				if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
					return fmt.Errorf("image outside source root")
				}
				info, e := os.Stat(sourcePath)
				if e != nil {
					return e
				}
				if info.Size() == 0 || info.Size() > 10<<20 {
					return fmt.Errorf("invalid image size: %s", relative)
				}
				f, e := os.Open(sourcePath)
				if e != nil {
					return e
				}
				config, format, e := image.DecodeConfig(f)
				if e != nil || (format != "jpeg" && format != "png" && format != "webp") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 36000000 {
					f.Close()
					return fmt.Errorf("invalid image: %s", relative)
				}
				if _, e = f.Seek(0, 0); e != nil {
					f.Close()
					return e
				}
				img, _, e := image.Decode(f)
				f.Close()
				if e != nil {
					return e
				}
				extension, outputFormat := ".jpg", "jpeg"
				if format == "png" {
					extension, outputFormat = ".png", "png"
				}
				key := randomToken()
				media := Media{OwnerID: account.ID, Path: key + extension, ThumbPath: key + "-thumb" + extension}
				for _, target := range []struct {
					name string
					size int
				}{{media.Path, 2048}, {media.ThumbPath, 480}} {
					path := filepath.Join(a.Config.MediaDir, target.name)
					written = append(written, path)
					if e = writeImage(path, scaled(img, target.size), outputFormat); e != nil {
						return e
					}
				}
				if e = tx.Create(&media).Error; e != nil {
					return e
				}
				item.Data.MediaIDs = append(item.Data.MediaIDs, media.ID)
				result.Images++
			}
			if e = a.validatePayload(tx, account, kind, &item.Data); e != nil {
				return e
			}
			if e = tx.Create(&content).Error; e != nil {
				return e
			}
			if e = makeDraft(tx, &content, item.Data); e != nil {
				return e
			}
			now := time.Now().UTC()
			if e = tx.Model(&Revision{}).Where("id = ?", content.DraftRevisionID).Updates(map[string]any{"status": "approved", "review_note": note, "reviewer_id": admin.ID, "reviewed_at": now}).Error; e != nil {
				return e
			}
			if e = tx.Model(&content).Updates(map[string]any{"published_revision_id": content.DraftRevisionID, "visible": true, "epoch": 1}).Error; e != nil {
				return e
			}
			for _, action := range []string{"submit", "approve"} {
				if e = recordReview(tx, ReviewLog{ContentID: content.ID, RevisionID: content.DraftRevisionID, ActorID: admin.ID, Action: action, Note: note}); e != nil {
					return e
				}
			}
			if i == 0 {
				result.VendorID = content.ID
			} else {
				result.Products++
			}
		}
		return nil
	})
	return result, err
}
