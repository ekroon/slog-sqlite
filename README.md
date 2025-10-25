# slog-sqlite

A Go package that provides an SQLite backend for the `log/slog` package, optimized for LLM-based debugging and analysis.

## Features

- **Structured Logging**: Full support for slog's structured logging with attributes and groups
- **Full-Text Search**: Built-in FTS5 support for efficient log searching
- **LLM-Optimized Schema**: Flexible schema designed for easy querying by LLM agents
- **Error Tracking**: Special handling for errors with message extraction and stack traces
- **Source Code Context**: Automatic capture of source file, line number, and function names
- **Rich Querying**: Multiple query methods including time-based, level-based, and text search
- **Analytics**: Built-in methods for error summaries and execution flow analysis

## Installation

```bash
go get github.com/example/slog-sqlite
```

## Usage

### Basic Setup

```go
package main

import (
    "log/slog"
    slogsqlite "github.com/example/slog-sqlite"
)

func main() {
    handler, err := slogsqlite.NewSQLiteHandler(&slogsqlite.Options{
        Level:    slog.LevelDebug,
        Database: "app_logs.db",
    })
    if err != nil {
        panic(err)
    }
    defer handler.Close()

    logger := slog.New(handler)
    slog.SetDefault(logger)

    logger.Info("Application started",
        slog.String("version", "1.0.0"),
        slog.String("environment", "production"),
    )
}
```

### Querying Logs

```go
// Query recent errors
logs, err := handler.QueryLogs(slogsqlite.QueryOptions{
    Level:     "ERROR",
    Limit:     10,
    OrderDesc: true,
})

// Full-text search
logs, err := handler.QueryLogs(slogsqlite.QueryOptions{
    Search: "database timeout OR connection failed",
    Limit:  20,
})

// Time-based queries
startTime := time.Now().Add(-1 * time.Hour)
logs, err := handler.QueryLogs(slogsqlite.QueryOptions{
    StartTime: &startTime,
    HasError:  true,
})
```

### Error Analysis

```go
// Get error summary
summary, err := handler.GetErrorSummary(10)

// Get execution flow for debugging
flow, err := handler.GetExecutionFlow(startTime, endTime)

// Get context analysis
analysis, err := handler.GetContextAnalysis()
```

## Database Schema

The package creates a SQLite database with the following structure:

- `id`: Auto-incrementing primary key
- `timestamp`: RFC3339 formatted timestamp
- `level`: Log level (DEBUG, INFO, WARN, ERROR)
- `message`: Main log message
- `source_file`: Source file path
- `source_line`: Line number in source file
- `source_function`: Function name
- `attributes`: JSON-encoded attributes
- `groups`: JSON-encoded group hierarchy
- `context_chain`: Dot-separated group chain
- `error_message`: Extracted error message (if any)
- `stack_trace`: Stack trace (if provided)

## LLM Integration

This package is designed to be easily queryable by LLM agents for debugging:

1. **Structured Data**: All log data is stored in a structured format that's easy for LLMs to parse
2. **Full-Text Search**: LLMs can search for specific patterns or error messages
3. **Context Preservation**: Source code location and execution context are preserved
4. **Error Tracking**: Errors are extracted and indexed separately for easy analysis
5. **Flexible Attributes**: Arbitrary attributes stored as JSON for maximum flexibility

## Querying with SQLite3 CLI

You can query the log database directly using the `sqlite3` command-line tool:

### View Recent Logs

```bash
sqlite3 app_logs.db "SELECT timestamp, level, message FROM logs ORDER BY timestamp DESC LIMIT 10"
```

### Query Errors Only

```bash
sqlite3 app_logs.db "SELECT timestamp, message, error_message FROM logs WHERE level = 'ERROR' ORDER BY timestamp DESC"
```

### Full-Text Search

```bash
sqlite3 app_logs.db "SELECT message, level FROM logs WHERE id IN (SELECT rowid FROM logs_fts WHERE logs_fts MATCH 'database timeout')"
```

### Error Summary

```bash
sqlite3 app_logs.db "SELECT error_message, COUNT(*) as count FROM logs WHERE error_message IS NOT NULL GROUP BY error_message ORDER BY count DESC"
```

### Pretty Output with Headers

```bash
sqlite3 -header -column app_logs.db "SELECT id, datetime(timestamp) as time, level, message FROM logs LIMIT 10"
```

### JSON Output for LLM Processing

```bash
sqlite3 -json app_logs.db "SELECT * FROM logs WHERE level = 'ERROR' LIMIT 5"
```

### Export to CSV

```bash
sqlite3 -header -csv app_logs.db "SELECT timestamp, level, message, error_message FROM logs" > logs.csv
```

### Interactive Mode

```bash
sqlite3 app_logs.db
```

Once in interactive mode, you can run queries and use helpful commands:

```sql
-- View table schema
.schema logs

-- Show tables
.tables

-- Pretty output mode
.mode column
.headers on

-- Query logs
SELECT timestamp, level, message FROM logs ORDER BY timestamp DESC LIMIT 10;

-- Search with attributes
SELECT message, json_extract(attributes, '$.user_id') as user_id 
FROM logs 
WHERE json_extract(attributes, '$.user_id') IS NOT NULL;

-- Exit
.quit
```

### Advanced Queries for LLM Agents

```bash
# Get execution context for a specific time window
sqlite3 -json app_logs.db "
SELECT 
    timestamp,
    level,
    message,
    source_function,
    attributes,
    error_message
FROM logs 
WHERE timestamp BETWEEN '2024-01-01T10:00:00Z' AND '2024-01-01T11:00:00Z'
ORDER BY timestamp ASC
"

# Analyze error patterns
sqlite3 -column -header app_logs.db "
SELECT 
    error_message,
    COUNT(*) as occurrences,
    MIN(timestamp) as first_seen,
    MAX(timestamp) as last_seen,
    GROUP_CONCAT(DISTINCT source_function) as affected_functions
FROM logs
WHERE error_message IS NOT NULL
GROUP BY error_message
ORDER BY occurrences DESC
"

# Find related logs by context chain
sqlite3 app_logs.db "
SELECT timestamp, level, message, context_chain
FROM logs 
WHERE context_chain LIKE '%user-service%'
ORDER BY timestamp DESC
LIMIT 20
"
```

## Running the Example

```bash
cd example
go run main.go
```

This will create an `app_logs.db` file that you can query using the SQLite3 commands above.

## Testing

```bash
go test -v
```

## License

MIT
