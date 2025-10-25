package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"gosql/internal/conn"
	"gosql/internal/provider"
	"gosql/internal/sqlutil"
)

// Run orchestrates schema and data migration from source to destination.
func Run(ctx context.Context, opts Options) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Src == "" {
		return fmt.Errorf("source connection is required")
	}
	if opts.Dst == "" {
		return fmt.Errorf("destination connection is required")
	}

	srcProv, srcDSN, srcDBName, err := conn.Normalize(opts.Provider, opts.Src)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}
	dstProv, dstDSN, dstDBName, err := conn.Normalize(opts.Provider, opts.Dst)
	if err != nil {
		return fmt.Errorf("parse destination: %w", err)
	}

	if srcProv.DriverName() != dstProv.DriverName() {
		return fmt.Errorf("provider mismatch between source and destination")
	}

	srcDB, err := sql.Open(srcProv.DriverName(), srcDSN)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcDB.Close()

	if err := srcDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping source: %w", err)
	}

	meta, err := srcProv.DatabaseMetadata(ctx, srcDB, srcDBName)
	if err != nil {
		return fmt.Errorf("fetch source database metadata: %w", err)
	}
	if err := dstProv.EnsureDatabase(ctx, dstDSN, dstDBName, meta); err != nil {
		return fmt.Errorf("ensure destination database: %w", err)
	}

	dstDB, err := sql.Open(dstProv.DriverName(), dstDSN)
	if err != nil {
		return fmt.Errorf("open destination: %w", err)
	}
	defer dstDB.Close()

	if err := dstDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping destination: %w", err)
	}

	var finishSrc func(context.Context, bool) error
	var finishDst func(context.Context, bool) error

	if opts.SingleTransaction {
		finishSrc, err = sqlutil.BeginReadSnapshot(ctx, srcProv.DriverName(), srcDB)
		if err != nil {
			return err
		}
		finishDst, err = sqlutil.BeginWriteTransaction(ctx, dstProv.DriverName(), dstDB)
		if err != nil {
			return err
		}
		defer func() {
			if finishDst != nil {
				if cerr := finishDst(ctx, err == nil); err == nil {
					err = cerr
				}
			}
		}()
		defer func() {
			if finishSrc != nil {
				if cerr := finishSrc(ctx, err == nil); err == nil {
					err = cerr
				}
			}
		}()
	}

	restoreConstraints, err := dstProv.DisableConstraints(ctx, dstDB)
	if err != nil {
		return fmt.Errorf("disable constraints: %w", err)
	}
	if restoreConstraints != nil {
		defer func() {
			if cerr := restoreConstraints(ctx); err == nil {
				err = cerr
			}
		}()
	}

	if opts.Progress && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "Migrating database %s -> %s\n", srcDBName, dstDBName)
	}

	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}

	tables, err := srcProv.ListTables(ctx, srcDB)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}

	for _, table := range tables {
		if opts.Progress && opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "Migrating %s...\n", table)
		}

		if opts.DropFirst {
			dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", dstProv.QuoteIdent(table))
			if _, err := dstDB.ExecContext(ctx, dropStmt); err != nil {
				return fmt.Errorf("drop table %s: %w", table, err)
			}
		}

		ddl, err := srcProv.ShowCreateTable(ctx, srcDB, table)
		if err != nil {
			return fmt.Errorf("show create table %s: %w", table, err)
		}
		if _, err := dstDB.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create table %s: %w", table, err)
		}

		rows, columns, err := srcProv.StreamRows(ctx, srcDB, table)
		if err != nil {
			return fmt.Errorf("stream rows %s: %w", table, err)
		}

		totalRows, err := migrateTableData(ctx, srcProv, dstProv, dstDB, table, rows, columns, opts.ChunkSize)
		rows.Close()
		if err != nil {
			return err
		}

		if opts.Progress && opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "Migrated %s: %d rows\n", table, totalRows)
		}
	}

	return nil
}

func migrateTableData(ctx context.Context, srcProv provider.SQLProvider, dstProv provider.SQLProvider, dstDB *sql.DB, table string, rows *sql.Rows, columns []string, chunkSize int) (int, error) {
	if chunkSize <= 0 {
		chunkSize = 1000
	}

	total := 0
	chunk := make([][]any, 0, chunkSize)

	for {
		hasNext := rows.Next()
		if !hasNext {
			if err := rows.Err(); err != nil {
				return total, err
			}
			if len(chunk) > 0 {
				if err := dstProv.InsertRows(ctx, dstDB, table, columns, chunk); err != nil {
					return total, err
				}
			}
			break
		}

		dest := make([]any, len(columns))
		for i := range dest {
			dest[i] = new(any)
		}
		if err := rows.Scan(dest...); err != nil {
			return total, err
		}

		rowValues := make([]any, len(columns))
		for i, cell := range dest {
			value := *(cell.(*any))
			if b, ok := value.([]byte); ok {
				copyBytes := make([]byte, len(b))
				copy(copyBytes, b)
				value = copyBytes
			}
			rowValues[i] = value
			*(cell.(*any)) = nil
		}

		chunk = append(chunk, rowValues)
		total++

		if len(chunk) >= chunkSize {
			if err := dstProv.InsertRows(ctx, dstDB, table, columns, chunk); err != nil {
				return total, err
			}
			chunk = chunk[:0]
		}
	}

	return total, nil
}
