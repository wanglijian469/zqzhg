package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// ContentCreateRequest is private durable retry state, committed atomically with
// the content. Keys are scoped to the authenticated owner, never to a device.
type ContentCreateRequest struct {
	OwnerID     uint64 `gorm:"primaryKey"`
	RequestID   string `gorm:"primaryKey"`
	Fingerprint string
	ContentID   uint64
	CreatedAt   time.Time
}

var contentRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func validContentRequestID(key string) bool { return contentRequestIDPattern.MatchString(key) }
func contentCreateFingerprint(kind string, hall uint64, p Payload) string {
	if hall == 0 {
		hall = 1
	}
	p.Name = strings.TrimSpace(p.Name)
	p.RegionName = strings.TrimSpace(p.RegionName)
	p.IndustrialBeltIDs = nil
	// Omitted and empty collections represent the same creation payload.
	if len(p.MediaIDs) == 0 {
		p.MediaIDs = []uint64{}
	}
	if len(p.SecondaryCategoryIDs) == 0 {
		p.SecondaryCategoryIDs = []uint64{}
	}
	if len(p.ApplicableBrands) == 0 {
		p.ApplicableBrands = []string{}
	}
	if len(p.ApplicableModels) == 0 {
		p.ApplicableModels = []string{}
	}
	blob, _ := json.Marshal(struct {
		Kind string  `json:"kind"`
		Hall uint64  `json:"hall_id"`
		Data Payload `json:"data"`
	}{kind, hall, p})
	return digestText(string(blob))
}
