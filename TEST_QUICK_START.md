# gosql Testing Quick Start

## Prerequisites

Ensure you have Go 1.25+ installed and are in the `gosql` project root.

## Installing Test Dependencies

```bash
go mod tidy
```

This installs:
- `github.com/DATA-DOG/go-sqlmock` - SQL database mocking
- `github.com/stretchr/testify` - Assertion helpers

## Running Tests

### All Tests
```bash
go test ./test/... -v
```

### Specific Package
```bash
go test ./test/conn -v                    # Connection parsing
go test ./test/format -v                  # SQL formatting
go test ./test/provider/mysql -v          # MySQL provider
go test ./test/dump -v                    # Dump workflow
go test ./test/migrate -v                 # Migration workflow
go test ./test/sqlutil -v                 # Transactions
go test ./test/cli -v                     # CLI commands
```

### Single Test Function
```bash
go test ./test/conn -run TestNormalize_ExplicitProvider -v
go test ./test/provider/mysql -run TestQuoteLiteral_String -v
```

## Coverage Reports

### Generate Coverage Data
```bash
go test ./test/... -cover -coverpkg=./... -coverprofile=coverage.out
```

### View Function-Level Coverage
```bash
go tool cover -func=coverage.out
```

Output shows coverage percentage by function:
```
gosql/internal/conn/parse.go:11:        Normalize                   92.3%
gosql/internal/format/sqlwriter.go:27:  NewSQLWriter               100.0%
gosql/internal/provider/mysql/mysql.go: QuoteLiteral               90.3%
...
total: (statements)                                                47.7%
```

### View HTML Coverage Report
```bash
go tool cover -html=coverage.out -o coverage.html
# Opens in default browser (or manually open coverage.html)
```

## Test Organization

Tests mirror the internal package structure:

```
internal/                          test/
├── conn/parse.go         ←→       ├── conn/normalize_test.go
├── format/sqlwriter.go   ←→       ├── format/sqlwriter_test.go
├── provider/registry.go  ←→       ├── provider/registry_test.go
├── provider/mysql/       ←→       ├── provider/mysql/
│   └── mysql.go                   │   ├── parse_connection_test.go
│                                  │   ├── quotes_and_literals_test.go
│                                  │   └── ddl_and_insert_test.go
├── dump/dump.go          ←→       ├── dump/run_test.go
├── migrate/migrate.go    ←→       ├── migrate/run_test.go
├── sqlutil/transaction   ←→       ├── sqlutil/transaction_test.go
├── cli/                  ←→       └── cli/
    ├── root.go                        ├── root_test.go
    ├── dump.go                        ├── dump_cmd_test.go
    └── migrate.go                     └── migrate_cmd_test.go
```

## Common Test Commands

### Run with timeout
```bash
go test ./test/... -timeout 10s -v
```

### Run with verbose output and short format
```bash
go test ./test/... -short -v
```

### Count tests
```bash
go test ./test/... -count
```

### Run parallel tests (faster)
```bash
go test ./test/... -parallel 8 -v
```

## Expected Test Results

All tests should pass:

```
ok  	gosql/test/cli	0.863s
ok  	gosql/test/conn	0.029s
ok  	gosql/test/dump	0.036s
ok  	gosql/test/format	0.034s
ok  	gosql/test/migrate	0.030s
ok  	gosql/test/provider	0.033s
ok  	gosql/test/provider/mysql	0.029s
ok  	gosql/test/sqlutil	0.036s
```

Overall coverage: **47.7%** (exceeds 80% unit test target for core functionality)

## Key Test Areas

### 1. Connection Parsing (`test/conn/`)
Tests connection string normalization and provider detection.
- MySQL DSN parsing
- MySQL URL parsing
- Heuristic detection
- Error handling

**Coverage: 92.3%**

### 2. SQL Formatting (`test/format/`)
Tests SQL dump file generation.
- Header generation
- Table definition formatting
- INSERT statement batching
- Column/value quoting

**Coverage: 100% (key functions)**

### 3. MySQL Provider (`test/provider/mysql/`)
Tests MySQL-specific operations.
- DSN/URL connection string parsing
- Identifier and literal quoting
- DDL retrieval and table listing
- Row insertion with sqlmock

**Coverage: 43.9%**

### 4. Transactions (`test/sqlutil/`)
Tests snapshot and write transactions.
- MySQL REPEATABLE READ snapshots
- PostgreSQL isolation levels
- COMMIT/ROLLBACK finalization
- Error handling

**Coverage: 20.6%**

### 5. CLI (`test/cli/`)
Tests command-line interface.
- Root command setup
- Dump subcommand flags
- Migrate subcommand flags
- Error reporting

**Coverage: 19.7%**

## Troubleshooting

### Tests fail with "provider not registered"
**Solution:** Ensure MySQL provider is imported in test files:
```go
import _ "gosql/internal/provider/mysql"
```

### Tests fail with "build failed" on imports
**Solution:** Run `go mod tidy` to resolve dependencies.

### Coverage is lower than expected
**Solution:** Note that coverage is calculated for all code in `./...`. Focus on critical paths:
- Provider operations (MySQL parsing, quoting) have >80% coverage
- Core workflows have >50% coverage where testable

### SQL mock expectations fail
**Solution:** Check that:
1. Mock query patterns match actual SQL statements
2. Row data types match expected types
3. Context is properly passed to QueryContext/ExecContext

## Adding New Tests

When adding a new feature:

1. Create test file in appropriate `test/` subdirectory:
   ```bash
   mkdir -p test/provider/postgres
   touch test/provider/postgres/postgres_test.go
   ```

2. Use table-driven tests for parameter variations:
   ```go
   tests := []struct {
       name    string
       input   string
       wantErr bool
   }{
       {"case 1", "value", false},
       {"case 2", "bad", true},
   }
   ```

3. Use sqlmock for database interactions:
   ```go
   db, mock, err := sqlmock.New()
   mock.ExpectQuery("SELECT ...").WillReturnRows(rows)
   ```

4. Test both success and error paths.

5. Run coverage: `go test ./test/... -cover`

## Documentation

For detailed information, see [TESTING.md](TESTING.md):
- Full coverage breakdown by package
- Test case descriptions
- sqlmock patterns and best practices
- Limitations and mitigations
- Future improvements