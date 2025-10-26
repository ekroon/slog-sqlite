package slogsqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNewSQLiteHandler(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	if handler.db == nil {
		t.Error("Database connection is nil")
	}
}

func TestHandlerLogging(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	logger := slog.New(handler)

	testCases := []struct {
		name    string
		logFunc func()
	}{
		{
			name: "Info log",
			logFunc: func() {
				logger.Info("Test info message",
					slog.String("key", "value"),
				)
			},
		},
		{
			name: "Error log",
			logFunc: func() {
				logger.Error("Test error message",
					slog.String("error", "something went wrong"),
				)
			},
		},
		{
			name: "With group",
			logFunc: func() {
				logger.WithGroup("test-group").Info("Grouped message")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.logFunc()

			time.Sleep(10 * time.Millisecond)

			logs, err := handler.QueryLogs(QueryOptions{
				Limit:     1,
				OrderDesc: true,
			})
			if err != nil {
				t.Fatalf("Failed to query logs: %v", err)
			}

			if len(logs) == 0 {
				t.Fatal("No logs found")
			}
		})
	}
}

func TestQueryLogs(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	logger := slog.New(handler)

	logger.Info("First message")
	logger.Debug("Debug message")
	logger.Error("Error message")

	time.Sleep(50 * time.Millisecond)

	t.Run("Query all logs", func(t *testing.T) {
		logs, err := handler.QueryLogs(QueryOptions{})
		if err != nil {
			t.Fatalf("Failed to query logs: %v", err)
		}

		if len(logs) != 3 {
			t.Errorf("Expected 3 logs, got %d", len(logs))
		}
	})

	t.Run("Query by level", func(t *testing.T) {
		logs, err := handler.QueryLogs(QueryOptions{
			Level: "ERROR",
		})
		if err != nil {
			t.Fatalf("Failed to query logs: %v", err)
		}

		if len(logs) != 1 {
			t.Errorf("Expected 1 error log, got %d", len(logs))
		}
	})

	t.Run("Query with limit", func(t *testing.T) {
		logs, err := handler.QueryLogs(QueryOptions{
			Limit: 2,
		})
		if err != nil {
			t.Fatalf("Failed to query logs: %v", err)
		}

		if len(logs) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs))
		}
	})
}

func TestFullTextSearch(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	logger := slog.New(handler)

	logger.Info("User authentication successful")
	logger.Error("Database connection failed")
	logger.Info("Cache updated successfully")

	time.Sleep(10 * time.Millisecond)

	var tableCount int
	err = handler.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='logs_fts'").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		t.Skip("FTS5 not available in this SQLite build")
	}

	logs, err := handler.QueryLogs(QueryOptions{
		Search: "database",
	})
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log matching 'database', got %d", len(logs))
	}

	if len(logs) > 0 && logs[0].Message != "Database connection failed" {
		t.Errorf("Wrong log message: %s", logs[0].Message)
	}
}

func TestWithAttrs(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("app", "test-app"),
		slog.Int("version", 1),
	})

	logger := slog.New(handlerWithAttrs)
	logger.Info("Test message with attributes")

	time.Sleep(10 * time.Millisecond)

	logs, err := handler.QueryLogs(QueryOptions{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("No logs found")
	}

	if logs[0].Attributes == nil {
		t.Fatal("Attributes are nil")
	}

	if app, ok := logs[0].Attributes["app"].(string); !ok || app != "test-app" {
		t.Errorf("Expected app='test-app', got %v", logs[0].Attributes["app"])
	}
}

func TestEnabled(t *testing.T) {
	dbPath := "test_logs.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelInfo,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	ctx := context.Background()

	if !handler.Enabled(ctx, slog.LevelError) {
		t.Error("ERROR level should be enabled")
	}

	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("INFO level should be enabled")
	}

	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("DEBUG level should not be enabled")
	}
}

