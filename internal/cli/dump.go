package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gosql/internal/dump"
)

func newDumpCommand(common *commonOptions) *cobra.Command {
	var opts dump.Options

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump schema and data to SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Src == "" {
				return fmt.Errorf("--src is required")
			}
			opts.CommonOptions = dump.CommonOptions{
				Provider:          common.Provider,
				ChunkSize:         common.ChunkSize,
				SingleTransaction: common.SingleTransaction,
				Progress:          common.Progress,
				Stdout:            cmd.OutOrStdout(),
				Stderr:            cmd.ErrOrStderr(),
			}

			if opts.Writer == nil && opts.OutPath == "" {
				opts.Writer = cmd.OutOrStdout()
			}
			return dump.Run(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.Src, "src", "", "Source connection string")
	cmd.Flags().StringVar(&opts.OutPath, "out", "", "Destination file for the SQL dump (defaults to stdout)")

	return cmd
}
