package mysql

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

	driver "github.com/go-sql-driver/mysql"

	"gosql/internal/provider"
)

// Provider implements provider.SQLProvider for MySQL-compatible databases.
type Provider struct{}

var _ provider.SQLProvider = (*Provider)(nil)

func init() {
	provider.Register("mysql", &Provider{}, "mysql")
}

// DriverName returns the Go SQL driver name.
func (*Provider) DriverName() string {
	return "mysql"
}

// ParseConnection normalises supported MySQL connection strings to driver DSNs.
func (*Provider) ParseConnection(input string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(input), "mysql://") {
		return parseURLConnection(input)
	}
	return parseDSNConnection(input)
}

func parseDSNConnection(input string) (string, string, error) {
	cfg, err := driver.ParseDSN(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid MySQL DSN: %w", err)
	}
	if cfg.DBName == "" {
		return "", "", fmt.Errorf("MySQL DSN missing database name")
	}
	cfg.Net = normalizeNet(cfg.Net)
	if !containsInsensitive(input, "allownativepasswords") {
		cfg.AllowNativePasswords = true
		if cfg.Params == nil {
			cfg.Params = make(map[string]string)
		}
		cfg.Params["allowNativePasswords"] = "true"
	}
	return cfg.FormatDSN(), cfg.DBName, nil
}

func parseURLConnection(input string) (string, string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid MySQL URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "mysql") {
		return "", "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	cfg := &driver.Config{
		Net:  "tcp",
		Addr: ensurePort(u.Host, "3306"),
	}

	if cfg.Addr == "" {
		return "", "", fmt.Errorf("MySQL URL missing host")
	}

	if u.User != nil {
		cfg.User = u.User.Username()
		if pwd, ok := u.User.Password(); ok {
			cfg.Passwd = pwd
		}
	}

	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		cfg.DBName = path
	}

	if len(u.RawQuery) > 0 {
		values, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", "", fmt.Errorf("invalid MySQL URL query: %w", err)
		}
		cfg.Params = make(map[string]string, len(values))
		for key, list := range values {
			if len(list) == 0 {
				continue
			}
			cfg.Params[key] = list[len(list)-1]
		}
	}

	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	if !hasKeyInsensitive(cfg.Params, "allownativepasswords") {
		cfg.AllowNativePasswords = true
		cfg.Params["allowNativePasswords"] = "true"
	}

	if cfg.DBName == "" {
		return "", "", fmt.Errorf("MySQL URL missing database name")
	}

	return cfg.FormatDSN(), cfg.DBName, nil
}

func normalizeNet(net string) string {
	if net == "" {
		return "tcp"
	}
	return net
}

func ensurePort(addr, defaultPort string) string {
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	return addr + ":" + defaultPort
}

func containsInsensitive(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func hasKeyInsensitive(m map[string]string, key string) bool {
	for k := range m {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// ListTables returns the list of base tables for the current schema.
func (*Provider) ListTables(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME;
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

// ShowCreateTable returns the CREATE TABLE statement for the given table.
func (p *Provider) ShowCreateTable(ctx context.Context, db *sql.DB, table string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE TABLE %s", p.QuoteIdent(table))
	row := db.QueryRowContext(ctx, query)

	var name, ddl string
	if err := row.Scan(&name, &ddl); err != nil {
		return "", err
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
	placeholder := "(" + strings.TrimRight(strings.Repeat("?, ", len(cols)), ", ") + ")"

	for i, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("row %d column count mismatch for table %s", i, table)
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(placeholder)
		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	return err
}

// DisableConstraints temporarily disables foreign key checks, returning a restore callback.
func (*Provider) DisableConstraints(ctx context.Context, db *sql.DB) (func(context.Context) error, error) {
	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return nil, err
	}
	restore := func(ctx context.Context) error {
		_, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")
		return err
	}
	return restore, nil
}

// QuoteIdent quotes a MySQL identifier using backticks.
func (*Provider) QuoteIdent(ident string) string {
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}

var escapeReplacer = strings.NewReplacer(
	"\\", "\\\\",
	"'", "\\'",
	"\x00", "\\0",
	"\n", "\\n",
	"\r", "\\r",
	"\x1a", "\\Z",
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
		if utf8.Valid(b) {
			return quoteString(string(b)), nil
		}
		return "0x" + hex.EncodeToString(b), nil
	}

	return "", fmt.Errorf("unsupported literal type %T", value)
}

func quoteString(s string) string {
	return "'" + escapeReplacer.Replace(s) + "'"
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
	const layout = "2006-01-02 15:04:05.000000"
	formatted := t.Format(layout)
	formatted = strings.TrimRight(formatted, "0")
	if strings.HasSuffix(formatted, ".") {
		formatted = formatted[:len(formatted)-1]
	}
	if strings.Contains(formatted, ".") {
		return formatted
	}
	return t.Format("2006-01-02 15:04:05")
}
