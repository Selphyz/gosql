package conn

import (
	"testing"

	driver "github.com/go-sql-driver/mysql"

	_ "gosql/internal/provider/mysql"
)

func TestNormalizeWithExplicitProvider(t *testing.T) {
	prov, dsn, dbName, err := Normalize("mysql", "user:pass@tcp(localhost:3306)/sample")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if prov == nil {
		t.Fatal("expected provider")
	}
	if dbName != "sample" {
		t.Fatalf("expected db name %q, got %q", "sample", dbName)
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("invalid DSN: %v", err)
	}
	if cfg.DBName != "sample" {
		t.Fatalf("expected db name %q, got %q", "sample", cfg.DBName)
	}
}

func TestNormalizeDetectsByScheme(t *testing.T) {
	prov, dsn, dbName, err := Normalize("", "mysql://root:secret@localhost:3306/sample")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if prov == nil {
		t.Fatal("expected provider")
	}
	if dbName != "sample" {
		t.Fatalf("expected db name %q, got %q", "sample", dbName)
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("invalid DSN: %v", err)
	}
	if cfg.Addr != "localhost:3306" || cfg.User != "root" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}
}

func TestNormalizeDetectsMySQLDSN(t *testing.T) {
	_, _, _, err := Normalize("", "user:pass@tcp(localhost:3306)/sample")
	if err != nil {
		t.Fatalf("expected detection to succeed, got %v", err)
	}
}
