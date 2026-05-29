export type VMStatus =
  | "running"
  | "stopped"
  | "suspended"
  | "paused"
  | "unknown"
  | "provisioning"
  | "deleting"
  | "error";

export interface VM {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  provider_vm_id: string;
  name: string;
  description: string;
  status: VMStatus;
  cpu_count: number;
  memory_mb: number;
  disk_gb: number;
  ip_addresses: string[];
  mac_address: string;
  network_name: string;
  guest_os: string;
  guest_os_type: string;
  tools_status: string;
  tags: string[];
  tag_objects?: Tag[];
  metadata: VMMetadata;
  hypervisor?: Hypervisor;
  snapshots?: Snapshot[];
}

/** Provider-specific fields stored in the metadata JSONB column by the provider mapper. */
export interface VMMetadata extends Record<string, unknown> {
  // VMware vCenter / ESXi fields
  uuid?: string;
  instance_uuid?: string;
  mor?: string;
  esxi_host?: string;
  esxi_host_mor?: string;
  cluster?: string;
  datastore?: string;
  datastore_mor?: string;
  annotation?: string;
  vm_path?: string;
  tools_version?: string;
  change_version?: string;

  // Proxmox VE fields
  node?: string;
  vmid?: number;
  uptime?: number;
  cpu_usage?: number;
  mem_used?: number;
  disk_read?: number;
  disk_write?: number;
  net_in?: number;
  net_out?: number;
  description?: string;
  agent?: string;
}

export interface Snapshot {
  id: string;
  vm_id: string;
  provider_id: string;
  name: string;
  description: string;
  is_current: boolean;
  parent_id?: string;
  created_at: string;
}

export interface VMMetrics {
  cpu_usage_percent: number;
  memory_usage_mb: number;
  disk_read_kbps: number;
  disk_write_kbps: number;
  network_rx_kbps: number;
  network_tx_kbps: number;
}

export interface ConsoleSession {
  session_id: string;
  token: string;
  console_type?: "webmks" | "novnc" | "vnc" | "spice" | "xterm" | string;
  /** @deprecated use console_type */
  type?: string;
  url: string;
  proxy_ws_url?: string;
  /** Raw WebSocket URL for direct browser→hypervisor connection (webmks on standalone ESXi). */
  direct_ws_url?: string;
  expires_at?: string;
  provider?: string;
}

import type { Hypervisor } from "./hypervisor";
import type { Tag } from "./tag";
