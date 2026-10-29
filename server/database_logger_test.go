package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDatabaseLoggerProtectsParametersAndDriverMessages(t *testing.T) {
	var output bytes.Buffer
	l := newDatabaseLogger(&output)
	ctx := context.Background()
	query, params := l.ParamsFilter(ctx, "SELECT * FROM buyer_profiles WHERE phone=?", "13900009999")
	if query != "SELECT * FROM buyer_profiles WHERE phone=?" || len(params) != 0 {
		t.Fatal("parameters were exposed to SQL interpolation")
	}
	secret := "contact-13900009999-quote-12345.6789-code-976315"
	begin := time.Now().Add(-time.Second)
	l.Trace(ctx, begin, func() (string, int64) { return "UPDATE quotes SET notes='" + secret + "'", 2 }, nil)
	l.Trace(ctx, time.Now(), func() (string, int64) { return "INSERT INTO quotes VALUES (?)", 0 }, &mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry '" + secret + "'"})
	l.Error(ctx, "connection failed: "+secret, errors.New(secret), secret)
	l.Warn(ctx, "warning: %s", secret)
	text := output.String()
	for _, fragment := range []string{secret, "13900009999", "12345.6789", "976315", "Duplicate entry", "SET notes"} {
		if strings.Contains(text, fragment) {
			t.Fatalf("sensitive database detail logged: %q", fragment)
		}
	}
	for _, fragment := range []string{"kind=slow", "operation=UPDATE", "rows=2", "duration_ms=", "query_id=", "error=mysql_1062", "source="} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing diagnostic field %q", fragment)
		}
	}
}

func TestDatabaseLoggerLevelsAndExpectedMisses(t *testing.T) {
	var output bytes.Buffer
	l := newDatabaseLogger(&output)
	query := func() (string, int64) { return "SELECT * FROM procurement_projects WHERE id=?", 0 }
	l.Trace(context.Background(), time.Now(), query, nil)
	l.Trace(context.Background(), time.Now().Add(-time.Second), query, gorm.ErrRecordNotFound)
	l.Info(context.Background(), "ordinary query")
	if output.Len() != 0 {
		t.Fatal("normal fast queries and expected not-found results should be quiet")
	}
	l.LogMode(logger.Info).Trace(context.Background(), time.Now(), query, context.DeadlineExceeded)
	if !strings.Contains(output.String(), "error=deadline") {
		t.Fatal("debug logging lost the error category")
	}
	output.Reset()
	l.Trace(context.Background(), time.Now(), query, nil)
	l.LogMode(logger.Silent).Trace(context.Background(), time.Now(), query, errors.New("not logged"))
	if output.Len() != 0 {
		t.Fatal("LogMode mutated the original logger or ignored Silent")
	}
}

func TestDatabaseLoggerGORMDryRunKeepsQueryFingerprintStable(t *testing.T) {
	// No server connection or database creation: version lookup and ping are
	// disabled, every statement is DryRun and implicit transactions are disabled.
	var output bytes.Buffer
	l := newDatabaseLogger(&output).LogMode(logger.Info)
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{Logger: l, DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, phone := range []string{"13900009998", "13900009999"} {
		var rows []BuyerProfile
		if err = db.Where("phone=?", phone).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	text := output.String()
	if strings.Contains(text, "1390000") {
		t.Fatal("GORM expanded parameters before passing the SQL to Trace")
	}
	var fingerprints []string
	for _, field := range strings.Fields(text) {
		if strings.HasPrefix(field, "query_id=") {
			fingerprints = append(fingerprints, field)
		}
	}
	if len(fingerprints) != 2 || fingerprints[0] != fingerprints[1] {
		t.Fatalf("query template identity changed with the parameter: %v", fingerprints)
	}
}
