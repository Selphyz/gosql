package sqlserver

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gosql/internal/provider"
)

// Provider implements provider.SQLProvider for SQL Server databases.
type Provider struct{}

var _ provider.SQLProvider = (*Provider)(nil)

func init() {
	provider.Register("sqlserver", &Provider{}, "sqlserver", "mssql", "sqlsrv")
}

// DriverName returns the Go SQL driver name.
func (*Provider) DriverName() string {
	return "sqlserver"
}

// ParseConnection normalizes supported SQL Server connection strings to driver DSNs.
// Supports: sqlserver://user:pass@host:port?database=Db and ADO connection strings.
func (*Provider) ParseConnection(input string) (string, string, error) {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "sqlserver://") || strings.HasPrefix(lower, "mssql://") || strings.HasPrefix(lower, "sqlsrv://") {
		return parseURLConnection(input)
	}
	return parseADOConnection(input)
}

func parseURLConnection(input string) (string, string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid SQL Server URL: %w", err)
	}

	// Extract database from query parameters
	values := u.Query()
	dbName := values.Get("database")
	if dbName == "" {
		dbName = values.Get("Database")
	}
	if dbName == "" {
		return "", "", fmt.Errorf("SQL Server URL missing database parameter")
	}

	// Rebuild DSN in the format the driver expects
	var user, password string
	if u.User != nil {
		user = u.User.Username()
		if pwd, ok := u.User.Password(); ok {
			password = pwd
		}
	}

	host := u.Host
	if host == "" {
		host = "localhost:1433"
	} else if !strings.Contains(host, ":") {
		host = host + ":1433"
	}

	// Build connection string
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s?database=%s", user, password, host, dbName)

	// Add other parameters from original URL
	for k, v := range values {
		if strings.EqualFold(k, "database") {
			continue
		}
		if len(v) > 0 {
			dsn += fmt.Sprintf("&%s=%s", k, v[0])
		}
	}

	return dsn, dbName, nil
}

func parseADOConnection(input string) (string, string, error) {
	// Parse ADO-style: server=host;database=db;user id=user;password=pass
	params := make(map[string]string)
	parts := strings.Split(input, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])
		params[key] = value
	}

	// Extract required parameters
	server := params["server"]
	if server == "" {
		server = params["data source"]
	}
	if server == "" {
		return "", "", fmt.Errorf("SQL Server connection string missing server")
	}

	dbName := params["database"]
	if dbName == "" {
		dbName = params["initial catalog"]
	}
	if dbName == "" {
		return "", "", fmt.Errorf("SQL Server connection string missing database")
	}

	user := params["user id"]
	if user == "" {
		user = params["uid"]
	}

	password := params["password"]
	if password == "" {
		password = params["pwd"]
	}

	// Ensure port
	if !strings.Contains(server, ":") {
		server = server + ":1433"
	}

	// Build URL-style DSN
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s?database=%s", user, password, server, dbName)

	return dsn, dbName, nil
}

