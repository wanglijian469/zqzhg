package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"gorm.io/gorm"
)

// Platform and enterprise publication use different policy resources. A probe
// must also reject anonymous access outside them before enabling publication.
func (a *App) verifyPublicFileScopes(ctx context.Context, profile StorageProfile, origin string, data []byte, transport *http.Client) error {
	var vendor Content
	err := a.DB.Select("id").Where("kind='vendor'").Order("id").First(&vendor).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// Zero is a reserved synthetic scope when no enterprise exists yet. No
	// enterprise record or business association is created for readiness probes.
	companyPrefix := fmt.Sprintf("companies/%d/published/", vendor.ID)
	targets := []struct {
		prefix   string
		readable bool
		failure  string
	}{
		{"platform/published/preflight/", true, "oss_public_unverified"},
		{companyPrefix + "preflight-", true, "oss_company_public_unverified"},
		{"upload-temp/system-preflight/", false, "oss_public_bucket_prefix_required"},
	}
	client, err := a.cloudClient(profile, false)
	if err != nil {
		return err
	}
	for _, target := range targets {
		err = func() error {
			key := target.prefix + randomToken() + ".bin"
			_, err := client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(profile.Bucket), Key: oss.Ptr(key), Body: bytes.NewReader(data), ContentLength: oss.Ptr(int64(len(data))), ContentType: oss.Ptr("application/octet-stream"), ForbidOverwrite: oss.Ptr("true"), CacheControl: oss.Ptr("no-store")})
			if err != nil {
				return cloudError(err)
			}
			clean := false
			defer func() {
				if clean {
					return
				}
				cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
				defer stop()
				client.DeleteObject(cleanup, &oss.DeleteObjectRequest{Bucket: oss.Ptr(profile.Bucket), Key: oss.Ptr(key)})
			}()
			address, err := a.publicObjectURL(profile, key)
			if err != nil {
				return err
			}
			request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
			if err != nil {
				return errors.New(target.failure)
			}
			request.Header.Set("Origin", origin)
			response, err := transport.Do(request)
			if err != nil {
				return errors.New(target.failure)
			}
			body, readErr := io.ReadAll(io.LimitReader(response.Body, int64(len(data)+1)))
			response.Body.Close()
			if target.readable {
				if readErr != nil || response.StatusCode != 200 || !bytes.Equal(body, data) || response.Header.Get("Access-Control-Allow-Origin") != origin {
					return errors.New(target.failure)
				}
				if !cloudCORSExposes(response.Header, "ETag", "Content-Length", "Content-Range", "Accept-Ranges") {
					return errors.New("oss_cors_expose_headers_required")
				}
			} else if response.StatusCode != 403 && response.StatusCode != 404 {
				return errors.New(target.failure)
			}
			if _, err = client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(profile.Bucket), Key: oss.Ptr(key)}); err != nil {
				return cloudError(err)
			}
			_, err = client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(profile.Bucket), Key: oss.Ptr(key)})
			var service *oss.ServiceError
			if !errors.As(err, &service) || service.StatusCode != 404 {
				return errors.New("oss_unavailable")
			}
			clean = true
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
