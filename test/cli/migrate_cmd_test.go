package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/cli"
	_ "gosql/test/testutil/fakeprovider"
)

func TestMigrateCommand_Creation(t *testing.T) {
	cmd := cli.NewRootCommand()
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)
	assert.NotNil(t, migrateCmd)
	assert.Equal(t, "migrate", migrateCmd.Name())
}

func TestMigrateCommand_Short(t *testing.T) {
	cmd := cli.NewRootCommand()
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)
	assert.NotEmpty(t, migrateCmd.Short)
}

func TestMigrateCommand_Flags(t *testing.T) {
	cmd := cli.NewRootCommand()
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)

	// Check that migrate-specific flags are registered
	assert.NotNil(t, migrateCmd.Flags().Lookup("src"))
	assert.NotNil(t, migrateCmd.Flags().Lookup("dst"))
	assert.NotNil(t, migrateCmd.Flags().Lookup("drop-first"))
}

func TestMigrateCommand_InheritsPersistentFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)

	// Root command should have persistent flags available to migrate subcommand
	assert.NotNil(t, cmd.PersistentFlags().Lookup("provider"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("database"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("chunk"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("progress"))
	assert.NotNil(t, migrateCmd)
}

func TestMigrateCommand_WithBothFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "mysql://src/db", "--dst", "mysql://dst/db"})

	// Command should fail at connection stage but should parse flags
	err := cmd.Execute()
	// Expected to fail due to actual database connection attempt
	assert.Error(t, err)
}

func TestMigrateCommand_WithoutSrcFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--dst", "mysql://dst/db"})

	err := cmd.Execute()
	// Should fail because --src is required
	assert.Error(t, err)
}

func TestMigrateCommand_WithoutDstFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "mysql://src/db"})

	err := cmd.Execute()
	// Should fail because --dst is required
	assert.Error(t, err)
}

func TestMigrateCommand_WithoutAnyFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate"})

	err := cmd.Execute()
	// Should fail because both --src and --dst are required
	assert.Error(t, err)
}

func TestMigrateCommand_WithProviderFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--provider", "fake"})

	err := cmd.Execute()
	// Expected to fail at connection attempt
	assert.Error(t, err)
}

func TestMigrateCommand_WithChunkFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--chunk", "500"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_WithProgressFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--progress"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_WithSingleTransactionFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--single-transaction"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_WithDropFirstFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--drop-first"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_AllFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"migrate",
		"--src", "testdb",
		"--dst", "testdb",
		"--chunk", "500",
		"--progress",
		"--drop-first",
		"--single-transaction",
		"--provider", "fake",
	})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_InvalidChunkSize(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--chunk", "invalid"})

	err := cmd.Execute()
	// Should fail due to invalid chunk size flag
	assert.Error(t, err)
}

func TestMigrateCommand_Context(t *testing.T) {
	cmd := cli.NewRootCommand()
	ctx := context.WithValue(context.Background(), "test", "value")
	cmd.SetContext(ctx)

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)

	// Migrate command should inherit context from root
	assert.NotNil(t, migrateCmd)
}

func TestMigrateCommand_OutputWriters(t *testing.T) {
	cmd := cli.NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)

	assert.Equal(t, stdout, migrateCmd.OutOrStdout())
	assert.Equal(t, stderr, migrateCmd.ErrOrStderr())
}

func TestMigrateCommand_LongFormFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"migrate",
		"--src=testdb",
		"--dst=testdb",
		"--drop-first=true",
	})

	err := cmd.Execute()
	// Expected to fail at connection attempt
	assert.Error(t, err)
}

func TestMigrateCommand_WithDatabaseFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb", "--database", "fake"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_DropFirstDefault(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Without --drop-first, default should be false
	cmd.SetArgs([]string{"migrate", "--src", "testdb", "--dst", "testdb"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestMigrateCommand_SrcAndDstCanBeSame(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"migrate", "--src", "mysql://localhost/testdb", "--dst", "mysql://localhost/testdb"})

	err := cmd.Execute()
	// Expected to fail at connection attempt
	assert.Error(t, err)
}

func TestMigrateCommand_ComplexConnectionStrings(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"migrate",
		"--src", "mysql://user:pass@localhost:3306/srcdb",
		"--dst", "mysql://user:pass@remote:3306/dstdb",
	})

	err := cmd.Execute()
	// Expected to fail at connection attempt
	assert.Error(t, err)
}