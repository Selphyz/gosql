package postgres

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gosql/internal/provider"
)

// Provider implements provider.SQLProvider for PostgreSQL databases using pgx driver.
type Provider struct{}

var _ provider.SQLProvider = (*Provider)(nil)

func init() {
	provider.Register("postgres", &Provider{}, "postgres", "postgresql", "pgx")
}

// DriverName returns the Go SQL driver name.
func (*Provider) DriverName() string {
	return "pgx"
}

// ParseConnection normalises supported PostgreSQL connection strings to driver DSNs.
// Supports both URL format (postgres://user:pass@host:port/dbname) and keyword/value format.
func (*Provider) ParseConnection(input string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(input), "postgres://") ||
		strings.HasPrefix(strings.ToLower(input), "postgresql://") ||
		strings.HasPrefix(strings.ToLower(input), "pgx://") {
		return parseURLConnection(input)
	}
	return parseKeywordConnection(input)
}

func parseURLConnection(input string) (string, string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid PostgreSQL URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "postgres" && scheme != "postgresql" && scheme != "pgx" {
		return "", "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	// Extract database name from path
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", "", fmt.Errorf("PostgreSQL URL missing database name")
	}

	// pgx driver expects postgres:// or postgresql:// scheme
	if scheme == "pgx" {
		u.Scheme = "postgres"
	}

	return u.String(), dbName, nil
}

func parseKeywordConnection(input string) (string, string, error) {
	// Parse keyword/value pairs: "host=localhost port=5432 dbname=mydb user=postgres password=secret"
	params := make(map[string]string)
	parts := strings.Fields(input)

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return "", "", fmt.Errorf("invalid keyword connection format: %q", part)
		}
		params[strings.ToLower(kv[0])] = kv[1]
	}

	dbName, ok := params["dbname"]
	if !ok {
		return "", "", fmt.Errorf("PostgreSQL connection missing dbname")
	}

	return input, dbName, nil
}

// ListTables returns the list of base tables in the current schema and public schema.
func (*Provider) ListTables(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `
		SELECT schemaname || '.' || tablename AS qualified_name
		FROM pg_catalog.pg_tables
		WHERE schemaname IN (current_schema(), 'public')
		  AND schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, tablename;
	`
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

// ShowCreateTable reconstructs the CREATE TABLE DDL from PostgreSQL system catalogs.
func (p *Provider) ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error) {
	schema, tableName := parseSchemaTable(table)

	// Get table OID
	oid, err := p.getTableOID(ctx, db, schema, tableName)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(p.QuoteIdent(table))
	sb.WriteString(" (\n")

	// Get columns
	columns, err := p.getColumns(ctx, db, oid)
	if err != nil {
		return "", fmt.Errorf("get columns: %w", err)
	}

	// Get primary key and unique constraints
	pkCols, err := p.getPrimaryKeyColumns(ctx, db, oid)
	if err != nil {
		return "", fmt.Errorf("get primary key: %w", err)
	}

	uniqueConstraints, err := p.getUniqueConstraints(ctx, db, oid)
	if err != nil {
		return "", fmt.Errorf("get unique constraints: %w", err)
	}

	// Build column definitions
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("  ")
		sb.WriteString(p.QuoteIdent(col.Name))
		sb.WriteString(" ")
		sb.WriteString(col.DataType)

		if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}

		if col.Default != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.Default)
		}
	}

	// Add primary key constraint
	if len(pkCols) > 0 {
		sb.WriteString(",\n  PRIMARY KEY (")
		for i, col := range pkCols {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	// Add unique constraints
	for _, uc := range uniqueConstraints {
		sb.WriteString(",\n  CONSTRAINT ")
		sb.WriteString(p.QuoteIdent(uc.Name))
		sb.WriteString(" UNIQUE (")
		for i, col := range uc.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	sb.WriteString("\n)")

	// Get foreign keys and append as separate ALTER TABLE statements
	fks, err := p.getForeignKeys(ctx, db, oid, table)
	if err != nil {
		return "", fmt.Errorf("get foreign keys: %w", err)
	}

	ddl := sb.String()
	for _, fk := range fks {
		ddl += ";\n" + fk
	}

	return ddl, nil
}

// StreamRows selects all rows from a table using a streaming cursor.
func (p *Provider) StreamRows(ctx context.Context, db *sql.DB, table string) (*sql.Rows, []string, error) {
	query := fmt.Sprintf("SELECT * FROM %s", p.QuoteIdent(table))
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

// InsertRows performs a batched INSERT operation using positional parameters.
func (p *Provider) InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error {
	if len(cols) == 0 {
		return fmt.Errorf("no columns supplied for insert into %s", table)
	}
	if len(rows) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.Grow(len("INSERT INTO ") + len(table)*2)

	sb.WriteString("INSERT INTO ")
	sb.WriteString(p.QuoteIdent(table))
	sb.WriteString(" (")
	for i, col := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.QuoteIdent(col))
	}
	sb.WriteString(") VALUES ")

	args := make([]any, 0, len(rows)*len(cols))
	paramIndex := 1

	for i, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("row %d column count mismatch for table %s", i, table)
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for j := range cols {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("$")
			sb.WriteString(strconv.Itoa(paramIndex))
			paramIndex++
		}
		sb.WriteString(")")
		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	return err
}

// DisableConstraints temporarily disables constraint enforcement using session_replication_role.
func (*Provider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) {
	if _, err := db.ExecContext(ctx, "SET session_replication_role = replica"); err != nil {
		return nil, err
	}
	restore := func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, "SET session_replication_role = DEFAULT")
		return err
	}
	return restore, nil
}

