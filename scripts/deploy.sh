#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# VMOrbit — Production Deployment Script
#
# Usage:
#   ./scripts/deploy.sh [--version v1.2.3] [--skip-build] [--rollback]
#
# Requirements:
#   - Docker + Docker Compose v2
#   - .env.production file present
#   - nginx/ssl/ directory with TLS certs
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.production.yml"
ENV_FILE="${ROOT_DIR}/.env.production"
LOG_FILE="${ROOT_DIR}/logs/deploy.log"

# ── Defaults ──────────────────────────────────────────────────────────────────
APP_VERSION="${APP_VERSION:-$(git -C "${ROOT_DIR}" describe --tags --always 2>/dev/null || echo 'dev')}"
SKIP_BUILD=false
ROLLBACK=false
BACKUP_BEFORE_DEPLOY=true

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)   APP_VERSION="$2"; shift 2 ;;
    --skip-build) SKIP_BUILD=true; shift ;;
    --rollback)  ROLLBACK=true; shift ;;
    --no-backup) BACKUP_BEFORE_DEPLOY=false; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "${LOG_FILE}"; }
die() { log "ERROR: $*"; exit 1; }

mkdir -p "$(dirname "${LOG_FILE}")"

log "═══════════════════════════════════════════════════════"
log "VMOrbit Deployment — version: ${APP_VERSION}"
log "═══════════════════════════════════════════════════════"

# ── Pre-flight checks ─────────────────────────────────────────────────────────
[[ -f "${ENV_FILE}" ]] || die ".env.production not found. Copy .env.production.example and fill in secrets."
[[ -f "${ROOT_DIR}/nginx/ssl/fullchain.pem" ]] || log "WARNING: nginx/ssl/fullchain.pem not found — HTTPS will not work."
command -v docker >/dev/null 2>&1 || die "docker not found"
docker compose version >/dev/null 2>&1 || die "docker compose v2 not found"

# ── Rollback mode ─────────────────────────────────────────────────────────────
if [[ "${ROLLBACK}" == "true" ]]; then
  log "Rolling back to previous version..."
  PREV_VERSION=$(cat "${ROOT_DIR}/.deploy_version_prev" 2>/dev/null || die "No previous version recorded")
  log "Previous version: ${PREV_VERSION}"
  APP_VERSION="${PREV_VERSION}"
  SKIP_BUILD=true
fi

# ── Pre-deploy backup ─────────────────────────────────────────────────────────
if [[ "${BACKUP_BEFORE_DEPLOY}" == "true" ]]; then
  log "Running pre-deploy database backup..."
  "${SCRIPT_DIR}/backup.sh" --label "pre-deploy-${APP_VERSION}" || log "WARNING: Pre-deploy backup failed (continuing)"
fi

# ── Build images ──────────────────────────────────────────────────────────────
if [[ "${SKIP_BUILD}" == "false" ]]; then
  log "Building backend image..."
  docker build \
    --file "${ROOT_DIR}/Dockerfile.backend" \
    --target production \
    --tag "vmorbit/backend:${APP_VERSION}" \
    --tag "vmorbit/backend:latest" \
    --build-arg VERSION="${APP_VERSION}" \
    "${ROOT_DIR}"

  log "Building frontend image..."
  # shellcheck source=/dev/null
  source "${ENV_FILE}"
  docker build \
    --file "${ROOT_DIR}/frontend/Dockerfile.frontend" \
    --target production \
    --tag "vmorbit/frontend:${APP_VERSION}" \
    --tag "vmorbit/frontend:latest" \
    --build-arg NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL:-https://vmorbit.example.com}" \
    "${ROOT_DIR}/frontend"
fi

# ── Save current version for rollback ────────────────────────────────────────
CURRENT_VERSION=$(cat "${ROOT_DIR}/.deploy_version" 2>/dev/null || echo "")
if [[ -n "${CURRENT_VERSION}" ]]; then
  echo "${CURRENT_VERSION}" > "${ROOT_DIR}/.deploy_version_prev"
fi
echo "${APP_VERSION}" > "${ROOT_DIR}/.deploy_version"

# ── Deploy ────────────────────────────────────────────────────────────────────
log "Deploying version ${APP_VERSION}..."

export APP_VERSION
docker compose \
  -f "${COMPOSE_FILE}" \
  --env-file "${ENV_FILE}" \
  up -d \
  --remove-orphans \
  --wait \
  --wait-timeout 120

# ── Health check ─────────────────────────────────────────────────────────────
log "Waiting for services to be healthy..."
MAX_WAIT=60
ELAPSED=0
until docker compose -f "${COMPOSE_FILE}" ps --format json | \
      python3 -c "import sys,json; data=sys.stdin.read(); services=[json.loads(l) for l in data.strip().split('\n') if l]; unhealthy=[s for s in services if s.get('Health','') not in ('healthy','')]; sys.exit(len(unhealthy))" 2>/dev/null; do
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    log "ERROR: Services did not become healthy within ${MAX_WAIT}s"
    docker compose -f "${COMPOSE_FILE}" ps
    exit 1
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  log "  Waiting... (${ELAPSED}s)"
done

# Simple HTTP health check
BACKEND_HEALTH=$(curl -sf http://localhost/health 2>/dev/null || echo "FAILED")
if [[ "${BACKEND_HEALTH}" != *"ok"* ]]; then
  log "WARNING: Backend health check returned: ${BACKEND_HEALTH}"
fi

log "Deployment complete — version ${APP_VERSION} is live."
log "═══════════════════════════════════════════════════════"
