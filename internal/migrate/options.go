package migrate

import (
	"gosql/internal/options"
)

// CommonOptions aliases the shared CLI/runtime configuration.
type CommonOptions = options.Common

// Options drives the migrator logic.
type Options struct {
	CommonOptions

	Src       string
	Dst       string
	DropFirst bool
}
