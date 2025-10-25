package format

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"gosql/internal/provider/mysql"
)

func TestWriteHeader(t *testing.T) {
	prov := &mysql.Provider{}
	var buf bytes.Buffer
	writer := NewSQLWriter(&buf, prov)

	ts := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	err := writer.WriteHeader(Header{
		ProviderName:  "mysql",
		DatabaseName:  "sample",
		ServerVersion: "8.0.36",
		GeneratedAt:   ts,
	})
	if err != nil {
		t.Fatalf("WriteHeader error: %v", err)
	}

	output := buf.String()
	for _, needle := range []string{
		"-- gosql dump",
		"-- Provider: mysql (server 8.0.36)",
		"-- Database: sample",
		"-- Generated: 2024-05-06T07:08:09Z",
	} {
		if !strings.Contains(output, needle) {
			t.Fatalf("expected header to contain %q, got:\n%s", needle, output)
		}
	}
}

func TestWriteTableDefinitionAndData(t *testing.T) {
	prov := &mysql.Provider{}
	var buf bytes.Buffer
	writer := NewSQLWriter(&buf, prov)

	if err := writer.WriteTableDefinition("users", "CREATE TABLE `users` (`id` int)"); err != nil {
		t.Fatalf("WriteTableDefinition error: %v", err)
	}
	if err := writer.WriteTableDataPreamble("users"); err != nil {
		t.Fatalf("WriteTableDataPreamble error: %v", err)
	}

	rows := [][]string{
		{"1", "'alice'"},
		{"2", "'bob'"},
	}
	if err := writer.WriteInsertBatch("users", []string{"id", "name"}, rows); err != nil {
		t.Fatalf("WriteInsertBatch error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DROP TABLE IF EXISTS `users`;") {
		t.Fatalf("expected drop statement in output:\n%s", output)
	}
	expectedInsert := "INSERT INTO `users` (`id`, `name`) VALUES\n  (1, 'alice'),\n  (2, 'bob');"
	if !strings.Contains(output, expectedInsert) {
		t.Fatalf("expected insert batch:\n%s\n\ngot:\n%s", expectedInsert, output)
	}
}
