import type { VM } from "./vm";
import type { Tag } from "./tag";

export type EnvironmentType =
  | "production"
  | "staging"
  | "development"
  | "qa"
  | "disaster_recovery"
  | "custom";

export type EnvironmentStatus =
  | "healthy"
  | "degraded"
  | "unhealthy"
  | "unknown"
  | "starting"
  | "stopping";

export type OrchestrationOperation =
  | "start"
  | "stop"
  | "restart"
  | "snapshot"
  | "clone";

export type OrchestrationRunStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "rolling_back"
  | "rolled_back";

export type OrchestrationStepStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "skipped";

export type DependencyType = "start_before" | "stop_after" | "requires";

// ── Environment ───────────────────────────────────────────────────────────────

export interface Environment {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  description: string;
  type: EnvironmentType;
  status: EnvironmentStatus;
  owner_id?: string;
  tags: string[];
  color: string;
  metadata: Record<string, unknown>;
  vms?: EnvironmentVM[];
  env_tags?: EnvironmentTag[];
}

export interface EnvironmentVM {
  id: string;
  environment_id: string;
  vm_id: string;
  start_order: number;
  stop_order: number;
  role: string;
  notes: string;
  vm?: VM;
}

export interface EnvironmentTag {
  environment_id: string;
  tag_id: string;
  tag?: Tag;
}

// ── Dependencies ──────────────────────────────────────────────────────────────

export interface VMDependency {
  id: string;
  environment_id: string;
  source_vm_id: string;
  target_vm_id: string;
  type: DependencyType;
  delay_seconds: number;
  notes: string;
  source_vm?: VM;
  target_vm?: VM;
}

// ── Orchestration runs ────────────────────────────────────────────────────────

export interface OrchestrationRun {
  id: string;
  created_at: string;
  updated_at: string;
  environment_id: string;
  operation: OrchestrationOperation;
  status: OrchestrationRunStatus;
  progress: number;
  total_vms: number;
  completed_vms: number;
  failed_vms: number;
  skipped_vms: number;
  error_message?: string;
  payload?: Record<string, unknown>;
  result?: Record<string, unknown>;
  started_at?: string;
  completed_at?: string;
  created_by?: string;
  steps?: OrchestrationStep[];
}

export interface OrchestrationStep {
  id: string;
  run_id: string;
  vm_id: string;
  execution_order: number;
  status: OrchestrationStepStatus;
  task_id?: string;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  vm?: VM;
}

// ── Request payloads ──────────────────────────────────────────────────────────

export interface CreateEnvironmentPayload {
  name: string;
  description?: string;
  type?: EnvironmentType;
  owner_id?: string;
  tags?: string[];
  color?: string;
  metadata?: Record<string, unknown>;
}

export interface UpdateEnvironmentPayload {
  name?: string;
  description?: string;
  type?: EnvironmentType;
  owner_id?: string;
  tags?: string[];
  color?: string;
  metadata?: Record<string, unknown>;
}

export interface AddVMToEnvironmentPayload {
  vm_id: string;
  start_order?: number;
  stop_order?: number;
  role?: string;
  notes?: string;
}

export interface UpdateVMOrderingPayload {
  start_order: number;
  stop_order: number;
  role?: string;
}

export interface AddDependencyPayload {
  source_vm_id: string;
  target_vm_id: string;
  type: DependencyType;
  delay_seconds?: number;
  notes?: string;
}

export interface SnapshotEnvironmentPayload {
  snapshot_name: string;
  description?: string;
  memory?: boolean;
}

export interface CloneEnvironmentPayload {
  new_environment_name: string;
  name_suffix?: string;
}
