export interface VMTemplate {
  id: string;
  created_at: string;
  updated_at: string;
  hypervisor_id: string;
  provider_id: string;
  name: string;
  description: string;
  guest_os: string;
  cpu_count: number;
  memory_mb: number;
  disk_gb: number;
  tags: string[];
  metadata: Record<string, unknown>;
  hypervisor?: import("./hypervisor").Hypervisor;
}

export type ProvisioningJobStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "cancelled";

export type ProvisioningJobType = "clone" | "provision";

export interface ProvisioningJob {
  id: string;
  created_at: string;
  updated_at: string;
  type: ProvisioningJobType;
  status: ProvisioningJobStatus;
  template_id?: string;
  source_vm_id?: string;
  hypervisor_id: string;
  vm_name: string;
  cpu_count: number;
  memory_mb: number;
  disk_gb: number;
  network_name: string;
  data_store: string;
  node: string;
  linked: boolean;
  tags: string[];
  result_vm_id?: string;
  task_id?: string;
  error_message?: string;
  created_by?: string;
  metadata?: Record<string, unknown>;
  template?: VMTemplate;
  source_vm?: import("./vm").VM;
}

export interface CloneVMPayload {
  source_vm_id: string;
  name: string;
  data_store?: string;
  node?: string;
  linked?: boolean;
  tags?: string[];
}

export interface ProvisionVMPayload {
  template_id: string;
  name: string;
  cpu_count?: number;
  memory_mb?: number;
  disk_gb?: number;
  network_name?: string;
  data_store?: string;
  node?: string;
  tags?: string[];
}
