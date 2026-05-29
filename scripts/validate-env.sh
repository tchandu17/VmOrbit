#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# VMOrbit — Environment Validation Script
#
# Validates that all required configuration, secrets, and dependencies are
# properly set before starting the production stack.
#
# Usage:
#   ./scripts/validate-env.sh [--env-file .env.production]
#
# Exit codes:
#   0 — All checks passed
#   1 — Critical failure (deployment will not work)
#   2 — Warnings present (deployment may work but is not recommended)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env.production"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) ENV_FILE="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

pass() { echo -e "  ${GREEN}✓${NC} $1"; }
fail() { echo -e "  ${RED}✗${NC} $1"; ERRORS=$((ERRORS + 1)); }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; WARNINGS=$((WARNINGS + 1)); }

echo "═══════════════════════════════════════════════════════"
echo " VMOrbit — Production Environment Validation"
echo "═══════════════════════════════════════════════════════"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 1. Prerequisites
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ Prerequisites"

if command -v docker >/dev/null 2>&1; then
  DOCKER_VERSION=$(docker --version | grep -oP '\d+\.\d+\.\d+' | head -1)
  pass "Docker installed (${DOCKER_VERSION})"
else
  fail "Docker not installed"
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE_VERSION=$(docker compose version --short 2>/dev/null || echo "unknown")
  pass "Docker Compose v2 installed (${COMPOSE_VERSION})"
else
  fail "Docker Compose v2 not installed"
fi

if command -v git >/dev/null 2>&1; then
  pass "Git installed"
else
  warn "Git not installed (needed for updates)"
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 2. Configuration Files
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ Configuration Files"

if [[ -f "${ENV_FILE}" ]]; then
  pass ".env.production exists"
else
  fail ".env.production not found (copy from .env.production.example)"
fi

if [[ -f "${ROOT_DIR}/docker-compose.production.yml" ]]; then
  pass "docker-compose.production.yml exists"
else
  fail "docker-compose.production.yml not found"
fi

if [[ -f "${ROOT_DIR}/nginx/nginx.conf" ]]; then
  pass "nginx/nginx.conf exists"
else
  fail "nginx/nginx.conf not found"
fi

if [[ -f "${ROOT_DIR}/configs/config.production.yaml" ]]; then
  pass "configs/config.production.yaml exists"
else
  warn "configs/config.production.yaml not found (using defaults)"
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 3. TLS Certificates
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ TLS Certificates"

if [[ -f "${ROOT_DIR}/nginx/ssl/fullchain.pem" ]]; then
  pass "SSL certificate found (fullchain.pem)"
  # Check expiry
  if command -v openssl >/dev/null 2>&1; then
    EXPIRY=$(openssl x509 -enddate -noout -in "${ROOT_DIR}/nginx/ssl/fullchain.pem" 2>/dev/null | cut -d= -f2)
    EXPIRY_EPOCH=$(date -d "${EXPIRY}" +%s 2>/dev/null || echo 0)
    NOW_EPOCH=$(date +%s)
    DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
    if [[ ${DAYS_LEFT} -lt 0 ]]; then
      fail "SSL certificate EXPIRED (${EXPIRY})"
    elif [[ ${DAYS_LEFT} -lt 30 ]]; then
      warn "SSL certificate expires in ${DAYS_LEFT} days (${EXPIRY})"
    else
      pass "SSL certificate valid for ${DAYS_LEFT} days"
    fi
  fi
else
  fail "SSL certificate not found (nginx/ssl/fullchain.pem)"
fi

if [[ -f "${ROOT_DIR}/nginx/ssl/privkey.pem" ]]; then
  pass "SSL private key found (privkey.pem)"
else
  fail "SSL private key not found (nginx/ssl/privkey.pem)"
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 4. Environment Variables (Secrets)
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ Secrets & Environment Variables"

