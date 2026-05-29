export type AuditAction =
  | "create"
  | "read"
  | "update"
  | "delete"
  | "login"
  | "logout"
  | "execute";

export interface AuditLog {
  id: string;
  created_at: string;
  user_id?: string;
  username: string;
  action: AuditAction;
  resource: string;
  resource_id?: string;
  description: string;
  hypervisor_id?: string;
  ip_address: string;
  user_agent: string;
  request_id: string;
  changes?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  success: boolean;
  error_message?: string;
}