// QuoteIdent quotes a PostgreSQL identifier using double quotes.
func (*Provider) QuoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

// QuoteLiteral renders a literal suitable for embedding in SQL statements.
func (*Provider) QuoteLiteral(value any) (string, error) {
	if value == nil {
		return "NULL", nil
	}

	switch v := value.(type) {
	case []byte:
		if v == nil {
			return "NULL", nil
		}
		// PostgreSQL bytea format: '\x' followed by hex
		return "'\\x" + hex.EncodeToString(v) + "'", nil
	case string:
		return quoteString(v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case time.Time:
		return quoteString(formatTime(v)), nil
	case fmt.Stringer:
		return quoteString(v.String()), nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	}

	// Handle []uint8 alias when provided through database/sql
	if b, ok := asBytes(value); ok {
		return "'\\x" + hex.EncodeToString(b) + "'", nil
	}

	return "", fmt.Errorf("unsupported literal type %T", value)
}

// DatabaseMetadata returns empty metadata for PostgreSQL (no per-database charset).
func (*Provider) DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (provider.DatabaseMetadata, error) {
	// PostgreSQL uses cluster-wide encoding, not per-database charset/collation like MySQL
	return provider.DatabaseMetadata{}, nil
}

// EnsureDatabase makes sure the destination database exists, creating it if needed.
func (p *Provider) EnsureDatabase(ctx context.Context, dsn string, dbName string, meta provider.DatabaseMetadata) error {
	// Connect to template database to check/create target database
	templateDSN := replaceDatabaseInDSN(dsn, "template1")

	templateDB, err := sql.Open(p.DriverName(), templateDSN)
	if err != nil {
		return fmt.Errorf("open template database: %w", err)
	}
	defer templateDB.Close()

	if err := templateDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping template database: %w", err)
	}

	exists, err := databaseExists(ctx, templateDB, dbName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create database with quoted identifier
	stmt := "CREATE DATABASE " + p.QuoteIdent(dbName)
	if _, err := templateDB.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}
	return nil
}

