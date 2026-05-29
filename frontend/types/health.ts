export type HealthStatus = "healthy" | "degraded" | "unhealthy" | "unknown";

export interface ProviderHealth {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  status: HealthStatus;

  // Connectivity
  online: boolean;
  last_seen_at?: string;
  last_check_at?: string;
  consecutive_fails: number;

  // Latency (ms)
  latency_ms: number;
  avg_latency_ms: number;
  peak_latency_ms: number;

  // Sync health
  last_sync_at?: string;
  last_sync_status: string;
  sync_failures_24h: number;
  vm_count: number;

  // Task health
  tasks_total_24h: number;
  tasks_failed_24h: number;
  task_failure_rate: number;

  // Auth failures
  auth_failures_24h: number;

  // Inventory freshness
  inventory_age_minutes: number;
  inventory_stale: boolean;

  // Score
  health_score: number;

  // Joined hypervisor
  hypervisor?: {
    id: string;
    name: string;
    provider: string;
    host: string;
    port: number;
    connection_status: string;
  };
}

export interface ProviderHealthHistory {
  id: string;
  created_at: string;
  hypervisor_id: string;
  status: HealthStatus;
  online: boolean;
  latency_ms: number;
  health_score: number;
  error_message?: string;
}
