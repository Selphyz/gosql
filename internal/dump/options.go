package dump

import (
	"io"

	"gosql/internal/options"
)

// CommonOptions is shared runtime configuration for dump commands.
type CommonOptions = options.Common

// Options groups the inputs necessary to perform a dump.
type Options struct {
	CommonOptions

	Src     string
	OutPath string

	Writer io.Writer
}
