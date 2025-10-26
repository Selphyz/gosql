package fakeprovider

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/DATA-DOG/go-sqlmock"

	"gosql/internal/provider"
)

// FakeProvider implements provider.SQLProvider for testing with sqlmock.
type FakeProvider struct {
	mu sync.RWMutex
}

var _ provider.SQLProvider = (*FakeProvider)(nil)

// init registers the fake provider on package load.
func init() {
	provider.Register("fake", &FakeProvider{}, "fake")
}

// DriverName returns the driver name used by sqlmock.
func (fp *FakeProvider) DriverName() string {
	return "sqlmock"
}

// ParseConnection returns a DSN suitable for opening a sqlmock database.
// For testing, it simply returns predictable values.
func (fp *FakeProvider) ParseConnection(input string) (string, string, error) {
	if input == "" {
		return "", "", fmt.Errorf("connection string is required")
	}
	// In tests, the caller will use sql.Open with sqlmock driver
	// This just extracts or derives a database name from the input
	dsn := input // passed through as-is for sqlmock
	dbName := "testdb"
	if input == "error" {
		return "", "", fmt.Errorf("simulated parse error")
	}
	return dsn, dbName, nil
}

// ListTables returns a list of table names from the database.
func (fp *FakeProvider) ListTables(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

// ShowCreateTable returns the CREATE TABLE statement for a given table.
func (fp *FakeProvider) ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE TABLE %s", fp.QuoteIdent(table))
	var name, ddl string
	err := db.QueryRowContext(ctx, query).Scan(&name, &ddl)
	if err != nil {
		return "", err
	}
	return ddl, nil
}

// StreamRows selects all rows from a table and returns them with column names.
func (fp *FakeProvider) StreamRows(ctx context.Context, db *sql.DB, table string) (*sql.Rows, []string, error) {
	query := fmt.Sprintf("SELECT * FROM %s", fp.QuoteIdent(table))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, nil, err
	}
	return rows, cols, nil
}

// InsertRows performs a batched INSERT operation.
func (fp *FakeProvider) InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error {
	if len(cols) == 0 {
		return fmt.Errorf("no columns supplied for insert into %s", table)
	}
	if len(rows) == 0 {
		return nil
	}

	// Build INSERT statement with placeholders
	placeholders := "("
	for i := 0; i < len(cols); i++ {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
	}
	placeholders += ")"

	query := "INSERT INTO " + fp.QuoteIdent(table) + " ("
	for i, col := range cols {
		if i > 0 {
			query += ", "
		}
		query += fp.QuoteIdent(col)
	}
	query += ") VALUES "

	args := make([]any, 0, len(rows)*len(cols))
	for i, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("row %d column count mismatch for table %s", i, table)
		}
		if i > 0 {
			query += ", "
		}
		query += placeholders
		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, query, args...)
	return err
}

// DisableConstraints disables foreign key constraints and returns a restore callback.
func (fp *FakeProvider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) {
	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")
		return err
	}, nil
}

// QuoteIdent quotes an identifier using backticks.
func (fp *FakeProvider) QuoteIdent(ident string) string {
	return "`" + ident + "`"
}

// QuoteLiteral quotes a literal value for SQL embedding.
func (fp *FakeProvider) QuoteLiteral(value any) (string, error) {
	if value == nil {
		return "NULL", nil
	}
	switch v := value.(type) {
	case string:
		return "'" + v + "'", nil
	case int, int64, float64:
		return fmt.Sprintf("%v", v), nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	default:
		return fmt.Sprintf("'%v'", v), nil
	}
}

// DatabaseMetadata returns the database metadata.
func (fp *FakeProvider) DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (provider.DatabaseMetadata, error) {
	query := "SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?"
	var meta provider.DatabaseMetadata
	err := db.QueryRowContext(ctx, query, dbName).Scan(&meta.Charset, &meta.Collation)
	if err != nil {
		return provider.DatabaseMetadata{}, err
	}
	return meta, nil
}

// EnsureDatabase ensures the database exists, creating it if necessary.
func (fp *FakeProvider) EnsureDatabase(ctx context.Context, dsn string, dbName string, meta provider.DatabaseMetadata) error {
	// For testing purposes, just verify we can open a connection to the root (no DB specified)
	// and then create the database if it doesn't exist
	db, err := sql.Open(fp.DriverName(), dsn)
	if err != nil {
		return fmt.Errorf("open destination server: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping destination server: %w", err)
	}

	// Check if database exists
	var dummy string
	checkErr := db.QueryRowContext(ctx, "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&dummy)
	if checkErr == nil {
		return nil // Database already exists
	}
	if checkErr != sql.ErrNoRows {
		return checkErr
	}

	// Create the database
	stmt := "CREATE DATABASE " + fp.QuoteIdent(dbName)
	if meta.Charset != "" {
		stmt += " CHARACTER SET " + meta.Charset
	}
	if meta.Collation != "" {
		stmt += " COLLATE " + meta.Collation
	}
	_, err = db.ExecContext(ctx, stmt)
	return err
}

// TableExtraDDL returns additional DDL statements for a table (optional interface).
func (fp *FakeProvider) TableExtraDDL(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	// For testing, return empty or some mock statements if needed
	return []string{}, nil
}

// OpenMock opens a new sqlmock database for testing.
func OpenMock() (*sql.DB, sqlmock.Sqlmock, error) {
	return sqlmock.New()
}