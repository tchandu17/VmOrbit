export type ScheduleType = "once" | "daily" | "weekly" | "monthly" | "cron";
export type ScheduleStatus = "active" | "paused" | "disabled" | "expired";
export type ScheduleOperationType =
  | "inventory.sync"
  | "vm.power_on"
  | "vm.power_off"
  | "vm.reboot"
  | "vm.snapshot"
  | "vm.bulk.power_on"
  | "vm.bulk.power_off"
  | "vm.bulk.reboot"
  | "vm.bulk.snapshot";
export type ScheduleTargetType = "hypervisor" | "vm" | "tag";

export interface Schedule {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  description?: string;
  operation_type: ScheduleOperationType;
  target_type: ScheduleTargetType;
  target_ids: string[];
  schedule_type: ScheduleType;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
  status: ScheduleStatus;
  last_run_at?: string;
  next_run_at?: string;
  last_task_id?: string;
  last_run_status?: string;
  run_count: number;
  failure_count: number;
  max_runs: number;
  expires_at?: string;
  payload?: Record<string, unknown>;
  created_by?: string;
}

export interface ScheduleExecution {
  id: string;
  created_at: string;
  schedule_id: string;
  task_id?: string;
  status: "triggered" | "skipped" | "failed";
  error_message?: string;
  triggered_at: string;
  completed_at?: string;
}

export type WorkflowTriggerType =
  | "schedule"
  | "provider_disconnected"
  | "sync_failure"
  | "task_failure"
  | "vm_state_change"
  | "manual";

export type WorkflowStatus = "active" | "paused" | "disabled";
export type WorkflowRunStatus = "pending" | "running" | "completed" | "failed" | "cancelled";
export type WorkflowActionType =
  | "create_snapshot"
  | "power_on"
  | "power_off"
  | "reboot"
  | "send_notification"
  | "trigger_sync"
  | "webhook"
  | "delay";

export interface WorkflowAction {
  id: string;
  workflow_id: string;
  order: number;
  action_type: WorkflowActionType;
  name: string;
  description?: string;
  config?: Record<string, unknown>;
  retry_count: number;
  timeout_seconds: number;
  continue_on_error?: boolean;
}

export interface Workflow {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  description?: string;
  enabled: boolean;
  status: WorkflowStatus;
  trigger_type: WorkflowTriggerType;
  trigger_config?: Record<string, unknown>;
  conditions?: Record<string, unknown>;
  continue_on_error: boolean;
  max_concurrent_runs: number;
  run_count: number;
  failure_count: number;
  last_run_at?: string;
  last_run_status?: string;
  created_by?: string;
  actions?: WorkflowAction[];
}

export interface WorkflowRun {
  id: string;
  created_at: string;
  workflow_id: string;
  status: WorkflowRunStatus;
  trigger_type: WorkflowTriggerType;
  trigger_data?: Record<string, unknown>;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  actions_run: number;
  actions_failed: number;
  logs?: { entries: WorkflowRunLogEntry[] };
  triggered_by?: string;
}

export interface WorkflowRunLogEntry {
  action: string;
  action_type: string;
  order: number;
  started_at: string;
  completed_at?: string;
  status: "completed" | "failed";
  error?: string;
}
