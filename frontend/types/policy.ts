// ── Policy types ──────────────────────────────────────────────────────────────

export type PolicyType = "vm" | "environment" | "provider" | "task" | "user";

export type PolicyEffect =
  | "allow"
  | "deny"
  | "require_approval"
  | "require_snapshot"
  | "require_justification";

export type PolicyConditionType =
  | "vm_tag"
  | "environment"
  | "provider"
  | "user_role"
  | "operation"
  | "maintenance_window"
  | "time_schedule"
  | "vm_name"
  | "hypervisor"
  | "bulk_size";

export type PolicyConditionOperator =
  | "equals"
  | "not_equals"
  | "contains"
  | "in"
  | "not_in"
  | "greater_than"
  | "less_than"
  | "matches";

export type PolicyAssignmentTargetType =
  | "global"
  | "hypervisor"
  | "environment"
  | "vm"
  | "tag"
  | "role";

export type PolicyViolationStatus = "blocked" | "overridden" | "pending_approval";

export interface PolicyCondition {
  id: string;
  created_at: string;
  policy_id: string;
  type: PolicyConditionType;
  operator: PolicyConditionOperator;
  value: string;
  negate: boolean;
}

export interface PolicyAssignment {
  id: string;
  created_at: string;
  policy_id: string;
  target_type: PolicyAssignmentTargetType;
  target_id?: string;
  created_by?: string;
}

export interface Policy {
  id: string;
  created_at: string;
  updated_at: string;
  name: string;
  description: string;
  type: PolicyType;
  effect: PolicyEffect;
  priority: number;
  enabled: boolean;
  operations: string[];
  approval_config?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  conditions?: PolicyCondition[];
  assignments?: PolicyAssignment[];
}

export interface PolicyViolation {
  id: string;
  created_at: string;
  policy_id: string;
  policy_name: string;
  effect: PolicyEffect;
  status: PolicyViolationStatus;
  operation: string;
  resource_type: string;
  resource_id: string;
  resource_name: string;
  user_id?: string;
  username: string;
  approval_request_id?: string;
  justification?: string;
  metadata?: Record<string, unknown>;
}

export interface PolicyEvaluationResult {
  allowed: boolean;
  effect?: PolicyEffect;
  matched_policy?: Policy;
  violation_id?: string;
  approval_request_id?: string;
  message: string;
  requires_snapshot?: boolean;
  requires_justification?: boolean;
}

// ── Approval types ────────────────────────────────────────────────────────────

export type ApprovalStatus =
  | "pending"
  | "approved"
  | "rejected"
  | "expired"
  | "escalated"
  | "cancelled";

export type ApprovalStepStatus = "pending" | "approved" | "rejected" | "skipped";

export type ApprovalHistoryAction =
  | "created"
  | "approved"
  | "rejected"
  | "escalated"
  | "expired"
  | "cancelled"
  | "commented";

export interface ApprovalStep {
  id: string;
  created_at: string;
  request_id: string;
  step_order: number;
  status: ApprovalStepStatus;
  approver_id?: string;
  approver_role?: string;
  approver_name?: string;
  resolved_by?: string;
  resolved_by_name?: string;
  resolved_at?: string;
  comment?: string;
}

export interface ApprovalHistoryEntry {
  id: string;
  created_at: string;
  request_id: string;
  action: ApprovalHistoryAction;
  actor_id?: string;
  actor_name: string;
  comment?: string;
  metadata?: Record<string, unknown>;
}

export interface ApprovalRequest {
  id: string;
  created_at: string;
  updated_at: string;
  policy_id: string;
  policy_name: string;
  operation: string;
  resource_type: string;
  resource_id: string;
  resource_name: string;
  requester_id: string;
  requester_name: string;
  justification?: string;
  status: ApprovalStatus;
  expires_at?: string;
  resolved_at?: string;
  escalated_at?: string;
  escalated_to?: string;
  operation_payload?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  steps?: ApprovalStep[];
  history?: ApprovalHistoryEntry[];
}
