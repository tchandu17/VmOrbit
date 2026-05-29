#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# VMOrbit — PostgreSQL Backup Script
#
# Usage:
#   ./scripts/backup.sh [--label my-label] [--restore <backup-file>]
#
# Backups are stored in ./backups/ with timestamped filenames.
# Retention: configurable via BACKUP_RETENTION_DAYS (default 7).
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env.production"
BACKUP_DIR="${ROOT_DIR}/backups"
LOG_FILE="${ROOT_DIR}/logs/backup.log"

# ── Defaults ──────────────────────────────────────────────────────────────────
LABEL=""
RESTORE_FILE=""
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --label)   LABEL="$2"; shift 2 ;;
    --restore) RESTORE_FILE="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "${LOG_FILE}"; }
die() { log "ERROR: $*"; exit 1; }

mkdir -p "${BACKUP_DIR}" "$(dirname "${LOG_FILE}")"

# ── Load env ──────────────────────────────────────────────────────────────────
if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck source=/dev/null
  set -a; source "${ENV_FILE}"; set +a
fi

DB_USER="${DB_USER:-vmorbit}"
DB_NAME="${DB_NAME:-vmorbit}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
PGPASSWORD="${DB_PASSWORD:-}"
export PGPASSWORD

# ── Restore mode ─────────────────────────────────────────────────────────────
if [[ -n "${RESTORE_FILE}" ]]; then
  [[ -f "${RESTORE_FILE}" ]] || die "Restore file not found: ${RESTORE_FILE}"
  log "═══════════════════════════════════════════════════════"
  log "RESTORE: ${RESTORE_FILE}"
  log "WARNING: This will DROP and recreate the database!"
  log "═══════════════════════════════════════════════════════"
  read -r -p "Type 'yes' to confirm restore: " CONFIRM
  [[ "${CONFIRM}" == "yes" ]] || die "Restore cancelled."

  log "Dropping existing database..."
  psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d postgres \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${DB_NAME}' AND pid <> pg_backend_pid();" \
    -c "DROP DATABASE IF EXISTS ${DB_NAME};" \
    -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"

  log "Restoring from ${RESTORE_FILE}..."
  if [[ "${RESTORE_FILE}" == *.gz ]]; then
    gunzip -c "${RESTORE_FILE}" | psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}"
  else
    psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" < "${RESTORE_FILE}"
  fi

  log "Restore complete."
  exit 0
fi

# ── Backup ────────────────────────────────────────────────────────────────────
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LABEL_SUFFIX="${LABEL:+-${LABEL}}"
BACKUP_FILE="${BACKUP_DIR}/vmorbit_${TIMESTAMP}${LABEL_SUFFIX}.sql.gz"

log "═══════════════════════════════════════════════════════"
log "Backup starting: ${BACKUP_FILE}"

# Check if postgres container is running (Docker mode)
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "vmorbit_postgres"; then
  log "Using Docker container for backup..."
  docker exec vmorbit_postgres \
    pg_dump -U "${DB_USER}" -d "${DB_NAME}" --format=plain --no-password \
    | gzip -9 > "${BACKUP_FILE}"
else
  # Direct connection
  pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
    --format=plain --no-password \
    | gzip -9 > "${BACKUP_FILE}"
fi

BACKUP_SIZE=$(du -sh "${BACKUP_FILE}" | cut -f1)
log "Backup complete: ${BACKUP_FILE} (${BACKUP_SIZE})"

# ── Verify backup ─────────────────────────────────────────────────────────────
if gunzip -t "${BACKUP_FILE}" 2>/dev/null; then
  log "Backup integrity check: PASSED"
else
  die "Backup integrity check FAILED — backup may be corrupt!"
fi

# ── Rotate old backups ────────────────────────────────────────────────────────
log "Rotating backups older than ${RETENTION_DAYS} days..."
find "${BACKUP_DIR}" -name "vmorbit_*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete
REMAINING=$(find "${BACKUP_DIR}" -name "vmorbit_*.sql.gz" | wc -l)
log "Backup rotation complete. ${REMAINING} backup(s) retained."
log "═══════════════════════════════════════════════════════"