// TableExtraDDL implements AdditionalDDLProvider to return non-constraint indexes.
func (p *Provider) TableExtraDDL(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	schema, tableName := parseSchemaTable(table)

	const query = `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = $1
		  AND tablename = $2
		  AND indexdef NOT LIKE '%UNIQUE%'
		  AND indexname NOT IN (
			SELECT conname
			FROM pg_constraint
			WHERE conrelid = (
				SELECT oid
				FROM pg_class
				WHERE relname = $2
				  AND relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1)
			)
			AND contype IN ('p', 'u')
		  )
		ORDER BY indexname;
	`

	rows, err := db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statements []string
	for rows.Next() {
		var indexDef string
		if err := rows.Scan(&indexDef); err != nil {
			return nil, err
		}
		statements = append(statements, indexDef)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statements, nil
}

// Helper types and functions

type column struct {
	Name     string
	DataType string
	Nullable bool
	Default  string
	Position int
}

type uniqueConstraint struct {
	Name    string
	Columns []string
}

func parseSchemaTable(table string) (schema, tableName string) {
	parts := strings.SplitN(table, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", parts[0]
}

func (p *Provider) getTableOID(ctx context.Context, db *sql.DB, schema, table string) (int64, error) {
	const query = `
		SELECT c.oid
		FROM pg_class c
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname = $1 AND c.relname = $2;
	`
	var oid int64
	err := db.QueryRowContext(ctx, query, schema, table).Scan(&oid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("table %s.%s not found", schema, table)
	}
	return oid, err
}

func (p *Provider) getColumns(ctx context.Context, db *sql.DB, oid int64) ([]column, error) {
	const query = `
		SELECT
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			NOT a.attnotnull,
			pg_catalog.pg_get_expr(d.adbin, d.adrelid),
			a.attnum
		FROM pg_catalog.pg_attribute a
		LEFT JOIN pg_catalog.pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
		WHERE a.attrelid = $1
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum;
	`

	rows, err := db.QueryContext(ctx, query, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []column
	for rows.Next() {
		var col column
		var defaultSQL sql.NullString
		if err := rows.Scan(&col.Name, &col.DataType, &col.Nullable, &defaultSQL, &col.Position); err != nil {
			return nil, err
		}
		if defaultSQL.Valid {
			col.Default = defaultSQL.String
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *Provider) getPrimaryKeyColumns(ctx context.Context, db *sql.DB, oid int64) ([]string, error) {
	const query = `
		SELECT a.attname
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.conrelid = $1 AND c.contype = 'p'
		ORDER BY array_position(c.conkey, a.attnum);
	`

	rows, err := db.QueryContext(ctx, query, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func (p *Provider) getUniqueConstraints(ctx context.Context, db *sql.DB, oid int64) ([]uniqueConstraint, error) {
	const query = `
		SELECT
			c.conname,
			array_agg(a.attname ORDER BY array_position(c.conkey, a.attnum))
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.conrelid = $1 AND c.contype = 'u'
		GROUP BY c.conname
		ORDER BY c.conname;
	`

	rows, err := db.QueryContext(ctx, query, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []uniqueConstraint
	for rows.Next() {
		var uc uniqueConstraint
		var colArray string
		if err := rows.Scan(&uc.Name, &colArray); err != nil {
			return nil, err
		}
		// Parse PostgreSQL array format: {col1,col2}
		uc.Columns = parsePostgresArray(colArray)
		constraints = append(constraints, uc)
	}
	return constraints, rows.Err()
}

func (p *Provider) getForeignKeys(ctx context.Context, db *sql.DB, oid int64, table string) ([]string, error) {
	const query = `
		SELECT
			c.conname,
			c.confupdtype,
			c.confdeltype,
			array_agg(a.attname ORDER BY array_position(c.conkey, a.attnum)),
			(SELECT n.nspname || '.' || cl.relname FROM pg_class cl JOIN pg_namespace n ON cl.relnamespace = n.oid WHERE cl.oid = c.confrelid),
			array_agg(af.attname ORDER BY array_position(c.confkey, af.attnum))
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		JOIN pg_attribute af ON af.attrelid = c.confrelid AND af.attnum = ANY(c.confkey)
		WHERE c.conrelid = $1 AND c.contype = 'f'
		GROUP BY c.conname, c.confupdtype, c.confdeltype, c.confrelid
		ORDER BY c.conname;
	`

	rows, err := db.QueryContext(ctx, query, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fkStatements []string
	for rows.Next() {
		var name, updateAction, deleteAction, localCols, refTable, refCols string
		if err := rows.Scan(&name, &updateAction, &deleteAction, &localCols, &refTable, &refCols); err != nil {
			return nil, err
		}

		var sb strings.Builder
		sb.WriteString("ALTER TABLE ")
		sb.WriteString(p.QuoteIdent(table))
		sb.WriteString(" ADD CONSTRAINT ")
		sb.WriteString(p.QuoteIdent(name))
		sb.WriteString(" FOREIGN KEY (")

		localColsList := parsePostgresArray(localCols)
		for i, col := range localColsList {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}

		sb.WriteString(") REFERENCES ")
		sb.WriteString(p.QuoteIdent(refTable))
		sb.WriteString(" (")

		refColsList := parsePostgresArray(refCols)
		for i, col := range refColsList {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}

		sb.WriteString(")")

		if updateAction != "a" {
			sb.WriteString(" ON UPDATE ")
			sb.WriteString(mapFKAction(updateAction))
		}
		if deleteAction != "a" {
			sb.WriteString(" ON DELETE ")
			sb.WriteString(mapFKAction(deleteAction))
		}

		fkStatements = append(fkStatements, sb.String())
	}
	return fkStatements, rows.Err()
}

func parsePostgresArray(arr string) []string {
	// Simple parser for PostgreSQL array format: {item1,item2}
	arr = strings.TrimPrefix(arr, "{")
	arr = strings.TrimSuffix(arr, "}")
	if arr == "" {
		return nil
	}
	return strings.Split(arr, ",")
}

func mapFKAction(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
	}
}

func quoteString(s string) string {
	// PostgreSQL uses single quotes and doubles them for escaping
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func asBytes(value any) ([]byte, bool) {
	switch v := value.(type) {
	case []uint8:
		return []byte(v), true
	case sql.RawBytes:
		return []byte(v), true
	default:
		return nil, false
	}
}

func formatTime(t time.Time) string {
	// PostgreSQL timestamp format
	return t.Format("2006-01-02 15:04:05.000000")
}

func databaseExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func replaceDatabaseInDSN(dsn, newDB string) string {
	// Handle URL format
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		u.Path = "/" + newDB
		return u.String()
	}

	// Handle keyword format
	params := make(map[string]string)
	parts := strings.Fields(dsn)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[strings.ToLower(kv[0])] = kv[1]
		}
	}
	params["dbname"] = newDB

	// Rebuild in sorted order for consistency
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts2 []string
	for _, k := range keys {
		parts2 = append(parts2, k+"="+params[k])
	}
	return strings.Join(parts2, " ")
}