// ListTables returns the list of user tables in the current database.
func (*Provider) ListTables(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `SELECT name FROM sys.tables WHERE is_ms_shipped = 0 ORDER BY name`
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

// ShowCreateTable constructs a CREATE TABLE statement for the given table.
func (p *Provider) ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error) {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(p.QuoteIdent(table))
	sb.WriteString(" (\n")

	// Get columns
	columns, err := p.getColumns(ctx, db, table)
	if err != nil {
		return "", err
	}

	for i, col := range columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("  ")
		sb.WriteString(p.QuoteIdent(col.Name))
		sb.WriteString(" ")
		sb.WriteString(col.DataType)

		if col.IsIdentity {
			sb.WriteString(" IDENTITY")
		}

		if !col.IsNullable {
			sb.WriteString(" NOT NULL")
		}

		if col.DefaultValue != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.DefaultValue)
		}

		if col.IsComputed {
			sb.WriteString(" AS ")
			sb.WriteString(col.ComputedDefinition)
		}
	}

	// Add primary key
	pk, err := p.getPrimaryKey(ctx, db, table)
	if err != nil {
		return "", err
	}
	if len(pk) > 0 {
		sb.WriteString(",\n  PRIMARY KEY (")
		for i, col := range pk {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	// Add unique constraints
	uniques, err := p.getUniqueConstraints(ctx, db, table)
	if err != nil {
		return "", err
	}
	for _, uc := range uniques {
		sb.WriteString(",\n  UNIQUE (")
		for i, col := range uc.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	sb.WriteString("\n)")

	// Add foreign keys as separate ALTER TABLE statements
	fks, err := p.getForeignKeys(ctx, db, table)
	if err != nil {
		return "", err
	}
	for _, fk := range fks {
		sb.WriteString(";\nALTER TABLE ")
		sb.WriteString(p.QuoteIdent(table))
		sb.WriteString(" ADD CONSTRAINT ")
		sb.WriteString(p.QuoteIdent(fk.Name))
		sb.WriteString(" FOREIGN KEY (")
		for i, col := range fk.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(") REFERENCES ")
		sb.WriteString(p.QuoteIdent(fk.RefTable))
		sb.WriteString(" (")
		for i, col := range fk.RefColumns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	// Add indexes as separate CREATE INDEX statements
	indexes, err := p.getIndexes(ctx, db, table)
	if err != nil {
		return "", err
	}
	for _, idx := range indexes {
		sb.WriteString(";\nCREATE INDEX ")
		sb.WriteString(p.QuoteIdent(idx.Name))
		sb.WriteString(" ON ")
		sb.WriteString(p.QuoteIdent(table))
		sb.WriteString(" (")
		for i, col := range idx.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(p.QuoteIdent(col))
		}
		sb.WriteString(")")
	}

	return sb.String(), nil
}

type columnInfo struct {
	Name                string
	DataType            string
	IsNullable          bool
	DefaultValue        string
	IsIdentity          bool
	IsComputed          bool
	ComputedDefinition  string
}

func (p *Provider) getColumns(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	query := `
		SELECT
			c.name,
			t.name AS type_name,
			c.max_length,
			c.precision,
			c.scale,
			c.is_nullable,
			c.is_identity,
			c.is_computed,
			ISNULL(dc.definition, '') AS default_value,
			ISNULL(cc.definition, '') AS computed_definition
		FROM sys.columns c
		INNER JOIN sys.types t ON c.user_type_id = t.user_type_id
		LEFT JOIN sys.default_constraints dc ON c.default_object_id = dc.object_id
		LEFT JOIN sys.computed_columns cc ON c.object_id = cc.object_id AND c.column_id = cc.column_id
		WHERE c.object_id = OBJECT_ID(@p1)
		ORDER BY c.column_id
	`
	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var name, typeName, defaultVal, computedDef string
		var maxLength, precision, scale int
		var isNullable, isIdentity, isComputed bool

		if err := rows.Scan(&name, &typeName, &maxLength, &precision, &scale,
			&isNullable, &isIdentity, &isComputed, &defaultVal, &computedDef); err != nil {
			return nil, err
		}

		// Build data type string
		var typeStr string
		switch strings.ToLower(typeName) {
		case "varchar", "nvarchar", "char", "nchar":
			if maxLength == -1 {
				typeStr = fmt.Sprintf("%s(MAX)", typeName)
			} else {
				displayLen := maxLength
				if strings.HasPrefix(typeName, "n") {
					displayLen = maxLength / 2
				}
				typeStr = fmt.Sprintf("%s(%d)", typeName, displayLen)
			}
		case "varbinary", "binary":
			if maxLength == -1 {
				typeStr = fmt.Sprintf("%s(MAX)", typeName)
			} else {
				typeStr = fmt.Sprintf("%s(%d)", typeName, maxLength)
			}
		case "decimal", "numeric":
			typeStr = fmt.Sprintf("%s(%d,%d)", typeName, precision, scale)
		default:
			typeStr = typeName
		}

		col := columnInfo{
			Name:               name,
			DataType:           typeStr,
			IsNullable:         isNullable,
			IsIdentity:         isIdentity,
			IsComputed:         isComputed,
			ComputedDefinition: strings.TrimSpace(computedDef),
		}

		if defaultVal != "" {
			col.DefaultValue = strings.TrimSpace(defaultVal)
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (p *Provider) getPrimaryKey(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	query := `
		SELECT COL_NAME(ic.object_id, ic.column_id) AS column_name
		FROM sys.indexes i
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		WHERE i.object_id = OBJECT_ID(@p1)
		  AND i.is_primary_key = 1
		ORDER BY ic.key_ordinal
	`
	rows, err := db.QueryContext(ctx, query, table)
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

type uniqueConstraint struct {
	Name    string
	Columns []string
}

func (p *Provider) getUniqueConstraints(ctx context.Context, db *sql.DB, table string) ([]uniqueConstraint, error) {
	query := `
		SELECT i.name
		FROM sys.indexes i
		WHERE i.object_id = OBJECT_ID(@p1)
		  AND i.is_unique = 1
		  AND i.is_primary_key = 0
		  AND i.type_desc = 'NONCLUSTERED'
	`
	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []uniqueConstraint
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		// Get columns for this index
		colQuery := `
			SELECT COL_NAME(ic.object_id, ic.column_id) AS column_name
			FROM sys.index_columns ic
			INNER JOIN sys.indexes i ON ic.object_id = i.object_id AND ic.index_id = i.index_id
			WHERE i.object_id = OBJECT_ID(@p1) AND i.name = @p2
			ORDER BY ic.key_ordinal
		`
		colRows, err := db.QueryContext(ctx, colQuery, table, name)
		if err != nil {
			return nil, err
		}

		var cols []string
		for colRows.Next() {
			var col string
			if err := colRows.Scan(&col); err != nil {
				colRows.Close()
				return nil, err
			}
			cols = append(cols, col)
		}
		colRows.Close()

		constraints = append(constraints, uniqueConstraint{
			Name:    name,
			Columns: cols,
		})
	}

	return constraints, rows.Err()
}

type foreignKey struct {
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
}

func (p *Provider) getForeignKeys(ctx context.Context, db *sql.DB, table string) ([]foreignKey, error) {
	query := `
		SELECT
			fk.name,
			OBJECT_NAME(fk.referenced_object_id) AS ref_table
		FROM sys.foreign_keys fk
		WHERE fk.parent_object_id = OBJECT_ID(@p1)
	`
	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		var name, refTable string
		if err := rows.Scan(&name, &refTable); err != nil {
			return nil, err
		}

		// Get local columns
		colQuery := `
			SELECT COL_NAME(fkc.parent_object_id, fkc.parent_column_id) AS column_name
			FROM sys.foreign_key_columns fkc
			INNER JOIN sys.foreign_keys fk ON fkc.constraint_object_id = fk.object_id
			WHERE fk.name = @p1
			ORDER BY fkc.constraint_column_id
		`
		colRows, err := db.QueryContext(ctx, colQuery, name)
		if err != nil {
			return nil, err
		}

		var cols []string
		for colRows.Next() {
			var col string
			if err := colRows.Scan(&col); err != nil {
				colRows.Close()
				return nil, err
			}
			cols = append(cols, col)
		}
		colRows.Close()

		// Get referenced columns
		refColQuery := `
			SELECT COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) AS column_name
			FROM sys.foreign_key_columns fkc
			INNER JOIN sys.foreign_keys fk ON fkc.constraint_object_id = fk.object_id
			WHERE fk.name = @p1
			ORDER BY fkc.constraint_column_id
		`
		refColRows, err := db.QueryContext(ctx, refColQuery, name)
		if err != nil {
			return nil, err
		}

		var refCols []string
		for refColRows.Next() {
			var col string
			if err := refColRows.Scan(&col); err != nil {
				refColRows.Close()
				return nil, err
			}
			refCols = append(refCols, col)
		}
		refColRows.Close()

		fks = append(fks, foreignKey{
			Name:       name,
			Columns:    cols,
			RefTable:   refTable,
			RefColumns: refCols,
		})
	}

	return fks, rows.Err()
}

type indexInfo struct {
	Name    string
	Columns []string
}

func (p *Provider) getIndexes(ctx context.Context, db *sql.DB, table string) ([]indexInfo, error) {
	query := `
		SELECT i.name
		FROM sys.indexes i
		WHERE i.object_id = OBJECT_ID(@p1)
		  AND i.is_primary_key = 0
		  AND i.is_unique_constraint = 0
		  AND i.type_desc = 'NONCLUSTERED'
	`
	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []indexInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		// Get columns for this index
		colQuery := `
			SELECT COL_NAME(ic.object_id, ic.column_id) AS column_name
			FROM sys.index_columns ic
			INNER JOIN sys.indexes i ON ic.object_id = i.object_id AND ic.index_id = i.index_id
			WHERE i.object_id = OBJECT_ID(@p1) AND i.name = @p2
			ORDER BY ic.key_ordinal
		`
		colRows, err := db.QueryContext(ctx, colQuery, table, name)
		if err != nil {
			return nil, err
		}

		var cols []string
		for colRows.Next() {
			var col string
			if err := colRows.Scan(&col); err != nil {
				colRows.Close()
				return nil, err
			}
			cols = append(cols, col)
		}
		colRows.Close()

		indexes = append(indexes, indexInfo{
			Name:    name,
			Columns: cols,
		})
	}

	return indexes, rows.Err()
}

// StreamRows selects all rows from a table using a streaming cursor.
func (p *Provider) StreamRows(ctx context.Context, db *sql.DB, table string) (*sql.Rows, []string, error) {
	// Get columns in order
	colQuery := `
		SELECT name
		FROM sys.columns
		WHERE object_id = OBJECT_ID(@p1)
		ORDER BY column_id
	`
	colRows, err := db.QueryContext(ctx, colQuery, table)
	if err != nil {
		return nil, nil, err
	}
	defer colRows.Close()

	var cols []string
	for colRows.Next() {
		var col string
		if err := colRows.Scan(&col); err != nil {
			return nil, nil, err
		}
		cols = append(cols, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, nil, err
	}

	// Build SELECT query
	var colList strings.Builder
	for i, col := range cols {
		if i > 0 {
			colList.WriteString(", ")
		}
		colList.WriteString(p.QuoteIdent(col))
	}

	query := fmt.Sprintf("SELECT %s FROM %s", colList.String(), p.QuoteIdent(table))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	return rows, cols, nil
}

// InsertRows performs batched multi-row INSERT operations using @p1, @p2, etc. placeholders.
func (p *Provider) InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error {
	if len(cols) == 0 {
		return fmt.Errorf("no columns supplied for insert into %s", table)
	}
	if len(rows) == 0 {
		return nil
	}

	// Build INSERT statement with multi-row VALUES
	var sb strings.Builder
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

	// Build VALUES clauses with incrementing placeholders
	args := make([]any, 0, len(rows)*len(cols))
	paramNum := 1

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
			sb.WriteString(fmt.Sprintf("@p%d", paramNum))
			paramNum++
		}
		sb.WriteString(")")
		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	return err
}

// DisableConstraints temporarily disables all constraints for all tables.
func (p *Provider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) {
	// Get all user tables
	tablesQuery := `SELECT name FROM sys.tables WHERE is_ms_shipped = 0`
	rows, err := db.QueryContext(ctx, tablesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Disable all constraints for each table
	for _, table := range tables {
		stmt := fmt.Sprintf("ALTER TABLE %s NOCHECK CONSTRAINT ALL", p.QuoteIdent(table))
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("disable constraints on %s: %w", table, err)
		}
	}

	// Return restore function
	restore := func(ctx context.Context) error {
		for _, table := range tables {
			stmt := fmt.Sprintf("ALTER TABLE %s WITH CHECK CHECK CONSTRAINT ALL", p.QuoteIdent(table))
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("enable constraints on %s: %w", table, err)
			}
		}
		return nil
	}

	return restore, nil
}

// QuoteIdent quotes a SQL Server identifier using brackets.
func (*Provider) QuoteIdent(ident string) string {
	return "[" + strings.ReplaceAll(ident, "]", "]]") + "]"
}

var sqlServerEscapeReplacer = strings.NewReplacer(
	"'", "''",
)

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
		if utf8.Valid(v) {
			return quoteString(string(v)), nil
		}
		return "0x" + hex.EncodeToString(v), nil
	case string:
		return quoteString(v), nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case time.Time:
		return quoteString(v.Format("2006-01-02 15:04:05.000")), nil
	case fmt.Stringer:
		return quoteString(v.String()), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	}

	// Handle []uint8 alias when provided through database/sql
	if b, ok := asBytes(value); ok {
		if utf8.Valid(b) {
			return quoteString(string(b)), nil
		}
		return "0x" + hex.EncodeToString(b), nil
	}

	return "", fmt.Errorf("unsupported literal type %T", value)
}

