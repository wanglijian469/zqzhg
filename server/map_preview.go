package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const previewOrigin = "https://geo.datav.aliyun.com/areas_v3/bound/"
const previewUsage = "local-learning-only"

var previewFilePattern = regexp.MustCompile(`^[0-9]{6}(_full)?$`)

type previewSource struct {
	URL        string `json:"url"`
	SHA256     string `json:"sha256"`
	AcquiredAt string `json:"acquired_at"`
	Usage      string `json:"usage"`
}
type previewFeature struct {
	Type       string          `json:"type"`
	Properties map[string]any  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}
type previewGeometry struct {
	Type     string           `json:"type"`
	Features []previewFeature `json:"features"`
}
type previewBoundary struct {
	RegionCode     string          `json:"region_code"`
	Geometry       previewGeometry `json:"geometry"`
	Sources        []previewSource `json:"sources"`
	MissingCodes   []string        `json:"missing_codes"`
	UnmappedCount  int             `json:"unmapped_count"`
	SHA256         string          `json:"sha256"`
	Usage          string          `json:"usage"`
	CatalogVersion string          `json:"catalog_version"`
}
type previewResolver struct {
	app       *App
	ctx       context.Context
	sources   map[string]previewSource
	directory string
}

func (a *App) mapPreviewAllowed(c *gin.Context) bool {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(host)
	requestHost := c.Request.Host
	if h, _, e := net.SplitHostPort(requestHost); e == nil {
		requestHost = h
	}
	hostIP := net.ParseIP(strings.Trim(requestHost, "[]"))
	return a.Config.Env == "development" && a.Config.MapPreviewEnabled && ip != nil && ip.IsLoopback() &&
		(strings.EqualFold(requestHost, "localhost") || hostIP != nil && hostIP.IsLoopback()) &&
		c.GetHeader("Forwarded") == "" && c.GetHeader("X-Forwarded-For") == ""
}
func (a *App) mapPreviewConfiguration(c *gin.Context) {
	if !a.mapPreviewAllowed(c) {
		fail(c, 404, "not_found")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"enabled": true, "usage": previewUsage, "source": "DataV / 高德开放平台", "source_terms": "https://help.aliyun.com/zh/datav/datav-7-0/user-guide/datav-geoatlas-widgets/"})
}
func previewDigest(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
func previewCode(feature previewFeature) string { return fmt.Sprint(feature.Properties["adcode"]) }
func previewName(feature previewFeature) string {
	value, _ := feature.Properties["name"].(string)
	return value
}
func previewAtomicFile(path string, bytes []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".preview-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(bytes); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func (a *App) mapPreviewDirectory() (string, error) {
	dir := a.Config.MapPreviewDir
	if dir == "" {
		dir = "../.local/map-preview"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if a.Config.WebDir != "" {
		web, e := filepath.Abs(a.Config.WebDir)
		if e != nil {
			return "", e
		}
		relative, e := filepath.Rel(web, abs)
		if e != nil {
			return "", e
		}
		if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("preview cache must be outside public web directory")
		}
	}
	return abs, os.MkdirAll(abs, 0700)
}
func (r *previewResolver) source(file string) (previewGeometry, error) {
	var geometry previewGeometry
	if !previewFilePattern.MatchString(file) {
		return geometry, fmt.Errorf("invalid preview file")
	}
	rawPath := filepath.Join(r.directory, file+".json")
	metaPath := filepath.Join(r.directory, file+".metadata.json")
	bytes, readErr := os.ReadFile(rawPath)
	metadata, metaErr := os.ReadFile(metaPath)
	var source previewSource
	validCache := readErr == nil && metaErr == nil && json.Unmarshal(metadata, &source) == nil && source.URL == previewOrigin+file+".json" && source.Usage == previewUsage && source.SHA256 == previewDigest(bytes)
	if !validCache {
		client := r.app.mapPreviewClient
		if client == nil {
			client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		}
		request, err := http.NewRequestWithContext(r.ctx, "GET", previewOrigin+file+".json", nil)
		if err != nil {
			return geometry, err
		}
		response, err := client.Do(request)
		if err != nil {
			return geometry, err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return geometry, fmt.Errorf("boundary source returned %d", response.StatusCode)
		}
		bytes, err = io.ReadAll(io.LimitReader(response.Body, (12<<20)+1))
		if err != nil {
			return geometry, err
		}
		if len(bytes) > 12<<20 {
			return geometry, fmt.Errorf("boundary too large")
		}
		source = previewSource{previewOrigin + file + ".json", previewDigest(bytes), time.Now().UTC().Format(time.RFC3339), previewUsage}
	}
	if json.Unmarshal(bytes, &geometry) != nil || geometry.Type != "FeatureCollection" || len(geometry.Features) == 0 {
		return geometry, fmt.Errorf("invalid boundary data")
	}
	for _, feature := range geometry.Features {
		var shape struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		}
		if feature.Type != "Feature" || json.Unmarshal(feature.Geometry, &shape) != nil || (shape.Type != "Polygon" && shape.Type != "MultiPolygon") || len(shape.Coordinates) == 0 || len(feature.Properties) == 0 {
			return geometry, fmt.Errorf("invalid boundary feature")
		}
	}
	if !validCache {
		meta, _ := json.MarshalIndent(source, "", "  ")
		if err := previewAtomicFile(rawPath, bytes); err != nil {
			return geometry, err
		}
		if err := previewAtomicFile(metaPath, meta); err != nil {
			return geometry, err
		}
	}
	r.sources[file] = source
	return geometry, nil
}
func previewMunicipal(node *AdministrativeRegion) bool {
	return node != nil && node.Level == 2 && (node.ParentCode == "11" || node.ParentCode == "12" || node.ParentCode == "31" || node.ParentCode == "50")
}
func previewMatch(feature previewFeature, candidates []AdministrativeRegion) *AdministrativeRegion {
	code := previewCode(feature)
	name := previewName(feature)
	var matched *AdministrativeRegion
	for _, node := range candidates {
		if node.Name != name {
			continue
		}
		if node.Code == code {
			return &node
		}
		if matched != nil {
			return nil
		}
		copy := node
		matched = &copy
	}
	return matched
}
func (r *previewResolver) sourceCode(node AdministrativeRegion) (string, error) {
	if previewMunicipal(&node) {
		return r.sourceCode(administrativeByCode[node.ParentCode])
	}
	file := "100000_full"
	if node.ParentCode != "" {
		parent, err := r.sourceCode(administrativeByCode[node.ParentCode])
		if err != nil {
			return "", err
		}
		file = parent + "_full"
	}
	geometry, err := r.source(file)
	if err != nil {
		return "", err
	}
	var match string
	for _, feature := range geometry.Features {
		// An existing complete county code is authoritative only with the same name.
		if previewName(feature) == node.Name {
			code := previewCode(feature)
			if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
				continue
			}
			if code == node.Code {
				return code, nil
			}
			if match != "" {
				return "", fmt.Errorf("ambiguous region name")
			}
			match = code
		}
	}
	if match == "" {
		return "", fmt.Errorf("no exact boundary mapping for %s", node.Code)
	}
	return match, nil
}
func (r *previewResolver) boundary(node *AdministrativeRegion) (previewBoundary, error) {
	result := previewBoundary{Usage: previewUsage, CatalogVersion: "2023-06-30", MissingCodes: []string{}, Sources: []previewSource{}, Geometry: previewGeometry{Type: "FeatureCollection", Features: []previewFeature{}}}
	file := "100000_full"
	children := administrativeChildren[""]
	sourceCode := ""
	if node != nil {
		result.RegionCode = node.Code
		var err error
		sourceCode, err = r.sourceCode(*node)
		if err != nil {
			return result, err
		}
		children = administrativeChildren[node.Code]
		file = sourceCode + "_full"
		if !node.HasChildren {
			file = sourceCode
		}
		// A single virtual city level is the same physical outline as its province.
		if node.Level == 1 && len(children) == 1 && previewMunicipal(&children[0]) {
			file = sourceCode
		}
	}
	geometry, err := r.source(file)
	if err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for _, feature := range geometry.Features {
		var matched *AdministrativeRegion
		if node != nil && !node.HasChildren {
			copy := *node
			matched = &copy
		} else if node != nil && node.Level == 1 && len(children) == 1 && previewMunicipal(&children[0]) {
			copy := children[0]
			matched = &copy
		} else if node != nil && node.Code == "50" {
			county, ok := administrativeByCode[previewCode(feature)]
			if ok && administrativeByCode[county.ParentCode].ParentCode == "50" {
				copy := administrativeByCode[county.ParentCode]
				matched = &copy
			}
		} else {
			matched = previewMatch(feature, children)
			if node != nil && previewMunicipal(node) && matched == nil {
				continue
			}
		}
		props := map[string]any{"adcode": "context:" + previewCode(feature), "name": previewName(feature), "source_adcode": previewCode(feature), "mapped": false}
		if matched != nil {
			props["adcode"] = matched.Code
			props["name"] = matched.Name
			props["mapped"] = true
			seen[matched.Code] = true
		} else if previewCode(feature) == "100000_JD" {
			props["decoration"] = true
		} else {
			result.UnmappedCount++
		}
		// Retain source label centres only for one-to-one geometries, never grouped counties.
		if matched != nil && !(node != nil && node.Code == "50") {
			if cp, ok := feature.Properties["centroid"]; ok {
				props["cp"] = cp
			} else if cp, ok := feature.Properties["center"]; ok {
				props["cp"] = cp
			}
		}
		result.Geometry.Features = append(result.Geometry.Features, previewFeature{Type: "Feature", Properties: props, Geometry: feature.Geometry})
	}
	for _, child := range children {
		if !seen[child.Code] {
			result.MissingCodes = append(result.MissingCodes, child.Code)
		}
	}
	if node != nil && !node.HasChildren {
		result.MissingCodes = []string{}
	}
	for _, source := range r.sources {
		result.Sources = append(result.Sources, source)
	}
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].URL < result.Sources[j].URL })
	payload, _ := json.Marshal(result.Geometry)
	result.SHA256 = previewDigest(payload)
	mapping, _ := json.MarshalIndent(result, "", "  ")
	name := result.RegionCode
	if name == "" {
		name = "national"
	}
	if err := previewAtomicFile(filepath.Join(r.directory, name+".mapping.json"), mapping); err != nil {
		return result, err
	}
	return result, nil
}
func (a *App) mapPreviewRegions(c *gin.Context) {
	if !a.mapPreviewAllowed(c) {
		fail(c, 404, "not_found")
		return
	}
	node, ok := industryRegion(c)
	if !ok {
		return
	}
	a.mapPreviewMu.Lock()
	defer a.mapPreviewMu.Unlock()
	directory, err := a.mapPreviewDirectory()
	if err != nil {
		fail(c, 503, "map_preview_unavailable")
		return
	}
	resolver := previewResolver{app: a, ctx: c.Request.Context(), sources: map[string]previewSource{}, directory: directory}
	result, err := resolver.boundary(node)
	if err != nil {
		fail(c, 502, "map_preview_unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, result)
}
