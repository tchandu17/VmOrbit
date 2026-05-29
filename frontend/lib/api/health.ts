import api from "./client";
import type { ApiResponse, ProviderHealth, ProviderHealthHistory } from "@/types";

export const healthApi = {
  /** Get health snapshots for all providers */
  listAll: () =>
    api
      .get<ApiResponse<ProviderHealth[]>>("/v1/providers/health")
      .then((r) => r.data),

  /** Get health snapshot for a single provider */
  getByHypervisor: (id: string) =>
    api
      .get<ApiResponse<ProviderHealth>>(`/v1/providers/${id}/health`)
      .then((r) => r.data.data),

  /** Get latency/health history for a provider */
  getHistory: (id: string, limit = 60) =>
    api
      .get<ApiResponse<ProviderHealthHistory[]>>(
        `/v1/providers/${id}/health/history`,
        { params: { limit } }
      )
      .then((r) => r.data.data ?? []),

  /** Trigger an immediate health check */
  triggerCheck: (id: string) =>
    api
      .post<ApiResponse<ProviderHealth>>(`/v1/providers/${id}/health/check`)
      .then((r) => r.data.data),
};