// DatabaseMetadata returns the collation for the given database.
func (*Provider) DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (provider.DatabaseMetadata, error) {
	const query = `SELECT collation_name FROM sys.databases WHERE name = @p1`
	var collation string
	err := db.QueryRowContext(ctx, query, dbName).Scan(&collation)
	if errors.Is(err, sql.ErrNoRows) {
		return provider.DatabaseMetadata{}, fmt.Errorf("database %s not found", dbName)
	}
	if err != nil {
		return provider.DatabaseMetadata{}, err
	}
	return provider.DatabaseMetadata{Collation: collation}, nil
}

// EnsureDatabase makes sure the destination database exists, creating it with matching collation if needed.
func (p *Provider) EnsureDatabase(ctx context.Context, dsn string, dbName string, meta provider.DatabaseMetadata) error {
	// Parse DSN to connect to master database
	masterDSN := strings.Replace(dsn, "database="+dbName, "database=master", 1)

	masterDB, err := sql.Open(p.DriverName(), masterDSN)
	if err != nil {
		return fmt.Errorf("open master database: %w", err)
	}
	defer masterDB.Close()

	if err := masterDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping master database: %w", err)
	}

	// Check if database exists
	var exists int
	checkQuery := `SELECT COUNT(*) FROM sys.databases WHERE name = @p1`
	if err := masterDB.QueryRowContext(ctx, checkQuery, dbName).Scan(&exists); err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}

	if exists > 0 {
		return nil
	}

	// Create database with collation
	var createStmt string
	if meta.Collation != "" {
		createStmt = fmt.Sprintf("CREATE DATABASE %s COLLATE %s", p.QuoteIdent(dbName), meta.Collation)
	} else {
		createStmt = fmt.Sprintf("CREATE DATABASE %s", p.QuoteIdent(dbName))
	}

	if _, err := masterDB.ExecContext(ctx, createStmt); err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}

	return nil
}

func quoteString(s string) string {
	return "'" + sqlServerEscapeReplacer.Replace(s) + "'"
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
