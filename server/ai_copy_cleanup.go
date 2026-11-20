package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"time"
)

var aiCopyProcessPrefix = regexp.MustCompile(`(?m)^(?:根据|据)(?:本|原|提供的)?(?:产品)?(?:画册|彩页|PDF)(?:介绍|显示|所示|资料)?[，,:： ]+`)

func aiCleanCopy(p Payload) Payload {
	p = standardAIProductPayload(p)
	p.Description = aiCopyProcessPrefix.ReplaceAllString(p.Description, "")
	return p
}

// Dry-run by default. Shared draft service preserves the published revision and
// all media references. Existing pending or edited drafts are never overwritten.
func (a *App) aiCleanupCopies(apply bool) ([]gin.H, error) {
	var admin Account
	if e := a.DB.Where("role='admin'").Order("id").First(&admin).Error; e != nil {
		return nil, e
	}
	var contents []Content
	if e := a.DB.Where("kind='product' AND published_revision_id>0 AND id IN (SELECT content_id FROM ai_import_product_drafts WHERE content_id>0)").Order("owner_id,id").Find(&contents).Error; e != nil {
		return nil, e
	}
	results := []gin.H{}
	for _, item := range contents {
		result := gin.H{"content_id": item.ID, "owner_id": item.OwnerID, "status": "unchanged"}
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			var c Content
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, item.ID).Error; e != nil {
				return e
			}
			if c.DraftRevisionID != c.PublishedRevisionID {
				result["status"] = "existing_draft_kept"
				return nil
			}
			var rev Revision
			if e := tx.First(&rev, c.PublishedRevisionID).Error; e != nil {
				return e
			}
			if rev.Status != "approved" {
				result["status"] = "not_approved"
				return nil
			}
			var before Payload
			if e := json.Unmarshal([]byte(rev.Payload), &before); e != nil {
				return e
			}
			after := aiCleanCopy(before)
			if aiJSON(before) == aiJSON(after) {
				return nil
			}
			result["status"] = "suggested"
			result["base_revision_id"] = rev.ID
			result["changed_fields"] = []string{"description", "parameters", "product_details"}
			if !apply {
				return nil
			}
			if e := a.saveOwnedDraft(tx, &c, admin, c.ID, rev.ID, &after); e != nil {
				return e
			}
			result["status"] = "draft_created"
			result["draft_revision_id"] = c.DraftRevisionID
			return recordOperation(tx, admin.ID, "product", c.ID, "ai_copy_cleanup_draft", "source_revision="+aiJSON(rev.ID)+";private_draft;no_publish;"+time.Now().UTC().Format(time.RFC3339))
		})
		if e != nil {
			return results, e
		}
		results = append(results, result)
	}
	return results, nil
}
