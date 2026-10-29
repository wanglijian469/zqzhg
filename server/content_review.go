package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"time"
)

const companyVerificationMode = "content_review"

func verificationMerged(c *gin.Context) { fail(c, 410, "verification_merged") }

// The legacy read shape stays usable for document confirmation, never identity data.
func (a *App) companyAgreementState(c *gin.Context) {
	docs, err := activeServiceDocuments(a.DB)
	if err != nil {
		dbFail(c, err)
		return
	}
	accepted := []ServiceAcceptance{}
	if err = a.DB.Where("account_id=?", actor(c).ID).Find(&accepted).Error; err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(200, gin.H{"company_verification_mode": companyVerificationMode,
		"verification": gin.H{"status": "merged", "account_id": actor(c).ID},
		"documents":    docs, "acceptances": accepted, "next": "/merchant/booth/edit?step=2"})
}

// Publish additive versions once, preserving every prior document and acceptance.
func migrateContentReview(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := serviceLock(tx, true); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&SystemSetting{}).Where("setting_key=?", "content_review_documents_v1").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		raw, err := serviceDocumentFiles.ReadFile("service-documents.json")
		if err != nil {
			return err
		}
		var docs []ServiceDocument
		if err = json.Unmarshal(raw, &docs); err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Millisecond)
		for _, d := range docs {
			var version uint64
			if err = tx.Model(&ServiceDocument{}).Select("COALESCE(MAX(version),0)").Where("doc_key=?", d.DocKey).Scan(&version).Error; err != nil {
				return err
			}
			d.ID = 0
			d.Version = version + 1
			d.Status = "published"
			d.EffectiveAt = now
			d.CreatedAt = now
			d.PublishedAt = &now
			d.Digest = documentDigest(d)
			if err = tx.Create(&d).Error; err != nil {
				return err
			}
		}
		return tx.Create(&SystemSetting{SettingKey: "content_review_documents_v1", Value: "published", UpdatedAt: now}).Error
	})
}

// Add a corrected immutable notice only if an earlier deployed notice differs.
// Fresh databases already receive the same complete text in migration 26.
func migrateContentReviewPrivacy(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := serviceLock(tx, true); err != nil {
			return err
		}
		raw, err := serviceDocumentFiles.ReadFile("service-documents.json")
		if err != nil {
			return err
		}
		var docs []ServiceDocument
		if err = json.Unmarshal(raw, &docs); err != nil {
			return err
		}
		for _, d := range docs {
			if d.DocKey != "privacy" {
				continue
			}
			var previous ServiceDocument
			if err = tx.Where("doc_key=? AND status='published'", "privacy").Order("version DESC").First(&previous).Error; err != nil {
				return err
			}
			if previous.Body == d.Body && previous.BodyEN == d.BodyEN {
				return nil
			}
			var version uint64
			if err = tx.Model(&ServiceDocument{}).Select("COALESCE(MAX(version),0)").Where("doc_key=?", "privacy").Scan(&version).Error; err != nil {
				return err
			}
			now := time.Now().UTC().Truncate(time.Millisecond)
			d.ID = 0
			d.Version = version + 1
			d.Status = "published"
			d.CreatedAt = now
			d.EffectiveAt = now
			d.PublishedAt = &now
			d.Digest = documentDigest(d)
			return tx.Create(&d).Error
		}
		return nil
	})
}
