package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type AIReviewResult struct {
	Issues []string `json:"review_issues"`
}

// Models sometimes serialize an explicitly printed value as a JSON number.
// Preserve its exact JSON lexical representation, never float-round a technical
// value or synthesize a unit/range/model. Unexpected compound values are rejected.
func aiDecode(raw string, out any) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var value any
	if e := dec.Decode(&value); e != nil {
		return e
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return fmt.Errorf("schema_field:trailing_json")
	}
	notes := []string{}
	normalized, e := aiShape(value, reflect.TypeOf(out).Elem(), "", &notes)
	if e != nil {
		return e
	}
	b, e := json.Marshal(normalized)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return e
	}
	// A syntactically valid {} must never count as a successful extraction.
	// Returning a schema error lets the existing audited repair request retry it.
	switch v := out.(type) {
	case *AIReviewResult:
		if m, ok := value.(map[string]any); !ok || m["review_issues"] == nil {
			return fmt.Errorf("schema_field:review_issues")
		}
	case *AIProductExtraction:
		if strings.TrimSpace(v.ProductName) == "" {
			return fmt.Errorf("schema_field:product_name")
		}
		if len(v.SourcePages) == 0 {
			return fmt.Errorf("schema_field:source_pages")
		}
	case *AICompanyExtraction:
		if len(v.SourcePages) == 0 || v.Data.Name == "" {
			return fmt.Errorf("schema_field:company_source")
		}
	case *AIPageIndex:
		for _, b := range v.Blocks {
			if strings.TrimSpace(b.Title) == "" || (b.Kind != "product" && b.Kind != "support") {
				return fmt.Errorf("schema_field:blocks.title_or_kind")
			}
		}
		m, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("schema_field:page_index")
		}
		for _, key := range []string{"blocks", "company_text", "logo_regions"} {
			if _, ok = m[key]; !ok {
				return fmt.Errorf("schema_field:%s", key)
			}
		}
	case *AIInventory:
		if len(v.Products) == 0 {
			return fmt.Errorf("schema_field:products")
		}
	}
	if len(notes) > 0 {
		note := fmt.Sprintf("识别结果中%d处字段格式已转换，技术原值字符未改写；请对照来源页核对。", len(notes))
		switch v := out.(type) {
		case *AIProductExtraction:
			v.ReviewIssues = append(v.ReviewIssues, note)
		case *AICompanyExtraction:
			v.ReviewIssues = append(v.ReviewIssues, note)
		}
	}
	return nil
}
func aiShape(v any, t reflect.Type, path string, notes *[]string) (any, error) {
	if v == nil {
		return nil, nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fail := func() (any, error) { return nil, fmt.Errorf("schema_field:%s", path) }
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		out := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			if f.PkgPath != "" || key == "-" {
				continue
			}
			if key == "" {
				key = f.Name
			}
			if raw, ok := m[key]; ok {
				next, e := aiShape(raw, f.Type, strings.TrimPrefix(path+"."+key, "."), notes)
				if e != nil {
					return nil, e
				}
				out[key] = next
			}
		}
		return out, nil
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		out := map[string]any{}
		for k, raw := range m {
			next, e := aiShape(raw, t.Elem(), path+"."+k, notes)
			if e != nil {
				return nil, e
			}
			out[k] = next
		}
		return out, nil
	case reflect.Slice:
		list, ok := v.([]any)
		if !ok {
			if t.Elem().Kind() == reflect.String || t.Elem().Kind() == reflect.Int {
				list = []any{v}
				*notes = append(*notes, path)
			} else {
				return fail()
			}
		}
		out := []any{}
		for i, raw := range list {
			if t.Elem().Kind() == reflect.String && strings.HasSuffix(path, "review_issues") {
				if obj, ok := raw.(map[string]any); ok {
					if len(obj) != 1 {
						return fail()
					}
					message, ok := obj["message"].(string)
					if !ok || strings.TrimSpace(message) == "" {
						return fail()
					}
					raw = message
					*notes = append(*notes, path)
				}
			}
			next, e := aiShape(raw, t.Elem(), fmt.Sprintf("%s.%d", path, i), notes)
			if e != nil {
				return nil, e
			}
			out = append(out, next)
		}
		return out, nil
	case reflect.String:
		switch x := v.(type) {
		case string:
			return x, nil
		case json.Number:
			*notes = append(*notes, path)
			return x.String(), nil
		case []any:
			if strings.HasSuffix(path, "company_text") {
				lines := []string{}
				for _, row := range x {
					s, ok := row.(string)
					if !ok {
						return fail()
					}
					lines = append(lines, s)
				}
				return strings.Join(lines, "\n"), nil
			}
		}
		return fail()
	case reflect.Int, reflect.Int64, reflect.Uint64:
		var text string
		switch x := v.(type) {
		case json.Number:
			text = x.String()
		case string:
			text = x
			*notes = append(*notes, path)
		default:
			return fail()
		}
		n, e := strconv.ParseInt(text, 10, 64)
		if e != nil || n < 0 {
			return fail()
		}
		return n, nil
	case reflect.Float64:
		switch x := v.(type) {
		case json.Number:
			return x, nil
		case string:
			n, e := strconv.ParseFloat(x, 64)
			if e == nil {
				return n, nil
			}
		}
		return fail()
	case reflect.Bool:
		if x, ok := v.(bool); ok {
			return x, nil
		}
		if x, ok := v.(string); ok && (x == "true" || x == "false") {
			return x == "true", nil
		}
		return fail()
	}
	return v, nil
}

type aiModelCheckpoint struct {
	Fingerprint string          `json:"fingerprint"`
	Response    json.RawMessage `json:"response"`
}
type aiImportCheckpoint struct {
	Catalog   []AIInventoryProduct         `json:"catalog,omitempty"`
	Work      *AIImportWork                `json:"work,omitempty"`
	TextPages map[string]string            `json:"text_pages,omitempty"`
	Models    map[string]aiModelCheckpoint `json:"models,omitempty"`
}

func (a *App) aiReadCheckpoint(task uint64) aiImportCheckpoint {
	var row AIImportTask
	a.DB.Select("checkpoint").First(&row, task)
	var c aiImportCheckpoint
	json.Unmarshal([]byte(row.Checkpoint), &c)
	if c.Models == nil {
		c.Models = map[string]aiModelCheckpoint{}
	}
	return c
}
func (a *App) aiRememberModel(task AIImportTask, stage, fp string, out any) error {
	c := a.aiReadCheckpoint(task.ID)
	var normalized any
	d := json.NewDecoder(bytes.NewReader([]byte(aiJSON(out))))
	d.UseNumber()
	if e := d.Decode(&normalized); e != nil {
		return e
	}
	c.Models[stage] = aiModelCheckpoint{fp, json.RawMessage(aiJSON(normalized))}
	return a.DB.Model(&AIImportTask{}).Where("id=? AND lease=? AND status='processing'", task.ID, task.Lease).Update("checkpoint", aiJSON(c)).Error
}
