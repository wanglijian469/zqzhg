package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQL parameters and driver messages may contain contact data, quotes or codes.
// Keep diagnostics (operation, stable template fingerprint, timing, rows, source
// and error code) without printing either the query or the driver's raw message.
type databaseLogger struct {
	out   *log.Logger
	level logger.LogLevel
}

func newDatabaseLogger(writer io.Writer) *databaseLogger {
	return &databaseLogger{out: log.New(writer, "", log.LstdFlags), level: logger.Warn}
}

func (l *databaseLogger) LogMode(level logger.LogLevel) logger.Interface {
	copy := *l
	copy.level = level
	return &copy
}

// GORM calls this before Explain; LogMode(Debug) must not enable interpolation.
func (l *databaseLogger) ParamsFilter(_ context.Context, query string, _ ...interface{}) (string, []interface{}) {
	return query, nil
}

func databaseErrorKind(err error) string {
	if err == nil {
		return "none"
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) {
		return fmt.Sprintf("mysql_%d", mysqlError.Number)
	}
	for _, entry := range []struct {
		err  error
		name string
	}{{context.Canceled, "cancelled"}, {context.DeadlineExceeded, "deadline"}, {gorm.ErrRecordNotFound, "not_found"}, {gorm.ErrDuplicatedKey, "duplicate_key"}, {gorm.ErrForeignKeyViolated, "foreign_key"}} {
		if errors.Is(err, entry.err) {
			return entry.name
		}
	}
	return fmt.Sprintf("%T", err)
}

func databaseLogSource() string {
	for skip := 2; skip < 20; skip++ {
		_, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		if filepath.Base(file) != "database_logger.go" && !strings.Contains(filepath.ToSlash(file), "gorm.io/") {
			return fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}
	}
	return "unknown"
}

func (l *databaseLogger) message(level logger.LogLevel, message string, args ...interface{}) {
	if l.level < level {
		return
	}
	kind := "none"
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			kind = databaseErrorKind(err)
		}
	}
	l.out.Printf("event=db_message level=%d message_id=%x error=%s source=%s", level, sha256.Sum256([]byte(message)), kind, databaseLogSource())
}

func (l *databaseLogger) Info(_ context.Context, message string, args ...interface{}) {
	l.message(logger.Info, message, args...)
}
func (l *databaseLogger) Warn(_ context.Context, message string, args ...interface{}) {
	l.message(logger.Warn, message, args...)
}
func (l *databaseLogger) Error(_ context.Context, message string, args ...interface{}) {
	l.message(logger.Error, message, args...)
}

func (l *databaseLogger) Trace(_ context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.level <= logger.Silent || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	elapsed := time.Since(begin)
	kind := "query"
	if err != nil && l.level >= logger.Error {
		kind = "error"
	} else if elapsed >= 200*time.Millisecond && l.level >= logger.Warn {
		kind = "slow"
	} else if l.level < logger.Info {
		return
	}
	sql, rows := query()
	operation := "OTHER"
	if fields := strings.Fields(sql); len(fields) > 0 {
		switch strings.ToUpper(fields[0]) {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "SHOW", "BEGIN", "COMMIT", "ROLLBACK":
			operation = strings.ToUpper(fields[0])
		}
	}
	l.out.Printf("event=db_query kind=%s operation=%s query_id=%x duration_ms=%.3f rows=%d error=%s source=%s", kind, operation, sha256.Sum256([]byte(sql)), float64(elapsed.Microseconds())/1000, rows, databaseErrorKind(err), databaseLogSource())
}
