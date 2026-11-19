package main

import (
	"context"
	"time"
)

// One bounded cloud processor across application instances, compatible with
// MySQL 5.7 development and MySQL 8 production. The connection owns the lock.
func (a *App) withCloudWorkerLock(ctx context.Context, work func(context.Context) bool) bool {
	pool, e := a.DB.DB()
	if e != nil {
		return false
	}
	connection, e := pool.Conn(ctx)
	if e != nil {
		return false
	}
	defer connection.Close()
	var acquired int
	if e = connection.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(),':oss-files'),0)").Scan(&acquired); e != nil || acquired != 1 {
		return false
	}
	defer connection.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(),':oss-files'))")
	bounded, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return work(bounded)
}
