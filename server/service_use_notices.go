package main

import (
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// Revise the active documents, retaining operator-authored text and all prior versions.
func migrateServiceUseNotices(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if e := serviceLock(tx, true); e != nil {
			return e
		}
		docs, e := activeServiceDocuments(tx)
		if e != nil {
			return e
		}
		created := []uint64{}
		var previousMarker SystemSetting
		if tx.First(&previousMarker, "setting_key=?", "service_use_notices_v2").Error == nil {
			json.Unmarshal([]byte(previousMarker.Value), &created)
		}
		replacements := []string{
			"发布前主动确认当前协议", "厂商后台使用AI识别或提交资料时，操作前展示当前协议链接，执行该操作即表示已阅读并同意当前协议",
			"服务接受方：主动确认本协议的入驻企业及其获授权经办人", "服务接受方：完成注册并使用本服务的入驻企业及其获授权经办人",
			"实质变更公布后，在下次提交资料前主动确认，不追溯伪造同意", "实质变更公布后，在下次使用AI识别或提交资料时展示当前版本并记录真实操作，不追溯补记历史同意",
			"注册和必要业务操作记录主动同意，不使用默认勾选；变更后按提示确认", "首次注册保留主动勾选同意；已登录厂商后台使用AI识别或提交资料时，执行操作即表示已阅读并同意已展示的当前协议，不再重复勾选或单独确认",
			"可选访问统计默认关闭，只有主动开启后才创建随机访问标识、记录页面、检索、资源及联系动作并发送至本平台后端", "未保存过选择时，可选访问统计默认开启；已明确关闭的选择保持关闭。开启时创建随机访问标识，记录公开页面、检索、资源及联系动作并发送至本平台后端，不采集厂商后台画册、企业草稿或技术参数",
			"访问统计默认关闭，只有主动开启后才创建随机访问标识、记录页面、检索、资源及联系动作并发送至本平台后端", "未保存过选择时，可选访问统计默认开启；已明确关闭的选择保持关闭。开启时创建随机访问标识，记录公开页面、检索、资源及联系动作并发送至本平台后端，不采集厂商后台画册、企业草稿或技术参数",
			"实质变更会提示主动确认", "实质变更会在相关操作前展示当前版本，保留版本及真实操作记录",
		}
		for _, old := range docs {
			if strings.Contains(old.Body, "【2026-11 厂商后台操作说明】") {
				continue
			}
			next := old
			next.ID = 0
			next.Version++
			next.Body = strings.NewReplacer(replacements...).Replace(old.Body)
			next.Body += "\n【2026-11 厂商后台操作说明】\n首次注册仍需主动选择协议。已登录厂商启动AI识别或提交企业、产品资料前展示协议链接；使用本服务并执行上述操作，即表示已阅读并同意相关服务协议。普通浏览和保存草稿不会补记协议确认。协议记录包含当前版本、摘要、时间及来源，不追溯伪造历史同意。未保存选择时可选访问统计默认开启，既有关闭选择不变，可在隐私设置关闭；统计仅限公开页面。基础服务及本轮AI功能未新增收费或扣费。"
			// Explicitly supersede prior English interaction wording as well.
			english := strings.NewReplacer(
				"actively accept current agreements", "acknowledge the displayed current agreements by starting AI recognition or submitting materials in the merchant workspace",
				"who actively accepts this agreement", "who registers and uses this service",
				"Analytics is off by default. Only an explicit opt-in creates", "When no preference is saved, optional analytics is on by default. An existing off preference remains off. While enabled, analytics creates",
			).Replace(old.BodyEN)
			next.BodyEN = "Current interaction policy (supersedes any earlier checkbox/default-analytics wording below): Registration still requires an explicit agreement selection. Signed-in merchants are shown links to the current agreements before starting AI recognition or submitting company or product materials. Performing that action acknowledges the displayed versions; a separate acceptance button is not required. Viewing a page or saving a draft does not record acceptance. Existing acceptance history is retained. Optional public-page analytics is on when no preference has been saved; an existing off preference remains off. It can be disabled in privacy settings. Private merchant brochures, drafts and technical parameters are excluded. This update introduces no AI charge or credit deduction.\n\n" + english
			now := time.Now().UTC().Truncate(time.Millisecond)
			next.CreatedAt = now
			next.EffectiveAt = now
			next.PublishedAt = &now
			next.UpdatedBy = 0
			next.Digest = documentDigest(next)
			if e := tx.Create(&next).Error; e != nil {
				return e
			}
			created = append(created, next.ID)
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "service_use_notices_v2", Value: aiJSON(created), UpdatedAt: time.Now().UTC()}).Error
	})
}

// Schema 36 sites may retain the original sentence without the optional prefix.
// Publish a new privacy version rather than editing any historical document.
func migrateAnalyticsNoticeCorrection(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if e := serviceLock(tx, true); e != nil {
			return e
		}
		docs, e := activeServiceDocuments(tx)
		if e != nil {
			return e
		}
		for _, old := range docs {
			if old.DocKey != "privacy" || !strings.Contains(old.Body, "访问统计默认关闭，只有主动开启后才") {
				continue
			}
			next := old
			next.ID = 0
			next.Version++
			next.Body = strings.ReplaceAll(old.Body, "访问统计默认关闭，只有主动开启后才", "未保存过选择时，访问统计默认开启；已明确关闭的选择保持关闭。开启时")
			now := time.Now().UTC().Truncate(time.Millisecond)
			next.CreatedAt, next.EffectiveAt = now, now
			next.PublishedAt = &now
			next.UpdatedBy = 0
			next.Digest = documentDigest(next)
			if e := tx.Create(&next).Error; e != nil {
				return e
			}
		}
		return nil
	})
}
