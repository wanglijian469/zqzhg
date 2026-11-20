package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestAIBatchMigrationPreservesLegacySubmissionsAndConsent(t *testing.T) {
	a := importTestAppAtSchema(t, 34)
	oldDocs, e := activeServiceDocuments(a.DB)
	if e != nil {
		t.Fatal(e)
	}
	oldVersions := map[string]uint64{}
	for _, doc := range oldDocs {
		oldVersions[doc.DocKey] = doc.Version
	}
	old := AIImportTask{OwnerID: 1, VendorID: 7, RequestID: "legacy-source", FileHash: strings.Repeat("a", 64), FileName: "合成旧任务.pdf", SourceIDs: "[]", Status: "review_pending", Stage: "review_pending", Total: 1, Checkpoint: "{}", PublishKey: "legacy-key", PublishManifest: "{}", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e := a.DB.Omit("SubmissionMode", "PromptVersionID", "RecognitionOutcome", "ResultVersion", "NextRetryAt").Create(&old).Error; e != nil {
		t.Fatal(e)
	}
	d := AIImportDraft{TaskID: old.ID, OwnerID: 1, TempID: "legacy-product", Version: 4, ReviewedVersion: 4, ContentID: 19, RevisionID: 28, Data: "{}", SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	if e := a.DB.Table(aiTable("product")).Omit("ActiveBatchID", "GenerationState").Create(&d).Error; e != nil {
		t.Fatal(e)
	}
	old.PublishManifest = aiJSON(map[string]any{"company_version": 1, "products": []aiPublishSelection{{ID: d.ID, Version: d.Version}}})
	a.DB.Model(&old).Update("publish_manifest", old.PublishManifest)
	before := int64(0)
	a.DB.Model(&ServiceAcceptance{}).Count(&before)
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var batch AISubmissionBatch
	if e := a.DB.Where("task_id=?", old.ID).First(&batch).Error; e != nil {
		t.Fatal(e)
	}
	var item AISubmissionItem
	a.DB.Where("batch_id=?", batch.ID).First(&item)
	if batch.Kind != "legacy" || item.Status != "submitted" || item.ContentID != 19 || item.RevisionID != 28 || item.Version != 4 {
		t.Fatal(batch, item)
	}
	var retained AIImportDraft
	a.DB.Table(aiTable("product")).First(&retained, d.ID)
	if retained.Version != 4 || retained.ReviewedVersion != 4 || retained.ContentID != 19 {
		t.Fatal("legacy metadata changed", retained)
	}
	after := int64(0)
	a.DB.Model(&ServiceAcceptance{}).Count(&after)
	if after != before {
		t.Fatal("migration fabricated agreement acceptance")
	}
	docs, e := activeServiceDocuments(a.DB)
	if e != nil {
		t.Fatal(e)
	}
	for _, doc := range docs {
		if doc.Version != oldVersions[doc.DocKey]+1 || !strings.Contains(doc.Body, "【2026-11 厂商后台操作说明】") {
			t.Fatal("policy not versioned", doc.DocKey)
		}
	}
	for _, name := range []string{"../deploy/linux/rollback-service-use-036.sql", "../deploy/linux/rollback-ai-import-035.sql"} {
		b, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		for _, sql := range strings.Split(string(b), ";") {
			if strings.TrimSpace(sql) != "" {
				if e = a.DB.Exec(sql).Error; e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	if a.DB.Migrator().HasTable(&AISubmissionBatch{}) {
		t.Fatal("batch downgrade failed")
	}
	if !a.DB.Migrator().HasTable("ai_import_product_drafts") {
		t.Fatal("downgrade removed drafts")
	}
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	a.DB.Table(aiTable("product")).First(&retained, d.ID)
	if retained.ContentID != 19 {
		t.Fatal("downgrade lost final business id")
	}
}
