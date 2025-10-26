package sqlutil

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/sqlutil"
)

func TestBeginReadSnapshot_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("START TRANSACTION WITH CONSISTENT SNAPSHOT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "mysql", db)
	require.NoError(t, err)
	require.NotNil(t, finish)

	err = finish(context.Background(), true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginReadSnapshot_Postgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "pgx", db)
	require.NoError(t, err)
	require.NotNil(t, finish)

	err = finish(context.Background(), true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginReadSnapshot_UnsupportedDriver(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "unsupported", db)
	assert.Error(t, err)
	assert.Nil(t, finish)
	assert.Contains(t, err.Error(), "not supported")
}

func TestBeginReadSnapshot_SetIsolationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ").
		WillReturnError(errors.New("isolation error"))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "mysql", db)
	assert.Error(t, err)
	assert.Nil(t, finish)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginReadSnapshot_StartTransactionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("START TRANSACTION WITH CONSISTENT SNAPSHOT").
		WillReturnError(errors.New("transaction error"))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "mysql", db)
	assert.Error(t, err)
	assert.Nil(t, finish)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginReadSnapshot_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("START TRANSACTION WITH CONSISTENT SNAPSHOT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "mysql", db)
	require.NoError(t, err)

	err = finish(context.Background(), true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginReadSnapshot_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("START TRANSACTION WITH CONSISTENT SNAPSHOT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginReadSnapshot(context.Background(), "mysql", db)
	require.NoError(t, err)

	err = finish(context.Background(), false)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginWriteTransaction_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("START TRANSACTION").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "mysql", db)
	require.NoError(t, err)
	require.NotNil(t, finish)

	err = finish(context.Background(), true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginWriteTransaction_Postgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("BEGIN").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "pgx", db)
	require.NoError(t, err)
	require.NotNil(t, finish)

	err = finish(context.Background(), true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginWriteTransaction_UnsupportedDriver(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "unsupported", db)
	assert.Error(t, err)
	assert.Nil(t, finish)
	assert.Contains(t, err.Error(), "not supported")
}

func TestBeginWriteTransaction_StartError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("START TRANSACTION").
		WillReturnError(errors.New("transaction error"))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "mysql", db)
	assert.Error(t, err)
	assert.Nil(t, finish)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBeginWriteTransaction_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("START TRANSACTION").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK").
		WillReturnResult(sqlmock.NewResult(0, 0))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "mysql", db)
	require.NoError(t, err)

	err = finish(context.Background(), false)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFinishFunc_CommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("START TRANSACTION").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("COMMIT").
		WillReturnError(errors.New("commit error"))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "mysql", db)
	require.NoError(t, err)

	err = finish(context.Background(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "commit error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFinishFunc_RollbackError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("START TRANSACTION").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK").
		WillReturnError(errors.New("rollback error"))

	finish, err := sqlutil.BeginWriteTransaction(context.Background(), "mysql", db)
	require.NoError(t, err)

	err = finish(context.Background(), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rollback error")
	assert.NoError(t, mock.ExpectationsWereMet())
}