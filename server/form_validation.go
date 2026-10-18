package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed form_rules.json
var formRulesJSON []byte

type FormRule struct {
	Max      int      `json:"max"`
	Unit     string   `json:"unit"`
	Required []string `json:"required"`
}
type FieldIssue struct {
	Field string `json:"field"`
	Code  string `json:"code"`
	Limit int    `json:"limit,omitempty"`
	Unit  string `json:"unit,omitempty"`
}
type FieldValidationError struct {
	Code   string
	Fields []FieldIssue
}

func (e *FieldValidationError) Error() string { return e.Code }
func fieldFailure(code, field, reason string) error {
	return &FieldValidationError{code, []FieldIssue{{Field: field, Code: reason}}}
}
func payloadFieldIssues(kind string, p Payload) []FieldIssue {
	var rules map[string]FormRule
	json.Unmarshal(formRulesJSON, &rules)
	raw, _ := json.Marshal(p)
	var values map[string]any
	json.Unmarshal(raw, &values)
	issues := []FieldIssue{}
	var visit func(map[string]any, string)
	visit = func(m map[string]any, prefix string) {
		for key, value := range m {
			path := prefix + key
			switch v := value.(type) {
			case string:
				lookup := path
				parts := strings.Split(path, ".")
				for i, x := range parts {
					if _, e := strconv.Atoi(x); e == nil {
						parts[i] = "*"
					}
				}
				lookup = strings.Join(parts, ".")
				r, ok := rules[lookup]
				if !ok {
					continue
				}
				for _, k := range r.Required {
					if k == kind && strings.TrimSpace(v) == "" {
						issues = append(issues, FieldIssue{Field: path, Code: "required"})
					}
				}
				n := utf8.RuneCountInString(v)
				if r.Unit == "bytes" {
					n = len(v)
				}
				if n > r.Max {
					issues = append(issues, FieldIssue{path, "too_long", r.Max, r.Unit})
				}
				if path == "company_profile.website" && v != "" {
					u, e := url.Parse(v)
					if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
						issues = append(issues, FieldIssue{Field: path, Code: "invalid_url"})
					}
				}
			case map[string]any:
				visit(v, path+".")
			case []any:
				for i, row := range v {
					if child, ok := row.(map[string]any); ok {
						visit(child, fmt.Sprintf("%s.%d.", path, i))
					}
				}
			}
		}
	}
	visit(values, "")
	if kind == "vendor" && strings.TrimSpace(p.Phone) == "" && strings.TrimSpace(p.Wechat) == "" {
		for _, key := range []string{"phone", "wechat"} {
			issues = append(issues, FieldIssue{Field: key, Code: "contact_required"})
		}
	}
	if p.CompanyProfile != nil {
		profile := p.CompanyProfile
		if len(profile.Credentials) > 20 {
			issues = append(issues, FieldIssue{"company_profile.credentials", "too_many", 20, "items"})
		}
		if len(profile.GalleryAnnotations) > 12 {
			issues = append(issues, FieldIssue{"company_profile.gallery_annotations", "too_many", 12, "items"})
		}
		checkImage := func(row GalleryAnnotation, prefix string, required bool) {
			if row.MediaID == 0 {
				issues = append(issues, FieldIssue{Field: prefix + ".media_id", Code: "required"})
			}
			if required && strings.TrimSpace(row.Title) == "" {
				issues = append(issues, FieldIssue{Field: prefix + ".title", Code: "required"})
			}
			for key, value := range map[string]string{"title": row.Title, "title_en": row.TitleEN, "description": row.Description, "description_en": row.DescriptionEN} {
				limit := 100
				if strings.HasPrefix(key, "description") {
					limit = 500
				}
				if utf8.RuneCountInString(value) > limit {
					issues = append(issues, FieldIssue{prefix + "." + key, "too_long", limit, "characters"})
				}
			}
		}
		for i, row := range profile.Credentials {
			prefix := fmt.Sprintf("company_profile.credentials.%d", i)
			checkImage(row.GalleryAnnotation, prefix, true)
			if row.Kind != "qualification" && row.Kind != "award" {
				issues = append(issues, FieldIssue{Field: prefix + ".kind", Code: "invalid_request"})
			}
		}
		seen := map[uint64]bool{}
		gallery := map[uint64]bool{}
		for _, mid := range p.MediaIDs {
			gallery[mid] = true
		}
		for i, row := range profile.GalleryAnnotations {
			prefix := fmt.Sprintf("company_profile.gallery_annotations.%d", i)
			checkImage(row, prefix, false)
			if seen[row.MediaID] || !gallery[row.MediaID] {
				issues = append(issues, FieldIssue{Field: prefix + ".media_id", Code: "invalid_media"})
			}
			seen[row.MediaID] = true
		}
		if len(p.CompanyProfile.Contacts) > 20 {
			issues = append(issues, FieldIssue{"company_profile.contacts", "too_many", 20, "items"})
		}
		for i, c := range p.CompanyProfile.Contacts {
			if strings.TrimSpace(c.Phone) == "" && strings.TrimSpace(c.Wechat) == "" {
				for _, key := range []string{"phone", "wechat"} {
					issues = append(issues, FieldIssue{Field: fmt.Sprintf("company_profile.contacts.%d.%s", i, key), Code: "contact_required"})
				}
			}
		}
	}
	if len(p.MediaIDs) > 12 {
		issues = append(issues, FieldIssue{"media_ids", "too_many", 12, "items"})
	}
	return issues
}
