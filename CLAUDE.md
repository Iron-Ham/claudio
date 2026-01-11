# Claudio Development Guidelines

## Go Best Practices

### Code Formatting

All Go code must be properly formatted before committing:

```bash
# Format all Go files
gofmt -w .

# Or use goimports to also organize imports
goimports -w .
```

- Run `gofmt -d .` to check for formatting issues without modifying files
- Imports should be organized in groups: standard library, external packages, internal packages

### Linting

Run static analysis before committing:

```bash
# Basic linting (always run)
go vet ./...

# If golangci-lint is available (recommended)
golangci-lint run
```

Address all linting warnings before committing code.

### Building

```bash
# Build the project
go build ./...

# Ensure the build succeeds before committing
```

## Testing Requirements

### Coverage Expectations

- **All new code must have corresponding tests**
- **Target: 100% test coverage on new code**
- Tests should live alongside the code they test (e.g., `foo.go` and `foo_test.go` in the same package)

If 100% coverage isn't achievable, document why in a code comment. Acceptable exceptions:
- `main()` functions and CLI entrypoints
- Defensive error handling that's unreachable in practice
- Platform-specific code paths that can't run in the test environment
- Code that requires external services that can't be reasonably mocked

For these cases, add a comment like:
```go
// Coverage: This branch handles [scenario] which requires [external dependency/condition]
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage report
go test -cover ./...

# Run tests with detailed coverage output
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View in browser

# Run tests for a specific package
go test ./internal/config/...

# Run tests with verbose output
go test -v ./...
```

### Test Patterns

This project uses standard Go testing conventions:

1. **Table-driven tests** - Preferred for testing multiple cases:
   ```go
   func TestFoo(t *testing.T) {
       tests := []struct {
           name     string
           input    string
           expected string
       }{
           {"empty input", "", ""},
           {"normal input", "hello", "HELLO"},
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got := Foo(tt.input)
               if got != tt.expected {
                   t.Errorf("Foo(%q) = %q, want %q", tt.input, got, tt.expected)
               }
           })
       }
   }
   ```

2. **Subtests** - Use `t.Run()` for grouping related test cases

3. **Error messages** - Use descriptive error messages that show got vs want:
   ```go
   t.Errorf("FunctionName() = %v, want %v", got, want)
   ```

4. **Test helpers** - Mark helper functions with `t.Helper()` for better error reporting

### What to Test

- Public functions and methods
- Edge cases and error conditions
- Concurrent behavior where applicable
- Integration points between packages

## Pre-Commit Checklist

Before committing, ensure:

1. Code is formatted: `gofmt -d .` shows no output
2. Linting passes: `go vet ./...` has no errors
3. Build succeeds: `go build ./...`
4. All tests pass: `go test ./...`
5. New code has tests with reasonable coverage

## Project Structure

- `cmd/claudio/` - Main application entry point
- `internal/` - Private application packages
  - `config/` - Configuration handling
  - `instance/` - Claude instance management
  - `orchestrator/` - Session and instance coordination
  - `tui/` - Terminal UI components
  - `worktree/` - Git worktree management
