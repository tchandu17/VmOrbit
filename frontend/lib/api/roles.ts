import api from "./client";
import type { ApiResponse, Role, Permission } from "@/types";

export const roleApi = {
  // Roles
  list: () =>
    api.get<ApiResponse<Role[]>>("/v1/roles").then((r) => r.data.data),

  get: (id: string) =>
    api.get<ApiResponse<Role>>(`/v1/roles/${id}`).then((r) => r.data.data),

  create: (payload: { name: string; description?: string; permission_ids?: string[] }) =>
    api.post<ApiResponse<Role>>("/v1/roles", payload).then((r) => r.data.data),

  update: (id: string, payload: { name?: string; description?: string }) =>
    api.put<ApiResponse<Role>>(`/v1/roles/${id}`, payload).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/roles/${id}`),

  assignPermission: (roleId: string, permissionId: string) =>
    api.post(`/v1/roles/${roleId}/permissions/${permissionId}`),

  revokePermission: (roleId: string, permissionId: string) =>
    api.delete(`/v1/roles/${roleId}/permissions/${permissionId}`),

  setPermissions: (roleId: string, permissionIds: string[]) =>
    api.put(`/v1/roles/${roleId}/permissions`, { permission_ids: permissionIds }),

  // Permissions
  listPermissions: () =>
    api.get<ApiResponse<Permission[]>>("/v1/permissions").then((r) => r.data.data),

  createPermission: (payload: { resource: string; action: string }) =>
    api.post<ApiResponse<Permission>>("/v1/permissions", payload).then((r) => r.data.data),

  deletePermission: (id: string) =>
    api.delete(`/v1/permissions/${id}`),
};
