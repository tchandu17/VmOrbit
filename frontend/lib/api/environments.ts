import api from "./client";
import type {
  ApiResponse,
  PageParams,
  Environment,
  EnvironmentVM,
  VMDependency,
  OrchestrationRun,
  OrchestrationStep,
  CreateEnvironmentPayload,
  UpdateEnvironmentPayload,
  AddVMToEnvironmentPayload,
  UpdateVMOrderingPayload,
  AddDependencyPayload,
  SnapshotEnvironmentPayload,
  CloneEnvironmentPayload,
} from "@/types";

export const environmentApi = {
  // ── CRUD ──────────────────────────────────────────────────────────────────
  list: (params?: { type?: string; status?: string; search?: string } & PageParams) =>
    api.get<ApiResponse<Environment[]>>("/v1/environments", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Environment>>(`/v1/environments/${id}`).then((r) => r.data.data),

  create: (payload: CreateEnvironmentPayload) =>
    api.post<ApiResponse<Environment>>("/v1/environments", payload).then((r) => r.data.data),

  update: (id: string, payload: UpdateEnvironmentPayload) =>
    api.put<ApiResponse<Environment>>(`/v1/environments/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) => api.delete(`/v1/environments/${id}`),

  // ── VM membership ─────────────────────────────────────────────────────────
  listVMs: (id: string) =>
    api.get<ApiResponse<EnvironmentVM[]>>(`/v1/environments/${id}/vms`).then((r) => r.data.data ?? []),

  addVM: (id: string, payload: AddVMToEnvironmentPayload) =>
    api.post<ApiResponse<unknown>>(`/v1/environments/${id}/vms`, payload).then((r) => r.data),

  removeVM: (id: string, vmId: string) =>
    api.delete(`/v1/environments/${id}/vms/${vmId}`),

  updateVMOrdering: (id: string, vmId: string, payload: UpdateVMOrderingPayload) =>
    api.put<ApiResponse<unknown>>(`/v1/environments/${id}/vms/${vmId}`, payload).then((r) => r.data),

  // ── Dependencies ──────────────────────────────────────────────────────────
  listDependencies: (id: string) =>
    api.get<ApiResponse<VMDependency[]>>(`/v1/environments/${id}/dependencies`).then((r) => r.data.data ?? []),

  addDependency: (id: string, payload: AddDependencyPayload) =>
    api.post<ApiResponse<VMDependency>>(`/v1/environments/${id}/dependencies`, payload).then((r) => r.data.data),

  removeDependency: (id: string, depId: string) =>
    api.delete(`/v1/environments/${id}/dependencies/${depId}`),

  // ── Orchestration ─────────────────────────────────────────────────────────
  start: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/environments/${id}/start`).then((r) => r.data.data),

  stop: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/environments/${id}/stop`).then((r) => r.data.data),

  restart: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/environments/${id}/restart`).then((r) => r.data.data),

  snapshot: (id: string, payload: SnapshotEnvironmentPayload) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/environments/${id}/snapshot`, payload).then((r) => r.data.data),

  clone: (id: string, payload: CloneEnvironmentPayload) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/environments/${id}/clone`, payload).then((r) => r.data.data),

  refreshStatus: (id: string) =>
    api.post<ApiResponse<Environment>>(`/v1/environments/${id}/refresh-status`).then((r) => r.data.data),

  // ── Run tracking ──────────────────────────────────────────────────────────
  listRuns: (id: string, params?: PageParams) =>
    api.get<ApiResponse<OrchestrationRun[]>>(`/v1/environments/${id}/runs`, { params }).then((r) => r.data),

  getRun: (id: string, runId: string) =>
    api.get<ApiResponse<OrchestrationRun>>(`/v1/environments/${id}/runs/${runId}`).then((r) => r.data.data),

  getRunSteps: (id: string, runId: string) =>
    api.get<ApiResponse<OrchestrationStep[]>>(`/v1/environments/${id}/runs/${runId}/steps`).then((r) => r.data.data ?? []),
};
