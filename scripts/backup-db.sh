#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="backups"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

log()  { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
err()  { printf '[%s] ERROR %s\n' "$(date '+%H:%M:%S')" "$*" >&2; exit 1; }

command -v cockroach >/dev/null || err "cockroach CLI not found"
[ -f .env ] || err ".env not found"

DATABASE_URL=$(grep -oP '^DATABASE_URL\s*=\s*"?\K[^"]*' .env || true)
[ -n "$DATABASE_URL" ] || err "DATABASE_URL not found in .env"

mkdir -p "$BACKUP_DIR"

TABLES=(transactions tasks case_memory audit_log)

log "Starting backup to ${BACKUP_DIR}/${TIMESTAMP}/"
mkdir -p "${BACKUP_DIR}/${TIMESTAMP}"

for table in "${TABLES[@]}"; do
  log "  Dumping ${table}"
  cockroach sql --url "$DATABASE_URL" \
    --execute "SELECT * FROM ${table};" \
    --format csv \
    > "${BACKUP_DIR}/${TIMESTAMP}/${table}.csv" \
    || err "Failed to dump ${table}"
done

log "  Dumping schema"
cockroach sql --url "$DATABASE_URL" \
  --execute "SHOW CREATE TABLE transactions; SHOW CREATE TABLE tasks; SHOW CREATE TABLE case_memory; SHOW CREATE TABLE audit_log;" \
  > "${BACKUP_DIR}/${TIMESTAMP}/schema.sql" \
  || err "Failed to dump schema"

TOTAL_SIZE=$(du -sh "${BACKUP_DIR}/${TIMESTAMP}" | cut -f1)
log "Backup complete: ${BACKUP_DIR}/${TIMESTAMP}/ (${TOTAL_SIZE})"

echo ""
echo "To restore, see: scripts/restore-db.sh ${BACKUP_DIR}/${TIMESTAMP}"
