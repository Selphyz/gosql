package dump

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"gosql/internal/conn"
	"gosql/internal/format"
	"gosql/internal/provider"
	"gosql/internal/sqlutil"
)

// Run executes the dump workflow end-to-end.
func Run(ctx context.Context, opts Options) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Src == "" {
		return fmt.Errorf("source connection is required")
	}

	writer, cleanup, err := resolveWriter(opts)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer func() {
			if cerr := cleanup(); err == nil {
				err = cerr
			}
		}()
	}

	prov, dsn, dbName, err := conn.Normalize(opts.Provider, opts.Src)
	if err != nil {
		return err
	}

	providerName := opts.Provider
	if providerName == "" {
		providerName = prov.DriverName()
	}

	db, err := sql.Open(prov.DriverName(), dsn)
	if err != nil {
		return fmt.Errorf("open source connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping source: %w", err)
	}

	var finishSnapshot func(context.Context, bool) error
	if opts.SingleTransaction {
		finishSnapshot, err = sqlutil.BeginReadSnapshot(ctx, prov.DriverName(), db)
		if err != nil {
			return err
		}
		defer func() {
			if finishSnapshot != nil {
				if cerr := finishSnapshot(ctx, err == nil); err == nil {
					err = cerr
				}
			}
		}()
	}

	serverVersion, _ := fetchServerVersion(ctx, db)

	tables, err := prov.ListTables(ctx, db)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}

	sw := format.NewSQLWriter(writer, prov)
	if err := sw.WriteHeader(format.Header{
		ProviderName:  providerName,
		DatabaseName:  dbName,
		ServerVersion: serverVersion,
		GeneratedAt:   time.Now().UTC(),
	}); err != nil {
		return err
	}

	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}

	for _, table := range tables {
		if opts.Progress && opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "Dumping %s...\n", table)
		}

		ddl, err := prov.ShowCreateTable(ctx, db, table)
		if err != nil {
			return fmt.Errorf("show create table %s: %w", table, err)
		}

		if err := sw.WriteTableDefinition(table, ddl); err != nil {
			return fmt.Errorf("write table definition %s: %w", table, err)
		}

		// Write additional DDL statements if provider supports them (e.g., indexes)
		if extraDDLProv, ok := prov.(provider.AdditionalDDLProvider); ok {
			extraStatements, err := extraDDLProv.TableExtraDDL(ctx, db, table)
			if err != nil {
				return fmt.Errorf("get extra DDL for table %s: %w", table, err)
			}
			if err := sw.WriteStatements(extraStatements); err != nil {
				return fmt.Errorf("write extra DDL for table %s: %w", table, err)
			}
		}

		if err := sw.WriteTableDataPreamble(table); err != nil {
			return err
		}

		rows, columns, err := prov.StreamRows(ctx, db, table)
		if err != nil {
			return fmt.Errorf("stream rows %s: %w", table, err)
		}

		totalRows, err := dumpTableData(ctx, prov, sw, table, rows, columns, opts.ChunkSize)
		rows.Close()
		if err != nil {
			return err
		}

		if opts.Progress && opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "Dumped %s: %d rows\n", table, totalRows)
		}
	}

	return nil
}

func resolveWriter(opts Options) (io.Writer, func() error, error) {
	if opts.Writer != nil {
		return opts.Writer, nil, nil
	}
	if opts.OutPath != "" {
		file, err := os.Create(opts.OutPath)
		if err != nil {
			return nil, nil, err
		}
		return file, file.Close, nil
	}
	if opts.Stdout != nil {
		return opts.Stdout, nil, nil
	}
	return nil, nil, fmt.Errorf("no writer available for dump output")
}

func fetchServerVersion(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func dumpTableData(ctx context.Context, prov provider.SQLProvider, sw *format.SQLWriter, table string, rows *sql.Rows, columns []string, chunkSize int) (int, error) {
	if chunkSize <= 0 {
		chunkSize = 1000
	}

	total := 0
	chunk := make([][]string, 0, chunkSize)

	for {
		hasNext := rows.Next()
		if !hasNext {
			if err := rows.Err(); err != nil {
				return total, err
			}
			if len(chunk) > 0 {
				if err := sw.WriteInsertBatch(table, columns, chunk); err != nil {
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

		rowValues := make([]string, len(columns))
		for i, cell := range dest {
			value := *(cell.(*any))
			if b, ok := value.([]byte); ok {
				copyBytes := make([]byte, len(b))
				copy(copyBytes, b)
				value = copyBytes
			}
			literal, err := prov.QuoteLiteral(value)
			if err != nil {
				return total, err
			}
			rowValues[i] = literal
			*(cell.(*any)) = nil
		}

		chunk = append(chunk, rowValues)
		total++

		if len(chunk) >= chunkSize {
			if err := sw.WriteInsertBatch(table, columns, chunk); err != nil {
				return total, err
			}
			chunk = chunk[:0]
		}
	}

	return total, nil
}
