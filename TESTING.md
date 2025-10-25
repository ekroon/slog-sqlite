# Testing Guide

This document describes the test suite for slog-sqlite, including tests that directly query the SQLite database using the `sqlite3` command-line tool.

## Running Tests

### Run all tests
```bash
go test -v
```

### Run specific test
```bash
go test -v -run TestDirectSQLiteQuery
```

## Test Categories

### 1. Handler Tests
Standard Go tests that verify the slog.Handler implementation:
- `TestNewSQLiteHandler` - Handler initialization
- `TestHandlerLogging` - Basic logging functionality
- `TestQueryLogs` - Go API query methods
- `TestFullTextSearch` - FTS5 search (if available)
- `TestWithAttrs` - Attribute handling
- `TestEnabled` - Log level filtering

### 2. Direct SQLite CLI Tests
The `TestDirectSQLiteQuery` test suite verifies that logs can be queried directly using the `sqlite3` command-line tool. This is important for:
- LLM agents that need to query logs directly
- Manual debugging and inspection
- Integration with existing SQLite tools

#### Test Cases:

**Count total logs**
```bash
sqlite3 test_direct_query.db "SELECT COUNT(*) FROM logs"
```

**Query errors only**
```bash
sqlite3 test_direct_query.db "SELECT COUNT(*) FROM logs WHERE level = 'ERROR'"
```

**Full-text search** (requires FTS5)
```bash
sqlite3 test_direct_query.db "SELECT message FROM logs WHERE id IN (SELECT rowid FROM logs_fts WHERE logs_fts MATCH 'database')"
```

**JSON output format**
```bash
sqlite3 -json test_direct_query.db "SELECT level, message FROM logs WHERE level = 'ERROR' LIMIT 1"
```

**Error summary**
```bash
sqlite3 -json test_direct_query.db "SELECT error_message, COUNT(*) as count FROM logs WHERE error_message IS NOT NULL GROUP BY error_message"
```

**Query with JSON attributes**
```bash
sqlite3 test_direct_query.db "SELECT json_extract(attributes, '$.version') as version FROM logs WHERE json_extract(attributes, '$.version') IS NOT NULL"
```

**Schema verification**
```bash
sqlite3 test_direct_query.db ".schema logs"
```

**FTS table verification** (requires FTS5)
```bash
sqlite3 test_direct_query.db "SELECT name FROM sqlite_master WHERE type='table' AND name='logs_fts'"
```

## FTS5 Support

Full-text search (FTS5) is optional. Tests will skip FTS-related functionality if FTS5 is not available in your SQLite build.

To check if your SQLite has FTS5 support:
```bash
sqlite3 :memory: "CREATE VIRTUAL TABLE test USING fts5(content);"
```

If you get an error about "no such module: fts5", then FTS5 is not available.

### Building SQLite with FTS5

If you need FTS5 support, you can build the Go SQLite driver with FTS5 enabled:

```bash
go get -tags "fts5" github.com/mattn/go-sqlite3
```

Or set build tags in your test:
```bash
go test -tags fts5 -v
```

## Test Database

Tests create temporary databases that are automatically cleaned up:
- `test_logs.db` - Used by most handler tests
- `test_direct_query.db` - Used by CLI query tests

You can inspect these databases manually during debugging by commenting out the `defer os.Remove(dbPath)` lines.

## Example Test Output

```
=== RUN   TestDirectSQLiteQuery
=== RUN   TestDirectSQLiteQuery/Count_total_logs
=== RUN   TestDirectSQLiteQuery/Query_errors_only
=== RUN   TestDirectSQLiteQuery/Full-text_search
    handler_test.go:327: FTS5 not available, skipping full-text search test
=== RUN   TestDirectSQLiteQuery/JSON_output_format
=== RUN   TestDirectSQLiteQuery/Error_summary
=== RUN   TestDirectSQLiteQuery/Query_with_JSON_attributes
=== RUN   TestDirectSQLiteQuery/Schema_verification
=== RUN   TestDirectSQLiteQuery/FTS_table_exists
    handler_test.go:432: FTS5 not available in this SQLite build
--- PASS: TestDirectSQLiteQuery (0.09s)
    --- PASS: TestDirectSQLiteQuery/Count_total_logs (0.01s)
    --- PASS: TestDirectSQLiteQuery/Query_errors_only (0.00s)
    --- SKIP: TestDirectSQLiteQuery/Full-text_search (0.00s)
    --- PASS: TestDirectSQLiteQuery/JSON_output_format (0.00s)
    --- PASS: TestDirectSQLiteQuery/Error_summary (0.00s)
    --- PASS: TestDirectSQLiteQuery/Query_with_JSON_attributes (0.00s)
    --- PASS: TestDirectSQLiteQuery/Schema_verification (0.00s)
    --- SKIP: TestDirectSQLiteQuery/FTS_table_exists (0.00s)
```

## Debugging Failed Tests

If a test fails, you can run it with verbose output and inspect the database:

```bash
# Run specific test
go test -v -run TestDirectSQLiteQuery/Error_summary

# If you need to preserve the database for inspection,
# comment out the defer os.Remove(dbPath) line in the test
# and then manually inspect:
sqlite3 test_direct_query.db
sqlite> .schema
sqlite> SELECT * FROM logs;
```
