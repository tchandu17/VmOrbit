-- =============================================================================
-- Migration: 001_initial_schema
-- Description: Full initial schema for VmOrbit — Unified Hypervisor Management
-- Strategy: idempotent (IF NOT EXISTS / CREATE INDEX CONCURRENTLY IF NOT EXISTS)
--           Safe to re-run; use a migration tool (golang-migrate, goose) to
--           track which version has been applied.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Extensions
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- uuid_generate_v4()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";    -- gen_random_uuid(), crypt()
CREATE EXTENSION IF NOT EXISTS "pg_trgm";     -- trigram indexes for ILIKE search

-- ---------------------------------------------------------------------------
-- USERS & RBAC
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,                          -- soft-delete
    email           VARCHAR(320) NOT NULL,
    username        VARCHAR(64)  NOT NULL,
    password_hash   TEXT         NOT NULL,
    first_name      VARCHAR(128) NOT NULL DEFAULT '',
    last_name       VARCHAR(128) NOT NULL DEFAULT '',
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    is_verified     BOOLEAN      NOT NULL DEFAULT FALSE,
    last_login_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_email
    ON users (email) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_username
    ON users (username) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_is_active
    ON users (is_active) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_deleted_at
    ON users (deleted_at);

-- Trigram index for fast username / email search
CREATE INDEX IF NOT EXISTS idx_users_username_trgm
    ON users USING gin (username gin_trgm_ops);

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS roles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    name        VARCHAR(64)  NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_roles_name
    ON roles (name) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS permissions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    resource    VARCHAR(64) NOT NULL,
    action      VARCHAR(64) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_permissions_resource_action
    ON permissions (resource, action) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- RBAC join tables (no surrogate key — composite PK is sufficient)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles (role_id);

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id
    ON role_permissions (permission_id);

