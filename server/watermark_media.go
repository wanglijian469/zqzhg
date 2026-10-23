package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MediaWatermark struct {
	MediaID       uint64 `gorm:"primaryKey"`
	SourceMediaID uint64
	VariantKey    *string
	Purpose       string
	State         string
	ConfigJSON    string
	ConfigVersion uint64
	VendorID      uint64
	VendorText    string
	CreatedAt     time.Time
}

func validImagePurpose(s string) bool {
	switch s {
	case "unknown", "company_gallery", "company_cover", "product_image", "company_logo", "credential", "home_display", "category_image", "resource_cover":
		return true
	}
	return false
}
func uploadImagePurpose(c *gin.Context) (string, error) {
	for _, pair := range [][2]string{{"upload_home_display", "home_display"}, {"upload_category_media", "category_image"}, {"upload_resource_id", "resource_cover"}} {
		if _, ok := c.Get(pair[0]); ok {
			return pair[1], nil
		}
	}
	s := strings.TrimSpace(c.PostForm("purpose"))
	if s == "" {
		s = "unknown"
	}
	if !validImagePurpose(s) {
		return "", fieldFailure("invalid_request", "purpose", "invalid")
	}
	return s, nil
}
func vendorWatermarkName(p Payload) string {
	if p.CompanyProfile != nil && strings.TrimSpace(p.CompanyProfile.ShortName) != "" {
		return strings.TrimSpace(p.CompanyProfile.ShortName)
	}
	return strings.TrimSpace(p.Name)
}
func watermarkVendor(tx *gorm.DB, owner uint64, fallback *Content, p Payload) (uint64, string, error) {
	var vendor Content
	e := tx.Where("owner_id=? AND kind='vendor'", owner).First(&vendor).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		if fallback != nil && fallback.Kind == "vendor" {
			return fallback.ID, vendorWatermarkName(p), nil
		}
		return 0, "", nil
	}
	if e != nil {
		return 0, "", e
	}
	rid := vendor.PublishedRevisionID
	if rid > 0 {
		var rev Revision
		if e := tx.First(&rev, rid).Error; e != nil {
			return 0, "", e
		}
		var old Payload
		if e := json.Unmarshal([]byte(rev.Payload), &old); e != nil {
			return 0, "", e
		}
		return vendor.ID, vendorWatermarkName(old), nil
	}
	if fallback != nil && fallback.Kind == "vendor" && fallback.OwnerID == owner {
		return vendor.ID, vendorWatermarkName(p), nil
	}
	if vendor.DraftRevisionID > 0 {
		var rev Revision
		if e := tx.First(&rev, vendor.DraftRevisionID).Error; e != nil {
			return 0, "", e
		}
		var old Payload
		if e := json.Unmarshal([]byte(rev.Payload), &old); e != nil {
			return 0, "", e
		}
		return vendor.ID, vendorWatermarkName(old), nil
	}
	return vendor.ID, "", nil
}
func stampWatermarkSnapshot(tx *gorm.DB, m Media, purpose string) error {
	cfg, e := systemConfigDB(tx)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(cfg.Watermark)
	if e != nil {
		return e
	}
	vid, name, e := watermarkVendor(tx, m.OwnerID, nil, Payload{})
	if e != nil {
		return e
	}
	state := "pending"
	if !cfg.Watermark.Enabled {
		state = "disabled"
	}
	return tx.Create(&MediaWatermark{MediaID: m.ID, Purpose: purpose, State: state, ConfigJSON: string(raw), ConfigVersion: cfg.Version, VendorID: vid, VendorText: name, CreatedAt: time.Now().UTC()}).Error
}

type watermarkObject struct {
	Key   string
	Store ObjectStore
}

// Object writes are tracked until the surrounding SQL transaction commits. A
// failed save removes only random, newly-created objects, never source media.
type watermarkEffectsKey struct{}

