package conn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/conn"
	"gosql/internal/provider"
	_ "gosql/internal/provider/mysql"
	_ "gosql/test/testutil/fakeprovider"
)

func TestNormalize_ExplicitProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		input    string
		wantErr  bool
	}{
		{
			name:     "explicit fake provider success",
			provider: "fake",
			input:    "testdb",
			wantErr:  false,
		},
		{
			name:     "explicit unknown provider",
			provider: "unknown",
			input:    "testdb",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, dsn, dbName, err := conn.Normalize(tt.provider, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, prov)
				assert.NotEmpty(t, dsn)
				assert.NotEmpty(t, dbName)
			}
		})
	}
}

func TestNormalize_DetectByScheme(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "mysql scheme",
			input:   "mysql://user:pass@localhost:3306/testdb",
			wantErr: false,
		},
		{
			name:    "unknown scheme",
			input:   "unknownscheme://localhost/db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, dsn, dbName, err := conn.Normalize("", tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, prov)
				assert.NotEmpty(t, dsn)
				assert.NotEmpty(t, dbName)
			}
		})
	}
}

func TestNormalize_DetectByHeuristic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "mysql DSN with @tcp",
			input:   "user:pass@tcp(localhost:3306)/testdb",
			wantErr: false,
		},
		{
			name:    "mysql DSN with @unix",
			input:   "user:pass@unix(/tmp/mysql.sock)/testdb",
			wantErr: false,
		},
		{
			name:    "mysql DSN with @/",
			input:   "user:pass@/testdb",
			wantErr: false,
		},
		{
			name:    "undetectable connection string",
			input:   "some-random-string",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, dsn, dbName, err := conn.Normalize("", tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, prov)
				assert.NotEmpty(t, dsn)
				assert.NotEmpty(t, dbName)
			}
		})
	}
}

func TestNormalize_ParseConnectionError(t *testing.T) {
	// When a provider is detected but ParseConnection fails
	prov, dsn, dbName, err := conn.Normalize("", "error")
	assert.Error(t, err)
	assert.Nil(t, prov)
	assert.Empty(t, dsn)
	assert.Empty(t, dbName)
}

func TestNormalize_ReturnedProvider(t *testing.T) {
	// Verify that the returned provider implements SQLProvider
	prov, _, _, err := conn.Normalize("fake", "testdb")
	require.NoError(t, err)

	// Provider should implement the SQLProvider interface
	var _ provider.SQLProvider = prov
	assert.NotNil(t, prov)
	assert.Equal(t, "sqlmock", prov.DriverName())
}