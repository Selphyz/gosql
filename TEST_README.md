# gosql Testing Guide

## Quick Start

```bash
# Run all tests
go test ./test/... -v

# Generate coverage report
go test ./test/... -cover -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## What's Tested

The gosql testing suite includes **202 tests** across 8 test packages with **47.7% code coverage**:

### Test Packages

- **conn** (6 tests): Connection string parsing and provider detection
- **format** (18 tests): SQL dump file generation
- **provider** (9 tests): Provider registry and lookups
- **provider/mysql** (73 tests): MySQL-specific operations
- **dump** (17 tests): Dump workflow orchestration
- **migrate** (18 tests): Migration workflow orchestration
- **sqlutil** (14 tests): Transaction and snapshot handling
- **cli** (47 tests): Command-line interface routing

### Key Coverage Areas

| Function | Coverage | Status |
|----------|----------|--------|
| conn.Normalize | 92.3% | ✅ Excellent |
| format.SQLWriter | 100% | ✅ Perfect |
| mysql.QuoteIdent | 100% | ✅ Perfect |
| mysql.QuoteLiteral | 90.3% | ✅ Excellent |
| mysql.ParseConnection | 100% | ✅ Perfect |
| sqlutil.BeginReadSnapshot | ✅ | Tested |
| cli.NewRootCommand | 81.0% | ✅ Good |

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

### Single Test
```bash
go test ./test/conn -run TestNormalize_ExplicitProvider -v
go test ./test/provider/mysql -run TestQuoteLiteral_String -v
```

## Coverage Reports

### Function-Level Summary
```bash
go test ./test/... -cover -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Output shows coverage for each function:
```
gosql/internal/conn/parse.go:11:        Normalize                   92.3%
gosql/internal/format/sqlwriter.go:27:  NewSQLWriter               100.0%
gosql/internal/provider/mysql/mysql.go: QuoteLiteral               90.3%
...
total: (statements)                                                47.7%
```

### HTML Coverage Report
```bash
go test ./test/... -cover -coverpkg=./... -coverprofile=coverage.out
go tool cover -html=coverage.out
# Opens in your default browser
```

## Test Organization

Tests are organized to mirror the internal package structure:

```
internal/conn/parse.go              ←→  test/conn/normalize_test.go
internal/format/sqlwriter.go        ←→  test/format/sqlwriter_test.go
internal/provider/registry.go       ←→  test/provider/registry_test.go
internal/provider/mysql/mysql.go    ←→  test/provider/mysql/*.go
internal/dump/dump.go               ←→  test/dump/run_test.go
internal/migrate/migrate.go         ←→  test/migrate/run_test.go
internal/sqlutil/transaction.go     ←→  test/sqlutil/transaction_test.go
internal/cli/*.go                   ←→  test/cli/*.go
```

## Testing Approach

### sqlmock for Database Simulation
Tests use `github.com/DATA-DOG/go-sqlmock` to simulate SQL database interactions:

```go
db, mock, err := sqlmock.New()
mock.ExpectQuery("SELECT.*").WillReturnRows(rows)
prov.ListTables(ctx, db)
```

No live databases or Docker required.

### Black-Box Testing
All tests import packages as public interfaces:

```go
import "gosql/internal/conn"      // Import from outside package
```

### Table-Driven Tests
Parameter variations use table-driven test patterns:

```go
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"success", "value", false},
    {"failure", "bad", true},
}
```

## Test Utilities

### FakeProvider
A mock `provider.SQLProvider` implementation for testing:

- Implements all required provider methods
- Works with sqlmock for database operations
- Registered as "fake" provider via init()
- Located in `test/testutil/fakeprovider/`

### Test Dependencies
```bash
go mod tidy  # Installs:
             # - github.com/DATA-DOG/go-sqlmock v1.5.2
             # - github.com/stretchr/testify v1.9.0
```

## Common Test Commands

### Run with timeout
```bash
go test ./test/... -timeout 10s -v
```

### Run in parallel (faster)
```bash
go test ./test/... -parallel 8 -v
```

### Run specific tests matching pattern
```bash
go test ./test/... -run "TestQuote" -v
```

### Verbose output with short format
```bash
go test ./test/... -short -v
```

## Adding New Tests

When adding tests for new functionality:

1. Create test file in appropriate `test/` subdirectory:
   ```bash
   mkdir -p test/provider/postgres
   touch test/provider/postgres/postgres_test.go
   ```

2. Use table-driven tests for parameter variations

3. Use sqlmock for database interactions:
   ```go
   db, mock, err := sqlmock.New()
   defer db.Close()
   
   mock.ExpectQuery("SELECT ...").WillReturnRows(rows)
   ```

4. Test both success and error paths

5. Run coverage: `go test ./test/... -cover`

6. Verify all tests pass before committing

## Documentation

For detailed information, see:
- **TESTING.md** - Comprehensive testing documentation and coverage breakdown
- **TEST_QUICK_START.md** - Quick reference for running tests
- **TESTING_SUMMARY.md** - Implementation summary and statistics

## Expected Results

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

**Overall coverage: 47.7%** (exceeds unit test target)

## Troubleshooting

### "provider not registered" error
Ensure provider is imported:
```go
import _ "gosql/internal/provider/mysql"
```

### "build failed" error
Run dependency management:
```bash
go mod tidy
```

### SQL mock expectations not matching
Check:
1. Mock query patterns match actual SQL
2. Row data types match expected types
3. Context is properly passed to QueryContext/ExecContext

### Tests running slowly
Run in parallel:
```bash
go test ./test/... -parallel 8 -v
```

## Best Practices

1. **Test names describe scenarios**: `TestQuoteLiteral_UnsupportedType`
2. **Use table-driven tests** for parameter variations
3. **Test error paths** as thoroughly as success paths
4. **Use sqlmock ExpectQuery/ExpectExec** for deterministic database testing
5. **Keep tests isolated** - no shared state between tests
6. **Use testify require/assert** for clear assertions

## Notes

- **No live databases required**: All tests use sqlmock
- **No Docker required**: Tests are pure Go units
- **Fast execution**: All 202 tests run in <1 second
- **Reproducible**: Same results every run
- **Black-box**: Tests treat packages as public interfaces

## Integration Tests

For full end-to-end testing with real databases, consider:
- Separate integration test suite with Docker
- Live MySQL, PostgreSQL, Oracle test instances
- Full dump and migration workflows
- Performance and stress testing

See TESTING.md for details on integrating these with the unit test suite.