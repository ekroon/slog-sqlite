package slogsqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LogEntry struct {
	ID             int64                  `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	Level          string                 `json:"level"`
	Message        string                 `json:"message"`
	SourceFile     string                 `json:"source_file,omitempty"`
	SourceLine     int                    `json:"source_line,omitempty"`
	SourceFunction string                 `json:"source_function,omitempty"`
	Attributes     map[string]interface{} `json:"attributes,omitempty"`
	Groups         []string               `json:"groups,omitempty"`
	ContextChain   string                 `json:"context_chain,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	StackTrace     string                 `json:"stack_trace,omitempty"`
}

type QueryOptions struct {
	Limit     int
	Offset    int
	Level     string
	Search    string
	StartTime *time.Time
	EndTime   *time.Time
	HasError  bool
	OrderDesc bool
}

func (h *SQLiteHandler) QueryLogs(opts QueryOptions) ([]LogEntry, error) {
	var conditions []string
	var args []interface{}

	if opts.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, opts.Level)
	}

	if opts.Search != "" {
		conditions = append(conditions, `
			id IN (
				SELECT rowid FROM logs_fts 
				WHERE logs_fts MATCH ?
			)
		`)
		args = append(args, opts.Search)
	}

	if opts.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, opts.StartTime.UTC().Format(time.RFC3339Nano))
	}

	if opts.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, opts.EndTime.UTC().Format(time.RFC3339Nano))
	}

	if opts.HasError {
		conditions = append(conditions, "error_message IS NOT NULL AND error_message != ''")
	}

	query := "SELECT id, timestamp, level, message, source_file, source_line, source_function, attributes, groups, context_chain, error_message, stack_trace FROM logs"

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if opts.OrderDesc {
		query += " ORDER BY timestamp DESC"
	} else {
		query += " ORDER BY timestamp ASC"
	}

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		var timestamp string
		var attrsJSON, groupsJSON sql.NullString
		var sourceFile, sourceFunction, contextChain, errorMessage, stackTrace sql.NullString
		var sourceLine sql.NullInt64

		err := rows.Scan(
			&log.ID,
			&timestamp,
			&log.Level,
			&log.Message,
			&sourceFile,
			&sourceLine,
			&sourceFunction,
			&attrsJSON,
			&groupsJSON,
			&contextChain,
			&errorMessage,
			&stackTrace,
		)
		if err != nil {
			return nil, err
		}

		log.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)

		if sourceFile.Valid {
			log.SourceFile = sourceFile.String
		}
		if sourceLine.Valid {
			log.SourceLine = int(sourceLine.Int64)
		}
		if sourceFunction.Valid {
			log.SourceFunction = sourceFunction.String
		}
		if contextChain.Valid {
			log.ContextChain = contextChain.String
		}
		if errorMessage.Valid {
			log.ErrorMessage = errorMessage.String
		}
		if stackTrace.Valid {
			log.StackTrace = stackTrace.String
		}

		if attrsJSON.Valid && attrsJSON.String != "" {
			json.Unmarshal([]byte(attrsJSON.String), &log.Attributes)
		}
		if groupsJSON.Valid && groupsJSON.String != "" {
			json.Unmarshal([]byte(groupsJSON.String), &log.Groups)
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func (h *SQLiteHandler) GetErrorSummary(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			error_message,
			COUNT(*) as count,
			MIN(timestamp) as first_seen,
			MAX(timestamp) as last_seen,
			GROUP_CONCAT(DISTINCT source_function) as functions
		FROM logs
		WHERE error_message IS NOT NULL AND error_message != ''
		GROUP BY error_message
		ORDER BY count DESC
		LIMIT ?
	`

	rows, err := h.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []map[string]interface{}
	for rows.Next() {
		var errorMsg string
		var count int
		var firstSeen, lastSeen string
		var functions sql.NullString

		err := rows.Scan(&errorMsg, &count, &firstSeen, &lastSeen, &functions)
		if err != nil {
			return nil, err
		}

		summary := map[string]interface{}{
			"error_message": errorMsg,
			"count":         count,
			"first_seen":    firstSeen,
			"last_seen":     lastSeen,
		}

		if functions.Valid {
			summary["functions"] = strings.Split(functions.String, ",")
		}

		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}

func (h *SQLiteHandler) GetExecutionFlow(startTime, endTime time.Time) ([]LogEntry, error) {
	return h.QueryLogs(QueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		OrderDesc: false,
	})
}

func (h *SQLiteHandler) GetContextAnalysis() (map[string]interface{}, error) {
	query := `
		SELECT 
			level,
			COUNT(*) as count
		FROM logs
		GROUP BY level
	`

	rows, err := h.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	levelCounts := make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		levelCounts[level] = count
	}

	totalQuery := `SELECT COUNT(*) FROM logs`
	var total int
	h.db.QueryRow(totalQuery).Scan(&total)

	errorRateQuery := `
		SELECT 
			CAST(COUNT(CASE WHEN error_message IS NOT NULL THEN 1 END) AS FLOAT) / 
			CAST(COUNT(*) AS FLOAT) * 100 as error_rate
		FROM logs
	`
	var errorRate float64
	h.db.QueryRow(errorRateQuery).Scan(&errorRate)

	return map[string]interface{}{
		"total_logs":   total,
		"level_counts": levelCounts,
		"error_rate":   errorRate,
	}, nil
}
