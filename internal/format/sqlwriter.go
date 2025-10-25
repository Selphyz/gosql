package format

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gosql/internal/provider"
)

// Header carries metadata for the emitted dump.
type Header struct {
	ProviderName  string
	DatabaseName  string
	ServerVersion string
	GeneratedAt   time.Time
}

// SQLWriter assists in writing SQL statements with consistent formatting.
type SQLWriter struct {
	w        io.Writer
	provider provider.SQLProvider
}

// NewSQLWriter constructs a writer bound to the given provider.
func NewSQLWriter(w io.Writer, prov provider.SQLProvider) *SQLWriter {
	return &SQLWriter{
		w:        w,
		provider: prov,
	}
}

// WriteHeader emits the dump header with metadata comments.
func (sw *SQLWriter) WriteHeader(meta Header) error {
	if meta.GeneratedAt.IsZero() {
		meta.GeneratedAt = time.Now().UTC()
	}

	lines := []string{
		"-- gosql dump",
		fmt.Sprintf("-- Provider: %s (server %s)", meta.ProviderName, meta.ServerVersion),
		fmt.Sprintf("-- Database: %s", meta.DatabaseName),
		fmt.Sprintf("-- Generated: %s", meta.GeneratedAt.Format(time.RFC3339)),
		"--",
		"",
		fmt.Sprintf("/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;"),
		fmt.Sprintf("/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;"),
		fmt.Sprintf("/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;"),
		fmt.Sprintf("/*!40101 SET NAMES utf8mb4 */;"),
		"",
	}

	for _, line := range lines {
		if _, err := io.WriteString(sw.w, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// WriteTableDefinition outputs DROP TABLE and CREATE TABLE statements for a table.
func (sw *SQLWriter) WriteTableDefinition(table, ddl string) error {
	if _, err := fmt.Fprintf(sw.w, "\n--\n-- Table structure for table %s\n--\n\n", sw.provider.QuoteIdent(table)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(sw.w, "DROP TABLE IF EXISTS %s;\n", sw.provider.QuoteIdent(table)); err != nil {
		return err
	}

	ddl = strings.TrimSpace(ddl)
	if !strings.HasSuffix(ddl, ";") {
		ddl += ";"
	}
	if _, err := io.WriteString(sw.w, ddl+"\n"); err != nil {
		return err
	}
	return nil
}

// WriteTableDataPreamble writes a comment before data rows.
func (sw *SQLWriter) WriteTableDataPreamble(table string) error {
	_, err := fmt.Fprintf(sw.w, "\n--\n-- Dumping data for table %s\n--\n\n", sw.provider.QuoteIdent(table))
	return err
}

// WriteInsertBatch writes a batched INSERT statement for the provided rows.
func (sw *SQLWriter) WriteInsertBatch(table string, columns []string, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.Grow(len(rows) * len(columns) * 8)

	sb.WriteString("INSERT INTO ")
	sb.WriteString(sw.provider.QuoteIdent(table))
	sb.WriteString(" (")
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(sw.provider.QuoteIdent(col))
	}
	sb.WriteString(") VALUES\n")

	for i, row := range rows {
		sb.WriteString("  (")
		for j, value := range row {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(value)
		}
		sb.WriteString(")")
		if i < len(rows)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString(";\n")
		}
	}

	_, err := io.WriteString(sw.w, sb.String())
	return err
}
