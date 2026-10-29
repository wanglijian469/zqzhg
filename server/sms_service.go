package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SMSSendAttempt struct {
	ID                                              string `gorm:"primaryKey"`
	RequestKey, PhoneKey, IPKey, Purpose            string
	AccountID                                       uint64
	State                                           string
	ProviderCode, ProviderMessage, RequestID, BizID string
	CreatedAt, UpdatedAt                            time.Time
}
type SMSGuard struct {
	ID        string `gorm:"primaryKey"`
	UpdatedAt time.Time
}
type SMSChallenge struct {
	ID                               string `gorm:"primaryKey"`
	PhoneKey, IPKey, Purpose, Digest string
	Used                             bool
	CreatedAt, ExpiresAt             time.Time
}
type smsCodeRequest struct {
	Phone           string `json:"phone"`
	Purpose         string `json:"purpose"`
	RequestID       string `json:"request_id"`
	ChallengeID     string `json:"challenge_id"`
	ChallengeAnswer string `json:"challenge_answer"`
}

func smsCodeDigest(salt, phone, purpose, code string) string {
	key := os.Getenv("SMS_HASH_KEY")
	if key == "" {
		key = "development-only-sms-digest-key"
	}
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(salt + "\x00" + phone + "\x00" + purpose + "\x00" + code))
	return hex.EncodeToString(h.Sum(nil))
}
func smsPhoneKey(phone string) string { return smsCodeDigest("phone", phone, "", "") }
func smsIPKey(ip string) string       { return smsCodeDigest("ip", ip, "", "") }
func (a *App) captchaRequired() bool {
	return a.Config.Env == "production" || os.Getenv("SMS_REQUIRE_CAPTCHA") == "true"
}
func validSMSPurpose(p string) bool {
	return p == "register" || p == "reset_password" || p == "bind" || p == "setup_password"
}
func smsLock(tx *gorm.DB, keys ...string) error {
	sort.Strings(keys)
	for _, key := range keys {
		row := SMSGuard{ID: key, UpdatedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id=?", key).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Update("updated_at", time.Now().UTC()).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) smsChallenge(c *gin.Context) {
	var req smsCodeRequest
	if c.ShouldBindJSON(&req) != nil || !mainlandPhone.MatchString(req.Phone) || !validSMSPurpose(req.Purpose) {
		fail(c, 400, "invalid_request")
		return
	}
	if !a.smsPurposeReady(req.Purpose) {
		fail(c, 503, "sms_template_unavailable")
		return
	}
	ipKey := smsIPKey(c.ClientIP())
	if !permit("sms-challenge:"+ipKey, 30, 5*time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(100000))
	if err != nil {
		fail(c, 503, "sms_unavailable")
		return
	}
	answer := fmt.Sprintf("%05d", n.Int64())
	now := time.Now().UTC()
	id := randomToken()
	row := SMSChallenge{ID: id, PhoneKey: smsPhoneKey(req.Phone), IPKey: ipKey, Purpose: req.Purpose, Digest: smsCodeDigest(id, req.Phone, req.Purpose, answer), CreatedAt: now, ExpiresAt: now.Add(2 * time.Minute)}
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := smsLock(tx, "challenge:"+ipKey); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&SMSChallenge{}).Where("ip_key=? AND created_at>?", ipKey, now.Add(-5*time.Minute)).Count(&count).Error; err != nil {
			return err
		}
		if count >= 30 {
			return fmt.Errorf("rate_limited")
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		if err.Error() == "rate_limited" {
			fail(c, 429, "rate_limited")
		} else {
			fail(c, 503, "sms_unavailable")
		}
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, 156, 52))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{243, 247, 255, 255}}, image.Point{}, draw.Src)
	noise := make([]byte, 160)
	if _, err = rand.Read(noise); err != nil {
		fail(c, 503, "sms_unavailable")
		return
	}
	for i := 0; i < len(noise)-1; i += 2 {
		img.Set(int(noise[i])%156, int(noise[i+1])%52, color.RGBA{160, 183, 218, 255})
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{20, 56, 105, 255}), Face: basicfont.Face7x13}
	for i, ch := range answer {
		d.Dot = fixed.P(20+i*25, 32)
		d.DrawString(string(ch))
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, img); err != nil {
		fail(c, 503, "sms_unavailable")
		return
	}
	c.JSON(200, gin.H{"challenge_id": id, "image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), "expires_in": 120})
}

// A wrong answer consumes the challenge as well. The caller commits that change
// before returning the validation error, so guessing cannot roll the counter back.
func checkSMSChallenge(tx *gorm.DB, req smsCodeRequest, ipKey string) (bool, error) {
	var row SMSChallenge
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id=?", req.ChallengeID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.Used || !time.Now().UTC().Before(row.ExpiresAt) || row.PhoneKey != smsPhoneKey(req.Phone) || row.IPKey != ipKey || row.Purpose != req.Purpose {
		return false, nil
	}
	if err = tx.Model(&row).Update("used", true).Error; err != nil {
		return false, err
	}
	got := smsCodeDigest(row.ID, req.Phone, req.Purpose, strings.TrimSpace(req.ChallengeAnswer))
	return subtle.ConstantTimeCompare([]byte(got), []byte(row.Digest)) == 1, nil
}

