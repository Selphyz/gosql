package oracle

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gosql/internal/provider"
)

// Provider implements provider.SQLProvider for Oracle databases.
type Provider struct{}

var _ provider.SQLProvider = (*Provider)(nil)

func init() {
	provider.Register("oracle", &Provider{}, "oracle", "oci", "ora")
}

// DriverName returns the Go SQL driver name.
func (*Provider) DriverName() string {
	return "oracle"
}

// ParseConnection normalizes supported Oracle connection strings to driver DSNs.
// Supports: oracle://user:pass@host:port/service or ezconnect-like strings.
func (*Provider) ParseConnection(input string) (string, string, error) {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "oracle://") || strings.HasPrefix(lower, "oci://") || strings.HasPrefix(lower, "ora://") {
		return parseURLConnection(input)
	}
	return parseEZConnect(input)
}

func parseURLConnection(input string) (string, string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid Oracle URL: %w", err)
	}

	var user, password, host, port, service string

	if u.User != nil {
		user = u.User.Username()
		if pwd, ok := u.User.Password(); ok {
			password = pwd
		}
	}

	host = u.Hostname()
	port = u.Port()
	if port == "" {
		port = "1521"
	}

	service = strings.TrimPrefix(u.Path, "/")
	if service == "" {
		return "", "", fmt.Errorf("Oracle URL missing service name")
	}

	// Build go-ora DSN: oracle://user:pass@host:port/service
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%s/%s", user, password, host, port, service)
	return dsn, service, nil
}

func parseEZConnect(input string) (string, string, error) {
	// Try to parse ezconnect format: user/pass@host:port/service
	// This is a simplified parser for common forms
	atIdx := strings.Index(input, "@")
	if atIdx == -1 {
		return "", "", fmt.Errorf("invalid Oracle connection string: missing '@'")
	}

	userPart := input[:atIdx]
	hostPart := input[atIdx+1:]

	// Split user/pass
	parts := strings.SplitN(userPart, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid Oracle connection string: user/password format required")
	}
	user := parts[0]
	password := parts[1]

	// Parse host:port/service
	slashIdx := strings.LastIndex(hostPart, "/")
	if slashIdx == -1 {
		return "", "", fmt.Errorf("invalid Oracle connection string: missing service name")
	}

	hostPort := hostPart[:slashIdx]
	service := hostPart[slashIdx+1:]

	host := hostPort
	port := "1521"
	if colonIdx := strings.LastIndex(hostPort, ":"); colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}

	dsn := fmt.Sprintf("oracle://%s:%s@%s:%s/%s", user, password, host, port, service)
	return dsn, service, nil
}

// ListTables returns the list of user tables in the current schema.
func (*Provider) ListTables(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `SELECT TABLE_NAME FROM USER_TABLES ORDER BY TABLE_NAME`
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

		if col.Nullable == "N" {
			sb.WriteString(" NOT NULL")
		}

		if col.DefaultValue != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.DefaultValue)
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
	Name         string
	DataType     string
	Nullable     string
	DefaultValue string
}

