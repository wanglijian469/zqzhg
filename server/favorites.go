package main

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Favorite struct {
	AccountID   uint64    `gorm:"primaryKey" json:"-"`
	Kind        string    `gorm:"primaryKey" json:"kind"`
	ContentID   uint64    `gorm:"primaryKey" json:"content_id"`
	Active      bool      `json:"-"`
	CollectedAt time.Time `json:"collected_at"`
	UpdatedAt   time.Time `json:"-"`
}
type FavoriteOperation struct {
	AccountID   uint64 `gorm:"primaryKey"`
	OperationID string `gorm:"primaryKey"`
	Fingerprint string
	CreatedAt   time.Time
}
type FavoriteTarget struct {
	Kind      string `json:"kind"`
	ContentID uint64 `json:"content_id"`
}
type FavoriteMutation struct {
	FavoriteTarget
	OperationID string `json:"operation_id"`
	Favorited   *bool  `json:"favorited"`
}

var favoriteOperationID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func validFavorite(t FavoriteTarget) bool {
	return t.ContentID > 0 && (t.Kind == "vendor" || t.Kind == "product")
}
func favoriteKey(kind string, id uint64) string { return fmt.Sprintf("%s:%d", kind, id) }
func favoriteActor(c *gin.Context) bool {
	role := actor(c).Role
	if role != "visitor" && role != "merchant" {
		fail(c, 403, "forbidden")
		return false
	}
	return true
}

// Resolve only public snapshots. Unknown, private and withdrawn IDs share one
// unavailable shape; no private content is loaded to fill a placeholder.
func (a *App) favoriteViews(targets []FavoriteTarget) ([]gin.H, error) {
	found := map[string]View{}
	for _, kind := range []string{"vendor", "product"} {
		ids := []uint64{}
		for _, t := range targets {
			if t.Kind == kind {
				ids = append(ids, t.ContentID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		views, e := a.publicViews(a.publicQuery(kind).Where("contents.id IN ?", ids))
		if e != nil {
			return nil, e
		}
		if kind == "product" {
			if e = a.productVendorSummaries(views); e != nil {
				return nil, e
			}
		}
		for _, v := range views {
			found[favoriteKey(kind, v.ID)] = v
		}
	}
	result := make([]gin.H, 0, len(targets))
	for _, t := range targets {
		row := gin.H{"kind": t.Kind, "content_id": t.ContentID, "available": false}
		if v, ok := found[favoriteKey(t.Kind, t.ContentID)]; ok {
			row["available"] = true
			row["item"] = v
		}
		result = append(result, row)
	}
	return result, nil
}
func (a *App) resolveFavorites(c *gin.Context) {
	var req struct {
		Items []FavoriteTarget `json:"items"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Items) > 100 {
		fail(c, 400, "invalid_request")
		return
	}
	for _, t := range req.Items {
		if !validFavorite(t) {
			fail(c, 400, "invalid_request")
			return
		}
	}
	items, e := a.favoriteViews(req.Items)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (a *App) listFavorites(c *gin.Context) {
	if !favoriteActor(c) {
		return
	}
	kind := c.Query("kind")
	if kind != "" && kind != "vendor" && kind != "product" {
		fail(c, 400, "invalid_request")
		return
	}
	owner := actor(c).ID
	p, n := page(c)
	q := a.DB.Model(&Favorite{}).Where("account_id=? AND active=1", owner)
	if kind != "" {
		q = q.Where("kind=?", kind)
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	var saved []Favorite
	if e := q.Order("collected_at DESC, content_id DESC, kind ASC").Offset((p - 1) * n).Limit(n).Find(&saved).Error; e != nil {
		dbFail(c, e)
		return
	}
	targets := make([]FavoriteTarget, len(saved))
	for i, f := range saved {
		targets[i] = FavoriteTarget{f.Kind, f.ContentID}
	}
	items, e := a.favoriteViews(targets)
	if e != nil {
		dbFail(c, e)
		return
	}
	for i := range items {
		items[i]["collected_at"] = saved[i].CollectedAt
	}
	var counts []struct {
		Kind  string
		Total int64
	}
	if e = a.DB.Model(&Favorite{}).Where("account_id=? AND active=1", owner).Select("kind,COUNT(*) AS total").Group("kind").Scan(&counts).Error; e != nil {
		dbFail(c, e)
		return
	}
	byKind := gin.H{"vendor": int64(0), "product": int64(0)}
	for _, row := range counts {
		byKind[row.Kind] = row.Total
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n, "counts": byKind})
}
func (a *App) syncFavorites(c *gin.Context) {
	if !favoriteActor(c) {
		return
	}
	var req struct {
		Operations []FavoriteMutation `json:"operations"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Operations) > 100 {
		fail(c, 400, "invalid_request")
		return
	}
	for _, op := range req.Operations {
		if !validFavorite(op.FavoriteTarget) || !favoriteOperationID.MatchString(op.OperationID) || op.Favorited == nil {
			fail(c, 400, "invalid_request")
			return
		}
	}
	owner := actor(c).ID
	ack := make([]string, 0, len(req.Operations))
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		// One account lock orders concurrent devices and serializes the dedup check.
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, owner).Error; e != nil {
			return e
		}
		for _, op := range req.Operations {
			fingerprint := digest(fmt.Sprintf("%s:%d:%t", op.Kind, op.ContentID, *op.Favorited))
			var old FavoriteOperation
			e := tx.First(&old, "account_id=? AND operation_id=?", owner, op.OperationID).Error
			if e == nil {
				if old.Fingerprint != fingerprint {
					return fmt.Errorf("operation_conflict")
				}
				ack = append(ack, op.OperationID)
				continue
			}
			if !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			now := time.Now().UTC()
			var f Favorite
			e = tx.First(&f, "account_id=? AND kind=? AND content_id=?", owner, op.Kind, op.ContentID).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				f = Favorite{AccountID: owner, Kind: op.Kind, ContentID: op.ContentID, Active: *op.Favorited, CollectedAt: now, UpdatedAt: now}
				if e = tx.Create(&f).Error; e != nil {
					return e
				}
			} else if e != nil {
				return e
			} else {
				if *op.Favorited && !f.Active {
					f.CollectedAt = now
				}
				f.Active = *op.Favorited
				f.UpdatedAt = now
				if e = tx.Save(&f).Error; e != nil {
					return e
				}
			}
			if e = tx.Create(&FavoriteOperation{AccountID: owner, OperationID: op.OperationID, Fingerprint: fingerprint, CreatedAt: now}).Error; e != nil {
				return e
			}
			ack = append(ack, op.OperationID)
		}
		return nil
	})
	if e != nil {
		if e.Error() == "operation_conflict" {
			fail(c, 409, "operation_conflict")
		} else {
			dbFail(c, e)
		}
		return
	}
	c.JSON(200, gin.H{"acknowledged": ack})
}
