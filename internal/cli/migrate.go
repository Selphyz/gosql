package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gosql/internal/migrate"
)

func newMigrateCommand(common *commonOptions) *cobra.Command {
	var opts migrate.Options

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate schema and data between connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Src == "" {
				return fmt.Errorf("--src is required")
			}
			if opts.Dst == "" {
				return fmt.Errorf("--dst is required")
			}
			opts.CommonOptions = migrate.CommonOptions{
				Provider:          common.Provider,
				ChunkSize:         common.ChunkSize,
				SingleTransaction: common.SingleTransaction,
				Progress:          common.Progress,
				Stdout:            cmd.OutOrStdout(),
				Stderr:            cmd.ErrOrStderr(),
			}
			return migrate.Run(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.Src, "src", "", "Source connection string")
	cmd.Flags().StringVar(&opts.Dst, "dst", "", "Destination connection string")
	cmd.Flags().BoolVar(&opts.DropFirst, "drop-first", false, "Drop existing tables on destination before migrating")

	return cmd
}
