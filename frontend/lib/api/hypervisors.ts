import api from "./client";
import type {
  ApiResponse,
  PaginatedResponse,
  PageParams,
  Hypervisor,
  RegisterHypervisorPayload,
  UpdateHypervisorPayload,
  TestConnectionResult,
} from "@/types";

export const hypervisorApi = {
  list: (params?: PageParams) =>
    api
      .get<ApiResponse<Hypervisor[]>>("/v1/hypervisors", { params })
      .then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Hypervisor>>(`/v1/hypervisors/${id}`).then((r) => r.data.data),

  register: (payload: RegisterHypervisorPayload) =>
    api
      .post<ApiResponse<Hypervisor>>("/v1/hypervisors", payload)
      .then((r) => r.data.data),

  update: (id: string, payload: UpdateHypervisorPayload) =>
    api
      .put<ApiResponse<Hypervisor>>(`/v1/hypervisors/${id}`, payload)
      .then((r) => r.data.data),

  delete: (id: string) => api.delete(`/v1/hypervisors/${id}`),

  testConnection: (id: string) =>
    api
      .post<ApiResponse<TestConnectionResult>>(
        `/v1/hypervisors/${id}/test-connection`
      )
      .then((r) => r.data.data),

  // Alias used by some UI flows
  test: (id: string) =>
    api
      .post<ApiResponse<TestConnectionResult>>(`/v1/hypervisors/${id}/test`)
      .then((r) => r.data.data),

  syncInventory: (id: string) =>
    api
      .post<ApiResponse<{ task_id: string }>>(`/v1/hypervisors/${id}/sync`)
      .then((r) => r.data.data),
};