func (a *App) smsEligible(tx *gorm.DB, phone, purpose string, accountID uint64) error {
	switch purpose {
	case "register":
		used, err := phoneCollision(tx, phone, 0)
		if err != nil {
			return err
		}
		if used {
			return fmt.Errorf("phone_exists")
		}
	case "reset_password":
		var count int64
		if err := tx.Model(&Account{}).Where("phone=? AND phone_verified_at IS NOT NULL AND role IN ('merchant','visitor')", phone).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("phone_reset_unavailable")
		}
	case "bind":
		used, err := phoneCollision(tx, phone, accountID)
		if err != nil {
			return err
		}
		if used {
			return fmt.Errorf("phone_exists")
		}
	case "setup_password":
		var count int64
		if err := tx.Model(&Account{}).Where("username=? AND role='merchant' AND password_hash=''", phone).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("password_setup_unavailable")
		}
	}
	return nil
}

func (a *App) sendSMSCode(c *gin.Context, binding, legacy bool) {
	if a.Config.SMSProvider == "disabled" {
		fail(c, 503, "sms_disabled")
		return
	}
	var req smsCodeRequest
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if legacy {
		req.Purpose = "login"
	}
	var subject uint64
	if binding {
		if !phoneAccount(actor(c)) {
			fail(c, 403, "forbidden")
			return
		}
		subject = actor(c).ID
		req.Purpose = "bind"
	}
	if !mainlandPhone.MatchString(req.Phone) {
		fail(c, 400, "invalid_phone")
		return
	}
	if req.Purpose == "login" {
		fail(c, 410, "sms_login_disabled")
		return
	}
	if !validSMSPurpose(req.Purpose) || (!binding && req.Purpose == "bind") {
		fail(c, 400, "invalid_request")
		return
	}
	if !a.smsPurposeReady(req.Purpose) {
		fail(c, 503, "sms_template_unavailable")
		return
	}
	ipKey, phoneKey := smsIPKey(c.ClientIP()), smsPhoneKey(req.Phone)
	if !permit("sms-request:"+ipKey, 30, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		key = req.RequestID
	}
	if key == "" && a.captchaRequired() {
		fail(c, 400, "sms_request_id_required")
		return
	}
	if key == "" {
		key = randomToken()
	}
	if len(key) < 16 || len(key) > 128 {
		fail(c, 400, "invalid_request")
		return
	}
	requestKey := digest(key)
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		fail(c, 503, "sms_unavailable")
		return
	}
	code, salt := fmt.Sprintf("%06d", n.Int64()), randomToken()
	now := time.Now().UTC()
	attempt := SMSSendAttempt{ID: randomToken(), RequestKey: requestKey, PhoneKey: phoneKey, IPKey: ipKey, Purpose: req.Purpose, AccountID: subject, State: "sending", CreatedAt: now, UpdatedAt: now}
	send, replay, invalidChallenge := false, false, false
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := smsLock(tx, "ip:"+ipKey, "request:"+requestKey); err != nil {
			return err
		}
		if err := lockPhoneIdentity(tx, req.Phone); err != nil {
			return err
		}
		var previous SMSSendAttempt
		e := tx.Where("request_key=?", requestKey).First(&previous).Error
		if e == nil {
			if previous.PhoneKey != phoneKey || previous.Purpose != req.Purpose || previous.AccountID != subject {
				return fmt.Errorf("sms_request_conflict")
			}
			attempt = previous
			replay = true
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if a.captchaRequired() {
			ok, e := checkSMSChallenge(tx, req, ipKey)
			if e != nil {
				return e
			}
			if !ok {
				invalidChallenge = true
				return nil
			}
		}
		if e := a.smsEligible(tx, req.Phone, req.Purpose, subject); e != nil {
			return e
		}
		var minute, hour, day, ipHour, ipDay int64
		for _, query := range []struct {
			field, key string
			since      time.Time
			out        *int64
		}{
			{"phone_key", phoneKey, now.Add(-time.Minute), &minute}, {"phone_key", phoneKey, now.Add(-time.Hour), &hour}, {"phone_key", phoneKey, now.Add(-24 * time.Hour), &day},
			{"ip_key", ipKey, now.Add(-time.Hour), &ipHour}, {"ip_key", ipKey, now.Add(-24 * time.Hour), &ipDay},
		} {
			if e := tx.Model(&SMSSendAttempt{}).Where(query.field+"=? AND created_at>?", query.key, query.since).Count(query.out).Error; e != nil {
				return e
			}
		}
		if minute >= 1 || hour >= 5 || day >= 10 || ipHour >= 20 || ipDay >= 50 {
			return fmt.Errorf("rate_limited")
		}
		var recent PhoneVerification
		if e := tx.Where("phone=?", req.Phone).Order("sent_at DESC").Limit(1).Find(&recent).Error; e != nil {
			return e
		}
		if !recent.SentAt.IsZero() && now.Sub(recent.SentAt) < time.Minute {
			return fmt.Errorf("rate_limited")
		}
		if e := tx.Create(&attempt).Error; e != nil {
			return e
		}
		row := PhoneVerification{Phone: req.Phone, Purpose: req.Purpose, AccountID: subject, SendID: attempt.ID, Digest: smsCodeDigest(salt, req.Phone, req.Purpose, code), Salt: salt, ExpiresAt: now.Add(5 * time.Minute), SentAt: now}
		if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "phone"}, {Name: "purpose"}, {Name: "account_id"}}, DoUpdates: clause.AssignmentColumns([]string{"send_id", "digest", "salt", "expires_at", "sent_at", "attempts", "used"})}).Create(&row).Error; e != nil {
			return e
		}
		send = true
		return nil
	})
	if invalidChallenge {
		fail(c, 400, "sms_challenge_invalid")
		return
	}
	if err != nil {
		switch err.Error() {
		case "rate_limited":
			c.Header("Retry-After", "60")
			fail(c, 429, "rate_limited")
		case "phone_exists", "phone_login_unavailable", "phone_reset_unavailable", "password_setup_unavailable", "sms_request_conflict":
			fail(c, 409, err.Error())
		default:
			fail(c, 503, "sms_unavailable")
		}
		return
	}
	if replay {
		a.smsAttemptResponse(c, attempt, "")
		return
	}
	if !send {
		fail(c, 503, "sms_unavailable")
		return
	}
	var result SMSResult
	// Detached from request cancellation: once persisted, a disconnected browser
	// cannot force another paid send. SDK timeouts bound this single invocation.
	if p, ok := a.SMS.(purposeSMSProvider); ok {
		result, err = p.SendPurpose(context.Background(), req.Phone, code, req.Purpose, attempt.ID)
	} else {
		err = a.SMS.Send(req.Phone, code)
		if err == nil {
			result.Code = "OK"
		}
	}
	state := "accepted"
	if err != nil {
		state = "rejected"
		var delivery *SMSDeliveryError
		if errors.As(err, &delivery) && delivery.Unknown {
			state = "unknown"
		}
	}
	values := map[string]any{"state": state, "provider_code": result.Code, "provider_message": safeSMSMessage(result.Message), "request_id": result.RequestID, "biz_id": result.BizID, "updated_at": time.Now().UTC()}
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Model(&SMSSendAttempt{}).Where("id=? AND state='sending'", attempt.ID).Updates(values).Error; e != nil {
			return e
		}
		if state == "rejected" {
			return tx.Model(&PhoneVerification{}).Where("send_id=?", attempt.ID).Update("used", true).Error
		}
		return nil
	})
	if err != nil {
		fail(c, 503, "sms_send_unknown")
		return
	}
	attempt.State = state
	dev := ""
	if a.Config.Env != "production" && a.Config.SMSProvider == "development" && state == "accepted" {
		dev = code
	}
	a.smsAttemptResponse(c, attempt, dev)
}

