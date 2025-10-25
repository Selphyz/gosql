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
}
