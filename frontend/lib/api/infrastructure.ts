import api from "./client";
import type { ApiResponse, InfraTreeNode, Host, HostDetail, Cluster, DataStore, Network } from "@/types";

export const infrastructureApi = {
  // Infrastructure tree
  getTree: (hypervisorId?: string) =>
    api
      .get<ApiResponse<InfraTreeNode[]>>("/v1/infrastructure/tree", {
        params: hypervisorId ? { hypervisor_id: hypervisorId } : undefined,
      })
      .then((r) => r.data.data ?? []),

  // Hosts
  listHosts: (hypervisorId?: string) =>
    api
      .get<ApiResponse<Host[]>>("/v1/hosts", {
        params: hypervisorId ? { hypervisor_id: hypervisorId } : undefined,
      })
      .then((r) => r.data.data ?? []),

  getHost: (id: string) =>
    api.get<ApiResponse<HostDetail>>(`/v1/hosts/${id}`).then((r) => r.data.data),

  // Clusters
  listClusters: (hypervisorId?: string) =>
    api
      .get<ApiResponse<Cluster[]>>("/v1/clusters", {
        params: hypervisorId ? { hypervisor_id: hypervisorId } : undefined,
      })
      .then((r) => r.data.data ?? []),

  getCluster: (id: string) =>
    api.get<ApiResponse<Cluster>>(`/v1/clusters/${id}`).then((r) => r.data.data),

  // Datastores
  listDataStores: (hypervisorId?: string) =>
    api
      .get<ApiResponse<DataStore[]>>("/v1/datastores", {
        params: hypervisorId ? { hypervisor_id: hypervisorId } : undefined,
      })
      .then((r) => r.data.data ?? []),

  // Networks
  listNetworks: (hypervisorId?: string) =>
    api
      .get<ApiResponse<Network[]>>("/v1/networks", {
        params: hypervisorId ? { hypervisor_id: hypervisorId } : undefined,
      })
      .then((r) => r.data.data ?? []),
};
