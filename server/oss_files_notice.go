package main

import (
	"gorm.io/gorm"
	"strings"
	"time"
)

func publishCloudStorageNotice(tx *gorm.DB) error {
	if e := serviceLock(tx, true); e != nil {
		return e
	}
	docs, e := activeServiceDocuments(tx)
	if e != nil {
		return e
	}
	for _, old := range docs {
		if old.DocKey != "service-info" && old.DocKey != "showroom-agreement" && old.DocKey != "privacy" {
			continue
		}
		if strings.Contains(old.Body, "【OSS 存储与计量规则】") {
			continue
		}
		next := old
		next.ID = 0
		next.Version++
		next.Body = strings.NewReplacer("200 MB", "300 MiB", "200MB", "300 MiB", "200,000,000", "314,572,800", "200000000", "314572800").Replace(old.Body)
		next.Body += "\n【OSS 存储与计量规则】\n启用企业 OSS 存储后，默认免费额度为300 MiB（314,572,800字节）。已经单独设置的企业额度保持原值，以容量页面为准。按实际保留的原件、处理图、缩略图、视频、PDF及公开副本计量；同一物理对象不重复计费，外链视频不计文件容量。使用达到80%提示，额度用满后停止新增上传，仍可删除未使用文件；真实删除及失效授权回收确认后释放容量。审核前文件私有；平台通过后才准备公开副本。历史本地备份不重复占用企业额度，本轮不新增收费或自动扣费。"
		next.BodyEN = strings.NewReplacer("200 MB", "300 MiB", "200,000,000", "314,572,800").Replace(old.BodyEN) + "\nOSS storage policy: After OSS activation the default allowance is 300 MiB (314,572,800 bytes). Explicit company overrides remain unchanged. Retained originals, processed images, thumbnails, video, PDFs and public copies count as physical stored bytes; shared objects count once. External video links do not count. Warn at 80%; block new uploads at capacity while permitting deletion of unused files. Capacity is released after actual deletion and expiry of upload authorizations. Files remain private until platform approval. No new charge or automatic deduction is introduced."
		now := time.Now().UTC().Truncate(time.Millisecond)
		next.CreatedAt = now
		next.EffectiveAt = now
		next.PublishedAt = &now
		next.UpdatedBy = 0
		next.Digest = documentDigest(next)
		if e := tx.Create(&next).Error; e != nil {
			return e
		}
	}
	return nil
}
