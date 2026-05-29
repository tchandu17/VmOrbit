# VMOrbit — Security Hardening Guide

Production security measures implemented and recommended for VMOrbit deployments.

---

## Implemented Security Controls

### 1. Transport Layer Security (TLS)

- **TLS 1.2 + 1.3 only** — older protocols disabled in nginx
- **Strong cipher suites** — ECDHE-based key exchange, AES-GCM and ChaCha20
- **HSTS** — 2-year max-age with includeSubDomains and preload
- **HTTP → HTTPS redirect** — all port 80 traffic redirected to 443

**Configuration:** `nginx/nginx.conf`

### 2. Security Headers

All responses include:

| Header | Value | Purpose |
|--------|-------|---------|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` | Force HTTPS |
| `X-Frame-Options` | `SAMEORIGIN` | Prevent clickjacking |
| `X-Content-Type-Options` | `nosniff` | Prevent MIME sniffing |
| `X-XSS-Protection` | `1; mode=block` | XSS filter |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limit referrer leakage |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Disable unused APIs |

### 3. Rate Limiting

| Zone | Rate | Burst | Scope |
|------|------|-------|-------|
| `api_limit` | 30 req/s | 50 | Per IP — all API endpoints |
| `auth_limit` | 5 req/s | 10 | Per IP — login/refresh only |
| `conn_limit` | 100 | — | Per IP — concurrent connections |
| Global (backend) | 100 req/s | — | Total across all clients |
| Per-user (backend) | 10 req/s | — | Per authenticated user |
| Provider operations | 5-10 req/s | — | Per IP — hypervisor operations |

### 4. Credential Encryption

- Provider passwords/tokens encrypted at rest with **AES-256-GCM**
- Encryption key provided via `VMORBIT_ENCRYPTION_KEY` environment variable
- Key format: 64 hex characters (32 bytes)
- Generate: `openssl rand -hex 32`

### 5. Authentication & Authorization

- **JWT-based authentication** with short-lived access tokens (15 min)
- **Refresh token rotation** — tokens stored in Redis with TTL
- **RBAC** — Role-Based Access Control with granular permissions
- **Permission middleware** — every API endpoint requires specific permissions
- **WebSocket auth** — JWT validated on connection upgrade

### 6. Container Security

- **Non-root execution** — backend runs as UID 65534, frontend as `nextjs` user
- **Minimal images** — backend uses `scratch` (no shell, no OS), frontend uses Alpine
- **Resource limits** — CPU and memory limits on all containers
- **Read-only volumes** — config files mounted as `:ro`
- **No privileged mode** — no containers run with elevated privileges
- **Network isolation** — dedicated Docker bridge network

### 7. Database Security

- **Connection pooling** — prevents connection exhaustion attacks
- **Parameterized queries** — GORM prevents SQL injection
- **SSL mode** — configurable (`disable` for internal Docker network, `require` for external DB)
- **Separate credentials** — DB user has minimal required permissions

### 8. API Protection

- **CORS** — restricted to configured origins only
- **Request ID tracking** — every request gets a unique ID for audit trail
- **Circuit breaker** — provider operations use circuit breaker pattern
- **Deduplication** — sync operations deduplicated to prevent abuse
- **Input validation** — request bodies validated before processing
- **Maintenance mode** — ability to reject traffic during upgrades

---

## Recommended Additional Hardening

### Before Go-Live

1. **Rotate the encryption key** — ensure `VMORBIT_ENCRYPTION_KEY` is unique per environment
2. **Set strong passwords** — use `openssl rand -base64 32` for DB and Redis passwords
3. **Enable RBAC enforcement** — wire `RequirePermission` middleware to actual policy checks
4. **Disable server tokens** — already done (`server_tokens off` in nginx)
5. **Review CORS origins** — ensure only your production domain is listed

### Network Security

```bash
# Restrict Docker daemon to localhost
# /etc/docker/daemon.json
{
  "iptables": true,
  "ip": "127.0.0.1"
}

# Firewall rules (UFW)
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### SSH Hardening

```bash
# /etc/ssh/sshd_config
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
ClientAliveInterval 300
ClientAliveCountMax 2
```

### Docker Daemon Security

```bash
# /etc/docker/daemon.json
{
  "no-new-privileges": true,
  "live-restore": true,
  "userland-proxy": false,
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "5"
  }
}
```

### Secrets Management

For production environments beyond single-server deployments:

1. **HashiCorp Vault** — store encryption keys and DB credentials
2. **AWS Secrets Manager** — if running on AWS
3. **Docker Secrets** — for Docker Swarm deployments
4. **Environment injection** — never bake secrets into images

### Audit Trail

VMOrbit logs all administrative actions to the audit table:
- User authentication events (login, logout, token refresh)
- Hypervisor operations (add, remove, sync)
- VM power operations
- Configuration changes
- RBAC changes (role assignments, permission changes)

Access audit logs via: `GET /api/v1/audit`

### Monitoring for Security Events

Set up alerts for:
- Failed login attempts (> 5 per minute from same IP)
- 401/403 response spikes
- Unusual API access patterns
- Provider credential test failures
- New user creation outside business hours

---

## Security Checklist

Use this checklist before production deployment:

- [ ] TLS certificates installed and valid
- [ ] `.env.production` has unique, strong secrets
- [ ] CORS origins restricted to production domain
- [ ] Firewall configured (only 80, 443, SSH open)
- [ ] SSH key-only authentication enabled
- [ ] Docker running as non-root
- [ ] Backup encryption configured (if using off-site backups)
- [ ] Monitoring alerts configured for security events
- [ ] Default admin password changed after first login
- [ ] Unused ports closed
- [ ] Log rotation configured
- [ ] SSL certificate auto-renewal configured
