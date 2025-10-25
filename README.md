# gosql

`gosql` is a Go-based CLI that can dump or migrate SQL databases. It supports MySQL, Oracle, and SQL Server databases with an extensible provider abstraction for future engines.

## Features

- Dump schema and data to SQL files (`gosql dump`)
- Migrate schema and data between two connections (`gosql migrate`)
- Single binary with root auto-mode (`gosql --src ... [--dst ...]`)
- Chunked streaming for large tables
- Optional consistent snapshots via `--single-transaction`
- Multi-database support: MySQL, Oracle, and SQL Server
- Provider registry for adding new SQL engines

## Installation

```bash
go build -o gosql ./cmd/gosql
```

## Usage

### Dump a database

```bash
gosql dump \
  --src "mysql://user:pass@localhost:3306/mydb" \
  --out mydb.sql \
  --chunk 2000 \
  --single-transaction \
  --progress
```

```bash
gosql dump \
  --src "mysql://root:abc@deveco.it:3155/enfocadevnos" \
  --out mydb.sql \
  --chunk 2000 \
  --single-transaction \
  --progress
```

When `--out` is omitted the dump is printed to stdout.

### Migrate between servers

```bash
# MySQL to MySQL
gosql migrate \
  --src "user:pass@tcp(src-host:3306)/mydb" \
  --dst "mysql://user:pass@dest-host:3306/targetdb" \
  --drop-first \
  --chunk 1000 \
  --single-transaction \
  --progress

# Oracle dump
gosql dump \
  --database oracle \
  --src "oracle://user:pass@localhost:1521/ORCL" \
  --out oracle_dump.sql

# SQL Server migration
gosql migrate \
  --database sqlserver \
  --src "sqlserver://user:pass@src-host:1433?database=sourcedb" \
  --dst "sqlserver://user:pass@dest-host:1433?database=targetdb" \
  --progress
```

By default, tables are created without dropping existing ones; use `--drop-first` to replace them.

### Root auto-mode

```bash
# Dump (no destination)
gosql --src "mysql://user:pass@localhost:3306/mydb" --out dump.sql

# Migrate when destination is provided
gosql --src "mysql://user:pass@src:3306/mydb" --dst "mysql://user:pass@dst:3306/mydb"
```

## Flags

- `--database`: Database provider to use: `mysql`, `oracle`, or `sqlserver` (default: `mysql`)
- `--provider`: Legacy alias for `--database` (deprecated)
- `--chunk`: Rows per batch when streaming data (default `1000`)
- `--single-transaction`: Use a consistent snapshot/transaction
- `--progress`: Print per-table progress information
- `--drop-first`: Drop existing tables on the destination (migrate only)
- The migrator will create the destination database automatically (matching the source charset/collation for MySQL or collation for SQL Server) when it is missing and credentials permit.

## Supported Connection Strings

### MySQL

```bash
# URL format
mysql://user:password@host:port/database

# DSN format
user:password@tcp(host:port)/database
```

### Oracle

```bash
# URL format
oracle://user:password@host:port/service

# EZConnect format
user/password@host:port/service
```

### SQL Server

```bash
# URL format
sqlserver://user:password@host:port?database=dbname

# ADO format
server=host;database=dbname;user id=user;password=pass
```

**Note:** When using non-URL connection strings (ADO/EZConnect), you may need to explicitly specify the `--database` flag if auto-detection fails.

## Testing

```bash
go test ./...
```

Unit tests cover DSN parsing, literal quoting, SQL emission, and connection normalization.
