package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"
	"time"
)

func (a *App) prepareVideoPayload(c *gin.Context, p *ResourcePayload) error {
	p.VideoSource = videoSource(*p)
	if p.VideoSource == videoLocal {
		p.OriginalShareURL = ""
		p.CanonicalVideoURL = ""
		p.ExternalVideoID = ""
		if p.RightsConfirmed {
			p.rightsActorID = actor(c).ID
		}
		return nil
	}
	if p.VideoSource != videoDouyin {
		return fmt.Errorf("unsupported_video_source")
	}
	if !videoV2(c) {
		return fmt.Errorf("video_client_upgrade_required")
	}
	if !douyinVideoEnabled() {
		return fmt.Errorf("douyin_video_disabled")
	}
	if !p.RightsConfirmed {
		return fmt.Errorf("video_rights_required")
	}
	if !permit(fmt.Sprintf("video-save:%d", actor(c).ID), 30, time.Minute) {
		return fmt.Errorf("rate_limited")
	}
	original, id, check, e := a.inspectVideoShare(c.Request.Context(), p.OriginalShareURL)
	if e != nil {
		return e
	}
	p.VideoSource = videoDouyin
	p.OriginalShareURL = original
	p.CanonicalVideoURL = check.CanonicalURL
	p.ExternalVideoID = id
	p.FileMediaID = 0
	p.PageMediaIDs = nil
	p.videoCheck = &check
	p.rightsActorID = actor(c).ID
	if strings.TrimSpace(p.Title) == "" {
		p.Title = check.Title
	}
	return nil
}
func (a *App) validateVideoResource(tx *gorm.DB, r Resource, p *ResourcePayload, ready bool) error {
	source := videoSource(*p)
	if source == videoLocal {
		return nil
	}
	if source != videoDouyin || (p.ExternalVideoID != "" && !videoIDPattern.MatchString(p.ExternalVideoID)) || p.FileMediaID != 0 || !p.RightsConfirmed {
		return fmt.Errorf("invalid_video_link")
	}
	if !douyinVideoEnabled() {
		return fmt.Errorf("douyin_video_disabled")
	}
	// All writers hold the same vendor row. Use a locking/current read, not a stale repeatable-read snapshot.
	var duplicates []Resource
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&Resource{}).Joins("JOIN resource_revisions d ON d.id=resources.draft_revision_id").Joins("LEFT JOIN resource_revisions p ON p.id=resources.published_revision_id").Where("resources.vendor_id=? AND resources.id<>?", r.VendorID, r.ID)
	if p.ExternalVideoID == "" {
		if ready {
			return fmt.Errorf("video_check_required")
		}
		u, e := validateShareURL(p.OriginalShareURL)
		if e != nil || u.Hostname() != "v.douyin.com" || !shortVideoPath.MatchString(u.Path) || p.videoCheck == nil || p.videoCheck.Status != "TEMPORARY_ERROR" {
			return fmt.Errorf("invalid_video_link")
		}
		query = query.Where("d.video_source=? AND d.external_video_id='' AND JSON_UNQUOTE(JSON_EXTRACT(d.payload,'$.original_share_url'))=?", source, p.OriginalShareURL)
	} else {
		query = query.Where("((d.video_source=? AND d.external_video_id=?) OR (p.video_source=? AND p.external_video_id=?))", source, p.ExternalVideoID, source, p.ExternalVideoID)
	}
	if e := query.Select("resources.*").Find(&duplicates).Error; e != nil {
		return e
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate_video")
	}
	if ready {
		check := p.videoCheck
		if check == nil || check.VideoID != p.ExternalVideoID || check.Status != "READY" || !check.ExpiresAt.After(time.Now()) {
			if check != nil && check.Reason != "" {
				return fmt.Errorf("%s", check.Reason)
			}
			return fmt.Errorf("video_check_required")
		}
	}
	return nil
}
func (a *App) prepareStoredVideo(c *gin.Context, rid, revid uint64) (*VideoLinkCheck, error) {
	r, e := loadResource(a.DB, rid, actor(c), false)
	if e != nil {
		return nil, e
	}
	var rev ResourceRevision
	if e = a.DB.Where("id=? AND resource_id=?", revid, r.ID).First(&rev).Error; e != nil {
		return nil, e
	}
	var p ResourcePayload
	if json.Unmarshal([]byte(rev.Payload), &p) != nil {
		return nil, fmt.Errorf("invalid_request")
	}
	if r.Kind != "video" || videoSource(p) == videoLocal {
		return nil, nil
	}
	if !videoV2(c) {
		return nil, fmt.Errorf("video_client_upgrade_required")
	}
	if rev.RightsActorID == 0 || !p.RightsConfirmed {
		return nil, fmt.Errorf("video_rights_required")
	}
	if p.ExternalVideoID == "" {
		return nil, fmt.Errorf("video_check_required")
	}
	result := a.checkDouyin(c.Request.Context(), p.ExternalVideoID)
	return &result, nil
}
func resourceClientFilter(q *gorm.DB, c *gin.Context, alias string) *gorm.DB {
	if !videoV2(c) || (alias == "pr" && !douyinVideoEnabled()) {
		return q.Where("("+alias+".video_source=? OR resources.kind<>'video')", videoLocal)
	}
	return q
}
func guardVideoMutationClient(c *gin.Context, tx *gorm.DB, r Resource) error {
	if r.Kind != "video" || videoV2(c) {
		return nil
	}
	var n int64
	if e := tx.Model(&ResourceRevision{}).Where("id IN ? AND video_source=?", []uint64{r.DraftRevisionID, r.PublishedRevisionID}, videoDouyin).Count(&n).Error; e != nil {
		return e
	}
	if n > 0 {
		return fmt.Errorf("video_client_upgrade_required")
	}
	return nil
}
func (a *App) videoPlayback(c *gin.Context, public bool) {
	c.Header("Cache-Control", "no-store")
	var r Resource
	var rev ResourceRevision
	if public {
		if a.resourceQuery().Where("resources.id=?", id(c)).Select("resources.*").First(&r).Error != nil {
			fail(c, 404, "not_found")
			return
		}
		if a.DB.First(&rev, r.PublishedRevisionID).Error != nil {
			fail(c, 404, "not_found")
			return
		}
	} else {
		var e error
		r, e = loadResource(a.DB, id(c), actor(c), false)
		if e != nil {
			dbFail(c, e)
			return
		}
		if a.DB.First(&rev, r.DraftRevisionID).Error != nil {
			fail(c, 404, "not_found")
			return
		}
	}
	if r.Kind != "video" {
		fail(c, 400, "invalid_request")
		return
	}
	var p ResourcePayload
	if json.Unmarshal([]byte(rev.Payload), &p) != nil {
		fail(c, 500, "internal_error")
		return
	}
	if videoSource(p) == videoLocal {
		c.JSON(200, gin.H{"source": videoLocal, "status": "READY", "media_id": p.FileMediaID})
		return
	}
	if !videoV2(c) {
		fail(c, 409, "video_client_upgrade_required")
		return
	}
	if !douyinVideoEnabled() {
		fail(c, 503, "douyin_video_disabled")
		return
	}
	if p.ExternalVideoID == "" {
		c.JSON(200, VideoLinkCheck{Source: videoDouyin, Status: "TEMPORARY_ERROR", Reason: "video_check_required"})
		return
	}
	if !permit(fmt.Sprintf("video-playback:%s:%d", c.ClientIP(), r.ID), 30, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	result := a.checkDouyin(c.Request.Context(), p.ExternalVideoID)
	if public {
		var n int64
		a.resourceQuery().Where("resources.id=? AND resources.published_revision_id=?", r.ID, rev.ID).Count(&n)
		if n == 0 {
			fail(c, 404, "not_found")
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
