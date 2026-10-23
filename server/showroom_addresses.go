package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mozillazg/go-pinyin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ShowroomAddress struct {
	Slug      string `gorm:"primaryKey"`
	VendorID  uint64
	CreatedAt time.Time
}

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}[a-z0-9]$`)
var slugReserved = map[string]bool{"admin": true, "api": true, "merchant": true, "login": true, "register": true, "public": true, "products": true, "vendors": true, "browse": true, "join": true, "app": true, "support": true, "help": true, "zqzg": true, "zqzt": true, "www": true, "system": true, "assets": true, "downloads": true}

func normalizedSlug(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func validSlug(s string) bool {
	return slugPattern.MatchString(s) && !strings.Contains(s, "--") && !slugReserved[s] && !regexp.MustCompile(`^v-[0-9]+$`).MatchString(s)
}
func slugTaken(db *gorm.DB, s string) (bool, error) {
	var n int64
	if e := db.Model(&ShowroomAddress{}).Where("slug=?", s).Count(&n).Error; e != nil {
		return false, e
	}
	if n > 0 {
		return true, nil
	}
	e := db.Model(&Content{}).Where("showroom_slug=?", s).Count(&n).Error
	return n > 0, e
}
func suggestedSlug(p Payload, id uint64) []string {
	name := p.NameEN
	if p.CompanyProfile != nil && p.CompanyProfile.ShortNameEN != "" {
		name = p.CompanyProfile.ShortNameEN
	}
	if strings.TrimSpace(name) == "" {
		name = strings.Join(pinyin.LazyPinyin(p.Name, pinyin.NewArgs()), "-")
	}
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-")
	name = strings.Trim(name, "-")
	if len(name) > 48 {
		name = strings.TrimRight(name[:48], "-")
	}
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "company-" + name
	}
	if len(name) < 2 {
		name += "-company"
	}
	return []string{name, fmt.Sprintf("%s-%d", name, id)}
}
func (a *App) merchantLink(c *gin.Context) { a.linkInfo(c, false) }
func (a *App) adminLink(c *gin.Context)    { a.linkInfo(c, true) }
func (a *App) linkInfo(c *gin.Context, admin bool) {
	if !permit(fmt.Sprintf("link-check:%d", actor(c).ID), 60, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var v Content
	if e := func() error {
		q := a.DB.Where("kind='vendor'")
		if admin {
			q = q.Where("id=?", id(c))
		} else {
			q = q.Where("owner_id=?", actor(c).ID)
		}
		return q.First(&v).Error
	}(); e != nil {
		dbFail(c, e)
		return
	}
	var rev Revision
	if e := a.DB.First(&rev, v.DraftRevisionID).Error; e != nil {
		dbFail(c, e)
		return
	}
	var p Payload
	json.Unmarshal([]byte(rev.Payload), &p)
	suggestions := []string{}
	for _, s := range suggestedSlug(p, v.ID) {
		taken, e := slugTaken(a.DB, s)
		if e != nil {
			dbFail(c, e)
			return
		}
		if validSlug(s) && !taken {
			suggestions = append(suggestions, s)
		}
	}
	result := gin.H{"showroom_slug": showroomSlug(v), "locked": v.ShowroomSlug != nil, "suggestions": suggestions, "base_url": a.publicBase(c)}
	if raw, exists := c.GetQuery("slug"); exists {
		s := normalizedSlug(raw)
		result["slug"] = s
		result["valid"] = validSlug(s)
		taken, e := slugTaken(a.DB, s)
		if e != nil {
			dbFail(c, e)
			return
		}
		result["available"] = validSlug(s) && !taken
	}
	c.JSON(200, result)
}
func (a *App) saveShowroomAddress(c *gin.Context, admin bool) {
	var req struct {
		Slug   string `json:"slug"`
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Slug = normalizedSlug(req.Slug)
	req.Reason = strings.TrimSpace(req.Reason)
	if !validSlug(req.Slug) {
		fail(c, 400, "invalid_showroom_slug")
		return
	}
	if !permit(fmt.Sprintf("link-save:%d", actor(c).ID), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var v Content
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind='vendor'")
		if admin {
			q = q.Where("id=?", id(c))
		} else {
			q = q.Where("owner_id=?", actor(c).ID)
		}
		if e := q.First(&v).Error; e != nil {
			return e
		}
		if v.ShowroomSlug != nil {
			if *v.ShowroomSlug == req.Slug {
				return nil
			}
			if !admin {
				return fmt.Errorf("showroom_slug_locked")
			}
			if req.Reason == "" || len([]rune(req.Reason)) > 500 {
				return fmt.Errorf("showroom_slug_locked")
			}
		}
		var registered ShowroomAddress
		e := tx.Where("slug=?", req.Slug).First(&registered).Error
		if e == nil {
			if registered.VendorID != v.ID {
				return fmt.Errorf("showroom_slug_exists")
			}
		} else if e != gorm.ErrRecordNotFound {
			return e
		} else {
			if e = tx.Create(&ShowroomAddress{Slug: req.Slug, VendorID: v.ID, CreatedAt: time.Now().UTC()}).Error; e != nil {
				if strings.Contains(e.Error(), "Duplicate entry") {
					return fmt.Errorf("showroom_slug_exists")
				}
				return e
			}
		}
		old := showroomSlug(v)
		if e = tx.Model(&v).Update("showroom_slug", req.Slug).Error; e != nil {
			if strings.Contains(e.Error(), "Duplicate entry") {
				return fmt.Errorf("showroom_slug_exists")
			}
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor", v.ID, "showroom_slug_set", old+" -> "+req.Slug+"; "+req.Reason)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"showroom_slug": req.Slug, "canonical_url": a.publicBase(c) + "/v/" + req.Slug, "locked": true})
}
func (a *App) merchantSaveLink(c *gin.Context) { a.saveShowroomAddress(c, false) }
func (a *App) redirectOldShowroom(c *gin.Context, item View) bool {
	path := c.Request.URL.Path
	target := "/v/" + url.PathEscape(item.ShowroomSlug)
	if (strings.HasPrefix(path, "/v/") || strings.HasPrefix(path, "/vendors/")) && path != target {
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(301, target)
		return true
	}
	return false
}
