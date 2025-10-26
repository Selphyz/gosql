package mysql

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/provider/mysql"
)

func TestListTables_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("users").
		AddRow("products").
		AddRow("orders")

	mock.ExpectQuery("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES").
		WithArgs().
		WillReturnRows(rows)

	prov := &mysql.Provider{}
	tables, err := prov.ListTables(context.Background(), db)
	require.NoError(t, err)
	assert.Equal(t, []string{"users", "products", "orders"}, tables)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListTables_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES").
		WithArgs().
		WillReturnError(errors.New("query error"))

	prov := &mysql.Provider{}
	tables, err := prov.ListTables(context.Background(), db)
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListTables_RowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Create rows with a single table that succeeds
	rows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("users")

	mock.ExpectQuery("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES").
		WithArgs().
		WillReturnRows(rows)

	prov := &mysql.Provider{}
	tables, err := prov.ListTables(context.Background(), db)
	require.NoError(t, err)
	assert.Equal(t, []string{"users"}, tables)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListTables_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME"})

	mock.ExpectQuery("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES").
		WithArgs().
		WillReturnRows(rows)

	prov := &mysql.Provider{}
	tables, err := prov.ListTables(context.Background(), db)
	require.NoError(t, err)
	assert.Nil(t, tables)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShowCreateTable_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ddl := "CREATE TABLE `users` (`id` INT PRIMARY KEY, `name` VARCHAR(100))"
	rows := sqlmock.NewRows([]string{"Table", "Create Table"}).
		AddRow("users", ddl)

	mock.ExpectQuery("SHOW CREATE TABLE `users`").
		WithArgs().
		WillReturnRows(rows)

	prov := &mysql.Provider{}
	result, err := prov.ShowCreateTable(context.Background(), db, "users")
	require.NoError(t, err)
	assert.Equal(t, ddl, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShowCreateTable_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SHOW CREATE TABLE `users`").
		WithArgs().
		WillReturnError(sql.ErrNoRows)

	prov := &mysql.Provider{}
	result, err := prov.ShowCreateTable(context.Background(), db, "users")
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStreamRows_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery("SELECT \\* FROM `users`").
		WithArgs().
		WillReturnRows(rows)

	prov := &mysql.Provider{}
	result, cols, err := prov.StreamRows(context.Background(), db, "users")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []string{"id", "name"}, cols)
	result.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStreamRows_ColumnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// For this test, we expect the query to fail entirely
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WithArgs().
		WillReturnError(errors.New("query columns error"))

	prov := &mysql.Provider{}
	result, cols, err := prov.StreamRows(context.Background(), db, "users")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, cols)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStreamRows_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT \\* FROM `users`").
		WithArgs().
		WillReturnError(errors.New("query error"))

	prov := &mysql.Provider{}
	result, cols, err := prov.StreamRows(context.Background(), db, "users")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, cols)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRows_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO `users` \\(`id`, `name`\\) VALUES").
		WithArgs(1, "Alice", 2, "Bob").
		WillReturnResult(sqlmock.NewResult(0, 2))

	prov := &mysql.Provider{}
	rows := [][]any{
		{1, "Alice"},
		{2, "Bob"},
	}
	err = prov.InsertRows(context.Background(), db, "users", []string{"id", "name"}, rows)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRows_NoColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	prov := &mysql.Provider{}
	rows := [][]any{{1, "Alice"}}
	err = prov.InsertRows(context.Background(), db, "users", []string{}, rows)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no columns supplied")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRows_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	prov := &mysql.Provider{}
	err = prov.InsertRows(context.Background(), db, "users", []string{"id", "name"}, [][]any{})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRows_MismatchedColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	prov := &mysql.Provider{}
	rows := [][]any{
		{1, "Alice", "extra"}, // 3 values for 2 columns
	}
	err = prov.InsertRows(context.Background(), db, "users", []string{"id", "name"}, rows)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "column count mismatch")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertRows_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO `users` \\(`id`, `name`\\) VALUES").
		WithArgs(1, "Alice").
		WillReturnError(errors.New("exec error"))

	prov := &mysql.Provider{}
	rows := [][]any{{1, "Alice"}}
	err = prov.InsertRows(context.Background(), db, "users", []string{"id", "name"}, rows)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDisableConstraints_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET FOREIGN_KEY_CHECKS=0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET FOREIGN_KEY_CHECKS=1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	prov := &mysql.Provider{}
	restore, err := prov.DisableConstraints(context.Background(), db)
	require.NoError(t, err)
	require.NotNil(t, restore)

	err = restore(context.Background())
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDisableConstraints_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET FOREIGN_KEY_CHECKS=0").
		WillReturnError(errors.New("exec error"))

	prov := &mysql.Provider{}
	restore, err := prov.DisableConstraints(context.Background(), db)
	assert.Error(t, err)
	assert.Nil(t, restore)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDisableConstraints_RestoreError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("SET FOREIGN_KEY_CHECKS=0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET FOREIGN_KEY_CHECKS=1").
		WillReturnError(errors.New("restore error"))

	prov := &mysql.Provider{}
	restore, err := prov.DisableConstraints(context.Background(), db)
	require.NoError(t, err)

	err = restore(context.Background())
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}