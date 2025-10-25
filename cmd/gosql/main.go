package main

import (
	"context"
	"fmt"
	"os"

	"gosql/internal/cli"

	_ "gosql/internal/provider/mysql"
)

func main() {
	ctx := context.Background()
	root := cli.NewRootCommand()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
