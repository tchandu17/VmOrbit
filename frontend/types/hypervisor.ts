export type ProviderType = "vmware" | "esxi" | "proxmox" | "nutanix" | "kvm" | "hyperv";
export type ConnectionStatus = "connected" | "disconnected" | "error" | "unknown";

export interface Hypervisor {
  id: string;
  created_at: string;
  updated_at: string;
  group_id?: string;
  name: string;
  description: string;
  provider: ProviderType;
  host: string;
  port: number;
  username: string;
  tls_verify: boolean;
  connection_status: ConnectionStatus;
  last_checked_at?: string;
  tags: string[];
  metadata: Record<string, unknown>;
  // VM count is populated from the VMs relation length when preloaded
  vm_count?: number;
}

export interface HypervisorGroup {
  id: string;
  name: string;
  description: string;
  tags: string[];
}

// ── Registration payloads ─────────────────────────────────────────────────────

export interface RegisterHypervisorPayload {
  name: string;
  description?: string;
  provider: ProviderType;
  host: string;
  port: number;
  username?: string;
  password?: string;
  token?: string;
  tls_verify: boolean;
  tags?: string[];
  // VMware-specific
  vcenter_url?: string;
  datacenter?: string;
  // Proxmox-specific
  node?: string;
  api_token_id?: string;
  api_token_secret?: string;
  // Nutanix-specific
  prism_type?: "element" | "central"; // defaults to "element"
}

export interface UpdateHypervisorPayload {
  name?: string;
  description?: string;
  host?: string;
  port?: number;
  username?: string;
  password?: string;
  token?: string;
  tls_verify?: boolean;
  tags?: string[];
  // VMware-specific
  vcenter_url?: string;
  datacenter?: string;
  // Proxmox-specific
  node?: string;
  api_token_id?: string;
  api_token_secret?: string;
  // Nutanix-specific
  prism_type?: "element" | "central";
}

export interface TestConnectionResult {
  connected: boolean;
  error?: string;
}
