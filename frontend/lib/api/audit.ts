import api from "./client";
import type { ApiResponse, PageParams, AuditLog } from "@/types";

export interface AuditListParams extends PageParams {
  resource?: string;
  action?: string;
  user_id?: string;
  hypervisor_id?: string;
  resource_id?: string;
  since?: string;   // RFC3339
  until?: string;   // RFC3339
  success?: boolean;
  search?: string;
}

export const auditApi = {
  list: (params?: AuditListParams) =>
    api
      .get<ApiResponse<AuditLog[]>>("/v1/audit", { params })
      .then((r) => r.data),

  /** Returns a URL that triggers a CSV download (opened in a new tab) */
  exportUrl: (params?: Omit<AuditListParams, "page" | "page_size">) => {
    const token =
      typeof window !== "undefined"
        ? localStorage.getItem("access_token") ?? ""
        : "";
    const qs = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== "") qs.set(k, String(v));
      });
    }
    qs.set("token", token);
    return `/api/proxy/api/v1/audit/export?${qs.toString()}`;
  },
};
