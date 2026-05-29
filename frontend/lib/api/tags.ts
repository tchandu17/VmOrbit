import api from "./client";
import type { ApiResponse, Tag } from "@/types";

export const tagApi = {
  /** List all tags */
  list: () =>
    api.get<ApiResponse<Tag[]>>("/v1/tags").then((r) => r.data.data ?? []),

  /** Get a single tag */
  get: (id: string) =>
    api.get<ApiResponse<Tag>>(`/v1/tags/${id}`).then((r) => r.data.data),

  /** Create a new tag */
  create: (payload: { name: string; color?: string; description?: string }) =>
    api.post<ApiResponse<Tag>>("/v1/tags", payload).then((r) => r.data.data),

  /** Update a tag */
  update: (id: string, payload: { name?: string; color?: string; description?: string }) =>
    api.put<ApiResponse<Tag>>(`/v1/tags/${id}`, payload).then((r) => r.data.data),

  /** Delete a tag */
  delete: (id: string) => api.delete(`/v1/tags/${id}`),

  /** List tags attached to a VM */
  listByVM: (vmId: string) =>
    api.get<ApiResponse<Tag[]>>(`/v1/vms/${vmId}/tags`).then((r) => r.data.data ?? []),

  /** Attach a tag to a VM */
  addToVM: (vmId: string, tagId: string) =>
    api.post(`/v1/vms/${vmId}/tags`, { tag_id: tagId }),

  /** Detach a tag from a VM */
  removeFromVM: (vmId: string, tagId: string) =>
    api.delete(`/v1/vms/${vmId}/tags/${tagId}`),
};
