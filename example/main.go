package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	slogsqlite "github.com/ekroon/slog-sqlite"
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
		slog.String("environment", "development"),
	)

	processUser := logger.With(
		slog.String("component", "user-service"),
		slog.Int("user_id", 12345),
	)

	processUser.Debug("Processing user request",
		slog.String("action", "profile_update"),
	)

	simulateWork(processUser)

	processUser.Error("Failed to update user profile",
		slog.Any("error", errors.New("database connection timeout")),
		slog.String("stack", "main.simulateWork:45\nmain.processUser:32\nmain.main:28"),
	)

	fmt.Println("\n=== Querying Logs ===")

	recentLogs, err := handler.QueryLogs(slogsqlite.QueryOptions{
		Limit:     10,
		OrderDesc: true,
	})
	if err != nil {
		logger.Error("Failed to query logs", slog.Any("error", err))
	}

	for _, log := range recentLogs {
		fmt.Printf("[%s] %s: %s\n", log.Timestamp.Format("15:04:05"), log.Level, log.Message)
		if log.ErrorMessage != "" {
			fmt.Printf("  Error: %s\n", log.ErrorMessage)
		}
	}

	fmt.Println("\n=== Error Summary ===")
	errorSummary, err := handler.GetErrorSummary(5)
	if err != nil {
		logger.Error("Failed to get error summary", slog.Any("error", err))
	}

	for _, summary := range errorSummary {
		fmt.Printf("Error: %s (occurred %d times)\n",
			summary["error_message"],
			summary["count"],
		)
	}

	fmt.Println("\n=== Full-Text Search Example ===")
	searchResults, err := handler.QueryLogs(slogsqlite.QueryOptions{
		Search: "database OR timeout",
		Limit:  5,
	})
	if err != nil {
		logger.Error("Failed to search logs", slog.Any("error", err))
	}

	for _, log := range searchResults {
		fmt.Printf("Found: %s\n", log.Message)
	}

	fmt.Println("\n=== Context Analysis ===")
	analysis, err := handler.GetContextAnalysis()
	if err != nil {
		logger.Error("Failed to get context analysis", slog.Any("error", err))
	}

	fmt.Printf("Total logs: %d\n", analysis["total_logs"])
	fmt.Printf("Error rate: %.2f%%\n", analysis["error_rate"])
	if levelCounts, ok := analysis["level_counts"].(map[string]int); ok {
		for level, count := range levelCounts {
			fmt.Printf("  %s: %d\n", level, count)
		}
	}
}

func simulateWork(logger *slog.Logger) {
	ctx := context.Background()

	logger.InfoContext(ctx, "Starting data processing",
		slog.Int("batch_size", 1000),
	)

	for i := 0; i < 3; i++ {
		logger.Debug("Processing batch",
			slog.Int("batch_id", i),
			slog.Float64("progress", float64(i+1)/3*100),
		)
		time.Sleep(100 * time.Millisecond)
	}

	logger.Warn("Slow query detected",
		slog.Duration("query_time", 2*time.Second),
		slog.String("query", "SELECT * FROM users WHERE ..."),
	)
}