func (a *App) watermarkTransaction(fn func(*gorm.DB) error) error {
	pending := []watermarkObject{}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		return fn(tx.WithContext(context.WithValue(tx.Statement.Context, watermarkEffectsKey{}, &pending)))
	})
	if e != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, obj := range pending {
			_ = obj.Store.Delete(ctx, obj.Key)
		}
	}
	return e
}
func (a *App) makeDraft(tx *gorm.DB, c *Content, p Payload) error {
	if e := a.prepareWatermarkMedia(tx, c, &p); e != nil {
		return e
	}
	return makeDraft(tx, c, p)
}
func (a *App) prepareWatermarkMedia(tx *gorm.DB, c *Content, p *Payload) error {
	// Lock the owner before generating variants; quota checks and per-source variant
	// creation serialize with other saves and upload reservations for this owner.
	ids := payloadMedia(*p)
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if e := tx.Model(&MediaWatermark{}).Where("media_id IN ?", ids).Count(&count).Error; e != nil {
		return e
	}
	if count == 0 {
		return nil
	}
	var owner Account
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&owner, c.OwnerID).Error; e != nil {
		return e
	}
	changed := map[uint64]uint64{}
	for i, id := range p.MediaIDs {
		next, e := a.watermarkVariant(tx, c, *p, id, func() string {
			if c.Kind == "product" {
				return "product_image"
			}
			return "company_gallery"
		}())
		if e != nil {
			return e
		}
		p.MediaIDs[i] = next
		changed[id] = next
	}
	if p.CompanyProfile != nil {
		profileCopy := *p.CompanyProfile
		p.CompanyProfile = &profileCopy
		if p.CompanyProfile.CoverMediaID > 0 {
			next, e := a.watermarkVariant(tx, c, *p, p.CompanyProfile.CoverMediaID, "company_cover")
			if e != nil {
				return e
			}
			p.CompanyProfile.CoverMediaID = next
		}
		clean := func(id uint64) (uint64, error) {
			if id == 0 {
				return 0, nil
			}
			var row MediaWatermark
			e := tx.First(&row, "media_id=?", id).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return id, nil
			}
			if e != nil {
				return 0, e
			}
			if row.SourceMediaID > 0 {
				return row.SourceMediaID, nil
			}
			return id, nil
		}
		var e error
		p.CompanyProfile.LogoMediaID, e = clean(p.CompanyProfile.LogoMediaID)
		if e != nil {
			return e
		}
		p.CompanyProfile.Credentials = append([]CompanyCredential(nil), p.CompanyProfile.Credentials...)
		for i := range p.CompanyProfile.Credentials {
			p.CompanyProfile.Credentials[i].MediaID, e = clean(p.CompanyProfile.Credentials[i].MediaID)
			if e != nil {
				return e
			}
		}
		p.CompanyProfile.GalleryAnnotations = append([]GalleryAnnotation(nil), p.CompanyProfile.GalleryAnnotations...)
		for i := range p.CompanyProfile.GalleryAnnotations {
			if next, ok := changed[p.CompanyProfile.GalleryAnnotations[i].MediaID]; ok {
				p.CompanyProfile.GalleryAnnotations[i].MediaID = next
			}
		}
	}
	return nil
}
func (a *App) watermarkVariant(tx *gorm.DB, c *Content, p Payload, id uint64, purpose string) (uint64, error) {
	var info MediaWatermark
	e := tx.First(&info, "media_id=?", id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return id, nil
	}
	if e != nil {
		return 0, e
	}
	var cfg WatermarkConfig
	if json.Unmarshal([]byte(info.ConfigJSON), &cfg) != nil {
		return 0, errors.New("invalid_watermark")
	}
	if !cfg.Enabled {
		return id, nil
	}
	var source Media
	if e := tx.First(&source, id).Error; e != nil {
		return 0, e
	}
	if source.OwnerID != c.OwnerID {
		return 0, errors.New("forbidden")
	}
	if info.SourceMediaID > 0 {
		if info.State != "ready" {
			return 0, errors.New("watermark_not_ready")
		}
		return id, nil
	}
	if info.VendorText == "" {
		info.VendorID, info.VendorText, e = watermarkVendor(tx, c.OwnerID, c, p)
		if e != nil {
			return 0, e
		}
		if info.VendorText == "" {
			return 0, errors.New("watermark_vendor_required")
		}
		if e := tx.Model(&info).Updates(map[string]any{"vendor_id": info.VendorID, "vendor_text": info.VendorText}).Error; e != nil {
			return 0, e
		}
	}
	variantKey := fmt.Sprintf("%d:%d", id, info.VendorID)
	var existing MediaWatermark
	e = tx.Where("variant_key=?", variantKey).First(&existing).Error
	if e == nil {
		return existing.MediaID, nil
	}
	if !errors.Is(e, gorm.ErrRecordNotFound) {
		return 0, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// Normal uploads have a StoredObject; file imports can still be uncommitted in
	// this transaction, so use their validated local path without a second DB write.
	var object StoredObject
	input := ""
	if e := tx.First(&object, "object_key=?", source.Path).Error; e == nil {
		input, e = a.materializeObject(ctx, source.Path)
		if e != nil {
			return 0, e
		}
		defer os.Remove(input)
	} else if errors.Is(e, gorm.ErrRecordNotFound) {
		input, e = localObjectPath(a.Config.MediaDir, source.Path)
		if e != nil {
			return 0, e
		}
	} else {
		return 0, e
	}
	f, e := os.Open(input)
	if e != nil {
		return 0, e
	}
	img, _, e := image.Decode(f)
	f.Close()
	if e != nil {
		return 0, e
	}
	extension := strings.ToLower(filepath.Ext(source.Path))
	format := "jpeg"
	if extension == ".png" {
		format = "png"
	} else {
		extension = ".jpg"
	}
	stem := randomToken()
	variant := Media{OwnerID: c.OwnerID, Path: stem + extension, ThumbPath: stem + "-thumb" + extension}
	paths := []string{a.workPath(variant.Path), a.workPath(variant.ThumbPath)}
	defer func() {
		for _, path := range paths {
			_ = os.Remove(path)
		}
	}()
	var size int64
	for i, maxSize := range []int{2048, 480} {
		marked, e := renderWatermark(scaled(img, maxSize), cfg, info.VendorText)
		if e != nil {
			return 0, e
		}
		if e = writeImage(paths[i], marked, format); e != nil {
			return 0, e
		}
		s, e := os.Stat(paths[i])
		if e != nil {
			return 0, e
		}
		size += s.Size()
	}
	if e := a.checkUploadDisks(uint64(size)); e != nil {
		return 0, e
	}
	if e := a.syncOwnerInventory(tx, c.OwnerID); e != nil {
		return 0, e
	}
	used, reserved, limit, e := a.ownerUsage(tx, c.OwnerID)
	if e != nil {
		return 0, e
	}
	var owner Account
	if e := tx.First(&owner, c.OwnerID).Error; e != nil {
		return 0, e
	}
	if owner.Role == "merchant" && used+reserved+size > limit {
		return 0, errors.New("storage_quota_exceeded")
	}
	value := tx.Statement.Context.Value(watermarkEffectsKey{})
	if value == nil {
		return 0, errors.New("watermark_transaction_required")
	}
	pending := value.(*[]watermarkObject)
	storeID := a.defaultStoreID()
	store, e := a.store(storeID)
	if e != nil {
		return 0, e
	}
	for i, key := range []string{variant.Path, variant.ThumbPath} {
		sha, n, e := fileSHA(paths[i])
		if e != nil {
			return 0, e
		}
		file, e := os.Open(paths[i])
		if e != nil {
			return 0, e
		}
		*pending = append(*pending, watermarkObject{key, store})
		e = store.Put(ctx, key, file, n)
		file.Close()
		if e != nil {
			return 0, errors.New("storage_unavailable")
		}
		if e = tx.Create(&StoredObject{Key: key, StoreID: storeID, Size: n, SHA256: sha, UpdatedAt: time.Now().UTC()}).Error; e != nil {
			return 0, e
		}
		if e = tx.Create(&MediaObjectOwner{ObjectKey: key, OwnerID: c.OwnerID, Size: n}).Error; e != nil {
			return 0, e
		}
	}
	if e := tx.Create(&variant).Error; e != nil {
		return 0, e
	}
	if e := tx.Create(&MediaWatermark{MediaID: variant.ID, SourceMediaID: source.ID, VariantKey: &variantKey, Purpose: purpose, State: "ready", ConfigJSON: info.ConfigJSON, ConfigVersion: info.ConfigVersion, VendorID: info.VendorID, VendorText: info.VendorText, CreatedAt: time.Now().UTC()}).Error; e != nil {
		return 0, e
	}
	return variant.ID, nil
}

// A guessed source ID or query parameter must never reveal a clean version of a
// new public gallery image. Excluded slots may explicitly publish clean media.
func (a *App) watermarkSourceIsPublicDisplay(id uint64) (bool, error) {
	var mark MediaWatermark
	e := a.DB.First(&mark, "media_id=?", id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if mark.SourceMediaID > 0 {
		return false, nil
	}
	var cfg WatermarkConfig
	if e = json.Unmarshal([]byte(mark.ConfigJSON), &cfg); e != nil {
		return false, e
	}
	if !cfg.Enabled {
		return false, nil
	}
	var revisions []Revision
	e = a.DB.Table("media_references mr").Joins("JOIN contents c ON c.published_revision_id=mr.revision_id JOIN revisions r ON r.id=mr.revision_id").Where("mr.media_id=? AND c.visible=1 AND r.status='approved'", id).Select("r.*").Find(&revisions).Error
	if e != nil {
		return false, e
	}
	for _, r := range revisions {
		var p Payload
		if e = json.Unmarshal([]byte(r.Payload), &p); e != nil {
			return false, e
		}
		for _, mid := range p.MediaIDs {
			if mid == id {
				return true, nil
			}
		}
		if p.CompanyProfile != nil && p.CompanyProfile.CoverMediaID == id {
			return true, nil
		}
	}
	return false, nil
}
func (a *App) publicWatermarkAlias(source uint64) (Media, error) {
	var result Media
	e := a.DB.Raw("SELECT m.* FROM media_watermarks mw JOIN media m ON m.id=mw.media_id JOIN media_references mr ON mr.media_id=m.id JOIN contents c ON c.published_revision_id=mr.revision_id JOIN revisions r ON r.id=mr.revision_id AND r.status='approved' JOIN halls h ON h.id=c.hall_id AND h.status='open' LEFT JOIN contents v ON v.id=c.vendor_id LEFT JOIN revisions vr ON vr.id=v.published_revision_id WHERE mw.source_media_id=? AND mw.state='ready' AND c.visible=1 AND (c.kind='vendor' OR (v.visible=1 AND vr.status='approved')) ORDER BY m.id LIMIT 1", source).Scan(&result).Error
	return result, e
}
