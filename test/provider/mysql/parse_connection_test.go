package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/provider/mysql"
)

func TestParseConnection_DSNFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDBErr bool
		wantDB    string
	}{
		{
			name:      "simple DSN",
			input:     "user:pass@tcp(localhost:3306)/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "DSN with unix socket",
			input:     "user:pass@unix(/tmp/mysql.sock)/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "DSN with localhost shorthand",
			input:     "user:pass@/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "DSN missing database",
			input:     "user:pass@tcp(localhost:3306)/",
			wantDBErr: true,
			wantDB:    "",
		},
		{
			name:      "invalid DSN format",
			input:     "not-a-valid-dsn@@@",
			wantDBErr: true,
			wantDB:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			dsn, dbName, err := prov.ParseConnection(tt.input)

			if tt.wantDBErr {
				assert.Error(t, err)
				assert.Empty(t, dbName)
				assert.Empty(t, dsn)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDB, dbName)
				assert.NotEmpty(t, dsn)
				// AllowNativePasswords should be set
				assert.Contains(t, dsn, "allowNativePasswords=true")
			}
		})
	}
}

func TestParseConnection_URLFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDBErr bool
		wantDB    string
	}{
		{
			name:      "mysql URL with user and password",
			input:     "mysql://user:pass@localhost:3306/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "mysql URL with user only",
			input:     "mysql://user@localhost/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "mysql URL with default port",
			input:     "mysql://user:pass@localhost/testdb",
			wantDBErr: false,
			wantDB:    "testdb",
		},
		{
			name:      "mysql URL missing database",
			input:     "mysql://user:pass@localhost:3306",
			wantDBErr: true,
			wantDB:    "",
		},
		{
			name:      "mysql URL missing host",
			input:     "mysql://user:pass@/testdb",
			wantDBErr: true,
			wantDB:    "",
		},
		{
			name:      "invalid URL",
			input:     "mysql://[invalid",
			wantDBErr: true,
			wantDB:    "",
		},
		{
			name:      "wrong scheme",
			input:     "postgres://user:pass@localhost/testdb",
			wantDBErr: true,
			wantDB:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mysql.Provider{}
			dsn, dbName, err := prov.ParseConnection(tt.input)

			if tt.wantDBErr {
				assert.Error(t, err)
				assert.Empty(t, dbName)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDB, dbName)
				assert.NotEmpty(t, dsn)
				assert.Contains(t, dsn, "allowNativePasswords=true")
			}
		})
	}
}

func TestParseConnection_AllowNativePasswords(t *testing.T) {
	// DSN without allowNativePasswords should have it added
	prov := &mysql.Provider{}
	dsn, _, err := prov.ParseConnection("user:pass@tcp(localhost)/testdb")
	require.NoError(t, err)
	assert.Contains(t, dsn, "allowNativePasswords=true")

	// URL without allowNativePasswords should have it added
	dsn, _, err = prov.ParseConnection("mysql://user:pass@localhost/testdb")
	require.NoError(t, err)
	assert.Contains(t, dsn, "allowNativePasswords=true")
}

func TestParseConnection_QueryParameters(t *testing.T) {
	prov := &mysql.Provider{}
	dsn, dbName, err := prov.ParseConnection("mysql://user:pass@localhost/testdb?charset=utf8mb4&parseTime=true")
	require.NoError(t, err)
	assert.Equal(t, "testdb", dbName)
	assert.Contains(t, dsn, "charset=utf8mb4")
	assert.Contains(t, dsn, "parseTime=true")
}

func TestDriverName(t *testing.T) {
	prov := &mysql.Provider{}
	assert.Equal(t, "mysql", prov.DriverName())
}