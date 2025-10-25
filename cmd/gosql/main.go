package main

import (
	"context"
	"fmt"
	"os"

	"gosql/internal/cli"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/sijms/go-ora/v2"

	_ "gosql/internal/provider/mysql"
	_ "gosql/internal/provider/oracle"
	_ "gosql/internal/provider/postgres"
	_ "gosql/internal/provider/sqlserver"
)

func main() {
	ctx := context.Background()
	root := cli.NewRootCommand()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
