import api from "./client";
import type {
  ApiResponse,
  PageParams,
  Workflow,
  WorkflowRun,
  WorkflowTriggerType,
  WorkflowActionType,
} from "@/types";

export interface WorkflowActionPayload {
  order?: number;
  action_type: WorkflowActionType;
  name?: string;
  description?: string;
  config?: Record<string, unknown>;
  retry_count?: number;
  timeout_seconds?: number;
  continue_on_error?: boolean;
}

export interface CreateWorkflowPayload {
  name: string;
  description?: string;
  enabled: boolean;
  trigger_type: WorkflowTriggerType;
  trigger_config?: Record<string, unknown>;
  conditions?: Record<string, unknown>;
  continue_on_error?: boolean;
  max_concurrent_runs?: number;
  actions?: WorkflowActionPayload[];
}

export const workflowApi = {
  list: (params?: { trigger_type?: string; enabled?: boolean; status?: string } & PageParams) =>
    api.get<ApiResponse<Workflow[]>>("/v1/workflows", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Workflow>>(`/v1/workflows/${id}`).then((r) => r.data.data),

  create: (payload: CreateWorkflowPayload) =>
    api.post<ApiResponse<Workflow>>("/v1/workflows", payload).then((r) => r.data.data),

  update: (id: string, payload: Partial<CreateWorkflowPayload>) =>
    api.put<ApiResponse<Workflow>>(`/v1/workflows/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/workflows/${id}`),

  enable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/workflows/${id}/enable`).then((r) => r.data),

  disable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/workflows/${id}/disable`).then((r) => r.data),

  triggerNow: (id: string, triggerData?: Record<string, unknown>) =>
    api.post<ApiResponse<{ run_id: string }>>(`/v1/workflows/${id}/trigger`, { trigger_data: triggerData }).then((r) => r.data.data),

  listRuns: (id: string, params?: PageParams) =>
    api.get<ApiResponse<WorkflowRun[]>>(`/v1/workflows/${id}/runs`, { params }).then((r) => r.data),

  getRun: (workflowId: string, runId: string) =>
    api.get<ApiResponse<WorkflowRun>>(`/v1/workflows/${workflowId}/runs/${runId}`).then((r) => r.data.data),
};