func (p *Provider) getColumns(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	query := `
		SELECT COLUMN_NAME, DATA_TYPE, DATA_LENGTH, DATA_PRECISION, DATA_SCALE, NULLABLE, DATA_DEFAULT
		FROM USER_TAB_COLUMNS
		WHERE TABLE_NAME = :1
		ORDER BY COLUMN_ID
	`
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var name, dataType, nullable string
		var length, precision, scale sql.NullInt64
		var defaultVal sql.NullString

		if err := rows.Scan(&name, &dataType, &length, &precision, &scale, &nullable, &defaultVal); err != nil {
			return nil, err
		}

		// Build data type string
		var typeStr string
		switch dataType {
		case "VARCHAR2", "NVARCHAR2", "CHAR", "NCHAR":
			typeStr = fmt.Sprintf("%s(%d)", dataType, length.Int64)
		case "NUMBER":
			if precision.Valid && scale.Valid {
				if scale.Int64 == 0 {
					typeStr = fmt.Sprintf("NUMBER(%d)", precision.Int64)
				} else {
					typeStr = fmt.Sprintf("NUMBER(%d,%d)", precision.Int64, scale.Int64)
				}
			} else if precision.Valid {
				typeStr = fmt.Sprintf("NUMBER(%d)", precision.Int64)
			} else {
				typeStr = "NUMBER"
			}
		default:
			typeStr = dataType
		}

		col := columnInfo{
			Name:     name,
			DataType: typeStr,
			Nullable: nullable,
		}

		if defaultVal.Valid {
			col.DefaultValue = strings.TrimSpace(defaultVal.String)
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (p *Provider) getPrimaryKey(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	query := `
		SELECT COLUMN_NAME
		FROM USER_CONS_COLUMNS
		WHERE CONSTRAINT_NAME = (
			SELECT CONSTRAINT_NAME
			FROM USER_CONSTRAINTS
			WHERE TABLE_NAME = :1 AND CONSTRAINT_TYPE = 'P'
		)
		ORDER BY POSITION
	`
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(table))
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
		SELECT CONSTRAINT_NAME
		FROM USER_CONSTRAINTS
		WHERE TABLE_NAME = :1 AND CONSTRAINT_TYPE = 'U'
	`
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(table))
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

		// Get columns for this constraint
		colQuery := `
			SELECT COLUMN_NAME
			FROM USER_CONS_COLUMNS
			WHERE CONSTRAINT_NAME = :1
			ORDER BY POSITION
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
		SELECT c.CONSTRAINT_NAME, c.R_CONSTRAINT_NAME,
		       (SELECT TABLE_NAME FROM USER_CONSTRAINTS WHERE CONSTRAINT_NAME = c.R_CONSTRAINT_NAME) AS REF_TABLE
		FROM USER_CONSTRAINTS c
		WHERE c.TABLE_NAME = :1 AND c.CONSTRAINT_TYPE = 'R'
	`
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKey
	for rows.Next() {
		var name, refConstraint, refTable string
		if err := rows.Scan(&name, &refConstraint, &refTable); err != nil {
			return nil, err
		}

		// Get local columns
		colQuery := `
			SELECT COLUMN_NAME
			FROM USER_CONS_COLUMNS
			WHERE CONSTRAINT_NAME = :1
			ORDER BY POSITION
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
		refColRows, err := db.QueryContext(ctx, colQuery, refConstraint)
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
	// Get non-unique indexes (exclude unique indexes and those created by constraints)
	query := `
		SELECT INDEX_NAME
		FROM USER_INDEXES
		WHERE TABLE_NAME = :1
		  AND UNIQUENESS = 'NONUNIQUE'
		  AND INDEX_NAME NOT IN (
		      SELECT CONSTRAINT_NAME FROM USER_CONSTRAINTS WHERE TABLE_NAME = :1
		  )
	`
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(table), strings.ToUpper(table))
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
			SELECT COLUMN_NAME
			FROM USER_IND_COLUMNS
			WHERE INDEX_NAME = :1
			ORDER BY COLUMN_POSITION
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
		SELECT COLUMN_NAME
		FROM USER_TAB_COLUMNS
		WHERE TABLE_NAME = :1
		ORDER BY COLUMN_ID
	`
	colRows, err := db.QueryContext(ctx, colQuery, strings.ToUpper(table))
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

// InsertRows performs row-by-row INSERT operations using positional parameters.
func (p *Provider) InsertRows(ctx context.Context, db *sql.DB, table string, cols []string, rows [][]any) error {
	if len(cols) == 0 {
		return fmt.Errorf("no columns supplied for insert into %s", table)
	}
	if len(rows) == 0 {
		return nil
	}

	// Build INSERT statement
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
	sb.WriteString(") VALUES (")
	for i := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf(":%d", i+1))
	}
	sb.WriteString(")")

	stmt := sb.String()

	// Execute row by row (can be optimized later with array binding)
	for i, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("row %d column count mismatch for table %s", i, table)
		}
		if _, err := db.ExecContext(ctx, stmt, row...); err != nil {
			return fmt.Errorf("insert row %d: %w", i, err)
		}
	}

	return nil
}

// DisableConstraints temporarily disables constraints for the current schema.
func (p *Provider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) {
	// Get all constraints to disable
	query := `
		SELECT TABLE_NAME, CONSTRAINT_NAME
		FROM USER_CONSTRAINTS
		WHERE CONSTRAINT_TYPE IN ('R', 'U', 'P')
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type constraint struct {
		table string
		name  string
	}
	var constraints []constraint

	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			return nil, err
		}
		constraints = append(constraints, constraint{table: table, name: name})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Disable all constraints
	for _, c := range constraints {
		stmt := fmt.Sprintf("ALTER TABLE %s DISABLE CONSTRAINT %s",
			p.QuoteIdent(c.table), p.QuoteIdent(c.name))
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("disable constraint %s.%s: %w", c.table, c.name, err)
		}
	}

	// Return restore function
	restore := func(ctx context.Context) error {
		for _, c := range constraints {
			stmt := fmt.Sprintf("ALTER TABLE %s ENABLE CONSTRAINT %s",
				p.QuoteIdent(c.table), p.QuoteIdent(c.name))
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("enable constraint %s.%s: %w", c.table, c.name, err)
			}
		}
		return nil
	}

	return restore, nil
}

// QuoteIdent quotes an Oracle identifier using double quotes.
func (*Provider) QuoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

var oracleEscapeReplacer = strings.NewReplacer(
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
		return "HEXTORAW('" + hex.EncodeToString(v) + "')", nil
	case string:
		return quoteString(v), nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case time.Time:
		return fmt.Sprintf("TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS')", v.Format("2006-01-02 15:04:05")), nil
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
		return "HEXTORAW('" + hex.EncodeToString(b) + "')", nil
	}

	return "", fmt.Errorf("unsupported literal type %T", value)
}

// DatabaseMetadata returns an empty metadata for Oracle (no per-schema charset).
func (*Provider) DatabaseMetadata(ctx context.Context, db *sql.DB, dbName string) (provider.DatabaseMetadata, error) {
	// Oracle doesn't have per-schema charset/collation in the same way MySQL does
	return provider.DatabaseMetadata{}, nil
}

// EnsureDatabase is a no-op for Oracle as schemas are tied to users.
func (*Provider) EnsureDatabase(ctx context.Context, dsn string, dbName string, meta provider.DatabaseMetadata) error {
	// Oracle schemas are user-specific; database creation requires DBA privileges
	// This is intentionally a no-op
	return nil
}

func quoteString(s string) string {
	return "'" + oracleEscapeReplacer.Replace(s) + "'"
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
