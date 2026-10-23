package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net"
	"time"
)

type procurementDemoAccount struct {
	Label string `json:"label"`
	Phone string `json:"phone"`
}

var demoPhones = []procurementDemoAccount{{"演示采购商", "13900009001"}, {"演示厂商甲", "13900009002"}, {"演示厂商乙", "13900009003"}}

func validateProcurementDemo(cfg Config, database string) error {
	if !cfg.ProcurementDemo {
		return nil
	}
	host, _, e := net.SplitHostPort(cfg.Addr)
	dbHost := env("DB_HOST", "127.0.0.1")
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || cfg.Env != "development" || cfg.SMSProvider != "development" || database != "zqzt_procurement_demo" || (dbHost != "127.0.0.1" && dbHost != "localhost" && dbHost != "::1") {
		return fmt.Errorf("procurement demo requires development SMS, loopback HTTP/MySQL and database zqzt_procurement_demo")
	}
	return nil
}
func (a *App) publicCapabilities() gin.H {
	return gin.H{"procurement": procurementEnabled(), "phone_sms_auth": a.Config.SMSProvider != "disabled", "development_sms": a.Config.Env == "development" && a.Config.SMSProvider == "development", "procurement_demo": a.Config.ProcurementDemo, "image_watermark": true, "phone_password_auth": true, "administrative_regions": true, "industry_map": true, "product_name_only": true}
}
func (a *App) procurementDemoAccounts() []procurementDemoAccount {
	if !a.Config.ProcurementDemo {
		return nil
	}
	return demoPhones
}

