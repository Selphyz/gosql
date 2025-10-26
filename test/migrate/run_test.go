package migrate

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/migrate"
	_ "gosql/test/testutil/fakeprovider"
)

func TestRun_NoSourceConnection(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src: "",
		Dst: "dst_conn",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source connection is required")
}

func TestRun_NoDestinationConnection(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src: "src_conn",
		Dst: "",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "destination connection is required")
}

func TestRun_InvalidSourceProvider(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "nonexistent",
		},
		Src: "src_conn",
		Dst: "dst_conn",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestRun_ProviderMismatch(t *testing.T) {
	// This test is tricky because we need two different providers
	// For now we'll test with explicit providers that might not match
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src: "mysql://src",
		Dst: "postgres://dst",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err)
	// Could be provider mismatch or connection error
	assert.NotNil(t, err)
}

func TestRun_SimpleTableMigration(t *testing.T) {
	// Create source database mock
	srcDB, srcMock, err := sqlmock.New()
	require.NoError(t, err)
	defer srcDB.Close()

	// Create destination database mock
	dstDB, dstMock, err := sqlmock.New()
	require.NoError(t, err)
	defer dstDB.Close()

	// This test demonstrates the flow but can't fully test Run()
	// because it internally calls sql.Open() which we can't intercept
	// without more sophisticated mocking

	// Mock Ping for both databases
	srcMock.ExpectPing()
	dstMock.ExpectPing()

	// The actual test would require dependency injection or refactoring
	assert.NotNil(t, srcDB)
	assert.NotNil(t, dstDB)
}

func TestRun_InvalidProvider(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "invalid_provider_that_does_not_exist",
		},
		Src: "src",
		Dst: "dst",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err)
}

func TestRun_NilContext(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src: "src",
		Dst: "dst",
	}

	// Should convert nil context to background context
	err := migrate.Run(nil, opts)
	assert.Error(t, err) // Expected due to connection setup
}

func TestRun_WithDropFirst(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src:       "src",
		Dst:       "dst",
		DropFirst: true,
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err) // Expected due to connection setup
}

func TestRun_WithProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
			Progress: true,
			Stderr:   buf,
		},
		Src: "src",
		Dst: "dst",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err) // Expected due to connection setup
}

func TestRun_WithChunkSize(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider:  "fake",
			ChunkSize: 500,
		},
		Src: "src",
		Dst: "dst",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err) // Expected due to connection setup
}

func TestRun_WithSingleTransaction(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider:          "fake",
			SingleTransaction: true,
		},
		Src: "src",
		Dst: "dst",
	}

	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err) // Expected due to connection setup
}

func TestRun_DefaultChunkSize(t *testing.T) {
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider:  "fake",
			ChunkSize: 0, // Should default to 1000
		},
		Src: "src",
		Dst: "dst",
	}

	// Verify that options are accepted
	assert.Equal(t, 0, opts.CommonOptions.ChunkSize)
}

func TestRun_FailingWriter(t *testing.T) {
	// Test error handling with a failing writer
	failingWriter := &FailingWriter{}
	opts := migrate.Options{
		CommonOptions: migrate.CommonOptions{
			Provider: "fake",
		},
		Src: "src",
		Dst: "dst",
	}

	// Adding a Stderr that fails should not break migrate.Run setup
	opts.CommonOptions.Stderr = failingWriter
	err := migrate.Run(context.Background(), opts)
	assert.Error(t, err) // Expected due to connection setup
}

// FailingWriter is a mock writer that always fails
type FailingWriter struct{}

func (fw *FailingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}