package cli

import (
	"github.com/spf13/cobra"

	"gosql/internal/dump"
	"gosql/internal/migrate"
)

type commonOptions struct {
	Provider          string
	ChunkSize         int
	SingleTransaction bool
	Progress          bool
}

type rootOptions struct {
	Src       string
	Dst       string
	OutPath   string
	DropFirst bool
}

// NewRootCommand wires the CLI hierarchy and returns the prepared root command.
func NewRootCommand() *cobra.Command {
	common := &commonOptions{
		Provider:  "mysql",
		ChunkSize: 1000,
	}

	rootOpts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "gosql",
		Short:         "gosql dumps and migrates SQL databases",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootOpts.Src == "" {
				return cmd.Usage()
			}

			commonPayload := dump.CommonOptions{
				Provider:          common.Provider,
				ChunkSize:         common.ChunkSize,
				SingleTransaction: common.SingleTransaction,
				Progress:          common.Progress,
				Stdout:            cmd.OutOrStdout(),
				Stderr:            cmd.ErrOrStderr(),
			}

			if rootOpts.Dst != "" {
				return migrate.Run(cmd.Context(), migrate.Options{
					CommonOptions: migrate.CommonOptions(commonPayload),
					Src:           rootOpts.Src,
					Dst:           rootOpts.Dst,
					DropFirst:     rootOpts.DropFirst,
				})
			}

			return dump.Run(cmd.Context(), dump.Options{
				CommonOptions: commonPayload,
				Src:           rootOpts.Src,
				OutPath:       rootOpts.OutPath,
			})
		},
	}

	cmd.PersistentFlags().StringVar(&common.Provider, "provider", common.Provider, "SQL provider to use (default: mysql)")
	cmd.PersistentFlags().IntVar(&common.ChunkSize, "chunk", common.ChunkSize, "Number of rows per batch when streaming data")
	cmd.PersistentFlags().BoolVar(&common.SingleTransaction, "single-transaction", false, "Attempt to use a single consistent transaction snapshot")
	cmd.PersistentFlags().BoolVar(&common.Progress, "progress", false, "Print progress information")

	cmd.Flags().StringVar(&rootOpts.Src, "src", "", "Source connection string")
	cmd.Flags().StringVar(&rootOpts.Dst, "dst", "", "Destination connection string; omit to perform a dump")
	cmd.Flags().StringVar(&rootOpts.OutPath, "out", "", "Path to write SQL dump output (defaults to stdout)")
	cmd.Flags().BoolVar(&rootOpts.DropFirst, "drop-first", false, "Drop existing tables on destination before migrating")

	cmd.AddCommand(newDumpCommand(common))
	cmd.AddCommand(newMigrateCommand(common))

	return cmd
}