-- ---------------------------------------------------------------------------
-- REFRESH TOKENS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL,                     -- SHA-256 of raw token
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     BOOLEAN     NOT NULL DEFAULT FALSE,
    user_agent  VARCHAR(512) NOT NULL DEFAULT '',
    ip_address  VARCHAR(45)  NOT NULL DEFAULT ''          -- IPv6 max = 45 chars
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_refresh_tokens_hash
    ON refresh_tokens (token_hash);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
    ON refresh_tokens (user_id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
    ON refresh_tokens (expires_at) WHERE revoked = FALSE;

-- ---------------------------------------------------------------------------
-- HYPERVISOR GROUPS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS hypervisor_groups (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    name        VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    tags        TEXT[]       NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_hypervisor_groups_name
    ON hypervisor_groups (name) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- HYPERVISORS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS hypervisors (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    group_id          UUID        REFERENCES hypervisor_groups(id) ON DELETE SET NULL,
    name              VARCHAR(128) NOT NULL,
    description       VARCHAR(512) NOT NULL DEFAULT '',
    provider          VARCHAR(32)  NOT NULL,              -- vmware | proxmox | kvm | hyperv
    host              VARCHAR(253) NOT NULL,
    port              INTEGER      NOT NULL,
    username          VARCHAR(128) NOT NULL DEFAULT '',
    encrypted_secret  TEXT         NOT NULL DEFAULT '',   -- AES-GCM ciphertext
    tls_verify        BOOLEAN      NOT NULL DEFAULT TRUE,
    connection_status VARCHAR(32)  NOT NULL DEFAULT 'unknown',
    last_checked_at   TIMESTAMPTZ,
    tags              TEXT[]       NOT NULL DEFAULT '{}',
    metadata          JSONB        NOT NULL DEFAULT '{}'
);

-- Prevent duplicate registrations of the same endpoint
CREATE UNIQUE INDEX IF NOT EXISTS uidx_hypervisors_endpoint
    ON hypervisors (host, port, provider) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_hypervisors_provider
    ON hypervisors (provider) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_hypervisors_connection_status
    ON hypervisors (connection_status) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_hypervisors_group_id
    ON hypervisors (group_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_hypervisors_deleted_at
    ON hypervisors (deleted_at);

-- ---------------------------------------------------------------------------
-- DATASTORES
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS data_stores (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    hypervisor_id UUID        NOT NULL REFERENCES hypervisors(id) ON DELETE CASCADE,
    provider_id   VARCHAR(256) NOT NULL,
    name          VARCHAR(256) NOT NULL,
    type          VARCHAR(64)  NOT NULL DEFAULT '',
    capacity_gb   NUMERIC(12,2) NOT NULL DEFAULT 0,
    used_gb       NUMERIC(12,2) NOT NULL DEFAULT 0,
    free_gb       NUMERIC(12,2) NOT NULL DEFAULT 0,
    accessible    BOOLEAN      NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_data_stores_provider
    ON data_stores (hypervisor_id, provider_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_data_stores_hypervisor_id
    ON data_stores (hypervisor_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_data_stores_accessible
    ON data_stores (accessible) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- NETWORKS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS networks (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    hypervisor_id UUID        NOT NULL REFERENCES hypervisors(id) ON DELETE CASCADE,
    provider_id   VARCHAR(256) NOT NULL,
    name          VARCHAR(256) NOT NULL,
    type          VARCHAR(64)  NOT NULL DEFAULT '',
    vlan          INTEGER      NOT NULL DEFAULT 0,
    accessible    BOOLEAN      NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_networks_provider
    ON networks (hypervisor_id, provider_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_networks_hypervisor_id
    ON networks (hypervisor_id) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- VM TEMPLATES
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS vm_templates (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    hypervisor_id UUID        NOT NULL REFERENCES hypervisors(id) ON DELETE CASCADE,
    provider_id   VARCHAR(256) NOT NULL,
    name          VARCHAR(256) NOT NULL,
    description   VARCHAR(512) NOT NULL DEFAULT '',
    guest_os      VARCHAR(128) NOT NULL DEFAULT '',
    cpu_count     INTEGER      NOT NULL DEFAULT 0,
    memory_mb     INTEGER      NOT NULL DEFAULT 0,
    disk_gb       INTEGER      NOT NULL DEFAULT 0,
    tags          TEXT[]       NOT NULL DEFAULT '{}',
    metadata      JSONB        NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_vm_templates_provider
    ON vm_templates (hypervisor_id, provider_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vm_templates_hypervisor_id
    ON vm_templates (hypervisor_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vm_templates_name
    ON vm_templates (name) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- VMs
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS vms (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    hypervisor_id   UUID        NOT NULL REFERENCES hypervisors(id) ON DELETE CASCADE,
    provider_vm_id  VARCHAR(256) NOT NULL,
    name            VARCHAR(256) NOT NULL,
    description     VARCHAR(512) NOT NULL DEFAULT '',
    status          VARCHAR(32)  NOT NULL DEFAULT 'unknown',
    cpu_count       INTEGER      NOT NULL DEFAULT 0,
    memory_mb       INTEGER      NOT NULL DEFAULT 0,
    disk_gb         INTEGER      NOT NULL DEFAULT 0,
    ip_addresses    TEXT[]       NOT NULL DEFAULT '{}',
    mac_address     VARCHAR(17)  NOT NULL DEFAULT '',
    network_name    VARCHAR(256) NOT NULL DEFAULT '',
    guest_os        VARCHAR(128) NOT NULL DEFAULT '',
    guest_os_type   VARCHAR(64)  NOT NULL DEFAULT '',
    tools_status    VARCHAR(64)  NOT NULL DEFAULT '',
    tags            TEXT[]       NOT NULL DEFAULT '{}',
    metadata        JSONB        NOT NULL DEFAULT '{}'
);

-- Core uniqueness: one DB row per provider-native VM per hypervisor
CREATE UNIQUE INDEX IF NOT EXISTS uidx_vms_provider
    ON vms (hypervisor_id, provider_vm_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vms_hypervisor_id
    ON vms (hypervisor_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vms_status
    ON vms (status) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vms_name
    ON vms (name) WHERE deleted_at IS NULL;

-- Trigram index for fast VM name search
CREATE INDEX IF NOT EXISTS idx_vms_name_trgm
    ON vms USING gin (name gin_trgm_ops);

-- GIN index for IP address array containment queries
-- e.g. WHERE ip_addresses @> ARRAY['10.0.0.1']
CREATE INDEX IF NOT EXISTS idx_vms_ip_addresses
    ON vms USING gin (ip_addresses);

-- GIN index for tag filtering
CREATE INDEX IF NOT EXISTS idx_vms_tags
    ON vms USING gin (tags);

-- JSONB index for metadata queries
CREATE INDEX IF NOT EXISTS idx_vms_metadata
    ON vms USING gin (metadata jsonb_path_ops);

-- ---------------------------------------------------------------------------
-- SNAPSHOTS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS snapshots (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    vm_id       UUID        NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
    provider_id VARCHAR(256) NOT NULL,
    name        VARCHAR(256) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    is_current  BOOLEAN      NOT NULL DEFAULT FALSE,
    parent_id   UUID        REFERENCES snapshots(id) ON DELETE SET NULL  -- self-referential tree
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_snapshots_provider
    ON snapshots (vm_id, provider_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_snapshots_vm_id
    ON snapshots (vm_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_snapshots_parent_id
    ON snapshots (parent_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_snapshots_is_current
    ON snapshots (vm_id, is_current) WHERE deleted_at IS NULL AND is_current = TRUE;

-- ---------------------------------------------------------------------------
-- TASKS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tasks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    type            VARCHAR(64)  NOT NULL,
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending',
    priority        INTEGER      NOT NULL DEFAULT 5,       -- 1 (highest) – 10 (lowest)
    payload         JSONB        NOT NULL DEFAULT '{}',
    result          JSONB,
    error_message   VARCHAR(2048) NOT NULL DEFAULT '',
    retry_count     INTEGER      NOT NULL DEFAULT 0,
    max_retries     INTEGER      NOT NULL DEFAULT 3,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    scheduled_at    TIMESTAMPTZ,
    timeout_at      TIMESTAMPTZ,
    created_by      UUID        NOT NULL REFERENCES users(id),
    hypervisor_id   UUID        REFERENCES hypervisors(id) ON DELETE SET NULL,
    vm_id           UUID        REFERENCES vms(id)         ON DELETE SET NULL,
    parent_task_id  UUID        REFERENCES tasks(id)       ON DELETE SET NULL
);

-- Primary polling index for the task engine worker loop
CREATE INDEX IF NOT EXISTS idx_tasks_worker_poll
    ON tasks (status, priority, scheduled_at)
    WHERE deleted_at IS NULL AND status IN ('pending', 'queued', 'retrying');

CREATE INDEX IF NOT EXISTS idx_tasks_status
    ON tasks (status) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_type
    ON tasks (type) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_created_by
    ON tasks (created_by) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_hypervisor_id
    ON tasks (hypervisor_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_vm_id
    ON tasks (vm_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id
    ON tasks (parent_task_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_timeout_at
    ON tasks (timeout_at)
    WHERE deleted_at IS NULL AND status = 'running' AND timeout_at IS NOT NULL;

-- ---------------------------------------------------------------------------
-- AUDIT LOGS  (append-only — no deleted_at, no updated_at)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS audit_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,
    username        VARCHAR(64)  NOT NULL DEFAULT '',
    action          VARCHAR(32)  NOT NULL,
    resource        VARCHAR(64)  NOT NULL,
    resource_id     UUID,
    description     VARCHAR(1024) NOT NULL DEFAULT '',
    hypervisor_id   UUID        REFERENCES hypervisors(id) ON DELETE SET NULL,
    ip_address      VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent      VARCHAR(512) NOT NULL DEFAULT '',
    request_id      VARCHAR(64)  NOT NULL DEFAULT '',
    changes         JSONB,
    metadata        JSONB,
    success         BOOLEAN      NOT NULL DEFAULT TRUE,
    error_message   VARCHAR(2048) NOT NULL DEFAULT ''
);

-- Audit logs are high-volume; partial indexes keep them lean
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
    ON audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id
    ON audit_logs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
    ON audit_logs (resource, resource_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action
    ON audit_logs (action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_hypervisor_id
    ON audit_logs (hypervisor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id
    ON audit_logs (request_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_success
    ON audit_logs (success, created_at DESC) WHERE success = FALSE;

-- ---------------------------------------------------------------------------
-- WEBSOCKET EVENTS  (append-only — no deleted_at, no updated_at)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS websocket_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_type      VARCHAR(64)  NOT NULL,
    room            VARCHAR(128) NOT NULL,
    hypervisor_id   UUID        REFERENCES hypervisors(id) ON DELETE SET NULL,
    resource_id     UUID,
    payload         JSONB        NOT NULL DEFAULT '{}',
    delivered_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ws_events_room
    ON websocket_events (room, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ws_events_event_type
    ON websocket_events (event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ws_events_resource_id
    ON websocket_events (resource_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ws_events_hypervisor_id
    ON websocket_events (hypervisor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ws_events_created_at
    ON websocket_events (created_at DESC);

-- Partial index for undelivered events (replay / catch-up queries)
CREATE INDEX IF NOT EXISTS idx_ws_events_undelivered
    ON websocket_events (room, created_at)
    WHERE delivered_at IS NULL;

-- ---------------------------------------------------------------------------
-- UPDATED_AT trigger (keeps updated_at current without application code)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to every table that has updated_at
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'roles', 'permissions', 'refresh_tokens',
        'hypervisor_groups', 'hypervisors',
        'data_stores', 'networks', 'vm_templates',
        'vms', 'snapshots', 'tasks'
    ]
    LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_trigger
            WHERE tgname = 'trg_' || t || '_updated_at'
        ) THEN
            EXECUTE format(
                'CREATE TRIGGER trg_%I_updated_at
                 BEFORE UPDATE ON %I
                 FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
                t, t
            );
        END IF;
    END LOOP;
END;
$$;

-- ---------------------------------------------------------------------------
-- SEED: built-in roles & permissions
-- ---------------------------------------------------------------------------

INSERT INTO roles (id, name, description)
VALUES
    (gen_random_uuid(), 'admin',    'Full platform access'),
    (gen_random_uuid(), 'operator', 'Manage VMs and hypervisors; no user management'),
    (gen_random_uuid(), 'viewer',   'Read-only access to all resources')
ON CONFLICT DO NOTHING;

INSERT INTO permissions (id, resource, action)
VALUES
    -- VM permissions
    (gen_random_uuid(), 'vm', 'read'),
    (gen_random_uuid(), 'vm', 'create'),
    (gen_random_uuid(), 'vm', 'update'),
    (gen_random_uuid(), 'vm', 'delete'),
    (gen_random_uuid(), 'vm', 'power'),
    (gen_random_uuid(), 'vm', 'snapshot'),
    (gen_random_uuid(), 'vm', 'console'),
    -- Hypervisor permissions
    (gen_random_uuid(), 'hypervisor', 'read'),
    (gen_random_uuid(), 'hypervisor', 'create'),
    (gen_random_uuid(), 'hypervisor', 'update'),
    (gen_random_uuid(), 'hypervisor', 'delete'),
    (gen_random_uuid(), 'hypervisor', 'sync'),
    -- User permissions
    (gen_random_uuid(), 'user', 'read'),
    (gen_random_uuid(), 'user', 'create'),
    (gen_random_uuid(), 'user', 'update'),
    (gen_random_uuid(), 'user', 'delete'),
    -- Task permissions
    (gen_random_uuid(), 'task', 'read'),
    (gen_random_uuid(), 'task', 'cancel'),
    -- Audit permissions
    (gen_random_uuid(), 'audit', 'read')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- SCALABILITY NOTES (comments only — apply manually when needed)
-- ---------------------------------------------------------------------------

-- 1. PARTITION audit_logs by month (requires pg_partman or manual DDL):
--
--    ALTER TABLE audit_logs RENAME TO audit_logs_default;
--    CREATE TABLE audit_logs (LIKE audit_logs_default INCLUDING ALL)
--        PARTITION BY RANGE (created_at);
--    CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
--        FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
--    -- pg_partman can automate monthly partition creation.

-- 2. PARTITION websocket_events similarly if event volume is high.

-- 3. READ REPLICAS: route all SELECT queries in repositories to a replica
--    connection pool. GORM supports this via ConnPool or a custom Dialector.

-- 4. CONNECTION POOLING: use PgBouncer in transaction mode in front of
--    Postgres to handle thousands of short-lived Go goroutine connections.

-- 5. JSONB GIN indexes on tasks.payload and audit_logs.changes can be added
--    if you query inside those documents frequently:
--    CREATE INDEX idx_tasks_payload ON tasks USING gin (payload jsonb_path_ops);
