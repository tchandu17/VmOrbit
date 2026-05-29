import api from "./client";
import type {
  ApiResponse,
  Policy,
  PolicyViolation,
  PolicyAssignment,
  PolicyEvaluationResult,
  ApprovalRequest,
  PolicyConditionType,
  PolicyConditionOperator,
  PolicyAssignmentTargetType,
  PolicyEffect,
  PolicyType,
} from "@/types";

// ── Policy API ────────────────────────────────────────────────────────────────

export const policyApi = {
  list: (params?: {
    type?: string;
    effect?: string;
    enabled?: boolean;
    search?: string;
    page?: number;
    page_size?: number;
  }) =>
    api.get<ApiResponse<Policy[]>>("/v1/policies", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Policy>>(`/v1/policies/${id}`).then((r) => r.data.data),

  create: (payload: {
    name: string;
    description?: string;
    type: PolicyType;
    effect: PolicyEffect;
    priority?: number;
    enabled?: boolean;
    operations?: string[];
    conditions?: Array<{
      type: PolicyConditionType;
      operator: PolicyConditionOperator;
      value: string;
      negate?: boolean;
    }>;
    approval_config?: Record<string, unknown>;
    metadata?: Record<string, unknown>;
  }) =>
    api.post<ApiResponse<Policy>>("/v1/policies", payload).then((r) => r.data.data),

  update: (
    id: string,
    payload: {
      name?: string;
      description?: string;
      effect?: PolicyEffect;
      priority?: number;
      enabled?: boolean;
      operations?: string[];
      conditions?: Array<{
        type: PolicyConditionType;
        operator: PolicyConditionOperator;
        value: string;
        negate?: boolean;
      }>;
      approval_config?: Record<string, unknown>;
      metadata?: Record<string, unknown>;
    }
  ) =>
    api.put<ApiResponse<Policy>>(`/v1/policies/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) => api.delete(`/v1/policies/${id}`),

  enable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/policies/${id}/enable`).then((r) => r.data.data),

  disable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/policies/${id}/disable`).then((r) => r.data.data),

  // Assignments
  listAssignments: (policyId: string) =>
    api
      .get<ApiResponse<PolicyAssignment[]>>(`/v1/policies/${policyId}/assignments`)
      .then((r) => r.data.data ?? []),

  assign: (
    policyId: string,
    payload: { target_type: PolicyAssignmentTargetType; target_id?: string }
  ) =>
    api
      .post<ApiResponse<PolicyAssignment>>(`/v1/policies/${policyId}/assignments`, payload)
      .then((r) => r.data.data),

  unassign: (policyId: string, assignmentId: string) =>
    api.delete(`/v1/policies/${policyId}/assignments/${assignmentId}`),

  // Violations
  listViolations: (params?: {
    policy_id?: string;
    user_id?: string;
    operation?: string;
    resource_type?: string;
    resource_id?: string;
    status?: string;
    since?: string;
    until?: string;
    page?: number;
    page_size?: number;
  }) =>
    api
      .get<ApiResponse<PolicyViolation[]>>("/v1/policy-violations", { params })
      .then((r) => r.data),

  // Evaluation
  evaluate: (payload: {
    operation: string;
    resource_type: string;
    resource_id?: string;
    resource_name?: string;
    vm_tags?: string[];
    provider_type?: string;
    hypervisor_id?: string;
    environment?: string;
    bulk_size?: number;
    metadata?: Record<string, unknown>;
  }) =>
    api
      .post<ApiResponse<PolicyEvaluationResult>>("/v1/policies/evaluate", payload)
      .then((r) => r.data.data),
};

// ── Approval API ──────────────────────────────────────────────────────────────

export const approvalApi = {
  list: (params?: {
    status?: string;
    requester_id?: string;
    policy_id?: string;
    operation?: string;
    resource_type?: string;
    resource_id?: string;
    since?: string;
    until?: string;
    page?: number;
    page_size?: number;
  }) =>
    api.get<ApiResponse<ApprovalRequest[]>>("/v1/approvals", { params }).then((r) => r.data),

  getPending: (params?: { page?: number; page_size?: number }) =>
    api
      .get<ApiResponse<ApprovalRequest[]>>("/v1/approvals/pending", { params })
      .then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<ApprovalRequest>>(`/v1/approvals/${id}`).then((r) => r.data.data),

  approve: (id: string, comment?: string) =>
    api
      .post<ApiResponse<{ status: string }>>(`/v1/approvals/${id}/approve`, { comment: comment ?? "" })
      .then((r) => r.data.data),

  reject: (id: string, comment?: string) =>
    api
      .post<ApiResponse<{ status: string }>>(`/v1/approvals/${id}/reject`, { comment: comment ?? "" })
      .then((r) => r.data.data),

  cancel: (id: string) =>
    api
      .post<ApiResponse<{ status: string }>>(`/v1/approvals/${id}/cancel`)
      .then((r) => r.data.data),

  escalate: (id: string, escalate_to: string, comment?: string) =>
    api
      .post<ApiResponse<{ status: string }>>(`/v1/approvals/${id}/escalate`, {
        escalate_to,
        comment: comment ?? "",
      })
      .then((r) => r.data.data),
};
