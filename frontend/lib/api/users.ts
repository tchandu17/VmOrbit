import api from "./client";
import type { ApiResponse, PageParams, User, Permission } from "@/types";

export const userApi = {
  list: (params?: PageParams) =>
    api.get<ApiResponse<User[]>>("/v1/users", { params }).then((r) => ({
      data: r.data.data ?? [],
      total: r.data.meta?.total_items ?? 0,
      page: r.data.meta?.page ?? 1,
      page_size: r.data.meta?.page_size ?? 20,
      total_pages: r.data.meta?.total_pages ?? 0,
    })),

  get: (id: string) =>
    api.get<ApiResponse<User>>(`/v1/users/${id}`).then((r) => r.data.data),

  create: (payload: { email: string; username: string; password: string; first_name?: string; last_name?: string; role_ids?: string[] }) =>
    api.post<ApiResponse<User>>("/v1/users", payload).then((r) => r.data.data),

  update: (id: string, payload: { first_name?: string; last_name?: string; is_active?: boolean }) =>
    api.put<ApiResponse<User>>(`/v1/users/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/users/${id}`),

  assignRole: (userId: string, roleId: string) =>
    api.post(`/v1/users/${userId}/roles/${roleId}`),

  revokeRole: (userId: string, roleId: string) =>
    api.delete(`/v1/users/${userId}/roles/${roleId}`),

  changePassword: (userId: string, currentPassword: string, newPassword: string) =>
    api.put(`/v1/users/${userId}/password`, { current_password: currentPassword, new_password: newPassword }),

  getPermissions: (userId: string) =>
    api.get<ApiResponse<Permission[]>>(`/v1/users/${userId}/permissions`).then((r) => r.data.data),
};
