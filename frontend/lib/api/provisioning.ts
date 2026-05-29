import api from "./client";
import type {
  ApiResponse,
  PageParams,
  ProvisioningJob,
  CloneVMPayload,
  ProvisionVMPayload,
} from "@/types";

export const provisioningApi = {
  // Clone an existing VM
  clone: (payload: CloneVMPayload) =>
    api
      .post<ApiResponse<ProvisioningJob>>("/v1/vms/clone", payload)
      .then((r) => r.data.data),

  // Provision a VM from a template
  provision: (payload: ProvisionVMPayload) =>
    api
      .post<ApiResponse<ProvisioningJob>>("/v1/vms/provision", payload)
      .then((r) => r.data.data),

  // List provisioning jobs
  listJobs: (
    params?: { hypervisor_id?: string; type?: string; status?: string } & PageParams
  ) =>
    api
      .get<ApiResponse<ProvisioningJob[]>>("/v1/provisioning-jobs", { params })
      .then((r) => r.data),

  // Get a single provisioning job
  getJob: (id: string) =>
    api
      .get<ApiResponse<ProvisioningJob>>(`/v1/provisioning-jobs/${id}`)
      .then((r) => r.data.data),
};
