package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"sync"
)

//go:embed catalog/administrative-regions-2023.json
var administrativeJSON []byte

type AdministrativeRegion struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	ParentCode  string   `json:"parent_code"`
	Level       int      `json:"level"`
	HasChildren bool     `json:"has_children"`
	Path        []string `json:"path"`
	FullName    string   `json:"full_name"`
}
type administrativeRaw struct {
	Code     string              `json:"code"`
	Name     string              `json:"name"`
	Children []administrativeRaw `json:"children"`
}

var administrativeOnce sync.Once
var administrativeByCode map[string]AdministrativeRegion
var administrativeChildren map[string][]AdministrativeRegion
var administrativeByName map[string]string

func normalizedRegionName(s string) string { return strings.Join(strings.Fields(s), "") }
func loadAdministrativeRegions() {
	administrativeOnce.Do(func() {
		administrativeByCode = map[string]AdministrativeRegion{}
		administrativeChildren = map[string][]AdministrativeRegion{}
		administrativeByName = map[string]string{}
		var provinces []administrativeRaw
		if e := json.Unmarshal(administrativeJSON, &provinces); e != nil {
			panic(e)
		}
		var add func(administrativeRaw, string, int, []string)
		add = func(raw administrativeRaw, parent string, level int, ancestors []string) {
			names := append(append([]string{}, ancestors...), raw.Name)
			display := []string{}
			for _, n := range names {
				if len(display) == 0 || display[len(display)-1] != n {
					display = append(display, n)
				}
			}
			node := AdministrativeRegion{Code: raw.Code, Name: raw.Name, ParentCode: parent, Level: level, HasChildren: len(raw.Children) > 0, Path: append([]string{}, names...), FullName: strings.Join(display, " ")}
			administrativeByCode[node.Code] = node
			administrativeChildren[parent] = append(administrativeChildren[parent], node)
			if level >= 2 {
				administrativeByName[normalizedRegionName(node.FullName)] = node.Code
			}
			for _, child := range raw.Children {
				add(child, raw.Code, level+1, names)
			}
		}
		for _, p := range provinces {
			cities := []administrativeRaw{}
			for _, city := range p.Children {
				if strings.Contains(city.Name, "直辖县级行政区划") {
					cities = append(cities, city.Children...)
				} else {
					if city.Name == "市辖区" || city.Name == "县" {
						city.Name = p.Name
					}
					cities = append(cities, city)
				}
			}
			p.Children = cities
			add(p, "", 1, nil)
		}
	})
}
func (a *App) administrativeRegions(c *gin.Context) {
	loadAdministrativeRegions()
	parent := c.Query("parent_code")
	if parent != "" {
		if _, ok := administrativeByCode[parent]; !ok {
			fail(c, 400, "invalid_region")
			return
		}
	}
	items := administrativeChildren[parent]
	if items == nil {
		items = []AdministrativeRegion{}
	}
	code := c.Query("code")
	if code == "" && c.Query("name") != "" {
		code = administrativeByName[normalizedRegionName(c.Query("name"))]
	}
	var selected *AdministrativeRegion
	if code != "" {
		n, ok := administrativeByCode[code]
		if !ok {
			fail(c, 400, "invalid_region")
			return
		}
		selected = &n
	}
	trail := []AdministrativeRegion{}
	if selected != nil {
		n := *selected
		for {
			trail = append([]AdministrativeRegion{n}, trail...)
			if n.ParentCode == "" {
				break
			}
			n = administrativeByCode[n.ParentCode]
		}
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.JSON(200, gin.H{"items": items, "selected": selected, "trail": trail, "version": "2023-06-30", "source": "modood/Administrative-divisions-of-China", "historical": true})
}
func resolveAdministrativeRegion(tx *gorm.DB, p *Payload) error {
	loadAdministrativeRegions()
	node, ok := administrativeByCode[p.RegionCode]
	if !ok || node.Level < 2 {
		return fieldFailure("invalid_region", "region_code", "invalid_region")
	}
	var gate Hall
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&gate, 1).Error; e != nil {
		return e
	}
	var region Region
	find := func() error {
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("administrative_code=?", node.Code).First(&region).Error
	}
	if e := find(); e != nil {
		if e != gorm.ErrRecordNotFound {
			return e
		}
		// Reuse an exact legacy name without renumbering existing references.
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name=? AND administrative_code IS NULL", node.FullName).Order("id").First(&region).Error
		if e == nil {
			e = tx.Model(&region).Update("administrative_code", node.Code).Error
		} else if e == gorm.ErrRecordNotFound {
			code := node.Code
			region = Region{Name: node.FullName, Enabled: true, AdministrativeCode: &code}
			e = tx.Create(&region).Error
		}
		if e != nil {
			if again := find(); again != nil {
				return fmt.Errorf("region resolution: %w", e)
			}
		}
	}
	if !region.Enabled {
		return fieldFailure("invalid_region", "region_code", "invalid_region")
	}
	p.RegionID = region.ID
	p.RegionName = region.Name
	return nil
}

func resolveTextRegion(tx *gorm.DB, p *Payload) error {
	if p.RegionID != 0 || strings.TrimSpace(p.RegionName) == "" {
		return nil
	}
	var gate Hall
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&gate, 1).Error; e != nil {
		return e
	}
	region := Region{Name: strings.TrimSpace(p.RegionName), Enabled: true}
	if e := tx.Where("name=?", region.Name).Order("id").FirstOrCreate(&region).Error; e != nil {
		return e
	}
	if !region.Enabled {
		return fieldFailure("invalid_region", "region_name", "invalid_region")
	}
	p.RegionID = region.ID
	p.RegionName = region.Name
	return nil
}
