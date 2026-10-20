package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"sync"
)

// These columns belong to a revision, never the vendor's mutable draft or region row.
type AdministrativeSnapshot struct {
	ProvinceCode *string `json:"-"`
	CityCode     *string `json:"-"`
	DistrictCode *string `json:"-"`
}

var snapshotNamesOnce sync.Once
var snapshotNames map[string]string

func snapshotCodeByName(name string) string {
	loadAdministrativeRegions()
	snapshotNamesOnce.Do(func() {
		snapshotNames = map[string]string{}
		for code, node := range administrativeByCode {
			key := normalizedRegionName(node.FullName)
			if previous, exists := snapshotNames[key]; exists && previous != code {
				snapshotNames[key] = "" // Ambiguous exact names must remain unresolved.
			} else if !exists {
				snapshotNames[key] = code
			}
		}
	})
	return snapshotNames[normalizedRegionName(name)]
}

func administrativeSnapshot(p Payload, legacy Region) AdministrativeSnapshot {
	loadAdministrativeRegions()
	code := p.RegionCode
	if _, ok := administrativeByCode[code]; !ok {
		code = ""
		if legacy.AdministrativeCode != nil {
			node, valid := administrativeByCode[*legacy.AdministrativeCode]
			if valid && normalizedRegionName(node.FullName) == normalizedRegionName(p.RegionName) {
				code = node.Code
			}
		}
		if code == "" {
			code = snapshotCodeByName(p.RegionName)
		}
	}
	result := AdministrativeSnapshot{}
	for code != "" {
		node, ok := administrativeByCode[code]
		if !ok {
			break
		}
		value := node.Code
		switch node.Level {
		case 1:
			result.ProvinceCode = &value
		case 2:
			result.CityCode = &value
		case 3:
			result.DistrictCode = &value
		}
		code = node.ParentCode
	}
	return result
}

// Bounded keyset batches; safe to retry after partially completed DDL/backfill.
// The version is recorded only after every revision has been processed.
func backfillAdministrativeSnapshots(db *gorm.DB) error {
	var regions []Region
	if e := db.Find(&regions).Error; e != nil {
		return e
	}
	byID := map[uint64]Region{}
	for _, region := range regions {
		byID[region.ID] = region
	}
	var last uint64
	for {
		var rows []Revision
		if e := db.Model(&Revision{}).Select("revisions.*").Joins("JOIN contents c ON c.id=revisions.content_id AND c.kind='vendor'").Where("revisions.id > ?", last).Order("revisions.id").Limit(500).Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) == 0 {
			return nil
		}
		if e := db.Transaction(func(tx *gorm.DB) error {
			for _, row := range rows {
				var p Payload
				if e := json.Unmarshal([]byte(row.Payload), &p); e != nil {
					return fmt.Errorf("revision %d administrative backfill: %w", row.ID, e)
				}
				s := administrativeSnapshot(p, byID[row.RegionID])
				if e := tx.Model(&Revision{}).Where("id=?", row.ID).UpdateColumns(map[string]any{"province_code": s.ProvinceCode, "city_code": s.CityCode, "district_code": s.DistrictCode}).Error; e != nil {
					return e
				}
			}
			return nil
		}); e != nil {
			return e
		}
		last = rows[len(rows)-1].ID
	}
}
