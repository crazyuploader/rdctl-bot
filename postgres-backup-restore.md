# PostgreSQL Backup and Restore

Use native PostgreSQL tools for database backups and restores. This file keeps the commands in one place.

## Tools

You need these commands installed:

- `pg_dump`
- `pg_restore`
- `psql`

## Common variables

Set the values for your environment before running the commands.

```bash
export DB_HOST="db-host"
export DB_PORT="5432"
export DB_USER="db_user"
export DB_NAME="app_db"
export DB_SSLMODE="require"
export PGPASSWORD="database_password"
```

## Create a backup

Create a custom-format backup file:

```bash
PGSSLMODE="$DB_SSLMODE" pg_dump \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  --format=custom \
  --file ./database.dump
```

Create a plain SQL backup file:

```bash
PGSSLMODE="$DB_SSLMODE" pg_dump \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  --format=plain \
  --file ./database.sql
```

## Inspect a backup

List objects inside a custom dump:

```bash
pg_restore -l ./database.dump | head -n 50
```

Check that the backup file exists:

```bash
ls -lh ./database.dump ./database.sql
```

## Restore a custom dump

Restore a custom-format backup with [`pg_restore`](docs/postgres-backup-restore.md:39):

```bash
PGSSLMODE="$DB_SSLMODE" pg_restore \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  --no-owner \
  --clean \
  --if-exists \
  ./database.dump
```

## Restore a plain SQL backup

Restore a plain SQL file with [`psql`](docs/postgres-backup-restore.md:11):

```bash
PGSSLMODE="$DB_SSLMODE" psql \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  -f ./database.sql
```

## Recreate the database before restore

This step deletes the current database.

```bash
PGSSLMODE="$DB_SSLMODE" psql \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d postgres \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();" \
  -c "DROP DATABASE IF EXISTS $DB_NAME;" \
  -c "CREATE DATABASE $DB_NAME;"
```

## Quick validation

List tables after restore:

```bash
PGSSLMODE="$DB_SSLMODE" psql \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  -c "\\dt"
```

Inspect one table:

```bash
PGSSLMODE="$DB_SSLMODE" psql \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  -c "\\d+ your_table_name"
```

## Notes

- Use [`pg_dump`](docs/postgres-backup-restore.md:25) with `--format=custom` if you want to restore with [`pg_restore`](docs/postgres-backup-restore.md:39).
- Use [`pg_dump`](docs/postgres-backup-restore.md:35) with `--format=plain` if you want a readable SQL file for [`psql`](docs/postgres-backup-restore.md:53).
- Pass host, port, user, and database with flags when using [`pg_restore`](docs/postgres-backup-restore.md:39). Do not pass a DSN as the final argument.
- Set `PGSSLMODE` to match your server requirements.
- Use `--no-owner` when the restore role should not take ownership from the source backup.
