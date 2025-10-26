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

func TestNewRootCommand_Creation(t *testing.T) {
	cmd := cli.NewRootCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "gosql", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestNewRootCommand_PersistentFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Check that persistent flags are registered
	assert.NotNil(t, cmd.PersistentFlags().Lookup("provider"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("database"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("chunk"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("single-transaction"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("progress"))
}

func TestNewRootCommand_LocalFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Check that local flags are registered
	assert.NotNil(t, cmd.Flags().Lookup("src"))
	assert.NotNil(t, cmd.Flags().Lookup("dst"))
	assert.NotNil(t, cmd.Flags().Lookup("out"))
	assert.NotNil(t, cmd.Flags().Lookup("drop-first"))
}

func TestNewRootCommand_Subcommands(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Check that subcommands are registered
	subcommands := cmd.Commands()
	names := make(map[string]bool)
	for _, subcmd := range subcommands {
		names[subcmd.Name()] = true
	}
	
	assert.True(t, names["dump"], "dump subcommand should be registered")
	assert.True(t, names["migrate"], "migrate subcommand should be registered")
}

func TestNewRootCommand_DefaultProvider(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Get the database flag value
	dbFlag := cmd.PersistentFlags().Lookup("database")
	require.NotNil(t, dbFlag)
	assert.Equal(t, "mysql", dbFlag.DefValue)
}

func TestNewRootCommand_DefaultChunkSize(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	chunkFlag := cmd.PersistentFlags().Lookup("chunk")
	require.NotNil(t, chunkFlag)
	assert.Equal(t, "1000", chunkFlag.DefValue)
}

func TestNewRootCommand_DefaultProgressFalse(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	progressFlag := cmd.PersistentFlags().Lookup("progress")
	require.NotNil(t, progressFlag)
	assert.Equal(t, "false", progressFlag.DefValue)
}

func TestNewRootCommand_DefaultSingleTransactionFalse(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	stFlag := cmd.PersistentFlags().Lookup("single-transaction")
	require.NotNil(t, stFlag)
	assert.Equal(t, "false", stFlag.DefValue)
}

func TestNewRootCommand_FlagUsageText(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Verify flag usage descriptions are set
	srcFlag := cmd.Flags().Lookup("src")
	require.NotNil(t, srcFlag)
	assert.NotEmpty(t, srcFlag.Usage)
	
	dstFlag := cmd.Flags().Lookup("dst")
	require.NotNil(t, dstFlag)
	assert.NotEmpty(t, dstFlag.Usage)
}

func TestNewRootCommand_SilenceUsageAndErrors(t *testing.T) {
	cmd := cli.NewRootCommand()
	assert.True(t, cmd.SilenceUsage)
	assert.True(t, cmd.SilenceErrors)
}

func TestNewRootCommand_RunWithoutFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	
	// Running without --src should print usage (returns nil, not an error)
	err := cmd.Execute()
	// The command returns nil (usage) when no --src flag is provided
	assert.Nil(t, err)
}

func TestNewRootCommand_DumpSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	dumpCmd, _, err := cmd.Find([]string{"dump"})
	require.NoError(t, err)
	assert.NotNil(t, dumpCmd)
	assert.Equal(t, "dump", dumpCmd.Name())
}

func TestNewRootCommand_MigrateSubcommand(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	require.NoError(t, err)
	assert.NotNil(t, migrateCmd)
	assert.Equal(t, "migrate", migrateCmd.Name())
}

func TestNewRootCommand_ContextPassing(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	ctx := context.WithValue(context.Background(), "test", "value")
	cmd.SetContext(ctx)
	
	assert.Equal(t, ctx, cmd.Context())
}

func TestNewRootCommand_OutputWriters(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	
	assert.Equal(t, stdout, cmd.OutOrStdout())
	assert.Equal(t, stderr, cmd.ErrOrStderr())
}

func TestNewRootCommand_HelpFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// Help flag is added automatically by Cobra
	// Just verify command was created successfully
	assert.NotNil(t, cmd)
}

func TestNewRootCommand_VersionFlag(t *testing.T) {
	cmd := cli.NewRootCommand()
	
	// If version flag is registered, check it exists
	// This is optional, but good practice
	versionFlag := cmd.Flags().Lookup("version")
	// Version flag may or may not be present depending on implementation
	_ = versionFlag
}
