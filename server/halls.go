package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) publicHalls(c *gin.Context) {
	items, e := a.hallConfigurations()
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"items": items})
}
func (a *App) requestHall(c *gin.Context) (Hall, bool) {
	slug := c.DefaultQuery("hall", "agriculture")
	var hall Hall
	if e := a.DB.Where("slug=?", slug).First(&hall).Error; e != nil {
		dbFail(c, e)
		return hall, false
	}
	return hall, true
}
func (a *App) hallQuery(c *gin.Context, q *gorm.DB) (*gorm.DB, bool) {
	hall, ok := a.requestHall(c)
	if !ok {
		return q, false
	}
	q = q.Where("contents.hall_id=?", hall.ID)
	if hall.Status != "open" {
		q = q.Where("1=0")
	}
	return q, true
}
func validateContentHall(tx *gorm.DB, content *Content, p Payload) error {
	if content.Kind == "product" {
		var vendor Content
		if e := tx.First(&vendor, content.VendorID).Error; e != nil {
			return e
		}
		content.HallID = vendor.HallID
	}
	if content.HallID == 0 {
		content.HallID = 1
	}
	var hall Hall
	if e := tx.First(&hall, content.HallID).Error; e != nil {
		return fmt.Errorf("invalid_request")
	}
	if p.CategoryID > 0 {
		var category Category
		if tx.First(&category, p.CategoryID).Error != nil || category.HallID != content.HallID {
			return fieldFailure("invalid_category", "category_id", "category_unavailable")
		}
	}
	return nil
}
