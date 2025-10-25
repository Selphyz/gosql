package options

import "io"

// Common captures shared runtime configuration for dump and migrate flows.
type Common struct {
	Provider          string
	ChunkSize         int
	SingleTransaction bool
	Progress          bool
	Stdout            io.Writer
	Stderr            io.Writer
}
