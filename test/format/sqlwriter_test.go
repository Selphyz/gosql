package format

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/format"
	"gosql/internal/provider"
	_ "gosql/test/testutil/fakeprovider"
)

// MockProvider is a minimal provider for testing SQLWriter.
type MockProvider struct{}

func (mp *MockProvider) DriverName() string                                                        { return "mock" }
func (mp *MockProvider) ParseConnection(input string) (string, string, error)                     { return "", "", nil }
func (mp *MockProvider) ListTables(ctx context.Context, db *sql.DB) ([]string, error)                             { return nil, nil }
func (mp *MockProvider) ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error)            { return "", nil }
func (mp *MockProvider) StreamRows(ctx context.Context, db *sql.DB, table string) (*sql.Rows, []string, error)          { return nil, nil, nil }
func (mp *MockProvider) InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error {
	return nil
}
func (mp *MockProvider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) { return nil, nil }
func (mp *MockProvider) QuoteIdent(ident string) string                               { return "`" + ident + "`" }
func (mp *MockProvider) QuoteLiteral(value any) (string, error)                       { return "'value'", nil }
func (mp *MockProvider) DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (provider.DatabaseMetadata, error) {
	return provider.DatabaseMetadata{}, nil
}
func (mp *MockProvider) EnsureDatabase(ctx context.Context, dsn string, dbName string, meta provider.DatabaseMetadata) error {
	return nil
}

var _ provider.SQLProvider = (*MockProvider)(nil)

func TestWriteHeader_GeneratesTimestamp(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	header := format.Header{
		ProviderName:  "mysql",
		DatabaseName:  "testdb",
		ServerVersion: "8.0.23",
		GeneratedAt:   time.Time{}, // Zero value
	}

	err := sw.WriteHeader(header)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "-- gosql dump")
	assert.Contains(t, output, "Provider: mysql")
	assert.Contains(t, output, "Database: testdb")
	assert.Contains(t, output, "server 8.0.23")
	// Should have generated a timestamp
	assert.NotContains(t, output, "Generated: 0001-01-01")
	assert.Contains(t, output, "Generated:")
	assert.Contains(t, output, "SET NAMES utf8mb4")
}

func TestWriteHeader_UsesProvidedTimestamp(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	ts := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	header := format.Header{
		ProviderName:  "postgres",
		DatabaseName:  "mydb",
		ServerVersion: "14.2",
		GeneratedAt:   ts,
	}

	err := sw.WriteHeader(header)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "2024-01-15T10:30:45Z")
}

func TestWriteTableDefinition_AddsDropAndDDL(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "users"
	ddl := "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))"

	err := sw.WriteTableDefinition(table, ddl)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Table structure for table")
	assert.Contains(t, output, "DROP TABLE IF EXISTS `users`")
	assert.Contains(t, output, "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100));")
}

func TestWriteTableDefinition_AddsTrailingSemicolon(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "products"
	ddl := "CREATE TABLE products (id INT, name VARCHAR(100))" // No semicolon

	err := sw.WriteTableDefinition(table, ddl)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CREATE TABLE products (id INT, name VARCHAR(100));")
}

func TestWriteTableDefinition_TrimsDDL(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "orders"
	ddl := "  \n\n  CREATE TABLE orders (id INT)  \n  "

	err := sw.WriteTableDefinition(table, ddl)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CREATE TABLE orders (id INT);")
	// Should not have excessive whitespace
	assert.NotContains(t, output, "  \n  CREATE")
}

func TestWriteStatements_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	err := sw.WriteStatements([]string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Empty(t, output)
}

func TestWriteStatements_SkipsBlankLines(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	stmts := []string{
		"CREATE INDEX idx1 ON table1(col1)",
		"  ",
		"\n",
		"CREATE INDEX idx2 ON table1(col2)",
	}

	err := sw.WriteStatements(stmts)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CREATE INDEX idx1 ON table1(col1);")
	assert.Contains(t, output, "CREATE INDEX idx2 ON table1(col2);")
	// Count lines to ensure blank lines are skipped
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(lines))
}

func TestWriteStatements_AddsSemicolons(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	stmts := []string{
		"CREATE INDEX idx1 ON table1(col1)",
		"CREATE INDEX idx2 ON table1(col2);",
	}

	err := sw.WriteStatements(stmts)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CREATE INDEX idx1 ON table1(col1);")
	assert.Contains(t, output, "CREATE INDEX idx2 ON table1(col2);")
	// Ensure no double semicolons
	assert.NotContains(t, output, ";;")
}

func TestWriteTableDataPreamble(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "users"
	err := sw.WriteTableDataPreamble(table)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Dumping data for table")
	assert.Contains(t, output, "`users`")
}

func TestWriteInsertBatch_SingleRow(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "users"
	columns := []string{"id", "name"}
	rows := [][]string{
		{"1", "'John'"},
	}

	err := sw.WriteInsertBatch(table, columns, rows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "INSERT INTO `users` (`id`, `name`) VALUES")
	assert.Contains(t, output, "(1, 'John');")
}

func TestWriteInsertBatch_MultipleRows(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "users"
	columns := []string{"id", "name"}
	rows := [][]string{
		{"1", "'Alice'"},
		{"2", "'Bob'"},
		{"3", "'Charlie'"},
	}

	err := sw.WriteInsertBatch(table, columns, rows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "INSERT INTO `users` (`id`, `name`) VALUES")
	assert.Contains(t, output, "(1, 'Alice'),")
	assert.Contains(t, output, "(2, 'Bob'),")
	assert.Contains(t, output, "(3, 'Charlie');")
}

func TestWriteInsertBatch_EmptyRows(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "users"
	columns := []string{"id", "name"}
	rows := [][]string{}

	err := sw.WriteInsertBatch(table, columns, rows)
	require.NoError(t, err)

	output := buf.String()
	assert.Empty(t, output)
}

func TestWriteInsertBatch_QuotedColumns(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	table := "products"
	columns := []string{"id", "name", "price"}
	rows := [][]string{
		{"1", "'Widget'", "9.99"},
	}

	err := sw.WriteInsertBatch(table, columns, rows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "INSERT INTO `products` (`id`, `name`, `price`)")
	assert.Contains(t, output, "(1, 'Widget', 9.99);")
}

func TestNewSQLWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	prov := &MockProvider{}
	sw := format.NewSQLWriter(buf, prov)

	assert.NotNil(t, sw)
	// Verify it can be used to write
	err := sw.WriteTableDataPreamble("test")
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}