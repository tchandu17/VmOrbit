import api from "./client";
import type {
  ApiResponse,
  PageParams,
  NotificationChannel,
  NotificationRule,
  NotificationHistory,
} from "@/types";

// ── Channels ──────────────────────────────────────────────────────────────────

export interface CreateChannelRequest {
  name: string;
  type: "email" | "slack" | "webhook";
  description?: string;
  enabled?: boolean;
  config: Record<string, unknown>;
}

export interface UpdateChannelRequest {
  name?: string;
  description?: string;
  enabled?: boolean;
  config?: Record<string, unknown>;
}

export const notificationChannelApi = {
  list: () =>
    api.get<ApiResponse<NotificationChannel[]>>("/v1/notification-channels").then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<NotificationChannel>>(`/v1/notification-channels/${id}`).then((r) => r.data.data),

  create: (req: CreateChannelRequest) =>
    api.post<ApiResponse<NotificationChannel>>("/v1/notification-channels", req).then((r) => r.data.data),

  update: (id: string, req: UpdateChannelRequest) =>
    api.put<ApiResponse<NotificationChannel>>(`/v1/notification-channels/${id}`, req).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/notification-channels/${id}`),

  test: (id: string) =>
    api.post<ApiResponse<{ message: string }>>(`/v1/notification-channels/${id}/test`).then((r) => r.data),
};

// ── Rules ─────────────────────────────────────────────────────────────────────

export interface CreateRuleRequest {
  name: string;
  description?: string;
  channel_id: string;
  event_types?: string[];
  severities?: string[];
  providers?: string[];
  throttle_seconds?: number;
  enabled?: boolean;
}

export interface UpdateRuleRequest {
  name?: string;
  description?: string;
  channel_id?: string;
  event_types?: string[];
  severities?: string[];
  providers?: string[];
  throttle_seconds?: number;
  enabled?: boolean;
}

export const notificationRuleApi = {
  list: () =>
    api.get<ApiResponse<NotificationRule[]>>("/v1/notification-rules").then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<NotificationRule>>(`/v1/notification-rules/${id}`).then((r) => r.data.data),

  create: (req: CreateRuleRequest) =>
    api.post<ApiResponse<NotificationRule>>("/v1/notification-rules", req).then((r) => r.data.data),

  update: (id: string, req: UpdateRuleRequest) =>
    api.put<ApiResponse<NotificationRule>>(`/v1/notification-rules/${id}`, req).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/notification-rules/${id}`),
};

// ── History ───────────────────────────────────────────────────────────────────

export interface HistoryListParams extends PageParams {
  rule_id?: string;
  channel_id?: string;
  event_id?: string;
  status?: string;
  since?: string;
  until?: string;
}

export const notificationHistoryApi = {
  list: (params?: HistoryListParams) =>
    api.get<ApiResponse<NotificationHistory[]>>("/v1/notification-history", { params }).then((r) => r.data),
};
