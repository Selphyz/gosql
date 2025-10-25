# gosql

`gosql` is a Go-based CLI that can dump or migrate SQL databases. The first release targets MySQL-compatible databases, with an extensible provider abstraction ready for future engines.

## Features

- Dump schema and data to SQL files (`gosql dump`)
- Migrate schema and data between two connections (`gosql migrate`)
- Single binary with root auto-mode (`gosql --src ... [--dst ...]`)
- Chunked streaming for large tables
- Optional consistent snapshots via `--single-transaction`
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
gosql migrate \
  --src "user:pass@tcp(src-host:3306)/mydb" \
  --dst "mysql://user:pass@dest-host:3306/targetdb" \
  --drop-first \
  --chunk 1000 \
  --single-transaction \
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

- `--provider`: SQL provider to use (defaults to `mysql`)
- `--chunk`: Rows per batch when streaming data (default `1000`)
- `--single-transaction`: Use a consistent snapshot/transaction
- `--progress`: Print per-table progress information
- `--drop-first`: Drop existing tables on the destination (migrate only)
- The migrator will create the destination database automatically (matching the source charset and collation) when it is missing and credentials permit.

## Testing

```bash
go test ./...
```

Unit tests cover DSN parsing, literal quoting, SQL emission, and connection normalization.
