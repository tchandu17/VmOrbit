import api from "./client";
import type { ApiResponse, PageParams, VM, VMMetrics, ConsoleSession } from "@/types";

export const vmApi = {
  // Returns the full ApiResponse so the page can read .data (VM[]) and .meta
  list: (params?: { hypervisor_id?: string; tag_ids?: string; status?: string } & PageParams) =>
    api.get<ApiResponse<VM[]>>("/v1/vms", { params }).then((r) => r.data),

  get: (id: string) =>
    api.get<ApiResponse<VM>>(`/v1/vms/${id}`).then((r) => r.data.data),

  powerOn: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${id}/power-on`).then((r) => r.data.data),

  powerOff: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${id}/power-off`).then((r) => r.data.data),

  reboot: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${id}/reboot`).then((r) => r.data.data),

  suspend: (id: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${id}/suspend`).then((r) => r.data.data),

  delete: (id: string) =>
    api.delete(`/v1/vms/${id}`),

  getMetrics: (id: string) =>
    api.get<ApiResponse<VMMetrics>>(`/v1/vms/${id}/metrics`).then((r) => r.data.data),

  getConsole: (id: string, opts?: { type?: string; ttl_seconds?: number }) =>
    api.post<ApiResponse<ConsoleSession>>(`/v1/vms/${id}/console`, opts ?? {}).then((r) => r.data.data),

  getConsoleByToken: (token: string) =>
    api.get<ApiResponse<ConsoleSession>>(`/v1/consoles/${token}`).then((r) => r.data.data),

  createSnapshot: (id: string, payload: { name: string; description?: string; memory?: boolean }) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${id}/snapshots`, payload).then((r) => r.data.data),

  listSnapshots: (id: string) =>
    api.get<ApiResponse<import("@/types").Snapshot[]>>(`/v1/vms/${id}/snapshots`).then((r) => r.data.data ?? []),

  deleteSnapshot: (vmId: string, snapshotId: string) =>
    api.delete<ApiResponse<{ task_id: string }>>(`/v1/vms/${vmId}/snapshots/${snapshotId}`).then((r) => r.data.data),

  revertSnapshot: (vmId: string, snapshotId: string) =>
    api.post<ApiResponse<{ task_id: string }>>(`/v1/vms/${vmId}/snapshots/${snapshotId}/revert`).then((r) => r.data.data),

  getTasks: (id: string, params?: PageParams) =>
    api.get<ApiResponse<import("@/types").Task[]>>(`/v1/vms/${id}/tasks`, { params }).then((r) => r.data),

  getActivity: (id: string, params?: PageParams) =>
    api.get<ApiResponse<import("@/types").AuditLog[]>>(`/v1/vms/${id}/activity`, { params }).then((r) => r.data),

  // ── Bulk operations ──────────────────────────────────────────────────────
  bulkPowerOn: (vmIds: string[]) =>
    api.post<ApiResponse<{ task_id: string; vm_count: number }>>("/v1/vms/bulk/poweron", { vm_ids: vmIds }).then((r) => r.data.data),

  bulkPowerOff: (vmIds: string[]) =>
    api.post<ApiResponse<{ task_id: string; vm_count: number }>>("/v1/vms/bulk/poweroff", { vm_ids: vmIds }).then((r) => r.data.data),

  bulkReboot: (vmIds: string[]) =>
    api.post<ApiResponse<{ task_id: string; vm_count: number }>>("/v1/vms/bulk/reboot", { vm_ids: vmIds }).then((r) => r.data.data),

  bulkSnapshot: (vmIds: string[], name: string, description?: string, memory?: boolean) =>
    api.post<ApiResponse<{ task_id: string; vm_count: number }>>("/v1/vms/bulk/snapshot", {
      vm_ids: vmIds,
      name,
      description,
      memory: memory ?? false,
    }).then((r) => r.data.data),
};
