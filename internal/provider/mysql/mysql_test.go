package mysql

import (
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

func TestParseConnectionDSN(t *testing.T) {
	p := &Provider{}
	input := "user:pass@tcp(localhost:3306)/sampledb?charset=utf8mb4"
	dsn, dbName, err := p.ParseConnection(input)
	if err != nil {
		t.Fatalf("ParseConnection returned error: %v", err)
	}
	if dbName != "sampledb" {
		t.Fatalf("expected db name %q, got %q", "sampledb", dbName)
	}
	if cfg, err := driver.ParseDSN(dsn); err != nil {
		t.Fatalf("resulting DSN invalid: %v", err)
	} else if cfg.User != "user" || cfg.Passwd != "pass" || cfg.DBName != "sampledb" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}
	if !strings.Contains(dsn, "allowNativePasswords=true") {
		t.Fatalf("expected DSN to contain allowNativePasswords, got %q", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Fatalf("expected DSN to contain charset, got %q", dsn)
	}
}

func TestParseConnectionURL(t *testing.T) {
	p := &Provider{}
	input := "mysql://user:pass@db.example.com:3307/sampledb?tls=skip-verify"
	dsn, dbName, err := p.ParseConnection(input)
	if err != nil {
		t.Fatalf("ParseConnection returned error: %v", err)
	}
	if dbName != "sampledb" {
		t.Fatalf("expected db name %q, got %q", "sampledb", dbName)
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("resulting DSN invalid: %v", err)
	}
	if cfg.Addr != "db.example.com:3307" || cfg.Net != "tcp" {
		t.Fatalf("unexpected host: %+v", cfg)
	}
	if !strings.Contains(dsn, "tls=skip-verify") {
		t.Fatalf("expected DSN to contain tls parameter, got %q", dsn)
	}
	if !strings.Contains(dsn, "allowNativePasswords=true") {
		t.Fatalf("expected DSN to contain allowNativePasswords, got %q", dsn)
	}
}

func TestQuoteIdent(t *testing.T) {
	p := &Provider{}
	got := p.QuoteIdent("weird`name")
	if want := "`weird``name`"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestQuoteLiteral(t *testing.T) {
	p := &Provider{}

	now := time.Date(2024, 1, 2, 3, 4, 5, 123000000, time.UTC)
	cases := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, "NULL"},
		{"string", "O'Brien", "'O\\'Brien'"},
		{"boolTrue", true, "1"},
		{"boolFalse", false, "0"},
		{"int", int64(42), "42"},
		{"float", 3.14, "3.14"},
		{"bytes", []byte("hello"), "'hello'"},
		{"bytesBinary", []byte{0x00, 0xff}, "0x00ff"},
		{"time", now, "'2024-01-02 03:04:05.123'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.QuoteLiteral(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
