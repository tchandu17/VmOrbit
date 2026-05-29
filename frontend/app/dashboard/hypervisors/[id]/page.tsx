"use client";
import { useState, useCallback, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import {
  ArrowLeft, ChevronRight, RefreshCw, Plug, RotateCcw, Edit2, Trash2,
  Server, Cpu, MemoryStick, HardDrive, Network, Activity, Info,
  CheckCircle2, XCircle, AlertTriangle, Loader2, X, Clock,
  Shield, Globe, Database, Tag as TagIcon, ExternalLink,
} from "lucide-react";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { vmApi } from "@/lib/api/vms";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import { HypervisorStatusBadge } from "@/components/hypervisors/HypervisorStatusBadge";
import { VMStatusBadge } from "@/components/vms/VMStatusBadge";
import { VMToolsBadge } from "@/components/vms/VMToolsBadge";
import { SyncProgressPanel, useSyncProgress } from "@/components/inventory/SyncProgressPanel";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { wsClient } from "@/lib/ws/WSClient";
import { cn, relativeTime, formatDate, formatBytes } from "@/lib/utils";
import { taskApi } from "@/lib/api/tasks";
import { infrastructureApi } from "@/lib/api/infrastructure";
import type { Hypervisor, VM, UpdateHypervisorPayload, ProviderType, Task, Host, Cluster, DataStore } from "@/types";

// ── Types ─────────────────────────────────────────────────────────────────────

type Tab = "overview" | "hardware" | "vms" | "tasks" | "settings";

// ── Toast ─────────────────────────────────────────────────────────────────────

type ToastType = "success" | "error" | "info";
interface Toast { id: number; type: ToastType; message: string; }
let toastCounter = 0;

function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const addToast = useCallback((type: ToastType, message: string) => {
    const id = ++toastCounter;
    setToasts((p) => [...p, { id, type, message }]);
    setTimeout(() => setToasts((p) => p.filter((t) => t.id !== id)), 4000);
  }, []);
  const removeToast = useCallback((id: number) => setToasts((p) => p.filter((t) => t.id !== id)), []);
  return { toasts, addToast, removeToast };
}

function ToastContainer({ toasts, onRemove }: { toasts: Toast[]; onRemove: (id: number) => void }) {
  if (!toasts.length) return null;
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((t) => (
        <div key={t.id} className={cn(
          "flex items-start gap-3 px-4 py-3 rounded-xl shadow-lg border text-sm animate-in slide-in-from-right-5",
          t.type === "success" ? "bg-green-950 border-green-800 text-green-200"
            : t.type === "error" ? "bg-red-950 border-red-800 text-red-200"
            : "bg-gray-800 border-gray-700 text-gray-200"
        )}>
          {t.type === "success" ? <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0 mt-0.5" />
            : t.type === "error" ? <XCircle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" /> : null}
          <span className="flex-1">{t.message}</span>
          <button onClick={() => onRemove(t.id)} className="text-gray-500 hover:text-gray-300">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      ))}
    </div>
  );
}

// ── Shared UI primitives ──────────────────────────────────────────────────────

function InfoRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2.5 border-b border-gray-800/60 last:border-0">
      <span className="text-xs text-gray-500 shrink-0 w-40">{label}</span>
      <span className={cn("text-sm text-gray-200 text-right break-all", mono && "font-mono text-xs")}>
        {value ?? "—"}
      </span>
    </div>
  );
}

function SectionCard({ title, icon: Icon, children, className }: {
  title: string; icon: React.ElementType; children: React.ReactNode; className?: string;
}) {
  return (
    <div className={cn("bg-gray-900 border border-gray-800 rounded-2xl p-5", className)}>
      <div className="flex items-center gap-2 mb-4">
        <Icon className="w-4 h-4 text-blue-400" />
        <h3 className="text-sm font-semibold text-white">{title}</h3>
      </div>
      {children}
    </div>
  );
}

function Skeleton({ className }: { className?: string }) {
  return <div className={cn("bg-gray-800 rounded animate-pulse", className)} />;
}

const TASK_STATUS_BADGE: Record<string, string> = {
  pending:   "bg-gray-700 text-gray-300",
  queued:    "bg-blue-900/50 text-blue-300",
  running:   "bg-blue-600/20 text-blue-400",
  completed: "bg-green-900/50 text-green-300",
  failed:    "bg-red-900/50 text-red-300",
  cancelled: "bg-gray-700 text-gray-400",
  retrying:  "bg-yellow-900/50 text-yellow-300",
  timed_out: "bg-orange-900/50 text-orange-300",
};

// ── Tab: Overview ─────────────────────────────────────────────────────────────

