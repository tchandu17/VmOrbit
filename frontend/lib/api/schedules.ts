import api from "./client";
import type {
  ApiResponse,
  PageParams,
  Schedule,
  ScheduleExecution,
  ScheduleOperationType,
  ScheduleTargetType,
  ScheduleType,
} from "@/types";

export interface CreateSchedulePayload {
  name: string;
  description?: string;
  operation_type: ScheduleOperationType;
  target_type: ScheduleTargetType;
  target_ids: string[];
  schedule_type: ScheduleType;
  cron_expression?: string;
  timezone?: string;
  enabled: boolean;
  max_runs?: number;
  expires_at?: string;
  payload?: Record<string, unknown>;
}

export const scheduleApi = {
  list: (params?: { operation_type?: string; target_type?: string; enabled?: boolean; status?: string } & PageParams) =>
    api.get<ApiResponse<Schedule[]>>("/v1/schedules", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Schedule>>(`/v1/schedules/${id}`).then((r) => r.data.data),

  create: (payload: CreateSchedulePayload) =>
    api.post<ApiResponse<Schedule>>("/v1/schedules", payload).then((r) => r.data.data),

  update: (id: string, payload: Partial<CreateSchedulePayload>) =>
    api.put<ApiResponse<Schedule>>(`/v1/schedules/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/schedules/${id}`),

  enable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/schedules/${id}/enable`).then((r) => r.data),

  disable: (id: string) =>
    api.post<ApiResponse<{ enabled: boolean }>>(`/v1/schedules/${id}/disable`).then((r) => r.data),

  triggerNow: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/schedules/${id}/trigger`).then((r) => r.data.data),

  listExecutions: (id: string, params?: PageParams) =>
    api.get<ApiResponse<ScheduleExecution[]>>(`/v1/schedules/${id}/executions`, { params }).then((r) => r.data),
};
