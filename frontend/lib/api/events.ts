import api from "./client";
import type { ApiResponse, PageParams, PlatformEvent } from "@/types";

export interface EventListParams extends PageParams {
  event_type?: string;
  severity?: string;
  provider?: string;
  resource_type?: string;
  resource_id?: string;
  hypervisor_id?: string;
  since?: string;
  until?: string;
  search?: string;
}

export const eventApi = {
  list: (params?: EventListParams) =>
    api.get<ApiResponse<PlatformEvent[]>>("/v1/events", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<PlatformEvent>>(`/v1/events/${id}`).then((r) => r.data.data),
};