function OverviewTab({ hypervisor, vmCount }: { hypervisor: Hypervisor; vmCount: number }) {
  const meta = hypervisor.metadata ?? {};
  const isVMware  = hypervisor.provider === "vmware" || hypervisor.provider === "esxi";
  const isProxmox = hypervisor.provider === "proxmox";

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      {/* Connection Details */}
      <SectionCard title="Connection Details" icon={Globe}>
        <InfoRow label="Provider"          value={<ProviderBadge provider={hypervisor.provider} />} />
        <InfoRow label="Status"            value={<HypervisorStatusBadge status={hypervisor.connection_status} />} />
        <InfoRow label="Host"              value={hypervisor.host} mono />
        <InfoRow label="Port"              value={String(hypervisor.port)} />
        <InfoRow label="Username"          value={hypervisor.username || "—"} />
        <InfoRow label="TLS Verify"        value={hypervisor.tls_verify ? "Enabled" : "Disabled"} />
        <InfoRow label="Last Checked"      value={hypervisor.last_checked_at ? relativeTime(hypervisor.last_checked_at) : "Never"} />
        <InfoRow label="Registered"        value={formatDate(hypervisor.created_at)} />
        <InfoRow label="Last Updated"      value={relativeTime(hypervisor.updated_at)} />
      </SectionCard>

      {/* Summary Stats */}
      <SectionCard title="Summary" icon={Activity}>
        <div className="grid grid-cols-2 gap-3 mb-4">
          <div className="bg-gray-800/60 rounded-xl p-4 text-center">
            <p className="text-2xl font-bold text-white">{vmCount}</p>
            <p className="text-xs text-gray-500 mt-1">Virtual Machines</p>
          </div>
          <div className="bg-gray-800/60 rounded-xl p-4 text-center">
            <p className={cn("text-2xl font-bold",
              hypervisor.connection_status === "connected" ? "text-green-400"
              : hypervisor.connection_status === "error" ? "text-red-400"
              : "text-gray-400")}>
              {hypervisor.connection_status === "connected" ? "●" : hypervisor.connection_status === "error" ? "✕" : "○"}
            </p>
            <p className="text-xs text-gray-500 mt-1 capitalize">{hypervisor.connection_status}</p>
          </div>
        </div>
        {hypervisor.description && (
          <div className="p-3 bg-gray-800/40 rounded-xl">
            <p className="text-xs text-gray-500 mb-1">Description</p>
            <p className="text-sm text-gray-300">{hypervisor.description}</p>
          </div>
        )}
        {(hypervisor.tags ?? []).length > 0 && (
          <div className="mt-3">
            <p className="text-xs text-gray-500 mb-2">Tags</p>
            <div className="flex flex-wrap gap-1.5">
              {hypervisor.tags.map((t) => (
                <span key={t} className="px-2 py-0.5 rounded-md bg-gray-800 text-gray-300 text-xs">{t}</span>
              ))}
            </div>
          </div>
        )}
      </SectionCard>

      {/* VMware-specific */}
      {isVMware && (
        <SectionCard title="VMware Details" icon={Server}>
          {!!meta.vcenter_url && <InfoRow label="vCenter URL"  value={String(meta.vcenter_url)} mono />}
          {!!meta.datacenter  && <InfoRow label="Datacenter"   value={String(meta.datacenter)} />}
          <InfoRow label="Provider Type" value={hypervisor.provider === "esxi" ? "Standalone ESXi" : "vCenter"} />
        </SectionCard>
      )}

      {/* Proxmox-specific */}
      {isProxmox && (
        <SectionCard title="Proxmox Details" icon={Server}>
          {!!meta.node         && <InfoRow label="Node"         value={String(meta.node)} />}
          {!!meta.api_token_id && <InfoRow label="API Token ID" value={String(meta.api_token_id)} mono />}
        </SectionCard>
      )}

      {/* Internal IDs */}
      <SectionCard title="Internal IDs" icon={Info}>
        <InfoRow label="Hypervisor ID" value={hypervisor.id} mono />
        {hypervisor.group_id && <InfoRow label="Group ID" value={hypervisor.group_id} mono />}
        <InfoRow label="Created At"    value={formatDate(hypervisor.created_at)} />
        <InfoRow label="Updated At"    value={formatDate(hypervisor.updated_at)} />
      </SectionCard>
    </div>
  );
}

// ── Tab: Hardware ─────────────────────────────────────────────────────────────