func (a *App) seedProcurementDemo() error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var hall Hall
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&hall, 1).Error; e != nil {
			return e
		}
		var count int64
		marker := "procurement_demo_v1"
		if e := tx.Model(&ProcurementEvent{}).Where("operation_id=?", marker).Count(&count).Error; e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
		now := time.Now().UTC()
		accounts := make([]Account, 3)
		for i, entry := range demoPhones {
			phone := entry.Phone
			role := "merchant"
			if i == 0 {
				role = "visitor"
			}
			accounts[i] = Account{Username: fmt.Sprintf("proc-demo-%d", i), Role: role, Phone: &phone, PhoneVerifiedAt: &now, CreatedAt: now}
			if e := tx.Create(&accounts[i]).Error; e != nil {
				return e
			}
			if e := tx.Create(&BuyerProfile{AccountID: accounts[i].ID, Company: entry.Label + "（虚构）", ContactName: entry.Label, RegionCode: "13", Version: 1, UpdatedAt: now}).Error; e != nil {
				return e
			}
		}
		var categories []Category
		if e := tx.Where("hall_id=1 AND enabled=1 AND category_kind IN ('product','vendor')").Order("id").Find(&categories).Error; e != nil {
			return e
		}
		var productCats []uint64
		var vendorCat uint64
		for _, c := range categories {
			if selectableCategory(tx, c.ID, c.CategoryKind, 1, 0, c.CategoryKind == "product") {
				if c.CategoryKind == "product" && len(productCats) < 2 {
					productCats = append(productCats, c.ID)
				} else if c.CategoryKind == "vendor" && vendorCat == 0 {
					vendorCat = c.ID
				}
			}
		}
		if len(productCats) < 2 || vendorCat == 0 {
			return fmt.Errorf("demo categories unavailable")
		}
		publish := func(owner uint64, kind string, vendor uint64, p Payload, slug string) (Content, error) {
			c := Content{Kind: kind, OwnerID: owner, VendorID: vendor, HallID: 1, Visible: true, CreatedAt: now, UpdatedAt: now}
			if kind == "vendor" {
				c.VendorOwnerKey = &owner
				c.ShowroomSlug = &slug
			}
			if e := tx.Create(&c).Error; e != nil {
				return c, e
			}
			if e := makeDraft(tx, &c, p); e != nil {
				return c, e
			}
			if e := tx.Model(&Revision{}).Where("id=?", c.DraftRevisionID).Updates(map[string]any{"status": "approved", "reviewed_at": now, "review_note": "隔离演示资料"}).Error; e != nil {
				return c, e
			}
			c.PublishedRevisionID = c.DraftRevisionID
			c.Epoch = 1
			if e := tx.Save(&c).Error; e != nil {
				return c, e
			}
			if kind == "vendor" {
				if e := tx.Create(&ShowroomAddress{Slug: slug, VendorID: c.ID, CreatedAt: now}).Error; e != nil {
					return c, e
				}
			}
			return c, nil
		}
		vendors := make([]Content, 2)
		products := make([]Content, 2)
		for i := range vendors {
			p := Payload{Name: demoPhones[i+1].Label + "机械有限公司（虚构）", MainProducts: "农业机械及配套机具", PrimaryCategoryID: vendorCat, RegionCode: "1305", Phone: demoPhones[i+1].Phone, Description: "仅用于本机采购询比价演示，不代表真实企业。", CompanyProfile: &CompanyProfile{SupplyProductCategoryIDs: productCats, SupplyRegionCodes: []string{"13"}}}
			var e error
			vendors[i], e = publish(accounts[i+1].ID, "vendor", 0, p, fmt.Sprintf("demo-supplier-%d", i+1))
			if e != nil {
				return e
			}
			products[i], e = publish(accounts[i+1].ID, "product", vendors[i].ID, Payload{Name: "演示农机设备", ProductCategoryID: productCats[0], CategoryID: productCats[0], Model: "DEMO-80", Parameters: "额定功率80千瓦；此为虚构演示产品", Description: "供体验询价和报价使用", Phone: demoPhones[i+1].Phone}, "")
			if e != nil {
				return e
			}
		}
		createProject := func(direct bool) (ProcurementProject, []ProcurementItem, error) {
			p := ProcurementProject{BuyerID: accounts[0].ID, HallID: 1, State: "open", Audience: "public", Version: 1, CreatedAt: now, UpdatedAt: now}
			title := "演示：农机与配套件采购（两家报价比较）"
			if direct {
				p.SourceProductID = products[0].ID
				p.SourceVendorID = vendors[0].ID
				p.Audience = "direct"
				title = "演示：农机设备定向询价（等待报价）"
			}
			if e := tx.Create(&p).Error; e != nil {
				return p, nil, e
			}
			revision := ProcurementRevision{ProjectID: p.ID, Status: "approved", RequestedAudience: p.Audience, VendorIDs: "[]", Title: title, DeliveryRegion: "1305", ExpectedDelivery: "确认后15天内", Deadline: now.Add(14 * 24 * time.Hour), Notes: "仅为本机演示，可体验报价、比价、意向和站内洽谈。", ContactName: "演示采购联系人", Phone: demoPhones[0].Phone, CreatedAt: now}
			if e := tx.Create(&revision).Error; e != nil {
				return p, nil, e
			}
			p.DraftRevisionID = revision.ID
			p.PublishedRevisionID = revision.ID
			if e := tx.Save(&p).Error; e != nil {
				return p, nil, e
			}
			items := []ProcurementItem{{RevisionID: revision.ID, CategoryID: productCats[0], Name: "演示农机设备", Specification: "80千瓦；需说明供货周期", Quantity: "2.000", Unit: "台"}}
			if !direct {
				items = append(items, ProcurementItem{RevisionID: revision.ID, CategoryID: productCats[1], Name: "演示配套件", Specification: "按主机接口配套", Quantity: "10.000", Unit: "件"})
			}
			if e := tx.Create(&items).Error; e != nil {
				return p, nil, e
			}
			n := 2
			if direct {
				n = 1
			}
			for i := 0; i < n; i++ {
				if e := a.procAddParticipant(tx, p, vendors[i].ID, "manual", "隔离演示采购邀请", direct); e != nil {
					return p, nil, e
				}
			}
			if !direct {
				var admin Account
				if e := tx.Where("role='admin'").First(&admin).Error; e != nil {
					return p, nil, e
				}
				if e := tx.Create(&ProcurementReview{ProjectID: p.ID, RevisionID: revision.ID, Action: "publish", Audience: "public", VendorIDs: "[]", Status: "approved", ReviewerID: admin.ID, Reason: "隔离演示预置需求", CreatedAt: now, ReviewedAt: &now}).Error; e != nil {
					return p, nil, e
				}
			}
			if e := tx.Create(&ProcurementEvent{ProjectID: p.ID, ActorID: accounts[0].ID, Action: "procurement_created", Result: "{}", CreatedAt: now}).Error; e != nil {
				return p, nil, e
			}
			return p, items, nil
		}
		if _, _, e := createProject(true); e != nil {
			return e
		}
		project, items, e := createProject(false)
		if e != nil {
			return e
		}
		for i := range vendors {
			quote := ProcurementQuote{ProjectID: project.ID, VendorID: vendors[i].ID, Version: 1, UpdatedAt: now}
			if e := tx.Create(&quote).Error; e != nil {
				return e
			}
			r := ProcurementQuoteRevision{QuoteID: quote.ID, DemandRevisionID: project.PublishedRevisionID, Status: "submitted", TaxMode: "included", FreightMode: "included", Freight: "0.00", ValidUntil: now.Add(14 * 24 * time.Hour), Notes: "虚构演示报价，可自行修订", CreatedAt: now}
			if e := tx.Create(&r).Error; e != nil {
				return e
			}
			for j, item := range items {
				status, price := "quoted", "12000.0000"
				if j > 0 {
					price = "80.0000"
				}
				if i == 1 && j == 0 {
					price = "11500.0000"
				}
				if i == 1 && j > 0 {
					status = "unavailable"
					price = "0"
				}
				if e := tx.Create(&ProcurementQuoteItem{QuoteRevisionID: r.ID, ItemID: item.ID, Status: status, UnitPrice: price, MinimumQuantity: "1.000", DeliveryDays: uint(10 + i*5)}).Error; e != nil {
					return e
				}
			}
			quote.DraftRevisionID = r.ID
			quote.SubmittedRevisionID = r.ID
			if e := tx.Save(&quote).Error; e != nil {
				return e
			}
			if e := a.procNotice(tx, accounts[0].ID, project.ID, "quote", "收到演示厂商报价", fmt.Sprintf("demo-quote-%d", r.ID), false); e != nil {
				return e
			}
		}
		var part ProcurementParticipant
		if e := tx.Where("project_id=? AND vendor_id=?", project.ID, vendors[0].ID).First(&part).Error; e != nil {
			return e
		}
		if e := tx.Create(&ProcurementMessage{ProjectID: project.ID, ParticipantID: part.ID, AuthorID: accounts[0].ID, Body: "请确认演示设备的供货时间，可以在此回复。", CreatedAt: now}).Error; e != nil {
			return e
		}
		if e := a.procNotice(tx, accounts[1].ID, project.ID, "message", "收到演示采购商消息", "demo-message", false); e != nil {
			return e
		}
		result, _ := json.Marshal(gin.H{"initialized": true})
		return tx.Create(&ProcurementEvent{ActorID: accounts[0].ID, Action: "procurement_demo_seed", OperationID: &marker, Fingerprint: digest(marker), Result: string(result), CreatedAt: now}).Error
	})
}
