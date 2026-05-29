export interface CapacitySummary {
  total_hypervisors: number;
  total_vms: number;
  running_vms: number;
  stopped_vms: number;
  total_cpu_cores: number;
  total_memory_mb: number;
  total_disk_gb: number;
  total_snapshots: number;
  total_environments: number;
  cpu_utilisation_pct: number;
  memory_utilisation_pct: number;
  storage_utilisation_pct: number;
  total_storage_gb: number;
  used_storage_gb: number;
  free_storage_gb: number;
  vm_density: number;
  tasks_completed_24h: number;
  tasks_failed_24h: number;
  critical_recommendations: number;
  warning_recommendations: number;
  info_recommendations: number;
  collected_at: string;
}

export interface TimeSeriesPoint {
  timestamp: string;
  value: number;
  label?: string;
}

export interface CapacityTrends {
  vm_growth: TimeSeriesPoint[];
  cpu_trend: TimeSeriesPoint[];
  memory_trend: TimeSeriesPoint[];
  storage_trend: TimeSeriesPoint[];
  snapshot_trend: TimeSeriesPoint[];
  task_trend: TimeSeriesPoint[];
  provider_vm_growth: Record<string, TimeSeriesPoint[]>;
}

export type RecommendationType =
  | "oversized_vm"
  | "idle_vm"
  | "stale_snapshot"
  | "underutilized_host"
  | "overcommitted_host"
  | "powered_off_stale_vm"
  | "orphaned_resource"
  | "snapshot_growth"
  | "storage_exhaustion";

export type RecommendationSeverity = "info" | "warning" | "critical";
export type RecommendationStatus = "active" | "dismissed" | "resolved";

export interface OptimizationRecommendation {
  id: string;
  created_at: string;
  updated_at: string;
  fingerprint: string;
  type: RecommendationType;
  severity: RecommendationSeverity;
  status: RecommendationStatus;
  hypervisor_id?: string;
  vm_id?: string;
  title: string;
  description: string;
  action: string;
  score: number;
  estimated_savings_gb: number;
  estimated_savings_cpu: number;
  estimated_savings_mb: number;
  dismissed_at?: string;
  resolved_at?: string;
  metadata?: Record<string, unknown>;
  hypervisor?: { id: string; name: string; provider: string };
  vm?: { id: string; name: string };
}

export interface RecommendationSummary {
  total_active: number;
  total_dismissed: number;
  total_resolved: number;
  by_severity: Record<string, number>;
  by_type: Record<string, number>;
}

export interface ProviderCapacity {
  id: string;
  updated_at: string;
  hypervisor_id: string;
  total_vms: number;
  running_vms: number;
  stopped_vms: number;
  total_cpu_cores: number;
  total_memory_mb: number;
  total_storage_gb: number;
  used_storage_gb: number;
  free_storage_gb: number;
  storage_used_pct: number;
  snapshot_count: number;
  hypervisor?: { id: string; name: string; provider: string };
}

export interface ForecastRisk {
  type: string;
  severity: "info" | "warning" | "critical";
  description: string;
  days_until?: number;
}

export interface HypervisorForecast {
  hypervisor_id: string;
  hypervisor_name: string;
  provider: string;
  storage_exhaustion_days?: number;
  storage_growth_rate_gb_day: number;
  current_storage_used_pct: number;
  vm_growth_rate_per_day: number;
  projected_vms_30_days: number;
  snapshot_growth_rate_per_day: number;
  snapshot_risk: "low" | "medium" | "high";
  risks: ForecastRisk[];
}

export interface ForecastReport {
  generated_at: string;
  forecasts: HypervisorForecast[];
  global_risks: ForecastRisk[];
}
