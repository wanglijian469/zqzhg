package main

import "time"

type BuyerProfile struct {
	AccountID   uint64    `gorm:"primaryKey" json:"account_id"`
	Company     string    `json:"company"`
	ContactName string    `json:"contact_name"`
	RegionCode  string    `json:"region_code"`
	Suspended   bool      `json:"suspended"`
	Version     uint64    `json:"version"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ProcurementProject struct {
	ID                  uint64    `json:"id"`
	BuyerID             uint64    `json:"-"`
	HallID              uint64    `json:"hall_id"`
	SourceProductID     uint64    `json:"source_product_id,omitempty"`
	SourceVendorID      uint64    `json:"-"`
	State               string    `json:"state"`
	Audience            string    `json:"audience"`
	DraftRevisionID     uint64    `json:"draft_revision_id"`
	PublishedRevisionID uint64    `json:"published_revision_id"`
	Version             uint64    `json:"version"`
	Suspended           bool      `json:"suspended"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
type ProcurementRevision struct {
	ID                uint64    `json:"id"`
	ProjectID         uint64    `json:"project_id"`
	Status            string    `json:"status"`
	RequestedAudience string    `json:"requested_audience"`
	VendorIDs         string    `json:"-"`
	Title             string    `json:"title"`
	DeliveryRegion    string    `json:"delivery_region"`
	ExpectedDelivery  string    `json:"expected_delivery"`
	Deadline          time.Time `json:"deadline"`
	Notes             string    `json:"notes"`
	ContactName       string    `json:"-"`
	Phone             string    `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
}
type ProcurementItem struct {
	ID            uint64 `json:"id"`
	RevisionID    uint64 `json:"revision_id"`
	CategoryID    uint64 `json:"category_id"`
	Name          string `json:"name"`
	Specification string `json:"specification"`
	Quantity      string `json:"quantity"`
	Unit          string `json:"unit"`
}
type ProcurementReview struct {
	ID         uint64     `json:"id"`
	ProjectID  uint64     `json:"project_id"`
	RevisionID uint64     `json:"revision_id"`
	Action     string     `json:"action"`
	Audience   string     `json:"audience"`
	VendorIDs  string     `json:"-"`
	Status     string     `json:"status"`
	ReviewerID uint64     `json:"reviewer_id"`
	Reason     string     `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at"`
}
type ProcurementParticipant struct {
	ID          uint64    `json:"id"`
	ProjectID   uint64    `json:"project_id"`
	VendorID    uint64    `json:"vendor_id"`
	Source      string    `json:"source"`
	State       string    `json:"state"`
	Intent      bool      `json:"intent"`
	MatchReason string    `json:"match_reason"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ProcurementQuote struct {
	ID                  uint64    `json:"id"`
	ProjectID           uint64    `json:"project_id"`
	VendorID            uint64    `json:"vendor_id"`
	SubmittedRevisionID uint64    `json:"submitted_revision_id"`
	DraftRevisionID     uint64    `json:"draft_revision_id"`
	Version             uint64    `json:"version"`
	Withdrawn           bool      `json:"withdrawn"`
	Excluded            bool      `json:"excluded"`
	UpdatedAt           time.Time `json:"updated_at"`
}
type ProcurementQuoteRevision struct {
	ID               uint64    `json:"id"`
	QuoteID          uint64    `json:"quote_id"`
	DemandRevisionID uint64    `json:"demand_revision_id"`
	Status           string    `json:"status"`
	TaxMode          string    `json:"tax_mode"`
	FreightMode      string    `json:"freight_mode"`
	Freight          string    `json:"freight"`
	ValidUntil       time.Time `json:"valid_until"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
}
type ProcurementQuoteItem struct {
	ID              uint64 `json:"id"`
	QuoteRevisionID uint64 `json:"quote_revision_id"`
	ItemID          uint64 `json:"item_id"`
	Status          string `json:"status"`
	UnitPrice       string `json:"unit_price"`
	MinimumQuantity string `json:"minimum_quantity"`
	DeliveryDays    uint   `json:"delivery_days"`
	Notes           string `json:"notes"`
}
type ProcurementMessage struct {
	ID            uint64    `json:"id"`
	ProjectID     uint64    `json:"project_id"`
	ParticipantID uint64    `json:"participant_id"`
	AuthorID      uint64    `json:"author_id"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
}
type ProcurementAttachment struct {
	ID          uint64    `json:"id"`
	ProjectID   uint64    `json:"project_id"`
	OwnerID     uint64    `json:"-"`
	Scope       string    `json:"scope"`
	ScopeID     uint64    `json:"scope_id"`
	ObjectKey   string    `json:"-"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}
type ProcurementEvent struct {
	ID          uint64    `json:"id"`
	ProjectID   uint64    `json:"project_id"`
	ActorID     uint64    `json:"actor_id"`
	Action      string    `json:"action"`
	OperationID *string   `json:"-"`
	Fingerprint string    `json:"-"`
	Result      string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}
type ProcurementNotification struct {
	TargetScope string     `gorm:"-" json:"target_scope"`
	ID          uint64     `json:"id"`
	AccountID   uint64     `json:"-"`
	ProjectID   uint64     `json:"project_id"`
	EventKey    string     `json:"-"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	AvailableAt time.Time  `json:"available_at"`
	ReadAt      *time.Time `json:"read_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
type ProcurementPreference struct {
	AccountID         uint64    `gorm:"primaryKey" json:"account_id"`
	OpportunityMode   string    `json:"opportunity_mode"`
	SupplierSuspended bool      `json:"supplier_suspended"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Commercial notifications deliberately use an interface separate from SMS codes.
// No implementation is enabled in v1; station messages are the delivery channel.
type ProcurementSMSNotifier interface {
	Notify(phone, template string, parameters map[string]string) error
}
