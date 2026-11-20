package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type AIImportTask struct {
	PromptVersionID    uint64     `json:"prompt_version_id"`
	RecognitionOutcome string     `json:"recognition_outcome"`
	ResultVersion      uint64     `json:"result_version"`
	NextRetryAt        *time.Time `json:"next_retry_at,omitempty"`
	SubmissionMode     string     `json:"submission_mode"`
	ModelMode          string     `json:"model_mode"`
	MaxConfirmedAt     *time.Time `json:"max_confirmed_at,omitempty"`
	ExtractionModel    string     `json:"extraction_model"`
	ReviewModel        string     `json:"review_model"`
	ID                 uint64     `json:"id"`
	OwnerID            uint64     `json:"-"`
	VendorID           uint64     `json:"vendor_id"`
	RequestID          string     `json:"-"`
	FileHash           string     `json:"file_hash"`
	FileName           string     `json:"file_name"`
	SourceIDs          string     `json:"-"`
	Status             string     `json:"status"`
	Stage              string     `json:"stage"`
	Progress           int        `json:"progress"`
	Total              int        `json:"total"`
	Checkpoint         string     `json:"-"`
	Error              string     `json:"error"`
	Lease              string     `json:"-"`
	LockedUntil        *time.Time `json:"-"`
	PublishKey         string     `json:"-"`
	PublishManifest    string     `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (AIImportTask) TableName() string { return "ai_import_tasks" }

type AIImportDraft struct {
	GenerationState string    `json:"generation_state"`
	ActiveBatchID   uint64    `json:"active_batch_id"`
	TargetContentID uint64    `gorm:"->" json:"target_content_id"`
	PublishMode     string    `gorm:"->" json:"publish_mode"`
	ID              uint64    `json:"id"`
	TaskID          uint64    `json:"task_id"`
	OwnerID         uint64    `json:"-"`
	TempID          string    `json:"temp_id"`
	Version         uint64    `json:"version"`
	ReviewedVersion uint64    `json:"reviewed_version"`
	Data            string    `json:"-"`
	SourcePages     string    `json:"-"`
	FieldSources    string    `json:"-"`
	ReviewIssues    string    `json:"-"`
	SelectedFields  string    `json:"-"`
	BaseRevisionID  uint64    `json:"base_revision_id"`
	ContentID       uint64    `json:"content_id"`
	RevisionID      uint64    `json:"revision_id"`
	Deleted         bool      `json:"deleted"`
	Error           string    `json:"error"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type AIImportAsset struct {
	ID            uint64    `json:"id"`
	TaskID        uint64    `json:"task_id"`
	OwnerID       uint64    `json:"-"`
	MediaID       uint64    `json:"media_id"`
	SourceAssetID uint64    `json:"source_asset_id"`
	Page          int       `json:"source_page"`
	Panel         string    `json:"panel"`
	PrintedPage   string    `json:"printed_page"`
	ImageType     string    `json:"image_type"`
	CropRegion    string    `json:"-"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	SHA256        string    `json:"sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

func (AIImportAsset) TableName() string { return "ai_import_assets" }

func (asset AIImportAsset) MarshalJSON() ([]byte, error) {
	type plain AIImportAsset
	region := json.RawMessage(asset.CropRegion)
	if !json.Valid(region) {
		region = json.RawMessage("null")
	}
	return json.Marshal(struct {
		plain
		Region json.RawMessage `json:"crop_region"`
	}{plain(asset), region})
}

type AIImportReviewItem struct {
	ID           uint64
	TaskID       uint64
	OwnerID      uint64
	DraftID      uint64
	DraftKind    string
	DraftVersion uint64
	Issue        string
	Resolution   string
	ActorID      uint64
	CreatedAt    time.Time
}

func (AIImportReviewItem) TableName() string { return "ai_import_review_items" }

type AIImportModelCall struct {
	ID            uint64
	TaskID        uint64
	Stage         string
	Fingerprint   string
	Model         string
	RequestID     string
	Attempt       int
	HTTPStatus    int
	Error         string
	UsageJSON     string
	EstimatedCost *float64
	PriceBasis    string
	ResponseJSON  string
	DurationMS    int64
	CreatedAt     time.Time
}

func (AIImportModelCall) TableName() string { return "ai_import_model_calls" }

type AIParameter struct {
	ParameterName   string  `json:"parameter_name"`
	RawValue        string  `json:"raw_value"`
	RawUnit         string  `json:"raw_unit"`
	NormalizedValue *string `json:"normalized_value"`
	NormalizedUnit  *string `json:"normalized_unit"`
	SourcePage      int     `json:"source_page"`
	ReviewRequired  bool    `json:"review_required"`
}
type AIProductModel struct {
	ModelName  string        `json:"model_name"`
	Variant    string        `json:"variant,omitempty"`
	Parameters []AIParameter `json:"parameters"`
}
type AIProductImage struct {
	MediaID    uint64 `json:"media_id"`
	ImageType  string `json:"image_type"`
	SourcePage int    `json:"source_page"`
	Caption    string `json:"caption,omitempty"`
}
type ProductDetails struct {
	SeriesName   string           `json:"series_name"`
	Features     []string         `json:"features"`
	Applications []string         `json:"applications"`
	Models       []AIProductModel `json:"models"`
	Images       []AIProductImage `json:"images"`
	SourcePages  []int            `json:"source_pages"`
}
type AIRegion struct {
	SourcePage  int       `json:"source_page"`
	Panel       string    `json:"panel"`
	PrintedPage string    `json:"printed_page"`
	ImageType   string    `json:"image_type"`
	Box         []float64 `json:"crop_region"`
	Caption     string    `json:"caption"`
}
type AIPageBlock struct {
	BlockID    string `json:"block_id"`
	Title      string `json:"title"`
	SeriesName string `json:"series_name"`
	Kind       string `json:"kind"`
}
type AIPageIndex struct {
	CompanyText string        `json:"company_text"`
	Blocks      []AIPageBlock `json:"blocks"`
	LogoRegions []AIRegion    `json:"logo_regions"`
}
type AIInventoryProduct struct {
	ReviewIssues []string `json:"review_issues,omitempty"`
	TempID       string   `json:"temp_id"`
	ProductName  string   `json:"product_name"`
	SeriesName   string   `json:"series_name"`
	BlockIDs     []string `json:"block_ids"`
	SourcePages  []int    `json:"source_pages"`
}
type AICompanyExtraction struct {
	Data         Payload          `json:"data"`
	SourcePages  []int            `json:"source_pages"`
	FieldSources map[string][]int `json:"field_sources"`
	ReviewIssues []string         `json:"review_issues"`
}
type AIProductExtraction struct {
	ProductName       string           `json:"product_name"`
	SeriesName        string           `json:"series_name"`
	Description       string           `json:"description"`
	Features          []string         `json:"features"`
	Applications      []string         `json:"applications"`
	CategoryCandidate string           `json:"category_candidate"`
	Models            []AIProductModel `json:"models"`
	Images            []AIRegion       `json:"images"`
	SourcePages       []int            `json:"source_pages"`
	ReviewIssues      []string         `json:"review_issues"`
}

func aiJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func aiTable(kind string) string {
	if kind == "company" {
		return "ai_import_company_drafts"
	}
	return "ai_import_product_drafts"
}
func aiPagesValid(pages []int, total int) bool {
	if len(pages) == 0 {
		return false
	}
	for _, p := range pages {
		if p < 1 || p > total {
			return false
		}
	}
	return true
}
func aiRegionValid(r AIRegion, total int) bool {
	if r.SourcePage < 1 || r.SourcePage > total || len(r.Box) != 4 {
		return false
	}
	for _, v := range r.Box {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return false
		}
	}
	return r.Box[2] > r.Box[0] && r.Box[3] > r.Box[1] && aiImageType(r.ImageType)
}
func aiImageType(s string) bool {
	switch s {
	case "logo", "product_main", "product_photo", "structure", "schematic", "dimensions", "parameter_table", "other":
		return true
	}
	return false
}
func validateProductDetails(p *ProductDetails) error {
	if p == nil {
		return nil
	}
	normalizeProductDetails(p)
	if len(p.Models) > 500 || len(p.Images) > 300 || len(p.SeriesName) > 600 {
		return fmt.Errorf("invalid_request")
	}
	for _, m := range p.Models {
		if strings.TrimSpace(m.ModelName) == "" || len(m.ModelName) > 600 || len(m.Parameters) > 500 {
			return fmt.Errorf("invalid_request")
		}
		for _, v := range m.Parameters {
			if v.SourcePage < 1 || v.ParameterName == "" || len(v.RawValue) > 4000 {
				return fmt.Errorf("invalid_request")
			}
		}
	}
	for _, im := range p.Images {
		if im.MediaID == 0 || im.SourcePage < 1 || !aiImageType(im.ImageType) {
			return fmt.Errorf("invalid_media")
		}
	}
	if len(aiJSON(p)) > 512000 {
		return fmt.Errorf("invalid_request")
	}
	return nil
}
func aiProductPayload(x AIProductExtraction) Payload {
	d := &ProductDetails{SeriesName: x.SeriesName, Features: x.Features, Applications: x.Applications, Models: x.Models, SourcePages: x.SourcePages, Images: []AIProductImage{}}
	normalizeProductDetails(d)
	p := Payload{Name: x.ProductName, Description: x.Description, ProductDetails: d}
	names := []string{}
	lines := []string{}
	for _, m := range x.Models {
		names = append(names, m.ModelName)
		lines = append(lines, "型号："+m.ModelName+" "+m.Variant)
		for _, v := range m.Parameters {
			lines = append(lines, v.ParameterName+"："+v.RawValue+" "+v.RawUnit)
		}
	}
	p.Model = strings.Join(names, " / ")
	if len([]rune(p.Model)) > 200 {
		p.Model = ""
	}
	p.Parameters = strings.Join(lines, "\n")
	return standardAIProductPayload(p)
}

func normalizeProductDetails(p *ProductDetails) {
	if p == nil {
		return
	}
	if p.Features == nil {
		p.Features = []string{}
	}
	if p.Applications == nil {
		p.Applications = []string{}
	}
	if p.Models == nil {
		p.Models = []AIProductModel{}
	}
	if p.Images == nil {
		p.Images = []AIProductImage{}
	}
	if p.SourcePages == nil {
		p.SourcePages = []int{}
	}
	for i := range p.Models {
		if p.Models[i].Parameters == nil {
			p.Models[i].Parameters = []AIParameter{}
		}
	}
}
