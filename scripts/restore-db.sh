#!/usr/bin/env bash
set -euo pipefail

log()  { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
err()  { printf '[%s] ERROR %s\n' "$(date '+%H:%M:%S')" "$*" >&2; exit 1; }

command -v cockroach >/dev/null || err "cockroach CLI not found"
[ -f .env ] || err ".env not found"

BACKUP_PATH="${1:-}"
[ -n "$BACKUP_PATH" ] || err "usage: $0 <backup_directory>"
[ -d "$BACKUP_PATH" ] || err "backup directory not found: ${BACKUP_PATH}"

DATABASE_URL=$(grep -oP '^DATABASE_URL\s*=\s*"?\K[^"]*' .env || true)
[ -n "$DATABASE_URL" ] || err "DATABASE_URL not found in .env"

read -p "This will INSERT data from ${BACKUP_PATH} into the live database. Continue? (yes/no) " confirm
[ "$confirm" = "yes" ] || { log "Aborted"; exit 0; }

TABLES=(transactions tasks case_memory audit_log)

for table in "${TABLES[@]}"; do
  csv_file="${BACKUP_PATH}/${table}.csv"
  [ -f "$csv_file" ] || { log "  ${table}: no backup file found, skipping"; continue; }

  row_count=$(($(wc -l < "$csv_file") - 1))
  log "  ${table}: found ${row_count} rows in backup"
done

log "Restore verification complete. For a full restore, use CockroachDB Cloud's"
log "point-in-time recovery from the automated backup schedule instead of this"
log "script - this script is for inspecting/validating manual CSV backups."