if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck source=/dev/null
  set -a; source "${ENV_FILE}" 2>/dev/null; set +a

  # Required secrets
  check_secret() {
    local var_name="$1"
    local var_value="${!var_name:-}"
    local placeholder="$2"

    if [[ -z "${var_value}" ]]; then
      fail "${var_name} is not set"
    elif [[ "${var_value}" == *"CHANGE_ME"* ]] || [[ "${var_value}" == *"${placeholder}"* ]]; then
      fail "${var_name} still has placeholder value"
    else
      pass "${var_name} is configured"
    fi
  }

  check_secret "DB_PASSWORD" "CHANGE_ME"
  check_secret "REDIS_PASSWORD" "CHANGE_ME"
  check_secret "VMORBIT_JWT_SECRET" "CHANGE_ME"
  check_secret "VMORBIT_ENCRYPTION_KEY" "CHANGE_ME"
  check_secret "NEXT_PUBLIC_API_URL" "example.com"

  # Check JWT secret length (should be at least 32 bytes)
  JWT_LEN=${#VMORBIT_JWT_SECRET}
  if [[ ${JWT_LEN} -lt 32 ]]; then
    warn "VMORBIT_JWT_SECRET is short (${JWT_LEN} chars) — recommend at least 64 chars"
  fi

  # Check encryption key format (should be 64 hex chars = 32 bytes)
  if [[ ${#VMORBIT_ENCRYPTION_KEY} -ne 64 ]]; then
    warn "VMORBIT_ENCRYPTION_KEY should be 64 hex characters (32 bytes)"
  fi

  # Check CORS matches API URL
  if [[ -n "${VMORBIT_SERVER_CORS_ORIGINS:-}" ]] && [[ -n "${NEXT_PUBLIC_API_URL:-}" ]]; then
    if [[ "${VMORBIT_SERVER_CORS_ORIGINS}" == *"${NEXT_PUBLIC_API_URL}"* ]] || \
       [[ "${NEXT_PUBLIC_API_URL}" == *"${VMORBIT_SERVER_CORS_ORIGINS}"* ]]; then
      pass "CORS origins match API URL"
    else
      warn "CORS origins may not match NEXT_PUBLIC_API_URL"
    fi
  fi
else
  fail "Cannot validate secrets — .env.production not found"
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 5. Docker Resources
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ System Resources"

# Check available disk space
DISK_AVAIL=$(df -BG "${ROOT_DIR}" 2>/dev/null | tail -1 | awk '{print $4}' | tr -d 'G')
if [[ -n "${DISK_AVAIL}" ]]; then
  if [[ ${DISK_AVAIL} -lt 10 ]]; then
    fail "Low disk space: ${DISK_AVAIL}GB available (need at least 10GB)"
  elif [[ ${DISK_AVAIL} -lt 20 ]]; then
    warn "Disk space: ${DISK_AVAIL}GB available (recommend 40GB+)"
  else
    pass "Disk space: ${DISK_AVAIL}GB available"
  fi
fi

# Check available memory
if command -v free >/dev/null 2>&1; then
  MEM_AVAIL=$(free -g | awk '/^Mem:/{print $7}')
  if [[ -n "${MEM_AVAIL}" ]] && [[ ${MEM_AVAIL} -lt 2 ]]; then
    warn "Low available memory: ${MEM_AVAIL}GB (recommend 4GB+)"
  elif [[ -n "${MEM_AVAIL}" ]]; then
    pass "Available memory: ${MEM_AVAIL}GB"
  fi
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 6. Network & Ports
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ Network & Ports"

check_port() {
  local port="$1"
  local desc="$2"
  if ss -tlnp 2>/dev/null | grep -q ":${port} " || netstat -tlnp 2>/dev/null | grep -q ":${port} "; then
    warn "Port ${port} (${desc}) is already in use"
  else
    pass "Port ${port} (${desc}) is available"
  fi
}

check_port 80 "HTTP"
check_port 443 "HTTPS"

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 7. Docker Images
# ─────────────────────────────────────────────────────────────────────────────
echo "▸ Docker Images"

if docker image inspect vmorbit/backend:latest >/dev/null 2>&1; then
  pass "Backend image exists (vmorbit/backend:latest)"
else
  warn "Backend image not built yet (run: docker compose -f docker-compose.production.yml build)"
fi

if docker image inspect vmorbit/frontend:latest >/dev/null 2>&1; then
  pass "Frontend image exists (vmorbit/frontend:latest)"
else
  warn "Frontend image not built yet"
fi

echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────────────────
echo "═══════════════════════════════════════════════════════"
if [[ ${ERRORS} -eq 0 ]] && [[ ${WARNINGS} -eq 0 ]]; then
  echo -e " ${GREEN}All checks passed — ready for deployment!${NC}"
  exit 0
elif [[ ${ERRORS} -eq 0 ]]; then
  echo -e " ${YELLOW}${WARNINGS} warning(s) — deployment may work but review above.${NC}"
  exit 2
else
  echo -e " ${RED}${ERRORS} error(s), ${WARNINGS} warning(s) — fix errors before deploying.${NC}"
  exit 1
fi
