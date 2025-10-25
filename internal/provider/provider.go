package provider

import (
	"context"
	"database/sql"
)

// SQLProvider defines the required capabilities for database-specific operations.
type SQLProvider interface {
	DriverName() string
	ParseConnection(input string) (dsn string, dbName string, err error)
	ListTables(ctx context.Context, db *sql.DB) ([]string, error)
	ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error)
	StreamRows(ctx context.Context, db *sql.DB, table string) (*sql.Rows, []string, error)
	InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error
	DisableConstraints(ctx context.Context, db *sql.DB) (restore func(context.Context) error, err error)
	QuoteIdent(string) string
	QuoteLiteral(any) (string, error)
	DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (DatabaseMetadata, error)
	EnsureDatabase(ctx context.Context, dsn string, dbName string, meta DatabaseMetadata) error
}

// AdditionalDDLProvider is an optional interface that providers can implement
// to supply additional DDL statements for a table (e.g., non-constraint indexes).
// These statements are executed after the main CREATE TABLE statement during migration
// and written after the table definition during dump operations.
type AdditionalDDLProvider interface {
	TableExtraDDL(ctx context.Context, db *sql.DB, table string) ([]string, error)
}
