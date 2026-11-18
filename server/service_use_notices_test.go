package main

import (
	"strings"
	"testing"
	"time"
)

func TestAIAnalyticsNoticeCorrectionPreservesHistoricalVersions(t *testing.T) {
	a := importTestApp(t)
	docs, err := activeServiceDocuments(a.DB)
	if err != nil {
		t.Fatal(err)
	}
	var old ServiceDocument
	for _, doc := range docs {
		if doc.DocKey == "privacy" {
			old = doc
		}
	}
	old.ID = 0
	old.Version++
	old.Body = "访问统计默认关闭，只有主动开启后才创建随机访问标识。\n【2026-11 厂商后台操作说明】\n未保存选择时可选访问统计默认开启。"
	now := time.Now().UTC().Truncate(time.Millisecond)
	old.CreatedAt, old.EffectiveAt, old.PublishedAt = now, now, &now
	old.Digest = documentDigest(old)
	if err = a.DB.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	var before int64
	a.DB.Model(&ServiceAcceptance{}).Count(&before)
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=37")
	if err = migrateThrough(a.DB, 37); err != nil {
		t.Fatal(err)
	}
	current, err := activeServiceDocuments(a.DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range current {
		if doc.DocKey == "privacy" && (doc.Version != old.Version+1 || strings.Contains(doc.Body, "统计默认关闭") || !strings.Contains(doc.Body, "已明确关闭的选择保持关闭")) {
			t.Fatal("incorrect active privacy document", doc.Version)
		}
	}
	var original ServiceDocument
	a.DB.First(&original, old.ID)
	if original.Body != old.Body || original.Digest != old.Digest {
		t.Fatal("historical policy changed")
	}
	var count, after int64
	a.DB.Model(&ServiceDocument{}).Count(&count)
	if err = migrateAnalyticsNoticeCorrection(a.DB); err != nil {
		t.Fatal(err)
	}
	a.DB.Model(&ServiceDocument{}).Count(&after)
	if after != count {
		t.Fatal("correction duplicated a policy version")
	}
	a.DB.Model(&ServiceAcceptance{}).Count(&after)
	if after != before {
		t.Fatal("correction fabricated acceptance")
	}
}
