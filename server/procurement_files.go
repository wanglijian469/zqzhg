package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func validateProcurementFile(path, extension string) (string, error) {
	data, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp":
		config, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || int64(config.Width)*int64(config.Height) > 100000000 || config.Width < 1 || config.Height < 1 {
			return "", procErr("invalid_image", 400)
		}
		if format == "jpeg" && (extension == ".jpg" || extension == ".jpeg") {
			return "image/jpeg", nil
		}
		if format == "png" && extension == ".png" {
			return "image/png", nil
		}
		if format == "webp" && extension == ".webp" {
			return "image/webp", nil
		}
		return "", procInvalid()
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return "", procErr("invalid_pdf", 400)
		}
		if _, e = pdfPages(path); e != nil {
			return "", procErr("invalid_pdf", 400)
		}
		return "application/pdf", nil
	case ".docx", ".xlsx":
		archive, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil || len(archive.File) > 2000 {
			return "", procErr("unsupported_file", 400)
		}
		var total uint64
		required := map[string]bool{"[Content_Types].xml": false}
		if extension == ".docx" {
			required["word/document.xml"] = false
		} else {
			required["xl/workbook.xml"] = false
		}
		seen := map[string]bool{}
		for _, f := range archive.File {
			name := strings.ToLower(f.Name)
			if seen[name] || strings.Contains(f.Name, "..") || strings.Contains(f.Name, "\\") || strings.HasPrefix(f.Name, "/") || strings.Contains(name, "vbaproject") || strings.Contains(name, "activex") || f.UncompressedSize64 > 20<<20 {
				return "", procErr("unsupported_file", 400)
			}
			seen[name] = true
			total += f.UncompressedSize64
			if total > 100<<20 {
				return "", procErr("file_too_large", 400)
			}
			if _, ok := required[f.Name]; ok {
				reader, re := f.Open()
				if re != nil {
					return "", procInvalid()
				}
				body, re := io.ReadAll(io.LimitReader(reader, (20<<20)+1))
				reader.Close()
				if re != nil || strings.Contains(strings.ToLower(string(body)), "macroenabled") {
					return "", procInvalid()
				}
				var document struct {
					XMLName   xml.Name
					Overrides []struct {
						PartName    string `xml:"PartName,attr"`
						ContentType string `xml:"ContentType,attr"`
					} `xml:"Override"`
				}
				if e := xml.Unmarshal(body, &document); e != nil {
					return "", procInvalid()
				}
				if f.Name == "[Content_Types].xml" {
					valid := false
					expectedPart := "/word/document.xml"
					expectedType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
					if extension == ".xlsx" {
						expectedPart = "/xl/workbook.xml"
						expectedType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
					}
					for _, override := range document.Overrides {
						if override.PartName == expectedPart && override.ContentType == expectedType {
							valid = true
						}
					}
					if document.XMLName.Local != "Types" || !valid {
						return "", procInvalid()
					}
				} else if extension == ".docx" && document.XMLName.Local != "document" || extension == ".xlsx" && document.XMLName.Local != "workbook" {
					return "", procInvalid()
				}
				required[f.Name] = true
			}
		}
		for _, exists := range required {
			if !exists {
				return "", procErr("unsupported_file", 400)
			}
		}
		if extension == ".docx" {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
		}
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	}
	return "", procErr("unsupported_file", 400)
}
func (a *App) procUploadAttachment(c *gin.Context) {
	if !procurementEnabled() {
		fail(c, 503, "proc_disabled")
		return
	}
	who := actor(c)
	if !verifiedPhone(who) {
		fail(c, 403, "phone_binding_required")
		return
	}
	if !permit(fmt.Sprintf("proc-upload:%d", who.ID), 60, time.Hour) {
		fail(c, 429, "rate_limited")
		return
	}
	var p ProcurementProject
	if e := a.DB.First(&p, id(c)).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	if _, _, e := a.procAccess(a.DB, who, p); e != nil {
		procResponse(c, nil, e)
		return
	}
	if p.Suspended || procClosed(p) {
		fail(c, 409, "proc_not_open")
		return
	}
	c.Request.Body = httpMaxBody(c, 21<<20)
	f, e := c.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if e != nil || f.Size < 1 || f.Size > 20<<20 {
		fail(c, 400, "file_too_large")
		return
	}
	name := filepath.Base(strings.ReplaceAll(f.Filename, "\\", "/"))
	if !procText(name, 200) {
		fail(c, 400, "proc_invalid")
		return
	}
	extension := strings.ToLower(filepath.Ext(name))
	full := a.workPath(randomToken() + extension)
	input, e := f.Open()
	if e != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	defer input.Close()
	output, e := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	defer os.Remove(full)
	n, e := io.Copy(output, io.LimitReader(input, (20<<20)+1))
	closeErr := output.Close()
	if e != nil || closeErr != nil || n > 20<<20 {
		fail(c, 400, "file_too_large")
		return
	}
	contentType, e := validateProcurementFile(full, extension)
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	lease, e := a.reserveUpload(who.ID, n)
	if e != nil {
		dbFail(c, e)
		return
	}
	defer lease.Release()
	key := "procurement/" + randomToken() + extension
	persisted := false
	defer func() {
		if !persisted {
			a.cleanupNewObjects(key)
		}
	}()
	if e = a.storeWorkFile(c.Request.Context(), key, full); e != nil {
		fail(c, 503, "storage_unavailable")
		return
	}
	if e = lease.Commit(key); e != nil {
		dbFail(c, e)
		return
	}
	attachment := ProcurementAttachment{ProjectID: p.ID, OwnerID: who.ID, ObjectKey: key, Name: name, ContentType: contentType, Size: n, CreatedAt: time.Now().UTC()}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		current, e := procLockProject(tx, p.ID)
		if e != nil {
			return e
		}
		if current.Suspended || procClosed(current) {
			return procErr("proc_not_open", 409)
		}
		if _, _, e = a.procAccess(tx, who, current); e != nil {
			return e
		}
		if e = tx.Create(&attachment).Error; e != nil {
			return e
		}
		return recordOperation(tx, who.ID, "procurement", p.ID, "procurement_attachment_uploaded", "")
	})
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	persisted = true
	c.JSON(201, attachment)
}
func (a *App) procAttachmentAccess(who Account, f ProcurementAttachment) error {
	var p ProcurementProject
	if e := a.DB.First(&p, f.ProjectID).Error; e != nil {
		return e
	}
	if platformAccount(who) {
		key := "procurement.read"
		if f.Scope == "quote" || f.Scope == "message" {
			key = "procurement.quote"
		}
		if !a.hasPermission(who, key) {
			return procMissing()
		}
		return nil
	}
	if f.OwnerID == who.ID {
		return nil
	}
	part, owner, e := a.procAccess(a.DB, who, p)
	if e != nil {
		return e
	}
	switch f.Scope {
	case "requirement":
		var revision ProcurementRevision
		if e = a.DB.First(&revision, f.ScopeID).Error; e != nil {
			return e
		}
		if owner || revision.Status == "approved" {
			return nil
		}
	case "quote":
		var revision ProcurementQuoteRevision
		var quote ProcurementQuote
		if e = a.DB.First(&revision, f.ScopeID).Error; e != nil {
			return e
		}
		if e = a.DB.First(&quote, revision.QuoteID).Error; e != nil {
			return e
		}
		if owner && revision.Status == "submitted" || part != nil && part.VendorID == quote.VendorID {
			return nil
		}
	case "message":
		var message ProcurementMessage
		if e = a.DB.First(&message, f.ScopeID).Error; e != nil {
			return e
		}
		if owner || part != nil && part.ID == message.ParticipantID {
			return nil
		}
	}
	return procMissing()
}
func (a *App) procReadAttachment(c *gin.Context) {
	var f ProcurementAttachment
	if e := a.DB.First(&f, id(c)).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	if e := a.procAttachmentAccess(actor(c), f); e != nil {
		procResponse(c, nil, e)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Robots-Tag", "noindex")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	disposition := "attachment"
	if c.Query("preview") == "1" && (strings.HasPrefix(f.ContentType, "image/") || f.ContentType == "application/pdf") {
		disposition = "inline"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	c.Header("Content-Type", f.ContentType)
	a.serveObject(c, f.ObjectKey)
}
