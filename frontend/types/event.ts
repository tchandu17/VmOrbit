export type PlatformEventType =
  | "provider_connected"
  | "provider_disconnected"
  | "sync_completed"
  | "sync_failed"
  | "vm_poweron_success"
  | "vm_poweron_failed"
  | "vm_poweroff_success"
  | "vm_poweroff_failed"
  | "vm_reboot_success"
  | "vm_reboot_failed"
  | "snapshot_created"
  | "snapshot_failed"
  | "snapshot_deleted"
  | "snapshot_reverted"
  | "task_failed"
  | "bulk_operation_failed"
  | "login_failed"
  | "permission_denied";

export type PlatformEventSeverity = "info" | "warning" | "critical";

export interface PlatformEvent {
  id: string;
  created_at: string;
  event_type: PlatformEventType;
  severity: PlatformEventSeverity;
  provider?: string;
  resource_type?: string;
  resource_id?: string;
  hypervisor_id?: string;
  message: string;
  metadata?: Record<string, unknown>;
}

export type NotificationChannelType = "email" | "slack" | "webhook";
export type NotificationStatus = "pending" | "delivered" | "failed" | "throttled";

export interface NotificationChannel {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  type: NotificationChannelType;
  description?: string;
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface NotificationRule {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  description?: string;
  enabled: boolean;
  channel_id: string;
  channel?: NotificationChannel;
  event_types: string[];
  severities: string[];
  providers: string[];
  throttle_seconds: number;
  last_triggered_at?: string;
}

export interface NotificationHistory {
  id: string;
  created_at: string;
  rule_id: string;
  rule?: NotificationRule;
  channel_id: string;
  channel?: NotificationChannel;
  event_id: string;
  event?: PlatformEvent;
  status: NotificationStatus;
  error_message?: string;
  attempt_count: number;
  delivered_at?: string;
}
