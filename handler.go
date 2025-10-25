package slogsqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteHandler struct {
	db     *sql.DB
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

type Options struct {
	Level    slog.Level
	Database string
}

func NewSQLiteHandler(opts *Options) (*SQLiteHandler, error) {
	if opts.Database == "" {
		opts.Database = "logs.db"
	}

	db, err := sql.Open("sqlite3", opts.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	handler := &SQLiteHandler{
		db:    db,
		level: opts.Level,
	}

	if err := handler.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return handler, nil
}

func (h *SQLiteHandler) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		source_file TEXT,
		source_line INTEGER,
		source_function TEXT,
		attributes TEXT,
		groups TEXT,
		context_chain TEXT,
		error_message TEXT,
		stack_trace TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
	CREATE INDEX IF NOT EXISTS idx_logs_message ON logs(message);
	CREATE INDEX IF NOT EXISTS idx_logs_error ON logs(error_message);
	`

	_, err := h.db.Exec(schema)
	if err != nil {
		return err
	}

	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(
		message,
		attributes,
		error_message,
		content=logs,
		content_rowid=id
	);

	CREATE TRIGGER IF NOT EXISTS logs_ai AFTER INSERT ON logs BEGIN
		INSERT INTO logs_fts(rowid, message, attributes, error_message)
		VALUES (new.id, new.message, new.attributes, new.error_message);
	END;
	`

	_, _ = h.db.Exec(ftsSchema)

	return nil
}

func (h *SQLiteHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *SQLiteHandler) Handle(ctx context.Context, r slog.Record) error {
	var sourceFile string
	var sourceLine int
	var sourceFunction string

	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		sourceFile = f.File
		sourceLine = f.Line
		sourceFunction = f.Function
	}

	attrs := make(map[string]interface{})

	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}

	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	attrsJSON, _ := json.Marshal(attrs)
	groupsJSON, _ := json.Marshal(h.groups)

	var errorMessage string
	var stackTrace string
	if err, ok := attrs["error"].(error); ok {
		errorMessage = err.Error()
	}
	if stack, ok := attrs["stack"].(string); ok {
		stackTrace = stack
	}

	contextChain := h.buildContextChain()

	query := `
		INSERT INTO logs (
			timestamp, level, message, source_file, source_line, 
			source_function, attributes, groups, context_chain,
			error_message, stack_trace
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := h.db.Exec(query,
		r.Time.UTC().Format(time.RFC3339Nano),
		r.Level.String(),
		r.Message,
		sourceFile,
		sourceLine,
		sourceFunction,
		string(attrsJSON),
		string(groupsJSON),
		contextChain,
		errorMessage,
		stackTrace,
	)

	return err
}

func (h *SQLiteHandler) buildContextChain() string {
	if len(h.groups) == 0 {
		return ""
	}
	return strings.Join(h.groups, ".")
}

func (h *SQLiteHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SQLiteHandler{
		db:     h.db,
		level:  h.level,
		attrs:  append(h.attrs, attrs...),
		groups: h.groups,
	}
}

func (h *SQLiteHandler) WithGroup(name string) slog.Handler {
	return &SQLiteHandler{
		db:     h.db,
		level:  h.level,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}

func (h *SQLiteHandler) Close() error {
	return h.db.Close()
}
