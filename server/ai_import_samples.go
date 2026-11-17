package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Explicit developer command. Never called from HTTP or production startup.
// Creates a private trial account and an ordinary queued import; it neither
// accepts agreements for a merchant nor approves/publishes any content.
func (a *App) aiImportSample(login, pdf string) error {
	if a.Config.Env != "development" || !regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`).MatchString(login) || !aiEnabled() {
		return errors.New("ai_sample_development_only")
	}
	var database string
	if e := a.DB.Raw("SELECT DATABASE()").Scan(&database).Error; e != nil || database != "zqzt_exhibition" {
		return errors.New("ai_sample_database_rejected")
	}
	dir := filepath.Join("..", ".local", "ai-import-samples")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	reportPath := filepath.Join(dir, login+".json")
	var saved struct {
		Password string `json:"password"`
	}
	if b, e := os.ReadFile(reportPath); e == nil {
		json.Unmarshal(b, &saved)
	}
	account := Account{}
	e := a.DB.Where("username=?", "ai-sample:"+login).First(&account).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		password := "Ai-" + randomToken()[:14] + "9!"
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		account = Account{Username: "ai-sample:" + login, LoginName: &login, PasswordHash: string(hash), Role: "merchant"}
		if e = a.DB.Create(&account).Error; e != nil {
			return errors.New("ai_sample_account_conflict")
		}
		saved.Password = password
	} else if e != nil {
		return e
	} else if account.Role != "merchant" || saved.Password == "" {
		return errors.New("ai_sample_private_credentials_missing")
	}
	session, e := a.newSession(account)
	if e != nil {
		return e
	}
	token := session["token"].(string)
	content, e := os.ReadFile(pdf)
	if e != nil {
		return errors.New("ai_sample_file_missing")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, e := form.CreateFormFile("files", filepath.Base(pdf))
	if e != nil {
		return e
	}
	file.Write(content)
	form.WriteField("request_id", "sample-"+randomToken())
	form.Close()
	req := httptest.NewRequest("POST", "/api/v1/merchant/ai-import/tasks", &body)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res := httptest.NewRecorder()
	a.router().ServeHTTP(res, req)
	var response map[string]any
	if e = json.Unmarshal(res.Body.Bytes(), &response); e != nil {
		return errors.New("ai_sample_invalid_response")
	}
	report := map[string]any{"login_name": login, "password": saved.Password, "account_id": account.ID, "token": token, "response": response, "http_status": res.Code, "private_only": true}
	b, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(reportPath, b, 0600); e != nil {
		return e
	}
	if res.Code != 200 && res.Code != 202 {
		return errors.New("ai_sample_upload_failed_see_private_report")
	}
	fmt.Println("Private sample credentials and task saved: " + reportPath)
	return nil
}
