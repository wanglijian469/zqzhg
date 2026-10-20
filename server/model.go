package main

import "time"

type Account struct {
	ID           uint64    `json:"id"`
	Username     string    `json:"username"`
	LoginName    *string   `json:"login_name,omitempty"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
type Session struct {
	TokenHash string `gorm:"primaryKey"`
	AccountID uint64
	ExpiresAt time.Time
	CreatedAt time.Time
}
type SMSCode struct {
	Phone     string `gorm:"primaryKey"`
	Digest    string
	Salt      string
	ExpiresAt time.Time
	SentAt    time.Time
	Attempts  int
	Used      bool
}

func (SMSCode) TableName() string { return "sms_codes" }

type Hall struct {
	Status        string `json:"status"`
	Icon          string `json:"icon"`
	Theme         string `json:"theme"`
	Description   string `json:"description"`
	DescriptionEN string `json:"description_en"`
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	NameEN        string `json:"name_en"`
	Slug          string `json:"slug"`
}
type Category struct {
	ID        uint64 `json:"id"`
	HallID    uint64 `json:"hall_id"`
	ParentID  uint64 `json:"parent_id"`
	Name      string `json:"name"`
	NameEN    string `json:"name_en"`
	SortOrder int    `json:"sort_order"`
	Enabled   bool   `json:"enabled"`
}
type Region struct {
	AdministrativeCode *string `json:"administrative_code,omitempty"`
	ID                 uint64  `json:"id"`
	Name               string  `json:"name"`
	NameEN             string  `json:"name_en"`
	Enabled            bool    `json:"enabled"`
}
type Content struct {
	HallID              uint64    `gorm:"default:1" json:"hall_id"`
	ShowroomSlug        *string   `json:"-"`
	ID                  uint64    `json:"id"`
	Kind                string    `json:"kind"`
	OwnerID             uint64    `json:"owner_id"`
	VendorID            uint64    `json:"vendor_id"`
	VendorOwnerKey      *uint64   `json:"-"`
	PublishedRevisionID uint64    `json:"published_revision_id"`
	DraftRevisionID     uint64    `json:"draft_revision_id"`
	Epoch               uint64    `json:"epoch"`
	Visible             bool      `json:"visible"`
	Featured            bool      `json:"featured"`
	SortOrder           int       `json:"sort_order"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
type Payload struct {
	RegionCode     string          `json:"region_code,omitempty"`
	CompanyProfile *CompanyProfile `json:"company_profile,omitempty"`
	Name           string          `json:"name"`
	NameEN         string          `json:"name_en"`
	MainProducts   string          `json:"main_products"`
	Keywords       string          `json:"keywords"`
	RegionID       uint64          `json:"region_id"`
	RegionName     string          `json:"region_name"`
	CategoryID     uint64          `json:"category_id"`
	Description    string          `json:"description"`
	DescriptionEN  string          `json:"description_en"`
	Phone          string          `json:"phone"`
	Wechat         string          `json:"wechat"`
	Address        string          `json:"address"`
	Parameters     string          `json:"parameters"`
	MediaIDs       []uint64        `json:"media_ids"`
}
type Revision struct {
	AdministrativeSnapshot
	ID           uint64     `json:"id"`
	ContentID    uint64     `json:"content_id"`
	BaseEpoch    uint64     `json:"base_epoch"`
	Status       string     `json:"status"`
	Name         string     `json:"name"`
	NameEN       string     `json:"name_en"`
	Keywords     string     `json:"keywords"`
	MainProducts string     `json:"main_products"`
	RegionID     uint64     `json:"region_id"`
	CategoryID   uint64     `json:"category_id"`
	Payload      string     `json:"-"`
	ReviewNote   string     `json:"review_note"`
	ReviewerID   uint64     `json:"reviewer_id"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
type Media struct {
	ID        uint64    `json:"id"`
	OwnerID   uint64    `json:"-"`
	Path      string    `json:"-"`
	ThumbPath string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (Media) TableName() string { return "media" }

type MediaReference struct {
	RevisionID uint64
	MediaID    uint64
}
type ReviewLog struct {
	ID         uint64    `json:"id"`
	ContentID  uint64    `json:"content_id"`
	RevisionID uint64    `json:"revision_id"`
	ActorID    uint64    `json:"actor_id"`
	Action     string    `json:"action"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}
type OperationLog struct {
	ID                uint64    `json:"id"`
	ActorID           uint64    `json:"actor_id"`
	TargetKind        string    `json:"target_kind"`
	TargetID          uint64    `json:"target_id"`
	Action            string    `json:"action"`
	Note              string    `json:"note"`
	SourceReviewLogID *uint64   `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
}
type View struct {
	Content
	ShowroomSlug     string         `json:"showroom_slug,omitempty"`
	PublicVisible    bool           `json:"public_visible"`
	Data             Payload        `json:"data"`
	Status           string         `json:"status"`
	RevisionID       uint64         `json:"revision_id"`
	ReviewNote       string         `json:"review_note"`
	MatchingProducts []View         `json:"matching_products,omitempty"`
	Vendor           *VendorSummary `json:"vendor,omitempty"`
	Published        *View          `json:"published,omitempty"`
	ProductCount     int64          `json:"product_count,omitempty"`
}

type VendorSummary struct {
	ShowroomSlug string `json:"showroom_slug"`
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	NameEN       string `json:"name_en"`
	RegionID     uint64 `json:"region_id"`
	RegionName   string `json:"region_name"`
}
