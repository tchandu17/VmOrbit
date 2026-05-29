#!/usr/bin/env sh
# ─────────────────────────────────────────────────────────────────────────────
# VMOrbit — Backup Container Entrypoint
# Runs inside the backup Docker service on a cron schedule.
# ─────────────────────────────────────────────────────────────────────────────
set -e

BACKUP_DIR="/backups"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] [backup] $*"; }

mkdir -p "${BACKUP_DIR}/wal"

run_backup() {
  TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
  BACKUP_FILE="${BACKUP_DIR}/vmorbit_${TIMESTAMP}.sql.gz"

  log "Starting backup → ${BACKUP_FILE}"
  PGPASSWORD="${PGPASSWORD}" pg_dump \
    -h "${DB_HOST}" \
    -U "${DB_USER}" \
    -d "${DB_NAME}" \
    --format=plain \
    --no-password \
    | gzip -9 > "${BACKUP_FILE}"

  if gunzip -t "${BACKUP_FILE}" 2>/dev/null; then
    SIZE=$(du -sh "${BACKUP_FILE}" | cut -f1)
    log "Backup complete: ${BACKUP_FILE} (${SIZE})"
  else
    log "ERROR: Backup integrity check failed!"
    rm -f "${BACKUP_FILE}"
    return 1
  fi

  # Rotate old backups
  find "${BACKUP_DIR}" -name "vmorbit_*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete
  REMAINING=$(find "${BACKUP_DIR}" -name "vmorbit_*.sql.gz" | wc -l)
  log "Rotation complete. ${REMAINING} backup(s) retained."
}

# Wait for postgres to be ready
log "Waiting for PostgreSQL..."
until PGPASSWORD="${PGPASSWORD}" pg_isready -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" -q; do
  sleep 5
done
log "PostgreSQL is ready."

# Run initial backup on startup
run_backup

# Schedule daily backups at 02:00 UTC using a simple sleep loop
log "Backup scheduler started (daily at 02:00 UTC)"
while true; do
  # Calculate seconds until next 02:00 UTC
  NOW=$(date -u +%s)
  NEXT_RUN=$(date -u -d 'tomorrow 02:00' +%s 2>/dev/null || \
             date -u -j -f "%Y-%m-%d %H:%M:%S" "$(date -u '+%Y-%m-%d') 02:00:00" +%s 2>/dev/null || \
             echo $((NOW + 86400)))
  SLEEP_SECS=$((NEXT_RUN - NOW))
  [ "${SLEEP_SECS}" -le 0 ] && SLEEP_SECS=86400
  log "Next backup in ${SLEEP_SECS}s ($(date -u -d "@${NEXT_RUN}" '+%Y-%m-%d %H:%M:%S UTC' 2>/dev/null || echo 'tomorrow 02:00 UTC'))"
  sleep "${SLEEP_SECS}"
  run_backup
done
