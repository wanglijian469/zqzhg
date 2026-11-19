package main

import "time"

const enterpriseOSSQuota int64 = 300 * 1024 * 1024

type EnterpriseFile struct {
	MediaID          uint64    `gorm:"primaryKey" json:"id"`
	OwnerID          uint64    `json:"-"`
	EnterpriseID     uint64    `json:"enterprise_id"`
	BusinessType     string    `json:"business_type"`
	BusinessID       uint64    `json:"business_id"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	Visibility       string    `json:"visibility"`
	FileStatus       string    `json:"file_status"`
	Version          uint64    `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type EnterpriseFileObject struct {
	ObjectKey    string    `gorm:"primaryKey" json:"object_key"`
	MediaID      uint64    `json:"media_id"`
	OwnerID      uint64    `json:"-"`
	EnterpriseID uint64    `json:"enterprise_id"`
	StoreID      uint64    `json:"store_id"`
	Bucket       string    `json:"bucket"`
	SourceKey    string    `json:"source_key,omitempty"`
	PhysicalKey  string    `json:"physical_key"`
	ObjectRole   string    `json:"object_role"`
	BusinessType string    `json:"business_type"`
	FileSize     int64     `json:"file_size"`
	SHA256       string    `json:"sha256"`
	ObjectState  string    `json:"object_state"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type FileUploadSession struct {
	ID                 string     `gorm:"primaryKey" json:"id"`
	OwnerID            uint64     `json:"-"`
	ActorID            uint64     `json:"-"`
	EnterpriseID       uint64     `json:"enterprise_id"`
	BusinessID         uint64     `json:"business_id"`
	BusinessType       string     `json:"business_type"`
	BusinessScope      string     `json:"-"`
	Purpose            string     `json:"purpose"`
	OriginalFilename   string     `json:"original_filename"`
	MimeType           string     `json:"mime_type"`
	ExpectedBytes      int64      `json:"expected_bytes"`
	StoreID            uint64     `json:"-"`
	ObjectKey          string     `json:"-"`
	IdempotencyKey     string     `json:"-"`
	Fingerprint        string     `json:"-"`
	UploadMode         string     `json:"upload_mode"`
	MultipartID        string     `json:"-"`
	UploadState        string     `json:"state"`
	MediaID            uint64     `json:"media_id,omitempty"`
	Error              string     `json:"error,omitempty"`
	ExpiresAt          time.Time  `json:"expires_at"`
	SignatureExpiresAt time.Time  `json:"-"`
	LeaseToken         string     `json:"-"`
	LeaseUntil         *time.Time `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
type FileStorageJob struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	JobKind       string     `json:"kind"`
	OwnerID       uint64     `json:"-"`
	EnterpriseID  uint64     `json:"enterprise_id"`
	ResourceKind  string     `json:"resource_kind"`
	ResourceID    uint64     `json:"resource_id"`
	RevisionID    uint64     `json:"revision_id"`
	ExpectedEpoch uint64     `json:"-"`
	Status        string     `json:"status"`
	Checkpoint    string     `json:"-"`
	Error         string     `json:"error,omitempty"`
	LeaseToken    string     `json:"-"`
	LeaseUntil    *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
type FileStorageJobItem struct {
	ID            uint64    `json:"id"`
	JobID         string    `json:"job_id"`
	SourceKey     string    `json:"source_key"`
	TargetKey     string    `json:"target_key"`
	SourceStoreID uint64    `json:"source_store_id"`
	TargetStoreID uint64    `json:"target_store_id"`
	MediaID       uint64    `json:"media_id"`
	FileSize      int64     `json:"file_size"`
	SHA256        string    `json:"sha256"`
	Status        string    `json:"status"`
	ReservationID string    `json:"-"`
	Error         string    `json:"error,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type FileBackupPin struct {
	SnapshotID string `gorm:"primaryKey"`
	ObjectKey  string `gorm:"primaryKey"`
	ExpiresAt  time.Time
}
