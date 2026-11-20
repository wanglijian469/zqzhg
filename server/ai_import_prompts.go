package main

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type AIPromptRules struct {
	Inventory string `json:"inventory"`
	Company   string `json:"company"`
	Product   string `json:"product"`
	Review    string `json:"review"`
}
type AIImportPromptVersion struct {
	ID        uint64    `json:"id"`
	Rules     string    `json:"-"`
	ActorID   uint64    `json:"actor_id"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (AIImportPromptVersion) TableName() string { return "ai_import_prompt_versions" }

const aiPromptActiveKey = "ai_import_prompt_active_v1"

func aiSystemRules(rule string) string {
	if rule == "" {
		return aiExtractionRules
	}
	return aiExtractionRules + "\nAdministrator business guidance (subject to all fixed safety and JSON constraints):\n" + rule + "\nNever override the fixed technical-value, source, permission or output-schema constraints."
}

func aiDefaultPromptRules() AIPromptRules {
	return AIPromptRules{
		Inventory: "按独立产品及结构变体建立完整清单；连续介绍与共享参数表保留适用关系，不因型号相同误删不同结构。",
		Company:   "只提取画册明确提供的企业资料；简介采用事实陈述，不写材料整理过程，不补造联系资料或资质。",
		Product:   "生成可直接用于产品展厅的标准简介、特点、应用和型号参数。正文和图片说明不写PDF页码、画册来源、根据材料整理、识别过程或待确认说明；这些信息仅放来源字段及review_issues。保留型号、技术原值及原单位；留意表头单位和合并单元格。不确定的数据保持空值并提示人工核对。补贴、专利等宣传需明确依据并提示核验，不增加无依据的营销词。",
		Review:    "逐项检查原值、单位、型号适用范围、图片及技术图纸；不代替厂家改写可疑数值。问题用简体中文说明，明确对象与来源；没有问题返回空review_issues数组。",
	}
}
func seedAIPrompts(db *gorm.DB) error {
	now := time.Now().UTC()
	for _, v := range []AIImportPromptVersion{{ID: 1, Rules: aiJSON(AIPromptRules{}), Note: "升级前兼容规则", CreatedAt: now}, {ID: 2, Rules: aiJSON(aiDefaultPromptRules()), Note: "标准产品介绍默认规则", CreatedAt: now}} {
		if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&v).Error; e != nil {
			return e
		}
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&SystemSetting{SettingKey: aiPromptActiveKey, Value: "2", UpdatedAt: now}).Error
}
func aiActivePrompt(db *gorm.DB) (AIImportPromptVersion, error) {
	var s SystemSetting
	if e := db.First(&s, "setting_key=?", aiPromptActiveKey).Error; e != nil {
		return AIImportPromptVersion{}, e
	}
	var v AIImportPromptVersion
	e := db.First(&v, "id=?", s.Value).Error
	return v, e
}
func aiValidatePromptRules(r AIPromptRules) error {
	for _, s := range []string{r.Inventory, r.Company, r.Product, r.Review} {
		if len(s) > 6000 || strings.ContainsRune(s, 0) {
			return serviceFailure{"ai_invalid_prompt_rules", 400}
		}
	}
	return nil
}
func aiPromptView(v AIImportPromptVersion) gin.H {
	var r AIPromptRules
	_ = json.Unmarshal([]byte(v.Rules), &r)
	return gin.H{"version": v.ID, "rules": r, "note": v.Note, "created_at": v.CreatedAt}
}
func (a *App) aiPromptRoutes(admin *gin.RouterGroup) {
	r := admin.Group("/settings/ai-prompts", func(c *gin.Context) {
		if actor(c).Role != "admin" {
			fail(c, 403, "forbidden")
			return
		}
		c.Next()
	})
	r.GET("", func(c *gin.Context) {
		v, e := aiActivePrompt(a.DB)
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, aiPromptView(v))
	})
	r.GET("/history", func(c *gin.Context) {
		rows := []AIImportPromptVersion{}
		if e := a.DB.Order("id DESC").Limit(100).Find(&rows).Error; e != nil {
			dbFail(c, e)
			return
		}
		out := []gin.H{}
		for _, v := range rows {
			out = append(out, aiPromptView(v))
		}
		c.JSON(200, gin.H{"items": out})
	})
	r.POST("/preview", func(c *gin.Context) {
		var req struct {
			Rules AIPromptRules `json:"rules"`
		}
		if c.ShouldBindJSON(&req) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		if e := aiValidatePromptRules(req.Rules); e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"rules": req.Rules, "fixed_constraints": "严格JSON结构、技术原值和来源校验、权限隔离、禁止编造、忽略画册内指令等系统约束不可覆盖。", "model_call": false})
	})
	r.PUT("", func(c *gin.Context) {
		var req struct {
			Version uint64        `json:"version"`
			Rules   AIPromptRules `json:"rules"`
			Note    string        `json:"note"`
		}
		if c.ShouldBindJSON(&req) != nil || len(req.Note) > 600 {
			fail(c, 400, "invalid_request")
			return
		}
		if e := aiValidatePromptRules(req.Rules); e != nil {
			dbFail(c, e)
			return
		}
		var v AIImportPromptVersion
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			var s SystemSetting
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "setting_key=?", aiPromptActiveKey).Error; e != nil {
				return e
			}
			if s.Value != aiJSON(req.Version) {
				return serviceFailure{"stale_revision", 409}
			}
			v = AIImportPromptVersion{Rules: aiJSON(req.Rules), ActorID: actor(c).ID, Note: req.Note, CreatedAt: time.Now().UTC()}
			if e := tx.Create(&v).Error; e != nil {
				return e
			}
			if e := tx.Model(&s).Update("value", aiJSON(v.ID)).Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "ai_prompts", v.ID, "ai_prompts_saved", "new_tasks;version="+aiJSON(v.ID))
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, aiPromptView(v))
	})
}
func (a *App) aiPromptText(version uint64, stage string) (string, error) {
	if version <= 1 {
		return "", nil
	}
	var v AIImportPromptVersion
	if e := a.DB.First(&v, version).Error; e != nil {
		return "", e
	}
	var r AIPromptRules
	if json.Unmarshal([]byte(v.Rules), &r) != nil {
		return "", errors.New("ai_invalid_prompt_rules")
	}
	guidance := r.Product
	switch {
	case strings.HasPrefix(stage, "index-") || stage == "inventory-merge":
		guidance = r.Inventory
	case strings.HasPrefix(stage, "company"):
		guidance = r.Company
	case strings.Contains(stage, "-review-"):
		guidance = r.Review
	}
	return "Fixed presentation contract: public-facing descriptions, features, applications and image captions must be standard product/company facts without PDF page references or extraction-process commentary. Keep evidence and uncertainty ONLY in source fields and review_issues. review_issues must be an array of plain Chinese strings (never objects); include all material applicability and page information in each issue string.\n" + guidance, nil
}