func TestDirectSQLiteQuery(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 command not found in PATH")
	}

	dbPath := "test_direct_query.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	logger := slog.New(handler)

	logger.Info("Application started", slog.String("version", "1.0.0"))
	logger.Error("Database connection failed", slog.Any("error", errors.New("connection timeout")))
	logger.Debug("Processing batch", slog.Int("batch_id", 42))
	logger.Warn("Slow query detected", slog.String("query", "SELECT * FROM users"))

	_ = handler.Close()
	time.Sleep(50 * time.Millisecond)

	t.Run("Count total logs", func(t *testing.T) {
		cmd := exec.Command("sqlite3", dbPath, "SELECT COUNT(*) FROM logs")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v\nOutput: %s", err, output)
		}

		result := strings.TrimSpace(string(output))
		if result != "4" {
			t.Errorf("Expected 4 logs, got %s", result)
		}
	})

	t.Run("Query errors only", func(t *testing.T) {
		cmd := exec.Command("sqlite3", dbPath, "SELECT COUNT(*) FROM logs WHERE level = 'ERROR'")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "1" {
			t.Errorf("Expected 1 error log, got %s", result)
		}
	})

	t.Run("Full-text search", func(t *testing.T) {
		checkFTS := exec.Command("sqlite3", dbPath, "SELECT name FROM sqlite_master WHERE type='table' AND name='logs_fts'")
		ftsOutput, _ := checkFTS.CombinedOutput()

		if strings.TrimSpace(string(ftsOutput)) == "" {
			t.Skip("FTS5 not available, skipping full-text search test")
		}

		query := "SELECT message FROM logs WHERE id IN (SELECT rowid FROM logs_fts WHERE logs_fts MATCH 'database')"
		cmd := exec.Command("sqlite3", dbPath, query)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v\nOutput: %s", err, output)
		}

		result := strings.TrimSpace(string(output))
		if !strings.Contains(result, "Database connection failed") {
			t.Errorf("Expected to find 'Database connection failed', got: %s", result)
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		cmd := exec.Command("sqlite3", "-json", dbPath, "SELECT level, message FROM logs WHERE level = 'ERROR' LIMIT 1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		var results []map[string]interface{}
		if err := json.Unmarshal(output, &results); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if results[0]["level"] != "ERROR" {
			t.Errorf("Expected level ERROR, got %v", results[0]["level"])
		}
	})

	t.Run("Error summary", func(t *testing.T) {
		query := "SELECT error_message, COUNT(*) as count FROM logs WHERE error_message IS NOT NULL GROUP BY error_message"
		cmd := exec.Command("sqlite3", "-json", dbPath, query)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		var results []map[string]interface{}
		if err := json.Unmarshal(output, &results); err != nil {
			t.Fatalf("Failed to parse JSON output: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least one error message")
		}

		found := false
		for _, result := range results {
			if strings.Contains(result["error_message"].(string), "connection timeout") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find 'connection timeout' error")
		}
	})

	t.Run("Query with JSON attributes", func(t *testing.T) {
		query := "SELECT json_extract(attributes, '$.version') as version FROM logs WHERE json_extract(attributes, '$.version') IS NOT NULL"
		cmd := exec.Command("sqlite3", dbPath, query)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if !strings.Contains(result, "1.0.0") {
			t.Errorf("Expected to find version '1.0.0', got: %s", result)
		}
	})

	t.Run("Schema verification", func(t *testing.T) {
		cmd := exec.Command("sqlite3", dbPath, ".schema logs")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		schema := string(output)
		requiredColumns := []string{"timestamp", "level", "message", "source_file", "error_message", "attributes"}
		for _, col := range requiredColumns {
			if !strings.Contains(schema, col) {
				t.Errorf("Schema missing expected column: %s", col)
			}
		}
	})

	t.Run("FTS table exists", func(t *testing.T) {
		cmd := exec.Command("sqlite3", dbPath, "SELECT name FROM sqlite_master WHERE type='table' AND name='logs_fts'")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sqlite3: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result == "" {
			t.Skip("FTS5 not available in this SQLite build")
		}

		if result != "logs_fts" {
			t.Errorf("Expected logs_fts table, got: %s", result)
		}
	})
}

func TestAsyncHandling(t *testing.T) {
	dbPath := "test_async.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	logger := slog.New(handler)

	// Test that logging is non-blocking
	start := time.Now()
	for i := 0; i < 100; i++ {
		logger.Info("Test message", slog.Int("iteration", i))
	}
	elapsed := time.Since(start)

	// Logging 100 messages should be very fast if it's truly async
	// If it were synchronous with DB writes, it would take much longer
	if elapsed > 100*time.Millisecond {
		t.Errorf("Logging took too long (%v), suggesting it's not async", elapsed)
	}

	// Close the handler to ensure all goroutines complete
	if err := handler.Close(); err != nil {
		t.Fatalf("Failed to close handler: %v", err)
	}

	// Verify all logs were written
	logs, err := handler.QueryLogs(QueryOptions{})
	if err != nil {
		// After close, we can't query, so we'll check the database directly
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer func() { _ = db.Close() }()

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count logs: %v", err)
		}

		if count != 100 {
			t.Errorf("Expected 100 logs, got %d", count)
		}
	} else {
		if len(logs) != 100 {
			t.Errorf("Expected 100 logs, got %d", len(logs))
		}
	}
}

func TestSourceInformation(t *testing.T) {
	dbPath := "test_source.db"
	defer func() { _ = os.Remove(dbPath) }()

	handler, err := NewSQLiteHandler(&Options{
		Level:    slog.LevelDebug,
		Database: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer func() { _ = handler.Close() }()

	logger := slog.New(handler)

	// Log a message
	logger.Info("Test message for source info")

	time.Sleep(50 * time.Millisecond)

	// Verify source information was captured correctly
	logs, err := handler.QueryLogs(QueryOptions{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("No logs found")
	}

	log := logs[0]

	// Verify source file contains the test file name
	if !strings.Contains(log.SourceFile, "handler_test.go") {
		t.Errorf("Expected source file to contain 'handler_test.go', got: %s", log.SourceFile)
	}

	// Verify source line is set
	if log.SourceLine == 0 {
		t.Error("Source line should not be 0")
	}

	// Verify source function is set
	if log.SourceFunction == "" {
		t.Error("Source function should not be empty")
	}
}