function formatMB(mb: number): string {
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(1)} TB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}
function formatGB(gb: number): string {
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`;
  return `${gb.toFixed(1)} GB`;
}
function formatUptimeSec(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function HwUsageBar({ used, total, label }: { used: number; total: number; label: string }) {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0;
  const color = pct > 85 ? "bg-red-500" : pct > 65 ? "bg-yellow-500" : "bg-blue-500";
  return (
    <div className="space-y-1.5">
      <div className="flex justify-between text-xs text-gray-400">
        <span>{label}</span>
        <span>{pct.toFixed(1)}%</span>
      </div>
      <div className="h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div className={cn("h-full rounded-full transition-all duration-700", color)} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function HwInfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 border-b border-gray-800/50 last:border-0">
      <span className="text-xs text-gray-500 shrink-0">{label}</span>
      <span className={cn("text-xs text-gray-200 text-right", mono && "font-mono")}>{value || "—"}</span>
    </div>
  );
}

function HwSectionCard({ title, icon: Icon, children }: {
  title: string; icon: React.ElementType; children: React.ReactNode;
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <div className="flex items-center gap-2 px-5 py-3.5 border-b border-gray-800">
        <Icon className="w-4 h-4 text-gray-400" />
        <h3 className="text-sm font-semibold text-white">{title}</h3>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

function HardwareTab({ hypervisorId }: { hypervisorId: string }) {
  const { data: hosts = [], isLoading: hostsLoading } = useQuery({
    queryKey: ["hypervisor-hosts", hypervisorId],
    queryFn: () => infrastructureApi.listHosts(hypervisorId),
  });
  const { data: clusters = [], isLoading: clustersLoading } = useQuery({
    queryKey: ["hypervisor-clusters", hypervisorId],
    queryFn: () => infrastructureApi.listClusters(hypervisorId),
  });
  const { data: datastores = [], isLoading: dsLoading } = useQuery({
    queryKey: ["hypervisor-datastores", hypervisorId],
    queryFn: () => infrastructureApi.listDataStores(hypervisorId),
  });
  const { data: networks = [], isLoading: netLoading } = useQuery({
    queryKey: ["hypervisor-networks", hypervisorId],
    queryFn: () => infrastructureApi.listNetworks(hypervisorId),
  });

  const isLoading = hostsLoading || clustersLoading || dsLoading || netLoading;

  if (isLoading) {
    return (
      <div className="space-y-5">
        {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-48 rounded-2xl" />)}
      </div>
    );
  }

  const noData = hosts.length === 0 && datastores.length === 0 && networks.length === 0;
  if (noData) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-4 bg-gray-900 border border-gray-800 rounded-2xl">
        <div className="w-14 h-14 rounded-2xl bg-gray-800 flex items-center justify-center">
          <Cpu className="w-7 h-7 text-gray-600" />
        </div>
        <div className="text-center">
          <p className="text-white font-semibold">No hardware data yet</p>
          <p className="text-gray-500 text-sm mt-1">Run an inventory sync to discover hosts, storage, and networks.</p>
        </div>
        <div className="flex items-center gap-1.5 text-xs text-gray-600 bg-gray-800/60 px-3 py-2 rounded-lg">
          <RotateCcw className="w-3 h-3" /> Use the Sync Inventory button above
        </div>
      </div>
    );
  }

  const totalDsGB = datastores.reduce((s: number, d: DataStore) => s + d.capacity_gb, 0);
  const usedDsGB  = datastores.reduce((s: number, d: DataStore) => s + d.used_gb, 0);

  return (
    <div className="space-y-5">

      {/* ── Per-host cards ────────────────────────────────────────────────── */}
      {hosts.map((host: Host) => {
        const statusColors: Record<string, string> = {
          connected:    "text-green-400",
          disconnected: "text-red-400",
          maintenance:  "text-yellow-400",
          unknown:      "text-gray-500",
        };

        return (
          <div key={host.id} className="space-y-4">
            {/* Quick-stat tiles */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              {[
                { label: "Total VMs",  value: host.vm_count,                                                                  icon: Server,  color: "text-blue-400",   bg: "bg-blue-900/20"   },
                { label: "CPU Cores",  value: host.cpu_cores || "—",                                                          icon: Cpu,     color: "text-purple-400", bg: "bg-purple-900/20" },
                { label: "Total RAM",  value: host.total_memory_mb > 0 ? formatMB(host.total_memory_mb) : "—",                icon: MemoryStick, color: "text-teal-400",   bg: "bg-teal-900/20"   },
                { label: "Uptime",     value: host.uptime_seconds > 0 ? formatUptimeSec(host.uptime_seconds) : "—",           icon: Clock,   color: "text-orange-400", bg: "bg-orange-900/20" },
              ].map((s) => (
                <div key={s.label} className={cn("border border-gray-800 rounded-xl p-4 flex items-center gap-3", s.bg)}>
                  <s.icon className={cn("w-5 h-5 shrink-0", s.color)} />
                  <div>
                    <p className="text-xl font-bold text-white">{s.value}</p>
                    <p className="text-xs text-gray-500">{s.label}</p>
                  </div>
                </div>
              ))}
            </div>

            {/* General + Compute side by side */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {/* General */}
              <HwSectionCard title="General" icon={Globe}>
                <HwInfoRow label="Hostname"   value={host.name} mono />
                <HwInfoRow label="Provider ID" value={host.provider_id} mono />
                <HwInfoRow label="Status"     value={host.status} />
                {host.cluster?.name && <HwInfoRow label="Cluster" value={host.cluster.name} />}
                {host.hypervisor_version && <HwInfoRow label="Version" value={host.hypervisor_version} />}
                {host.uptime_seconds > 0 && <HwInfoRow label="Uptime" value={formatUptimeSec(host.uptime_seconds)} />}
              </HwSectionCard>

              {/* Compute */}
              <HwSectionCard title="Compute" icon={Cpu}>
                {host.cpu_model && <HwInfoRow label="CPU Model" value={host.cpu_model} />}
                <div className="grid grid-cols-3 gap-3 py-3">
                  {[
                    { label: "Sockets", value: host.cpu_sockets },
                    { label: "Cores",   value: host.cpu_cores   },
                    { label: "Threads", value: host.cpu_threads  },
                  ].map((c) => c.value > 0 ? (
                    <div key={c.label} className="text-center bg-gray-800/50 rounded-lg p-2.5">
                      <p className="text-lg font-bold text-white">{c.value}</p>
                      <p className="text-xs text-gray-500">{c.label}</p>
                    </div>
                  ) : null)}
                </div>
                {host.total_memory_mb > 0 && (
                  <HwUsageBar
                    used={host.used_memory_mb}
                    total={host.total_memory_mb}
                    label={`Memory: ${formatMB(host.used_memory_mb)} / ${formatMB(host.total_memory_mb)}`}
                  />
                )}
              </HwSectionCard>
            </div>
          </div>
        );
      })}

      {/* ── Clusters ─────────────────────────────────────────────────────── */}
      {clusters.length > 0 && (
        <HwSectionCard title={`Clusters (${clusters.length})`} icon={Network}>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase tracking-wider">
                  <th className="text-left py-2 pr-4 font-medium">Name</th>
                  <th className="text-left py-2 pr-4 font-medium">Hosts</th>
                  <th className="text-left py-2 pr-4 font-medium">VMs</th>
                  <th className="text-left py-2 pr-4 font-medium">Total CPU</th>
                  <th className="text-left py-2 font-medium">Total Memory</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/60">
                {clusters.map((c: Cluster) => (
                  <tr key={c.id} className="hover:bg-gray-800/30 transition-colors">
                    <td className="py-2.5 pr-4 font-medium text-white">{c.name}</td>
                    <td className="py-2.5 pr-4 text-gray-300">{c.host_count}</td>
                    <td className="py-2.5 pr-4 text-gray-300">{c.vm_count}</td>
                    <td className="py-2.5 pr-4 text-gray-300">{c.total_cpu > 0 ? `${c.total_cpu} MHz` : "—"}</td>
                    <td className="py-2.5 text-gray-300">{c.total_memory_mb > 0 ? formatMB(c.total_memory_mb) : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </HwSectionCard>
      )}

      {/* ── Storage + Networks side by side ──────────────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">

        {/* Storage */}
        {datastores.length > 0 && (
          <HwSectionCard title={`Storage (${datastores.length} datastores)`} icon={HardDrive}>
            {totalDsGB > 0 && (
              <div className="mb-4">
                <HwUsageBar
                  used={usedDsGB}
                  total={totalDsGB}
                  label={`Total: ${formatGB(usedDsGB)} / ${formatGB(totalDsGB)}`}
                />
              </div>
            )}
            <div className="space-y-2">
              {datastores.map((ds: DataStore) => (
                <div key={ds.id} className="flex items-center gap-3 p-2.5 bg-gray-800/50 rounded-lg">
                  <Database className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-gray-200 truncate">{ds.name}</p>
                    <p className="text-[10px] text-gray-500">{ds.type}</p>
                  </div>
                  <div className="text-right shrink-0">
                    <p className="text-xs text-gray-300">{formatGB(ds.free_gb)} free</p>
                    <p className="text-[10px] text-gray-500">{formatGB(ds.capacity_gb)} total</p>
                  </div>
                  <div className={cn("w-2 h-2 rounded-full shrink-0", ds.accessible ? "bg-green-400" : "bg-red-400")} />
                </div>
              ))}
            </div>
          </HwSectionCard>
        )}

        {/* Networks */}
        {networks.length > 0 && (
          <HwSectionCard title={`Networks (${networks.length})`} icon={Network}>
            <div className="space-y-2">
              {networks.map((net: Network) => (
                <div key={net.id} className="flex items-center gap-3 p-2.5 bg-gray-800/50 rounded-lg">
                  <Network className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-gray-200 truncate">{net.name}</p>
                    <p className="text-[10px] text-gray-500">
                      {net.type === "DistributedVirtualPortgroup" ? "DVPortgroup" : net.type || "Network"}
                      {net.vlan > 0 ? ` · VLAN ${net.vlan}` : ""}
                    </p>
                  </div>
                  <div className={cn("w-2 h-2 rounded-full shrink-0", net.accessible ? "bg-green-400" : "bg-red-400")} />
                </div>
              ))}
            </div>
          </HwSectionCard>
        )}

      </div>
    </div>
  );
}

// ── Tab: VMs ──────────────────────────────────────────────────────────────────

function VMsTab({ hypervisorId }: { hypervisorId: string }) {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [page, setPage] = useState(1);
  const PAGE_SIZE = 20;

  const { data, isLoading } = useQuery({
    queryKey: ["hypervisor-vms", hypervisorId, page],
    queryFn: () => vmApi.list({ hypervisor_id: hypervisorId, page, page_size: PAGE_SIZE }),
    refetchInterval: 30_000,
  });

  const allVMs: VM[] = data?.data ?? [];
  const totalPages = data?.meta?.total_pages ?? 1;
  const totalItems = data?.meta?.total_items ?? 0;

  const filtered = allVMs.filter((vm) => {
    const q = search.toLowerCase();
    const matchSearch = !q || vm.name.toLowerCase().includes(q)
      || (vm.ip_addresses ?? []).some((ip) => ip.includes(q))
      || String(vm.metadata?.mor ?? "").toLowerCase().includes(q)
      || String(vm.metadata?.uuid ?? "").toLowerCase().includes(q);
    const matchStatus = statusFilter === "all" || vm.status === statusFilter;
    return matchSearch && matchStatus;
  });

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <div className="relative flex-1 min-w-[200px]">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name, IP, MOR, UUID…"
            className="w-full pl-4 pr-4 py-2 bg-gray-900 border border-gray-800 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
        >
          <option value="all">All statuses</option>
          {["running", "stopped", "suspended", "paused", "error", "unknown"].map((s) => (
            <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>
          ))}
        </select>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="px-5 py-3 border-b border-gray-800 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-white">Virtual Machines</h3>
          <span className="text-xs text-gray-500">{totalItems} total</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase tracking-wider">
                <th className="text-left px-5 py-3 font-medium">Name</th>
                <th className="text-left px-5 py-3 font-medium">Status</th>
                <th className="text-left px-5 py-3 font-medium">vCPU</th>
                <th className="text-left px-5 py-3 font-medium">RAM</th>
                <th className="text-left px-5 py-3 font-medium">Disk</th>
                <th className="text-left px-5 py-3 font-medium">IP Address</th>
                <th className="text-left px-5 py-3 font-medium">Cluster / Host</th>
                <th className="text-left px-5 py-3 font-medium">OS</th>
                <th className="text-left px-5 py-3 font-medium">Tools</th>
                <th className="text-right px-5 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 10 }).map((_, j) => (
                      <td key={j} className="px-5 py-4">
                        <Skeleton className="h-4 w-20" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-5 py-16 text-center text-gray-500">
                    <Server className="w-10 h-10 mx-auto mb-3 opacity-30" />
                    {search || statusFilter !== "all"
                      ? "No VMs match your filters."
                      : "No VMs found for this hypervisor. Try syncing inventory."}
                  </td>
                </tr>
              ) : (
                filtered.map((vm) => {
                  const cluster = String(vm.metadata?.cluster ?? vm.metadata?.node ?? "");
                  const host    = String(vm.metadata?.esxi_host ?? "");
                  const location = cluster && host ? `${cluster} / ${host}` : cluster || host || "—";
                  return (
                    <tr key={vm.id} className="hover:bg-gray-800/40 transition-colors group">
                      <td className="px-5 py-3.5">
                        <div className="font-medium text-white">{vm.name}</div>
                        {vm.metadata?.uuid && (
                          <div className="text-[10px] text-gray-600 font-mono mt-0.5 truncate max-w-[160px]">
                            {String(vm.metadata.uuid)}
                          </div>
                        )}
                      </td>
                      <td className="px-5 py-3.5"><VMStatusBadge status={vm.status} /></td>
                      <td className="px-5 py-3.5 text-gray-300">{vm.cpu_count}</td>
                      <td className="px-5 py-3.5 text-gray-300">{formatBytes(vm.memory_mb)}</td>
                      <td className="px-5 py-3.5 text-gray-300">{vm.disk_gb > 0 ? `${vm.disk_gb} GB` : "—"}</td>
                      <td className="px-5 py-3.5 font-mono text-xs text-gray-300">
                        {vm.ip_addresses?.[0] ?? "—"}
                        {(vm.ip_addresses?.length ?? 0) > 1 && (
                          <span className="text-gray-600 ml-1">+{vm.ip_addresses!.length - 1}</span>
                        )}
                      </td>
                      <td className="px-5 py-3.5 text-xs text-gray-400 max-w-[160px] truncate">{location}</td>
                      <td className="px-5 py-3.5 text-xs text-gray-400 truncate max-w-[120px]">{vm.guest_os || "—"}</td>
                      <td className="px-5 py-3.5"><VMToolsBadge status={vm.tools_status} /></td>
                      <td className="px-5 py-3.5 text-right">
                        <Link
                          href={`/dashboard/vms/${vm.id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300 hover:text-white text-xs transition-colors"
                        >
                          <ExternalLink className="w-3 h-3" /> View
                        </Link>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-5 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {totalPages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
                Previous
              </button>
              <button disabled={page === totalPages} onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Tab: Tasks ────────────────────────────────────────────────────────────────

function TasksTab({ hypervisorId }: { hypervisorId: string }) {
  const [page, setPage] = useState(1);
  const { data, isLoading } = useQuery({
    queryKey: ["hypervisor-tasks", hypervisorId, page],
    queryFn: () => taskApi.list({ page, page_size: 20, hypervisor_id: hypervisorId }),
  });

  const tasks: Task[] = data?.data ?? [];
  const meta = data?.meta;

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-white">Task History</h3>
        {meta && <span className="text-xs text-gray-500">{meta.total_items} total</span>}
      </div>
      {isLoading ? (
        <div className="p-5 space-y-3">
          {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      ) : tasks.length === 0 ? (
        <div className="py-12 text-center text-gray-500 text-sm">No tasks found for this hypervisor</div>
      ) : (
        <div className="divide-y divide-gray-800/60">
          {tasks.map((task) => (
            <div key={task.id} className="px-5 py-4 hover:bg-gray-800/30 transition-colors">
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-center gap-2 min-w-0">
                  {task.status === "completed" ? <CheckCircle2 className="w-3.5 h-3.5 text-green-400 shrink-0" />
                    : task.status === "failed" || task.status === "timed_out" ? <XCircle className="w-3.5 h-3.5 text-red-400 shrink-0" />
                    : task.status === "running" ? <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin shrink-0" />
                    : <Clock className="w-3.5 h-3.5 text-gray-400 shrink-0" />}
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-white">{task.type.replace(/\./g, " › ")}</p>
                    <p className="text-xs text-gray-500 font-mono mt-0.5">{task.id.slice(0, 8)}…</p>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <span className={cn("text-xs px-2 py-0.5 rounded-full", TASK_STATUS_BADGE[task.status] ?? "bg-gray-700 text-gray-300")}>
                    {task.status}
                  </span>
                  <span className="text-xs text-gray-500">{relativeTime(task.created_at)}</span>
                </div>
              </div>
              {task.error_message && (
                <p className="text-xs text-red-400 bg-red-500/10 rounded-lg px-3 py-2 mt-2 break-words">{task.error_message}</p>
              )}
              {task.status === "running" && (
                <div className="mt-2 h-1 bg-gray-800 rounded-full overflow-hidden">
                  <div className="h-full bg-blue-500 rounded-full transition-all duration-500" style={{ width: `${task.progress}%` }} />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
      {meta && meta.total_pages > 1 && (
        <div className="flex items-center justify-between px-5 py-3 border-t border-gray-800">
          <span className="text-xs text-gray-500">Page {page} of {meta.total_pages}</span>
          <div className="flex gap-2">
            <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Previous</button>
            <button disabled={page === meta.total_pages} onClick={() => setPage((p) => p + 1)}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Next</button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Tab: Settings ─────────────────────────────────────────────────────────────

const PROVIDERS_LIST: { value: ProviderType; label: string; defaultPort: number }[] = [
  { value: "vmware",  label: "VMware vCenter", defaultPort: 443  },
  { value: "esxi",    label: "VMware ESXi",    defaultPort: 443  },
  { value: "proxmox", label: "Proxmox VE",     defaultPort: 8006 },
  { value: "kvm",     label: "KVM / QEMU",     defaultPort: 22   },
  { value: "hyperv",  label: "Hyper-V",        defaultPort: 5985 },
];

function inputCls(err?: string) {
  return `w-full px-3 py-2 bg-gray-800 border ${err ? "border-red-500" : "border-gray-700"} rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 transition-colors`;
}

function SettingsTab({ hypervisor, onSaved, onDeleted }: {
  hypervisor: Hypervisor;
  onSaved: () => void;
  onDeleted: () => void;
}) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<UpdateHypervisorPayload>({
    name:        hypervisor.name,
    description: hypervisor.description ?? "",
    host:        hypervisor.host,
    port:        hypervisor.port,
    username:    hypervisor.username ?? "",
    password:    "",
    tls_verify:  hypervisor.tls_verify,
    tags:        hypervisor.tags ?? [],
    vcenter_url: String(hypervisor.metadata?.vcenter_url ?? ""),
    datacenter:  String(hypervisor.metadata?.datacenter ?? ""),
    node:        String(hypervisor.metadata?.node ?? ""),
    api_token_id: String(hypervisor.metadata?.api_token_id ?? ""),
    api_token_secret: "",
  });
  const [tagInput, setTagInput] = useState("");
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  const updateMut = useMutation({
    mutationFn: (payload: UpdateHypervisorPayload) => hypervisorApi.update(hypervisor.id, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["hypervisor", hypervisor.id] });
      onSaved();
    },
  });

  const deleteMut = useMutation({
    mutationFn: () => hypervisorApi.delete(hypervisor.id),
    onSuccess: onDeleted,
  });

  const setField = <K extends keyof UpdateHypervisorPayload>(k: K, v: UpdateHypervisorPayload[K]) =>
    setForm((f) => ({ ...f, [k]: v }));

  const addTag = () => {
    const t = tagInput.trim();
    if (t && !(form.tags ?? []).includes(t)) setField("tags", [...(form.tags ?? []), t]);
    setTagInput("");
  };

  const isVMware  = hypervisor.provider === "vmware" || hypervisor.provider === "esxi";
  const isProxmox = hypervisor.provider === "proxmox";

  return (
    <div className="space-y-5 max-w-2xl">
      <SectionCard title="Edit Hypervisor" icon={Edit2}>
        <div className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Name</label>
              <input value={form.name ?? ""} onChange={(e) => setField("name", e.target.value)} className={inputCls()} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Host / IP</label>
              <input value={form.host ?? ""} onChange={(e) => setField("host", e.target.value)} className={inputCls()} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Port</label>
              <input type="number" value={form.port ?? ""} onChange={(e) => setField("port", +e.target.value)} className={inputCls()} min={1} max={65535} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Username</label>
              <input value={form.username ?? ""} onChange={(e) => setField("username", e.target.value)} className={inputCls()} />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
            <textarea value={form.description ?? ""} onChange={(e) => setField("description", e.target.value)}
              className={`${inputCls()} resize-none`} rows={2} />
          </div>

          {/* Password */}
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">
              Password <span className="text-gray-600">(leave blank to keep current)</span>
            </label>
            <input type="password" value={form.password ?? ""} onChange={(e) => setField("password", e.target.value)}
              className={inputCls()} placeholder="••••••••" autoComplete="new-password" />
          </div>

          {/* VMware extras */}
          {isVMware && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4 bg-gray-800/40 rounded-xl border border-gray-700/50">
              <p className="text-xs font-semibold text-blue-400 uppercase tracking-wider sm:col-span-2">VMware Options</p>
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">vCenter URL</label>
                <input value={form.vcenter_url ?? ""} onChange={(e) => setField("vcenter_url", e.target.value)} className={inputCls()} />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">Datacenter</label>
                <input value={form.datacenter ?? ""} onChange={(e) => setField("datacenter", e.target.value)} className={inputCls()} />
              </div>
            </div>
          )}

          {/* Proxmox extras */}
          {isProxmox && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4 bg-gray-800/40 rounded-xl border border-gray-700/50">
              <p className="text-xs font-semibold text-orange-400 uppercase tracking-wider sm:col-span-2">Proxmox Options</p>
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">Node</label>
                <input value={form.node ?? ""} onChange={(e) => setField("node", e.target.value)} className={inputCls()} />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">API Token ID</label>
                <input value={form.api_token_id ?? ""} onChange={(e) => setField("api_token_id", e.target.value)} className={inputCls()} />
              </div>
              <div className="sm:col-span-2">
                <label className="block text-xs font-medium text-gray-400 mb-1.5">API Token Secret <span className="text-gray-600">(leave blank to keep)</span></label>
                <input type="password" value={form.api_token_secret ?? ""} onChange={(e) => setField("api_token_secret", e.target.value)}
                  className={inputCls()} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" autoComplete="new-password" />
              </div>
            </div>
          )}

          {/* TLS */}
          <label className="flex items-center gap-3 cursor-pointer">
            <div className="relative">
              <input type="checkbox" checked={form.tls_verify ?? true} onChange={(e) => setField("tls_verify", e.target.checked)} className="sr-only peer" />
              <div className="w-9 h-5 bg-gray-700 peer-checked:bg-blue-600 rounded-full transition-colors" />
              <div className="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-4" />
            </div>
            <span className="text-sm text-gray-300">Verify TLS certificate</span>
          </label>

          {/* Tags */}
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Tags</label>
            <div className="flex gap-2">
              <input value={tagInput} onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }}
                className={`${inputCls()} flex-1`} placeholder="Add tag and press Enter" />
              <button type="button" onClick={addTag}
                className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">Add</button>
            </div>
            {(form.tags ?? []).length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {(form.tags ?? []).map((t) => (
                  <span key={t} className="inline-flex items-center gap-1 px-2 py-0.5 bg-gray-800 text-gray-300 rounded-md text-xs">
                    {t}
                    <button type="button" onClick={() => setField("tags", (form.tags ?? []).filter((x) => x !== t))}
                      className="hover:text-red-400 transition-colors"><X className="w-3 h-3" /></button>
                  </span>
                ))}
              </div>
            )}
          </div>

          {updateMut.error && (
            <div className="flex items-start gap-2 p-3 bg-red-900/20 border border-red-800/50 rounded-lg">
              <AlertTriangle className="w-4 h-4 text-red-400 mt-0.5 shrink-0" />
              <p className="text-sm text-red-400">{(updateMut.error as Error).message}</p>
            </div>
          )}

          <button onClick={() => updateMut.mutate(form)} disabled={updateMut.isPending}
            className="flex items-center gap-2 px-5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors">
            {updateMut.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
            {updateMut.isPending ? "Saving…" : "Save Changes"}
          </button>
        </div>
      </SectionCard>

      {/* Danger zone */}
      <div className="bg-gray-900 border border-red-900/40 rounded-2xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-4 h-4 text-red-400" />
          <h3 className="text-sm font-semibold text-red-400">Danger Zone</h3>
        </div>
        <p className="text-sm text-gray-400 mb-4">
          Deleting this hypervisor will remove all associated VMs, datastores, and network records. This cannot be undone.
        </p>
        {!showDeleteConfirm ? (
          <button onClick={() => setShowDeleteConfirm(true)}
            className="flex items-center gap-2 px-4 py-2 bg-red-900/30 hover:bg-red-900/50 text-red-400 border border-red-800/50 rounded-lg text-sm transition-colors">
            <Trash2 className="w-4 h-4" /> Delete Hypervisor
          </button>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-red-300 font-medium">Are you sure? This action is irreversible.</p>
            {deleteMut.error && (
              <p className="text-xs text-red-400">{(deleteMut.error as Error).message}</p>
            )}
            <div className="flex gap-3">
              <button onClick={() => deleteMut.mutate()} disabled={deleteMut.isPending}
                className="flex items-center gap-2 px-4 py-2 bg-red-600 hover:bg-red-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors">
                {deleteMut.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                {deleteMut.isPending ? "Deleting…" : "Yes, Delete"}
              </button>
              <button onClick={() => setShowDeleteConfirm(false)}
                className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────

const TABS: { id: Tab; label: string; icon: React.ElementType }[] = [
  { id: "overview",  label: "Overview",  icon: Info },
  { id: "hardware",  label: "Hardware",  icon: Cpu },
  { id: "vms",       label: "VMs",       icon: Server },
  { id: "tasks",     label: "Tasks",     icon: Activity },
  { id: "settings",  label: "Settings",  icon: Shield },
];

export default function HypervisorDetailPage() {
  const params = useParams();
  const router = useRouter();
  const hypervisorId = params.id as string;
  const queryClient = useQueryClient();
  const upsertTask     = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, addToast, removeToast } = useToast();
  const { syncs, trackSync } = useSyncProgress();

  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [testResult, setTestResult] = useState<{ ok: boolean; msg?: string } | null>(null);
  const [isTesting, setIsTesting] = useState(false);

  // Fetch hypervisor
  const { data: hypervisor, isLoading, error, refetch } = useQuery({
    queryKey: ["hypervisor", hypervisorId],
    queryFn: () => hypervisorApi.get(hypervisorId),
    refetchInterval: 60_000,
  });

  // Fetch VM count (page 1, size 1 — just need total_items)
  const { data: vmData } = useQuery({
    queryKey: ["hypervisor-vm-count", hypervisorId],
    queryFn: () => vmApi.list({ hypervisor_id: hypervisorId, page: 1, page_size: 1 }),
    enabled: !!hypervisor,
  });
  const vmCount = vmData?.meta?.total_items ?? hypervisor?.vm_count ?? 0;

  // Real-time: refresh hypervisor when inventory sync completes
  useEffect(() => {
    const unsub = wsClient.subscribe("inventory", (msg) => {
      const payload = msg.payload as Record<string, unknown>;
      if (payload?.hypervisor_id === hypervisorId) {
        queryClient.invalidateQueries({ queryKey: ["hypervisor", hypervisorId] });
        queryClient.invalidateQueries({ queryKey: ["hypervisor-vms", hypervisorId] });
        queryClient.invalidateQueries({ queryKey: ["hypervisor-vm-count", hypervisorId] });
      }
    });
    return unsub;
  }, [hypervisorId, queryClient]);

  // Sync mutation
  const syncMut = useMutation({
    mutationFn: () => hypervisorApi.syncInventory(hypervisorId),
    onSuccess: (data) => {
      if (data?.task_id) {
        trackSync(data.task_id, hypervisorId, hypervisor?.name);
        upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "inventory.sync" } as never);
        openTaskDrawer();
        addToast("success", "Inventory sync started");
      }
    },
    onError: (err: Error) => addToast("error", `Sync failed: ${err.message}`),
  });

  const handleTest = async () => {
    setIsTesting(true);
    setTestResult(null);
    try {
      const res = await hypervisorApi.testConnection(hypervisorId);
      setTestResult({ ok: res!.connected, msg: res!.error });
      if (res!.connected) {
        addToast("success", "Connection successful");
        queryClient.invalidateQueries({ queryKey: ["hypervisor", hypervisorId] });
      } else {
        addToast("error", `Connection failed: ${res!.error ?? "Unknown error"}`);
      }
    } catch (e) {
      setTestResult({ ok: false, msg: (e as Error).message });
      addToast("error", `Connection test failed: ${(e as Error).message}`);
    } finally {
      setIsTesting(false);
    }
  };

  // ── Loading / Error ─────────────────────────────────────────────────────────

  if (isLoading) {
    return (
      <div className="space-y-5">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-32 rounded-2xl" />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-48 rounded-2xl" />)}
        </div>
      </div>
    );
  }

  if (error || !hypervisor) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-4">
        <XCircle className="w-12 h-12 text-red-400" />
        <p className="text-white font-semibold">Hypervisor not found</p>
        <p className="text-gray-400 text-sm">{(error as Error)?.message ?? "The requested hypervisor could not be loaded."}</p>
        <button onClick={() => router.back()}
          className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
          Go Back
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-gray-500">
        <Link href="/dashboard/hypervisors" className="hover:text-gray-300 transition-colors flex items-center gap-1">
          <ArrowLeft className="w-3.5 h-3.5" /> Hypervisors
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-300">{hypervisor.name}</span>
      </div>

      {/* Header card */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
        <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-gray-800 border border-gray-700 flex items-center justify-center shrink-0">
              <Cpu className="w-6 h-6 text-blue-400" />
            </div>
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <h1 className="text-xl font-bold text-white">{hypervisor.name}</h1>
                <HypervisorStatusBadge status={hypervisor.connection_status} />
                <ProviderBadge provider={hypervisor.provider} />
                {testResult && (
                  testResult.ok
                    ? <span className="flex items-center gap-1 text-xs text-green-400"><CheckCircle2 className="w-3.5 h-3.5" />Connected</span>
                    : <span className="flex items-center gap-1 text-xs text-red-400" title={testResult.msg}><XCircle className="w-3.5 h-3.5" />Failed</span>
                )}
              </div>
              <div className="flex items-center gap-3 mt-1.5 flex-wrap text-xs text-gray-400">
                <span className="font-mono">{hypervisor.host}:{hypervisor.port}</span>
                {hypervisor.username && <span>{hypervisor.username}</span>}
                <span className="flex items-center gap-1">
                  <Server className="w-3 h-3" />{vmCount} VMs
                </span>
                {hypervisor.last_checked_at && (
                  <span>Checked {relativeTime(hypervisor.last_checked_at)}</span>
                )}
              </div>
              {hypervisor.description && (
                <p className="text-xs text-gray-500 mt-1.5">{hypervisor.description}</p>
              )}
              {(hypervisor.tags ?? []).length > 0 && (
                <div className="flex flex-wrap gap-1 mt-2">
                  {hypervisor.tags.map((t) => (
                    <span key={t} className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 text-[10px]">{t}</span>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Action buttons */}
          <div className="flex items-center gap-2 flex-wrap">
            <button onClick={() => { refetch(); }}
              className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-xs transition-colors">
              <RefreshCw className="w-3.5 h-3.5" /> Refresh
            </button>
            <button onClick={handleTest} disabled={isTesting}
              className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 text-gray-300 rounded-lg text-xs transition-colors">
              {isTesting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Plug className="w-3.5 h-3.5" />}
              Test Connection
            </button>
            <button onClick={() => syncMut.mutate()} disabled={syncMut.isPending}
              className="flex items-center gap-1.5 px-3 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border border-blue-600/40 rounded-lg text-xs transition-colors disabled:opacity-50">
              {syncMut.isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RotateCcw className="w-3.5 h-3.5" />}
              Sync Inventory
            </button>
            <button onClick={() => setActiveTab("settings")}
              className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-xs transition-colors">
              <Edit2 className="w-3.5 h-3.5" /> Edit
            </button>
          </div>
        </div>
      </div>

      {/* Sync progress */}
      <SyncProgressPanel syncs={syncs} />

      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-gray-800 overflow-x-auto">
        {TABS.map(({ id, label, icon: Icon }) => (
          <button key={id} onClick={() => setActiveTab(id)}
            className={cn(
              "flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium whitespace-nowrap border-b-2 transition-colors",
              activeTab === id
                ? "border-blue-500 text-blue-400"
                : "border-transparent text-gray-500 hover:text-gray-300"
            )}>
            <Icon className="w-3.5 h-3.5" />
            {label}
            {id === "vms" && vmCount > 0 && (
              <span className="ml-1 text-[10px] bg-gray-800 text-gray-400 px-1.5 py-0.5 rounded-full">{vmCount}</span>
            )}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div>
        {activeTab === "overview"  && <OverviewTab hypervisor={hypervisor} vmCount={vmCount} />}
        {activeTab === "hardware"  && <HardwareTab hypervisorId={hypervisorId} />}
        {activeTab === "vms"       && <VMsTab hypervisorId={hypervisorId} />}
        {activeTab === "tasks"    && <TasksTab hypervisorId={hypervisorId} />}
        {activeTab === "settings" && (
          <SettingsTab
            hypervisor={hypervisor}
            onSaved={() => addToast("success", "Hypervisor updated")}
            onDeleted={() => router.push("/dashboard/hypervisors")}
          />
        )}
      </div>
    </div>
  );
}
