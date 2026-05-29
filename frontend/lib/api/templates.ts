import api from "./client";
import type { ApiResponse, PageParams, VMTemplate } from "@/types";

export const templateApi = {
  list: (params?: { hypervisor_id?: string } & PageParams) =>
    api.get<ApiResponse<VMTemplate[]>>("/v1/templates", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<VMTemplate>>(`/v1/templates/${id}`).then((r) => r.data.data),

  syncTemplates: (hypervisorId: string) =>
    api
      .post<ApiResponse<{ task_id: string }>>(`/v1/hypervisors/${hypervisorId}/templates/sync`)
      .then((r) => r.data.data),
};
