package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCloudMigration039RollbackGuard(t *testing.T) {
	a := importTestAppAtSchema(t, 38)
	if err := migrate(a.DB); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../deploy/linux/rollback-oss-039.sql")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	start := strings.Index(script, "CREATE PROCEDURE")
	end := strings.Index(script, "END//")
	pool, _ := a.DB.DB()
	conn, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	execSQL := func(query string) error { _, e := conn.ExecContext(context.Background(), query); return e }
	if err = execSQL(script[start : end+3]); err != nil {
		t.Fatal(err)
	}
	if execSQL("CALL guard_oss_039_downgrade()") == nil {
		t.Fatal("downgrade accepted without metadata export acknowledgement")
	}
	if err = execSQL("SET @oss_metadata_export_verified=1"); err != nil {
		t.Fatal(err)
	}
	if err = execSQL("INSERT INTO file_backup_pins(snapshot_id,object_key,expires_at) VALUES('guard-fixture','private-file',DATE_ADD(UTC_TIMESTAMP(),INTERVAL 1 HOUR))"); err != nil {
		t.Fatal(err)
	}
	if execSQL("CALL guard_oss_039_downgrade()") == nil {
		t.Fatal("downgrade ignored protected objects")
	}
	if !a.DB.Migrator().HasColumn(&StoredObject{}, "remote_key") {
		t.Fatal("failed guard removed remote mapping")
	}
	if err = execSQL("DELETE FROM file_backup_pins WHERE snapshot_id='guard-fixture'"); err != nil {
		t.Fatal(err)
	}
	if err = execSQL("CALL guard_oss_039_downgrade()"); err != nil {
		t.Fatal(err)
	}
	start = strings.Index(script, "DROP PROCEDURE guard_oss_039_downgrade;")
	for _, query := range strings.Split(script[start:], ";") {
		if strings.TrimSpace(query) != "" {
			if err = execSQL(query); err != nil {
				t.Fatal(err)
			}
		}
	}
	if a.DB.Migrator().HasColumn(&StoredObject{}, "remote_key") || a.DB.Migrator().HasTable(&EnterpriseFile{}) {
		t.Fatal("downgrade did not remove schema 039")
	}
	conn.Close()
	if err = migrate(a.DB); err != nil {
		t.Fatal("039 cannot be reapplied", err)
	}
	if !a.DB.Migrator().HasColumn(&StoredObject{}, "remote_key") {
		t.Fatal("039 was not reapplied")
	}
}
