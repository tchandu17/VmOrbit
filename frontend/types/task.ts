export type TaskStatus =
  | "pending"
  | "queued"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "retrying"
  | "timed_out";

export type TaskType =
  | "vm.power_on"
  | "vm.power_off"
  | "vm.reboot"
  | "vm.suspend"
  | "vm.create"
  | "vm.delete"
  | "vm.clone"
  | "vm.provision"
  | "vm.snapshot"
  | "vm.snapshot.delete"
  | "vm.restore"
  | "vm.migrate"
  | "vm.bulk.power_on"
  | "vm.bulk.power_off"
  | "vm.bulk.reboot"
  | "vm.bulk.snapshot"
  | "inventory.sync"
  | "hypervisor.sync"
  | "template.sync";

export interface Task {
  id: string;
  created_at: string;
  updated_at: string;
  type: TaskType;
  status: TaskStatus;
  priority: number;
  progress: number;
  payload?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error_message?: string;
  retry_count: number;
  max_retries: number;
  started_at?: string;
  completed_at?: string;
  created_by: string;
  hypervisor_id?: string;
  vm_id?: string;
  parent_task_id?: string;
}

export type TaskLogLevel = "debug" | "info" | "warn" | "error";

export interface TaskLog {
  id: string;
  created_at: string;
  task_id: string;
  level: TaskLogLevel;
  message: string;
  fields?: Record<string, unknown>;
}
