-- =============================================================================
-- Migration: 002_task_engine_v2
-- Description: Adds progress tracking and structured task logs to the task
--              engine. Safe to re-run (idempotent).
-- =============================================================================

-- ---------------------------------------------------------------------------
-- tasks: add progress column
-- ---------------------------------------------------------------------------

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS progress INTEGER NOT NULL DEFAULT 0
        CHECK (progress >= 0 AND progress <= 100);

-- ---------------------------------------------------------------------------
-- task_logs: per-task structured log entries (append-only)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS task_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    task_id     UUID        NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    level       VARCHAR(16)  NOT NULL DEFAULT 'info',     -- debug | info | warn | error
    message     VARCHAR(4096) NOT NULL,
    fields      JSONB
);

-- Primary access pattern: all logs for a task, oldest-first
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id
    ON task_logs (task_id, created_at ASC);

-- Partial index for error-level entries (useful for alerting queries)
CREATE INDEX IF NOT EXISTS idx_task_logs_errors
    ON task_logs (task_id, created_at DESC)
    WHERE level = 'error';

-- ---------------------------------------------------------------------------
-- tasks: update worker-poll index to include 'retrying' status
-- (the original index in 001 only covered pending/queued)
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS idx_tasks_worker_poll;

CREATE INDEX IF NOT EXISTS idx_tasks_worker_poll
    ON tasks (status, priority, created_at)
    WHERE deleted_at IS NULL
      AND status IN ('pending', 'queued', 'retrying');
