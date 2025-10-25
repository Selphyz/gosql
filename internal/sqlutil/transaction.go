package sqlutil

import (
	"context"
	"database/sql"
	"fmt"
)

// BeginReadSnapshot starts a consistent read transaction for supported drivers.
func BeginReadSnapshot(ctx context.Context, driverName string, db *sql.DB) (func(context.Context, bool) error, error) {
	switch driverName {
	case "mysql":
		limitSingleConnection(db)
		if _, err := db.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
			return nil, err
		}
		if _, err := db.ExecContext(ctx, "START TRANSACTION WITH CONSISTENT SNAPSHOT"); err != nil {
			return nil, err
		}
		return finishFunc(db), nil
	default:
		return nil, fmt.Errorf("single-transaction snapshot not supported for driver %q", driverName)
	}
}

// BeginWriteTransaction starts a transactional batch for supported drivers.
func BeginWriteTransaction(ctx context.Context, driverName string, db *sql.DB) (func(context.Context, bool) error, error) {
	switch driverName {
	case "mysql":
		limitSingleConnection(db)
		if _, err := db.ExecContext(ctx, "START TRANSACTION"); err != nil {
			return nil, err
		}
		return finishFunc(db), nil
	default:
		return nil, fmt.Errorf("transactions not supported for driver %q", driverName)
	}
}

func finishFunc(db *sql.DB) func(context.Context, bool) error {
	return func(ctx context.Context, commit bool) error {
		stmt := "ROLLBACK"
		if commit {
			stmt = "COMMIT"
		}
		_, err := db.ExecContext(ctx, stmt)
		return err
	}
}

func limitSingleConnection(db *sql.DB) {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
}
