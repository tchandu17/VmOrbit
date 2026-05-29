import type { Hypervisor } from "./hypervisor";
import type { VM } from "./vm";

// ── Infrastructure hierarchy types ────────────────────────────────────────────

export interface Cluster {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  provider_id: string;
  name: string;
  total_cpu: number;
  total_memory_mb: number;
  host_count: number;
  vm_count: number;
  metadata: Record<string, unknown>;
  hypervisor?: Hypervisor;
  hosts?: Host[];
}

export interface Host {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  cluster_id?: string;
  provider_id: string;
  name: string;
  status: "connected" | "disconnected" | "maintenance" | "unknown";
  // Compute
  cpu_model: string;
  cpu_sockets: number;
  cpu_cores: number;
  cpu_threads: number;
  cpu_usage_m_hz: number;
  total_memory_mb: number;
  used_memory_mb: number;
  // Info
  hypervisor_version: string;
  uptime_seconds: number;
  vm_count: number;
  metadata: Record<string, unknown>;
  hypervisor?: Hypervisor;
  cluster?: Cluster;
}

export interface HostDetail {
  host: Host;
  vms: VM[];
  datastores: DataStore[];
  networks: Network[];
}

export interface DataStore {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  provider_id: string;
  name: string;
  type: string;
  capacity_gb: number;
  used_gb: number;
  free_gb: number;
  accessible: boolean;
}

export interface Network {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  provider_id: string;
  name: string;
  type: string;
  vlan: number;
  accessible: boolean;
}

// ── Infrastructure tree ───────────────────────────────────────────────────────

export type InfraNodeType = "provider" | "cluster" | "host" | "vm";

export interface InfraTreeNode {
  id: string;
  type: InfraNodeType;
  name: string;
  status: string;
  provider_type?: string;
  vm_count: number;
  children?: InfraTreeNode[];
  metadata?: Record<string, unknown>;
}
