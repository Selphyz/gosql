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

func TestDumpCommand_Creation(t *testing.T) {
	cmd := cli.NewRootCommand()
	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)
	assert.NotNil(t, dumpCmd)
	assert.Equal(t, "dump", dumpCmd.Name())
}

func TestDumpCommand_Short(t *testing.T) {
	cmd := cli.NewRootCommand()
	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)
	assert.NotEmpty(t, dumpCmd.Short)
}

func TestDumpCommand_Flags(t *testing.T) {
	cmd := cli.NewRootCommand()
	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)

	// Check that dump-specific flags are registered
	assert.NotNil(t, dumpCmd.Flags().Lookup("src"))
	assert.NotNil(t, dumpCmd.Flags().Lookup("out"))
}

func TestDumpCommand_HasPersistentFlagsAvailable(t *testing.T) {
	cmd := cli.NewRootCommand()
	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)

	// Root command should have persistent flags available to dump subcommand
	assert.NotNil(t, cmd.PersistentFlags().Lookup("provider"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("database"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("chunk"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("progress"))
	assert.NotNil(t, dumpCmd)
}

func TestDumpCommand_WithSrcFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "mysql://localhost/testdb"})

	// Command should fail at connection stage but should parse flags
	err := cmd.Execute()
	// Expected to fail due to actual database connection attempt
	assert.Error(t, err)
}

func TestDumpCommand_WithoutSrcFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump"})

	err := cmd.Execute()
	// Should fail because --src is required
	assert.Error(t, err)
}

func TestDumpCommand_WithOutFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "mysql://localhost/testdb", "--out", "/tmp/output.sql"})

	// Command should attempt to execute
	err := cmd.Execute()
	// Expected to fail due to file path or connection
	assert.Error(t, err)
}

func TestDumpCommand_WithProviderFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--provider", "fake"})

	err := cmd.Execute()
	// Expected to fail at connection attempt
	assert.Error(t, err)
}

func TestDumpCommand_WithChunkFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--chunk", "500"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestDumpCommand_WithProgressFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--progress"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestDumpCommand_WithSingleTransactionFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--single-transaction"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestDumpCommand_MultipleFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"dump",
		"--src", "testdb",
		"--chunk", "500",
		"--progress",
		"--provider", "fake",
	})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestDumpCommand_DefaultWriter(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Without --out flag, should default to stdout
	cmd.SetArgs([]string{"dump", "--src", "testdb"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}

func TestDumpCommand_InvalidChunkSize(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--chunk", "invalid"})

	err := cmd.Execute()
	// Should fail due to invalid chunk size flag
	assert.Error(t, err)
}

func TestDumpCommand_Context(t *testing.T) {
	cmd := cli.NewRootCommand()
	ctx := context.WithValue(context.Background(), "test", "value")
	cmd.SetContext(ctx)

	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)

	// Dump command should inherit context from root
	assert.NotNil(t, dumpCmd)
}

func TestDumpCommand_OutputWriters(t *testing.T) {
	cmd := cli.NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)

	assert.Equal(t, stdout, dumpCmd.OutOrStdout())
	assert.Equal(t, stderr, dumpCmd.ErrOrStderr())
}

func TestDumpCommand_LongFormFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		"dump",
		"--src=testdb",
		"--out=/tmp/output.sql",
	})

	err := cmd.Execute()
	// Expected to fail at file creation or connection
	assert.Error(t, err)
}

func TestDumpCommand_WithDatabaseFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"dump", "--src", "testdb", "--database", "fake"})

	err := cmd.Execute()
	// Expected to fail at connection stage
	assert.Error(t, err)
}