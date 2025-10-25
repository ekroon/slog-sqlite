# Agent Guidelines for slog-sqlite

## Commands
- **Build**: `go build -v ./...`
- **Test all**: `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`
- **Test single**: `go test -v -run TestName ./...` (e.g., `go test -v -run TestNewSQLiteHandler`)
- **Lint**: `mise x ubi:golangci/golangci-lint -- golangci-lint run` or `go vet ./...`
- **Example**: `cd example && go build -v .`

## Code Style
- **Module**: `github.com/ekroon/slog-sqlite`, Go 1.21+
- **Package**: `slogsqlite` (single word, lowercase)
- **Imports**: Standard library first, blank imports for drivers (e.g., `_ "github.com/mattn/go-sqlite3"`), group external packages
- **Naming**: Exported types use PascalCase (e.g., `SQLiteHandler`, `QueryOptions`), unexported use camelCase
- **Error handling**: Always wrap errors with `fmt.Errorf("description: %w", err)`, never ignore errors
- **Types**: Use explicit types for struct fields with JSON tags using `json:"field_name,omitempty"` pattern
- **Testing**: Use table-driven tests with `t.Run()`, always defer cleanup (e.g., `defer os.Remove(dbPath)`), add `time.Sleep()` for async operations
- **Null handling**: Use `sql.Null*` types for nullable database fields
- **Context**: Accept `context.Context` for methods that may be long-running or cancellable
- **Database**: Use parameterized queries, never string concatenation for SQL
