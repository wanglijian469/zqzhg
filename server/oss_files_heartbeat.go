package main

import (
	"context"
	"time"
)

func (a *App) cloudLeaseContext(parent context.Context, kind, id, token string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				query := a.DB.WithContext(ctx).Model(&FileStorageJob{}).Where("id=? AND lease_token=? AND status='processing'", id, token)
				if kind == "upload" {
					query = a.DB.WithContext(ctx).Model(&FileUploadSession{}).Where("id=? AND lease_token=? AND upload_state='validating'", id, token)
				}
				result := query.Updates(map[string]any{"lease_until": time.Now().UTC().Add(2 * time.Minute), "updated_at": time.Now().UTC()})
				if result.Error != nil || result.RowsAffected != 1 {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}