func (a *App) smsAttemptResponse(c *gin.Context, attempt SMSSendAttempt, dev string) {
	if attempt.State == "sending" && time.Since(attempt.CreatedAt) > 15*time.Second {
		attempt.State = "unknown"
	}
	if attempt.State == "sending" {
		c.Header("Retry-After", "2")
		fail(c, 409, "sms_send_pending")
		return
	}
	if attempt.State == "unknown" {
		c.Header("Retry-After", "60")
		fail(c, 503, "sms_send_unknown")
		return
	}
	if attempt.State != "accepted" {
		fail(c, 503, "sms_unavailable")
		return
	}
	remaining := int(math.Ceil(time.Until(attempt.CreatedAt.Add(5 * time.Minute)).Seconds()))
	if remaining <= 0 {
		fail(c, 409, "sms_code_expired")
		return
	}
	retry := int(math.Ceil(time.Until(attempt.CreatedAt.Add(time.Minute)).Seconds()))
	if retry < 0 {
		retry = 0
	}
	out := gin.H{"sent": true, "status": "accepted", "request_id": attempt.ID, "expires_in": remaining, "retry_after": retry}
	if dev != "" {
		out["development_code"] = dev
	}
	c.JSON(200, out)
}

func (a *App) startSMSMaintenance(ctx context.Context) {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				now := time.Now().UTC()
				a.DB.Where("created_at<?", now.Add(-24*time.Hour)).Delete(&SMSChallenge{})
				a.DB.Where("expires_at<?", now.Add(-24*time.Hour)).Delete(&PhoneActionProof{})
				a.DB.Where("created_at<?", now.Add(-30*24*time.Hour)).Delete(&SMSSendAttempt{})
				a.DB.Where("updated_at<?", now.Add(-30*24*time.Hour)).Delete(&SMSGuard{})
			}
		}
	}()
}
