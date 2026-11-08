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
	if !supportsVideo(c, p.VideoSource, p.Purpose) {
		return fmt.Errorf("video_client_upgrade_required")
	}
	if p.VideoSource == videoLocal {
		p.OriginalShareURL = ""
		p.CanonicalVideoURL = ""
		p.ExternalVideoID = ""
		if p.RightsConfirmed {
			p.rightsActorID = actor(c).ID
		}
		return nil
	}
	if p.VideoSource != videoDouyin && p.VideoSource != videoKuaishou {
		return fmt.Errorf("unsupported_video_source")
	}
	if p.VideoSource == videoDouyin && !douyinVideoEnabled() {
		return fmt.Errorf("douyin_video_disabled")
	}
	if !p.RightsConfirmed {
		// Reuse the actor's real, current agreement acceptance; never forge a per-video consent.
		if e := a.DB.Transaction(func(tx *gorm.DB) error { return publicationCompliance(tx, actor(c).ID, 0, nil) }); e != nil {
			return e
		}
	}
	if !permit(fmt.Sprintf("video-save:%d", actor(c).ID), 30, time.Minute) {
		return fmt.Errorf("rate_limited")
	}
	var original, id string
	var check VideoLinkCheck
	var e error
	if p.VideoSource == videoKuaishou {
		check, e = checkKuaishou(p.OriginalShareURL)
		original, id = check.CanonicalURL, check.VideoID
	} else {
		original, id, check, e = a.inspectVideoShare(c.Request.Context(), p.OriginalShareURL)
	}
	if e != nil {
		return e
	}
	p.OriginalShareURL = original
	p.CanonicalVideoURL = check.CanonicalURL
	p.ExternalVideoID = id
	p.FileMediaID = 0
	p.PageMediaIDs = nil
	p.videoCheck = &check
	if p.RightsConfirmed {
		p.rightsActorID = actor(c).ID
	}
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
	if p.FileMediaID != 0 || (source != videoDouyin && source != videoKuaishou) {
		return fmt.Errorf("invalid_video_link")
	}
	if source == videoKuaishou {
		check, e := checkKuaishou(p.OriginalShareURL)
		if e != nil || check.VideoID != p.ExternalVideoID || check.CanonicalURL != p.CanonicalVideoURL {
			return fmt.Errorf("invalid_video_link")
		}
	} else if p.ExternalVideoID != "" && !videoIDPattern.MatchString(p.ExternalVideoID) {
		return fmt.Errorf("invalid_video_link")
	}
	if source == videoDouyin && !douyinVideoEnabled() {
		return fmt.Errorf("douyin_video_disabled")
	}
	if !p.RightsConfirmed {
		if e := publicationCompliance(tx, r.OwnerID, r.VendorID, nil); e != nil {
			return e
		}
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
		status := "READY"
		if source == videoKuaishou {
			status = "EXTERNAL_ONLY"
		}
		if check == nil || check.VideoID != p.ExternalVideoID || check.Status != status || !check.ExpiresAt.After(time.Now()) {
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
	if !supportsVideo(c, videoSource(p), p.Purpose) {
		return nil, fmt.Errorf("video_client_upgrade_required")
	}
	if rev.RightsActorID == 0 || !p.RightsConfirmed {
		if e := a.DB.Transaction(func(tx *gorm.DB) error { return publicationCompliance(tx, r.OwnerID, r.VendorID, nil) }); e != nil {
			return nil, e
		}
	}
	if p.ExternalVideoID == "" {
		return nil, fmt.Errorf("video_check_required")
	}
	result, e := a.checkStoredVideo(c, p)
	return &result, e
}
func (a *App) checkStoredVideo(c *gin.Context, p ResourcePayload) (VideoLinkCheck, error) {
	if videoSource(p) == videoKuaishou {
		return checkKuaishou(p.OriginalShareURL)
	}
	return a.checkDouyin(c.Request.Context(), p.ExternalVideoID), nil
}
func supportsVideo(c *gin.Context, source, purpose string) bool {
	if source == videoKuaishou || purpose == "other" {
		return videoV3(c)
	}
	return source != videoDouyin || videoV2(c)
}
func resourceClientFilter(q *gorm.DB, c *gin.Context, alias string) *gorm.DB {
	sources := []string{videoLocal}
	if videoV2(c) && (alias != "pr" || douyinVideoEnabled()) {
		sources = append(sources, videoDouyin)
	}
	if videoV3(c) {
		sources = append(sources, videoKuaishou)
	} else {
		q = q.Where("(" + alias + ".purpose<>'other' OR resources.kind<>'video')")
	}
	return q.Where("("+alias+".video_source IN ? OR resources.kind<>'video')", sources)
}
func guardVideoMutationClient(c *gin.Context, tx *gorm.DB, r Resource) error {
	if r.Kind != "video" || videoV3(c) {
		return nil
	}
	var n int64
	sources := []string{videoKuaishou}
	if !videoV2(c) {
		sources = append(sources, videoDouyin)
	}
	if e := tx.Model(&ResourceRevision{}).Where("id IN ? AND (video_source IN ? OR purpose='other')", []uint64{r.DraftRevisionID, r.PublishedRevisionID}, sources).Count(&n).Error; e != nil {
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
	if !supportsVideo(c, videoSource(p), p.Purpose) {
		fail(c, 409, "video_client_upgrade_required")
		return
	}
	if videoSource(p) == videoLocal {
		c.JSON(200, gin.H{"source": videoLocal, "status": "READY", "media_id": p.FileMediaID})
		return
	}
	if videoSource(p) == videoDouyin && !douyinVideoEnabled() {
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
	result, e := a.checkStoredVideo(c, p)
	if e != nil {
		dbFail(c, e)
		return
	}
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
