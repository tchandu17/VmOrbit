import api from "./client";
import type { ApiResponse, PageParams, Task, TaskLog } from "@/types";

export const taskApi = {
  list: (params?: PageParams) =>
    api.get<ApiResponse<Task[]>>("/v1/tasks", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<Task>>(`/v1/tasks/${id}`).then((r) => r.data.data),

  cancel: (id: string) =>
    api.delete(`/v1/tasks/${id}`),

  getLogs: (id: string, params?: PageParams) =>
    api.get<ApiResponse<TaskLog[]>>(`/v1/tasks/${id}/logs`, { params }).then((r) => r.data),
};
