import api from "./client";
import type {
  ApiResponse,
  CapacitySummary,
  CapacityTrends,
  ProviderCapacity,
  OptimizationRecommendation,
  RecommendationSummary,
  ForecastReport,
} from "@/types";

export const analyticsApi = {
  // ── Capacity ──────────────────────────────────────────────────────────────
  getCapacity: () =>
    api.get<ApiResponse<CapacitySummary>>("/v1/analytics/capacity").then((r) => r.data.data),

  getCapacityTrends: (params?: {
    hypervisor_id?: string;
    since?: string;
    granularity?: string;
  }) =>
    api
      .get<ApiResponse<CapacityTrends>>("/v1/analytics/capacity/trends", { params })
      .then((r) => r.data.data),

  getProviderCapacity: () =>
    api
      .get<ApiResponse<ProviderCapacity[]>>("/v1/analytics/capacity/providers")
      .then((r) => r.data.data ?? []),

  // ── Recommendations ───────────────────────────────────────────────────────
  getRecommendations: (params?: {
    type?: string;
    severity?: string;
    status?: string;
    hypervisor_id?: string;
    vm_id?: string;
    page?: number;
    page_size?: number;
  }) =>
    api
      .get<ApiResponse<OptimizationRecommendation[]>>("/v1/analytics/recommendations", { params })
      .then((r) => r.data),

  getRecommendationSummary: () =>
    api
      .get<ApiResponse<RecommendationSummary>>("/v1/analytics/recommendations/summary")
      .then((r) => r.data.data),

  dismissRecommendation: (id: string, note?: string) =>
    api.post(`/v1/analytics/recommendations/${id}/dismiss`, { note: note ?? "" }),

  resolveRecommendation: (id: string) =>
    api.post(`/v1/analytics/recommendations/${id}/resolve`),

  // ── Forecasting ───────────────────────────────────────────────────────────
  getForecasts: () =>
    api.get<ApiResponse<ForecastReport>>("/v1/analytics/forecasting").then((r) => r.data.data),

  // ── Manual trigger ────────────────────────────────────────────────────────
  triggerCollection: () => api.post("/v1/analytics/collect"),
};
