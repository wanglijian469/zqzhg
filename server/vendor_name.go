package main

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
	"gorm.io/gorm"
)

// Keep the entered display name, but compare insignificant spacing, case and
// full-width ASCII consistently. Do not remove legal suffixes or branch names.
func vendorNameKey(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff' {
			return -1
		}
		return unicode.ToLower(r)
	}, norm.NFC.String(width.Narrow.String(name)))
}

// All production content writers acquire the existing primary hall lock before
// their first read. This serializes the name check and write across processes,
// and avoids a stale REPEATABLE READ snapshot after waiting for another writer.
// Reuse the existing taxonomy lock order; no schema changes or data backfill.
func (a *App) contentTransaction(fn func(*gorm.DB) error) error {
	return a.watermarkTransaction(func(tx *gorm.DB) error {
		if err := lockTaxonomy(tx, 1); err != nil {
			return err
		}
		return fn(tx)
	})
}

func checkVendorName(tx *gorm.DB, content Content, name string) error {
	if content.Kind != "vendor" {
		return nil
	}
	key := vendorNameKey(name)
	if key == "" {
		return fieldFailure("invalid_request", "name", "required")
	}
	var rows []struct {
		ContentID uint64
		Name      string
	}
	// Only the current draft and public version reserve a name. Older revision
	// history must not prevent a legitimate future rename. Offline booths still
	// reserve their names, so creating another booth cannot evade this check.
	if err := tx.Table("contents c").Select("c.id AS content_id, r.name").
		Joins("JOIN revisions r ON r.id=c.draft_revision_id OR r.id=c.published_revision_id").
		Where("c.kind='vendor'").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.ContentID == content.ID && vendorNameKey(row.Name) == key {
			// Preserve the ability to edit legacy duplicates without creating
			// any new claim on the name or rewriting existing public records.
			return nil
		}
	}
	for _, row := range rows {
		if row.ContentID != content.ID && vendorNameKey(row.Name) == key {
			return fieldFailure("vendor_name_exists", "name", "vendor_name_exists")
		}
	}
	return nil
}
