"use client";
import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  ArrowLeft, RefreshCw, Power, RotateCcw, PauseCircle, Monitor,
  Cpu, MemoryStick, HardDrive, Network, Server, Database,
  Activity, FileText, Info, AlertTriangle, Loader2,
  CheckCircle2, XCircle, X, Clock, ChevronRight, Camera,
  Wifi, Globe, Shield, Copy,
} from "lucide-react";
import Link from "next/link";
import { vmApi } from "@/lib/api/vms";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { VMStatusBadge } from "@/components/vms/VMStatusBadge";
import { VMToolsBadge } from "@/components/vms/VMToolsBadge";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { wsClient } from "@/lib/ws/WSClient";
import { cn, formatBytes, formatUptime, formatDate, relativeTime } from "@/lib/utils";
import CloneVMDialog from "@/components/provisioning/CloneVMDialog";
import type { VM, VMMetrics, Task, AuditLog, VMStatus, Snapshot } from "@/types";

// ── Types ─────────────────────────────────────────────────────────────────────

type Tab = "overview" | "hardware" | "network" | "storage" | "snapshots" | "console" | "tasks" | "activity" | "provider" | "tags";
type PowerAction = "power-on" | "power-off" | "reboot" | "suspend";

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
          <button onClick={() => onRemove(t.id)} className="text-gray-500 hover:text-gray-300"><X className="w-3.5 h-3.5" /></button>
        </div>
      ))}
    </div>
  );
}

// ── Confirm Dialog ────────────────────────────────────────────────────────────

function ConfirmDialog({ open, title, description, confirmLabel, confirmClass = "bg-red-600 hover:bg-red-700", onConfirm, onCancel }: {
  open: boolean; title: string; description: string; confirmLabel: string;
  confirmClass?: string; onConfirm: () => void; onCancel: () => void;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={(v) => !v && onCancel()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" />
        <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-full max-w-md bg-gray-900 border border-gray-700 rounded-2xl p-6 shadow-2xl">
          <div className="flex items-start gap-4">
            <div className="p-2 bg-yellow-900/40 rounded-lg shrink-0">
              <AlertTriangle className="w-5 h-5 text-yellow-400" />
            </div>
            <div className="flex-1">
              <Dialog.Title className="text-white font-semibold text-base mb-1">{title}</Dialog.Title>
              <Dialog.Description className="text-gray-400 text-sm">{description}</Dialog.Description>
            </div>
          </div>
          <div className="flex justify-end gap-3 mt-6">
            <button onClick={onCancel} className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors">Cancel</button>
            <button onClick={onConfirm} className={cn("px-4 py-2 text-sm text-white rounded-lg transition-colors", confirmClass)}>{confirmLabel}</button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function metaStr(vm: VM, key: string): string {
  const v = vm.metadata?.[key];
  return typeof v === "string" ? v : "";
}

function metaNum(vm: VM, key: string): number {
  const v = vm.metadata?.[key];
  return typeof v === "number" ? v : 0;
}

function allowedActions(status: VMStatus): Set<PowerAction> {
  switch (status) {
    case "running": return new Set(["power-off", "reboot", "suspend"]);
    case "stopped": return new Set(["power-on"]);
    case "suspended": return new Set(["power-on", "power-off"]);
    case "paused": return new Set(["power-on", "power-off"]);
    default: return new Set(["power-on"]);
  }
}

const ACTION_LABELS: Record<PowerAction, string> = {
  "power-on": "Power On", "power-off": "Power Off", "reboot": "Reboot", "suspend": "Suspend",
};
const CONFIRM_ACTIONS = new Set<PowerAction>(["power-off", "reboot"]);

// ── Shared UI primitives ──────────────────────────────────────────────────────

function InfoRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2.5 border-b border-gray-800/60 last:border-0">
      <span className="text-xs text-gray-500 shrink-0 w-36">{label}</span>
      <span className={cn("text-sm text-gray-200 text-right break-all", mono && "font-mono text-xs")}>{value ?? "—"}</span>
    </div>
  );
}

function SectionCard({ title, icon: Icon, children }: { title: string; icon: React.ElementType; children: React.ReactNode }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
      <div className="flex items-center gap-2 mb-4">
        <Icon className="w-4 h-4 text-blue-400" />
        <h3 className="text-sm font-semibold text-white">{title}</h3>
      </div>
      {children}
    </div>
  );
}

function MetricBar({ label, value, max, unit }: { label: string; value: number; max: number; unit: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const color = pct > 85 ? "bg-red-500" : pct > 65 ? "bg-yellow-500" : "bg-blue-500";
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs">
        <span className="text-gray-400">{label}</span>
        <span className="text-gray-300">{value.toFixed(1)}{unit}</span>
      </div>
      <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
        <div className={cn("h-full rounded-full transition-all duration-700", color)} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
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

function TaskStatusIcon({ status }: { status: Task["status"] }) {
  if (status === "completed") return <CheckCircle2 className="w-3.5 h-3.5 text-green-400 shrink-0" />;
  if (status === "failed" || status === "timed_out") return <XCircle className="w-3.5 h-3.5 text-red-400 shrink-0" />;
  if (status === "running") return <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin shrink-0" />;
  return <Clock className="w-3.5 h-3.5 text-gray-400 shrink-0" />;
}

function Skeleton({ className }: { className?: string }) {
  return <div className={cn("bg-gray-800 rounded animate-pulse", className)} />;
}

// ── Tab: Overview ─────────────────────────────────────────────────────────────

function OverviewTab({ vm, metrics, metricsLoading }: { vm: VM; metrics?: VMMetrics; metricsLoading: boolean }) {
  const uptime = metaNum(vm, "uptime");
  const cluster = metaStr(vm, "cluster");
  const esxiHost = metaStr(vm, "esxi_host");
  const proxmoxNode = metaStr(vm, "node");
  const datastore = metaStr(vm, "datastore");
  const annotation = metaStr(vm, "annotation");

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      {/* Identity */}
      <SectionCard title="Identity" icon={Info}>
        <InfoRow label="Name" value={vm.name} />
        <InfoRow label="Status" value={<VMStatusBadge status={vm.status} />} />
        <InfoRow label="Description" value={vm.description || annotation || "—"} />
        <InfoRow label="Guest OS" value={vm.guest_os || "—"} />
        <InfoRow label="OS Type" value={vm.guest_os_type || "—"} />
        <InfoRow label="VMware Tools" value={<VMToolsBadge status={vm.tools_status} />} />
        <InfoRow label="Created" value={formatDate(vm.created_at)} />
        <InfoRow label="Last Updated" value={relativeTime(vm.updated_at)} />
      </SectionCard>

      {/* Location */}
      <SectionCard title="Location" icon={Server}>
        {cluster && <InfoRow label="Cluster" value={cluster} />}
        {esxiHost && <InfoRow label="ESXi Host" value={esxiHost} />}
        {proxmoxNode && <InfoRow label="Proxmox Node" value={proxmoxNode} />}
        {datastore && <InfoRow label="Datastore" value={datastore} />}
        {uptime > 0 && <InfoRow label="Uptime" value={formatUptime(uptime)} />}
        {vm.ip_addresses?.length > 0 && (
          <InfoRow label="IP Addresses" value={
            <div className="flex flex-col gap-0.5 items-end">
              {vm.ip_addresses.map((ip) => <span key={ip} className="font-mono text-xs">{ip}</span>)}
            </div>
          } />
        )}
        {vm.network_name && <InfoRow label="Network" value={vm.network_name} />}
      </SectionCard>

      {/* Live Metrics */}
      <SectionCard title="Live Metrics" icon={Activity}>
        {metricsLoading ? (
          <div className="space-y-3">{Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-6 w-full" />)}</div>
        ) : metrics ? (
          <div className="space-y-4">
            <MetricBar label="CPU Usage" value={metrics.cpu_usage_percent} max={100} unit="%" />
            <MetricBar label="Memory Usage" value={metrics.memory_usage_mb / 1024} max={vm.memory_mb / 1024} unit=" GB" />
            <div className="grid grid-cols-2 gap-3 pt-1">
              <div className="bg-gray-800/60 rounded-xl p-3 text-center">
                <p className="text-xs text-gray-500 mb-1">Disk Read</p>
                <p className="text-sm font-semibold text-white">{metrics.disk_read_kbps.toFixed(0)} <span className="text-xs text-gray-500">KB/s</span></p>
              </div>
              <div className="bg-gray-800/60 rounded-xl p-3 text-center">
                <p className="text-xs text-gray-500 mb-1">Disk Write</p>
                <p className="text-sm font-semibold text-white">{metrics.disk_write_kbps.toFixed(0)} <span className="text-xs text-gray-500">KB/s</span></p>
              </div>
              <div className="bg-gray-800/60 rounded-xl p-3 text-center">
                <p className="text-xs text-gray-500 mb-1">Net RX</p>
                <p className="text-sm font-semibold text-white">{metrics.network_rx_kbps.toFixed(0)} <span className="text-xs text-gray-500">KB/s</span></p>
              </div>
              <div className="bg-gray-800/60 rounded-xl p-3 text-center">
                <p className="text-xs text-gray-500 mb-1">Net TX</p>
                <p className="text-sm font-semibold text-white">{metrics.network_tx_kbps.toFixed(0)} <span className="text-xs text-gray-500">KB/s</span></p>
              </div>
            </div>
          </div>
        ) : (
          <p className="text-sm text-gray-500 py-4 text-center">Metrics unavailable — VM may be stopped</p>
        )}
      </SectionCard>

      {/* Snapshots summary */}
      <SectionCard title="Snapshots" icon={Camera}>
        {vm.snapshots && vm.snapshots.length > 0 ? (
          <div className="space-y-2">
            {vm.snapshots.slice(0, 5).map((s) => (
              <div key={s.id} className="flex items-center justify-between py-1.5 border-b border-gray-800/60 last:border-0">
                <div className="flex items-center gap-2 min-w-0">
                  {s.is_current && <span className="w-1.5 h-1.5 rounded-full bg-green-400 shrink-0" />}
                  <span className="text-sm text-gray-200 truncate">{s.name}</span>
                </div>
                <span className="text-xs text-gray-500 shrink-0 ml-2">{relativeTime(s.created_at)}</span>
              </div>
            ))}
            {vm.snapshots.length > 5 && (
              <p className="text-xs text-gray-500 pt-1">+{vm.snapshots.length - 5} more snapshots</p>
            )}
          </div>
        ) : (
          <p className="text-sm text-gray-500 py-4 text-center">No snapshots</p>
        )}
      </SectionCard>
    </div>
  );
}

// ── Tab: Hardware ─────────────────────────────────────────────────────────────

function HardwareTab({ vm, metrics }: { vm: VM; metrics?: VMMetrics }) {
  const cpuUsage = metrics?.cpu_usage_percent ?? 0;
  const memUsedMB = metrics?.memory_usage_mb ?? metaNum(vm, "mem_used") / (1024 * 1024);

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <SectionCard title="Compute" icon={Cpu}>
        <InfoRow label="vCPUs" value={`${vm.cpu_count} vCPU${vm.cpu_count !== 1 ? "s" : ""}`} />
        <InfoRow label="Memory" value={formatBytes(vm.memory_mb)} />
        {cpuUsage > 0 && <InfoRow label="CPU Usage" value={`${cpuUsage.toFixed(1)}%`} />}
        {memUsedMB > 0 && <InfoRow label="Memory Used" value={`${formatBytes(memUsedMB)} / ${formatBytes(vm.memory_mb)}`} />}
        {metaNum(vm, "cpu_usage") > 0 && (
          <InfoRow label="CPU (provider)" value={`${(metaNum(vm, "cpu_usage") * 100).toFixed(1)}%`} />
        )}
      </SectionCard>

      <SectionCard title="Storage" icon={HardDrive}>
        <InfoRow label="Total Disk" value={vm.disk_gb > 0 ? `${vm.disk_gb} GB` : "—"} />
        {metaStr(vm, "datastore") && <InfoRow label="Primary Datastore" value={metaStr(vm, "datastore")} />}
        {metaStr(vm, "vm_path") && <InfoRow label="VM Path" value={metaStr(vm, "vm_path")} mono />}
        {metrics && metrics.disk_read_kbps > 0 && (
          <InfoRow label="Disk Read" value={`${metrics.disk_read_kbps.toFixed(0)} KB/s`} />
        )}
        {metrics && metrics.disk_write_kbps > 0 && (
          <InfoRow label="Disk Write" value={`${metrics.disk_write_kbps.toFixed(0)} KB/s`} />
        )}
      </SectionCard>

      <SectionCard title="Resource Allocation" icon={Activity}>
        <div className="space-y-4">
          <div>
            <div className="flex justify-between text-xs mb-1.5">
              <span className="text-gray-400">CPU Allocation</span>
              <span className="text-gray-300">{vm.cpu_count} vCPU{vm.cpu_count !== 1 ? "s" : ""}</span>
            </div>
            <div className="flex gap-1">
              {Array.from({ length: Math.min(vm.cpu_count, 16) }).map((_, i) => (
                <div key={i} className="flex-1 h-6 bg-blue-600/30 border border-blue-600/50 rounded flex items-center justify-center">
                  <span className="text-[9px] text-blue-400">{i + 1}</span>
                </div>
              ))}
              {vm.cpu_count > 16 && <span className="text-xs text-gray-500 self-center ml-1">+{vm.cpu_count - 16}</span>}
            </div>
          </div>
          <div>
            <div className="flex justify-between text-xs mb-1.5">
              <span className="text-gray-400">Memory Allocation</span>
              <span className="text-gray-300">{formatBytes(vm.memory_mb)}</span>
            </div>
            <div className="h-6 bg-gray-800 rounded-lg overflow-hidden relative">
              {memUsedMB > 0 && (
                <div
                  className="h-full bg-purple-600/60 border-r border-purple-500 transition-all duration-700"
                  style={{ width: `${Math.min(100, (memUsedMB / vm.memory_mb) * 100)}%` }}
                />
              )}
              <span className="absolute inset-0 flex items-center justify-center text-xs text-gray-300">
                {memUsedMB > 0 ? `${formatBytes(memUsedMB)} used` : "Usage unknown"}
              </span>
            </div>
          </div>
        </div>
      </SectionCard>
    </div>
  );
}

// ── Tab: Network ──────────────────────────────────────────────────────────────

function NetworkTab({ vm, metrics }: { vm: VM; metrics?: VMMetrics }) {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <SectionCard title="Network Interfaces" icon={Network}>
        {vm.mac_address || vm.network_name || (vm.ip_addresses?.length > 0) ? (
          <div className="space-y-0">
            {vm.network_name && <InfoRow label="Network Name" value={vm.network_name} />}
            {vm.mac_address && <InfoRow label="MAC Address" value={vm.mac_address} mono />}
            {vm.ip_addresses?.length > 0 && (
              <InfoRow label="IP Addresses" value={
                <div className="flex flex-col gap-0.5 items-end">
                  {vm.ip_addresses.map((ip) => (
                    <span key={ip} className="font-mono text-xs bg-gray-800 px-2 py-0.5 rounded">{ip}</span>
                  ))}
                </div>
              } />
            )}
          </div>
        ) : (
          <p className="text-sm text-gray-500 py-4 text-center">No network interface data available</p>
        )}
      </SectionCard>

      <SectionCard title="IP Addresses" icon={Globe}>
        {vm.ip_addresses?.length > 0 ? (
          <div className="space-y-2">
            {vm.ip_addresses.map((ip, i) => (
              <div key={ip} className="flex items-center justify-between p-3 bg-gray-800/60 rounded-xl">
                <div className="flex items-center gap-2">
                  <Wifi className="w-3.5 h-3.5 text-blue-400" />
                  <span className="font-mono text-sm text-white">{ip}</span>
                </div>
                <span className="text-xs text-gray-500">{i === 0 ? "Primary" : `Interface ${i + 1}`}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-gray-500 py-4 text-center">No IP addresses assigned</p>
        )}
      </SectionCard>

      {metrics && (
        <SectionCard title="Network Throughput" icon={Activity}>
          <div className="grid grid-cols-2 gap-3">
            <div className="bg-gray-800/60 rounded-xl p-4 text-center">
              <p className="text-xs text-gray-500 mb-1">Receive</p>
              <p className="text-lg font-bold text-white">{metrics.network_rx_kbps.toFixed(0)}</p>
              <p className="text-xs text-gray-500">KB/s</p>
            </div>
            <div className="bg-gray-800/60 rounded-xl p-4 text-center">
              <p className="text-xs text-gray-500 mb-1">Transmit</p>
              <p className="text-lg font-bold text-white">{metrics.network_tx_kbps.toFixed(0)}</p>
              <p className="text-xs text-gray-500">KB/s</p>
            </div>
          </div>
        </SectionCard>
      )}
    </div>
  );
}

// ── Tab: Storage ──────────────────────────────────────────────────────────────

function StorageTab({ vm }: { vm: VM }) {
  const datastore = metaStr(vm, "datastore");
  const datastoreMor = metaStr(vm, "datastore_mor");
  const vmPath = metaStr(vm, "vm_path");

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <SectionCard title="Disk Configuration" icon={HardDrive}>
        <InfoRow label="Total Disk Size" value={vm.disk_gb > 0 ? `${vm.disk_gb} GB` : "—"} />
        {vmPath && <InfoRow label="VM Path" value={vmPath} mono />}
        {datastore && <InfoRow label="Primary Datastore" value={datastore} />}
        {datastoreMor && <InfoRow label="Datastore MOR" value={datastoreMor} mono />}
      </SectionCard>

      <SectionCard title="Snapshots" icon={Camera}>
        {vm.snapshots && vm.snapshots.length > 0 ? (
          <div className="space-y-2">
            {vm.snapshots.map((s: Snapshot) => (
              <div key={s.id} className="p-3 bg-gray-800/60 rounded-xl">
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    {s.is_current && (
                      <span className="text-[10px] bg-green-900/50 text-green-400 px-1.5 py-0.5 rounded font-medium">CURRENT</span>
                    )}
                    <span className="text-sm font-medium text-white">{s.name}</span>
                  </div>
                  <span className="text-xs text-gray-500">{relativeTime(s.created_at)}</span>
                </div>
                {s.description && <p className="text-xs text-gray-500 mt-1">{s.description}</p>}
                <p className="text-[10px] text-gray-600 font-mono mt-1">{s.provider_id}</p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-gray-500 py-4 text-center">No snapshots</p>
        )}
      </SectionCard>

      {/* Proxmox disk stats */}
      {(metaNum(vm, "disk_read") > 0 || metaNum(vm, "disk_write") > 0) && (
        <SectionCard title="Disk I/O (Proxmox)" icon={Activity}>
          <InfoRow label="Disk Read" value={`${(metaNum(vm, "disk_read") / 1024 / 1024).toFixed(2)} MB/s`} />
          <InfoRow label="Disk Write" value={`${(metaNum(vm, "disk_write") / 1024 / 1024).toFixed(2)} MB/s`} />
        </SectionCard>
      )}
    </div>
  );
}

// ── Tab: Tasks ────────────────────────────────────────────────────────────────

function TasksTab({ vmId }: { vmId: string }) {
  const [page, setPage] = useState(1);
  const { data, isLoading } = useQuery({
    queryKey: ["vm-tasks", vmId, page],
    queryFn: () => vmApi.getTasks(vmId, { page, page_size: 20 }),
  });

  const tasks: Task[] = data?.data ?? [];
  const meta = data?.meta;

  return (
    <div className="space-y-4">
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
          <div className="py-12 text-center text-gray-500 text-sm">No tasks found for this VM</div>
        ) : (
          <div className="divide-y divide-gray-800/60">
            {tasks.map((task) => (
              <div key={task.id} className="px-5 py-4 hover:bg-gray-800/30 transition-colors">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <TaskStatusIcon status={task.status} />
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
                  <div className="mt-2">
                    <div className="h-1 bg-gray-800 rounded-full overflow-hidden">
                      <div className="h-full bg-blue-500 rounded-full transition-all duration-500" style={{ width: `${task.progress}%` }} />
                    </div>
                  </div>
                )}
                <div className="flex gap-4 mt-2 text-xs text-gray-600">
                  {task.started_at && <span>Started {relativeTime(task.started_at)}</span>}
                  {task.completed_at && <span>Completed {relativeTime(task.completed_at)}</span>}
                  <span>Priority {task.priority}</span>
                </div>
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
    </div>
  );
}

// ── Tab: Activity ─────────────────────────────────────────────────────────────

const ACTION_COLORS: Record<string, string> = {
  create:  "text-green-400 bg-green-900/30",
  update:  "text-blue-400 bg-blue-900/30",
  delete:  "text-red-400 bg-red-900/30",
  execute: "text-purple-400 bg-purple-900/30",
  login:   "text-yellow-400 bg-yellow-900/30",
  logout:  "text-gray-400 bg-gray-800",
  read:    "text-gray-400 bg-gray-800",
};

function ActivityTab({ vmId }: { vmId: string }) {
  const [page, setPage] = useState(1);
  const { data, isLoading } = useQuery({
    queryKey: ["vm-activity", vmId, page],
    queryFn: () => vmApi.getActivity(vmId, { page, page_size: 25 }),
  });

  const logs: AuditLog[] = data?.data ?? [];
  const meta = data?.meta;

  return (
    <div className="space-y-4">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-white">Activity Log</h3>
          {meta && <span className="text-xs text-gray-500">{meta.total_items} events</span>}
        </div>
        {isLoading ? (
          <div className="p-5 space-y-3">
            {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : logs.length === 0 ? (
          <div className="py-12 text-center text-gray-500 text-sm">No activity recorded for this VM</div>
        ) : (
          <div className="divide-y divide-gray-800/60">
            {logs.map((log) => (
              <div key={log.id} className="px-5 py-3.5 hover:bg-gray-800/30 transition-colors">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 min-w-0">
                    <span className={cn("text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0 mt-0.5",
                      ACTION_COLORS[log.action] ?? "text-gray-400 bg-gray-800")}>
                      {log.action}
                    </span>
                    <div className="min-w-0">
                      <p className="text-sm text-gray-200">{log.description || `${log.action} on ${log.resource}`}</p>
                      <div className="flex items-center gap-2 mt-0.5">
                        <span className="text-xs text-gray-500">{log.username || "system"}</span>
                        {log.ip_address && <span className="text-xs text-gray-600 font-mono">{log.ip_address}</span>}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {!log.success && <XCircle className="w-3.5 h-3.5 text-red-400" />}
                    <span className="text-xs text-gray-500">{relativeTime(log.created_at)}</span>
                  </div>
                </div>
                {log.error_message && (
                  <p className="text-xs text-red-400 mt-1.5 ml-14">{log.error_message}</p>
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
    </div>
  );
}

// ── Tab: Provider Information ─────────────────────────────────────────────────

function ProviderTab({ vm }: { vm: VM }) {
  const isVMware = vm.hypervisor?.provider === "vmware" || vm.hypervisor?.provider === "esxi";
  const isProxmox = vm.hypervisor?.provider === "proxmox";

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <SectionCard title="Provider Details" icon={Shield}>
        {vm.hypervisor && (
          <>
            <InfoRow label="Provider" value={<ProviderBadge provider={vm.hypervisor.provider} />} />
            <InfoRow label="Hypervisor Name" value={vm.hypervisor.name} />
            <InfoRow label="Host" value={vm.hypervisor.host} mono />
            <InfoRow label="Port" value={String(vm.hypervisor.port)} />
            <InfoRow label="Connection" value={
              <span className={cn("text-xs font-medium",
                vm.hypervisor.connection_status === "connected" ? "text-green-400"
                  : vm.hypervisor.connection_status === "error" ? "text-red-400"
                  : "text-gray-400")}>
                {vm.hypervisor.connection_status}
              </span>
            } />
            {vm.hypervisor.last_checked_at && (
              <InfoRow label="Last Checked" value={relativeTime(vm.hypervisor.last_checked_at)} />
            )}
          </>
        )}
        <InfoRow label="Provider VM ID" value={vm.provider_vm_id} mono />
        <InfoRow label="Last Synced" value={relativeTime(vm.updated_at)} />
      </SectionCard>

      {isVMware && (
        <SectionCard title="VMware Details" icon={Server}>
          {metaStr(vm, "uuid") && <InfoRow label="BIOS UUID" value={metaStr(vm, "uuid")} mono />}
          {metaStr(vm, "instance_uuid") && <InfoRow label="Instance UUID" value={metaStr(vm, "instance_uuid")} mono />}
          {metaStr(vm, "mor") && <InfoRow label="ManagedObject Ref" value={metaStr(vm, "mor")} mono />}
          {metaStr(vm, "esxi_host_mor") && <InfoRow label="ESXi Host MOR" value={metaStr(vm, "esxi_host_mor")} mono />}
          {metaStr(vm, "datastore_mor") && <InfoRow label="Datastore MOR" value={metaStr(vm, "datastore_mor")} mono />}
          {metaStr(vm, "change_version") && <InfoRow label="Change Version" value={metaStr(vm, "change_version")} mono />}
          {metaStr(vm, "tools_version") && <InfoRow label="Tools Version" value={metaStr(vm, "tools_version")} />}
          {metaStr(vm, "annotation") && <InfoRow label="Annotation" value={metaStr(vm, "annotation")} />}
        </SectionCard>
      )}

      {isProxmox && (
        <SectionCard title="Proxmox Details" icon={Server}>
          {metaNum(vm, "vmid") > 0 && <InfoRow label="VMID" value={String(metaNum(vm, "vmid"))} mono />}
          {metaStr(vm, "node") && <InfoRow label="Node" value={metaStr(vm, "node")} />}
          {metaStr(vm, "agent") && <InfoRow label="QEMU Agent" value={metaStr(vm, "agent")} />}
          {metaNum(vm, "uptime") > 0 && <InfoRow label="Uptime" value={formatUptime(metaNum(vm, "uptime"))} />}
          {metaNum(vm, "net_in") > 0 && <InfoRow label="Net In (total)" value={`${(metaNum(vm, "net_in") / 1024 / 1024).toFixed(1)} MB`} />}
          {metaNum(vm, "net_out") > 0 && <InfoRow label="Net Out (total)" value={`${(metaNum(vm, "net_out") / 1024 / 1024).toFixed(1)} MB`} />}
        </SectionCard>
      )}

      <SectionCard title="Internal IDs" icon={Info}>
        <InfoRow label="VM ID (DB)" value={vm.id} mono />
        <InfoRow label="Hypervisor ID" value={vm.hypervisor_id} mono />
        <InfoRow label="Created At" value={formatDate(vm.created_at)} />
        <InfoRow label="Updated At" value={formatDate(vm.updated_at)} />
        {vm.tags?.length > 0 && (
          <InfoRow label="Tags" value={
            <div className="flex flex-wrap gap-1 justify-end">
              {vm.tags.map((t) => (
                <span key={t} className="text-[10px] bg-gray-800 text-gray-400 px-1.5 py-0.5 rounded">{t}</span>
              ))}
            </div>
          } />
        )}
      </SectionCard>
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────

const TABS: { id: Tab; label: string; icon: React.ElementType }[] = [
  { id: "overview",   label: "Overview",   icon: LayoutDashboard },
  { id: "hardware",   label: "Hardware",   icon: Cpu },
  { id: "network",    label: "Network",    icon: Network },
  { id: "storage",    label: "Storage",    icon: HardDrive },
  { id: "snapshots",  label: "Snapshots",  icon: Camera },
  { id: "console",    label: "Console",    icon: Monitor },
  { id: "tasks",      label: "Tasks",      icon: Activity },
  { id: "activity",   label: "Activity",   icon: FileText },
  { id: "provider",   label: "Provider",   icon: Server },
  { id: "tags",       label: "Tags",       icon: TagIcon },
];

import { LayoutDashboard, Tag as TagIcon } from "lucide-react";
import { ConsoleTab } from "@/components/console/ConsoleTab";
import { SnapshotsTab } from "@/components/vms/SnapshotsTab";
import { TagManager } from "@/components/vms/TagManager";

export default function VMDetailPage() {
  const params = useParams();
  const router = useRouter();
  const vmId = params.id as string;
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, addToast, removeToast } = useToast();

  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [pendingAction, setPendingAction] = useState<PowerAction | null>(null);
  const [confirm, setConfirm] = useState<{ action: PowerAction } | null>(null);
  const [showCloneDialog, setShowCloneDialog] = useState(false);

  // Fetch VM (with hypervisor + snapshots preloaded)
  const { data: vm, isLoading, error, refetch } = useQuery({
    queryKey: ["vm", vmId],
    queryFn: () => vmApi.get(vmId),
    refetchInterval: 30_000,
  });

  // Fetch metrics (only when VM is running)
  const { data: metrics, isLoading: metricsLoading } = useQuery({
    queryKey: ["vm-metrics", vmId],
    queryFn: () => vmApi.getMetrics(vmId),
    enabled: vm?.status === "running",
    refetchInterval: 15_000,
    retry: false,
  });

  // Real-time VM status updates via WebSocket
  useEffect(() => {
    const unsub = wsClient.subscribe("vms", (msg) => {
      const payload = msg.payload as Record<string, unknown>;
      if (payload?.vm_id === vmId || payload?.id === vmId) {
        queryClient.invalidateQueries({ queryKey: ["vm", vmId] });
      }
    });
    return unsub;
  }, [vmId, queryClient]);

  // Power action mutation
  const actionMut = useMutation({
    mutationFn: ({ act }: { act: PowerAction }) => {
      if (act === "power-on") return vmApi.powerOn(vmId);
      if (act === "power-off") return vmApi.powerOff(vmId);
      if (act === "reboot") return vmApi.reboot(vmId);
      return vmApi.suspend(vmId);
    },
    onSuccess: (data, { act }) => {
      upsertTask({ id: data.task_id, status: "pending", progress: 0, type: `vm.${act.replace(/-/g, "_")}` } as never);
      openTaskDrawer();
      addToast("success", `${ACTION_LABELS[act]} task queued`);
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ["vm", vmId] });
        setPendingAction(null);
      }, 4000);
    },
    onError: (err: Error, { act }) => {
      setPendingAction(null);
      addToast("error", `${ACTION_LABELS[act]} failed: ${err.message}`);
    },
  });

  const triggerAction = useCallback((act: PowerAction) => {
    if (CONFIRM_ACTIONS.has(act)) {
      setConfirm({ action: act });
    } else {
      setPendingAction(act);
      actionMut.mutate({ act });
    }
  }, [actionMut]);

  const handleConfirm = useCallback(() => {
    if (!confirm) return;
    const act = confirm.action;
    setConfirm(null);
    setPendingAction(act);
    actionMut.mutate({ act });
  }, [confirm, actionMut]);

  // ── Loading / Error states ──────────────────────────────────────────────────

  if (isLoading) {
    return (
      <div className="space-y-5">
        <div className="flex items-center gap-3">
          <Skeleton className="h-8 w-8 rounded-lg" />
          <Skeleton className="h-7 w-48" />
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-48 rounded-2xl" />)}
        </div>
      </div>
    );
  }

  if (error || !vm) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-4">
        <XCircle className="w-12 h-12 text-red-400" />
        <p className="text-white font-semibold">VM not found</p>
        <p className="text-gray-400 text-sm">{(error as Error)?.message ?? "The requested VM could not be loaded."}</p>
        <button onClick={() => router.back()} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
          Go Back
        </button>
      </div>
    );
  }

  const allowed = allowedActions(vm.status);
  const isBusy = !!pendingAction;
  const provider = vm.hypervisor?.provider;

  return (
    <div className="space-y-5">
      {/* Confirm dialog */}
      {confirm && (
        <ConfirmDialog
          open={true}
          title={`${ACTION_LABELS[confirm.action]} — ${vm.name}`}
          description={
            confirm.action === "power-off"
              ? `This will hard-stop "${vm.name}". Any unsaved work in the guest OS will be lost.`
              : `This will reboot "${vm.name}". The guest OS will receive a graceful reboot signal.`
          }
          confirmLabel={ACTION_LABELS[confirm.action]}
          confirmClass={confirm.action === "power-off" ? "bg-red-600 hover:bg-red-700" : "bg-yellow-600 hover:bg-yellow-700"}
          onConfirm={handleConfirm}
          onCancel={() => setConfirm(null)}
        />
      )}
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Clone dialog */}
      {showCloneDialog && (
        <CloneVMDialog
          vm={vm}
          onClose={() => setShowCloneDialog(false)}
          onSuccess={(job) => {
            setShowCloneDialog(false);
            addToast("success", `Cloning "${vm.name}" → "${job.vm_name}" started`);
            if (job.task_id) {
              upsertTask({ id: job.task_id, status: "pending", progress: 0, type: "vm.clone" } as never);
              openTaskDrawer();
            }
          }}
        />
      )}

      {/* Breadcrumb + back */}
      <div className="flex items-center gap-2 text-sm text-gray-500">
        <Link href="/dashboard/vms" className="hover:text-gray-300 transition-colors flex items-center gap-1">
          <ArrowLeft className="w-3.5 h-3.5" />
          Virtual Machines
        </Link>
        <ChevronRight className="w-3.5 h-3.5" />
        <span className="text-gray-300">{vm.name}</span>
      </div>

      {/* Header */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
        <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-gray-800 border border-gray-700 flex items-center justify-center shrink-0">
              <Server className="w-6 h-6 text-blue-400" />
            </div>
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <h1 className="text-xl font-bold text-white">{vm.name}</h1>
                <VMStatusBadge status={vm.status} />
                {provider && <ProviderBadge provider={provider} />}
              </div>
              <div className="flex items-center gap-3 mt-1.5 flex-wrap">
                <span className="text-xs text-gray-500 font-mono">{vm.provider_vm_id}</span>
                {vm.ip_addresses?.[0] && (
                  <span className="text-xs text-gray-400 font-mono flex items-center gap-1">
                    <Wifi className="w-3 h-3" />{vm.ip_addresses[0]}
                    {vm.ip_addresses.length > 1 && <span className="text-gray-600">+{vm.ip_addresses.length - 1}</span>}
                  </span>
                )}
                {vm.guest_os && <span className="text-xs text-gray-500">{vm.guest_os}</span>}
              </div>
              <div className="flex items-center gap-3 mt-1.5 text-xs text-gray-500">
                <span className="flex items-center gap-1"><Cpu className="w-3 h-3" />{vm.cpu_count} vCPU</span>
                <span className="flex items-center gap-1"><MemoryStick className="w-3 h-3" />{formatBytes(vm.memory_mb)}</span>
                {vm.disk_gb > 0 && <span className="flex items-center gap-1"><HardDrive className="w-3 h-3" />{vm.disk_gb} GB</span>}
              </div>
            </div>
          </div>

          {/* Action buttons */}
          <div className="flex items-center gap-2 flex-wrap">
            <button onClick={() => { refetch(); queryClient.invalidateQueries({ queryKey: ["vm-metrics", vmId] }); }}
              className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-xs transition-colors">
              <RefreshCw className="w-3.5 h-3.5" />Refresh
            </button>
            {(["power-on", "power-off", "reboot", "suspend"] as PowerAction[]).map((act) => {
              const isActive = pendingAction === act;
              const isAllowed = allowed.has(act);
              const colors: Record<PowerAction, string> = {
                "power-on": "bg-green-600/20 hover:bg-green-600/30 text-green-400 border-green-600/40",
                "power-off": "bg-red-600/20 hover:bg-red-600/30 text-red-400 border-red-600/40",
                "reboot": "bg-yellow-600/20 hover:bg-yellow-600/30 text-yellow-400 border-yellow-600/40",
                "suspend": "bg-orange-600/20 hover:bg-orange-600/30 text-orange-400 border-orange-600/40",
              };
              const icons: Record<PowerAction, React.ElementType> = {
                "power-on": Power, "power-off": Power, "reboot": RotateCcw, "suspend": PauseCircle,
              };
              const Icon = isActive ? Loader2 : icons[act];
              return (
                <button key={act} onClick={() => triggerAction(act)} disabled={!isAllowed || isBusy}
                  title={ACTION_LABELS[act]}
                  className={cn("flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs border transition-colors",
                    isAllowed && !isBusy ? colors[act] : "bg-gray-800/50 text-gray-600 border-gray-700/50 cursor-not-allowed opacity-50")}>
                  <Icon className={cn("w-3.5 h-3.5", isActive && "animate-spin")} />
                  {ACTION_LABELS[act]}
                </button>
              );
            })}
            {vm.status === "running" && (
              <button onClick={() => setActiveTab("console")}
                className="flex items-center gap-1.5 px-3 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border border-blue-600/40 rounded-lg text-xs transition-colors">
                <Monitor className="w-3.5 h-3.5" />Console
              </button>
            )}
            <button onClick={() => setShowCloneDialog(true)}
              className="flex items-center gap-1.5 px-3 py-2 bg-purple-600/20 hover:bg-purple-600/30 text-purple-400 border border-purple-600/40 rounded-lg text-xs transition-colors">
              <Copy className="w-3.5 h-3.5" />Clone
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-gray-800 overflow-x-auto">
        {TABS.map(({ id, label, icon: Icon }) => (
          <button key={id} onClick={() => setActiveTab(id)}
            className={cn("flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium whitespace-nowrap border-b-2 transition-colors",
              activeTab === id
                ? "border-blue-500 text-blue-400"
                : "border-transparent text-gray-500 hover:text-gray-300")}>
            <Icon className="w-3.5 h-3.5" />{label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div>
        {activeTab === "overview"  && <OverviewTab vm={vm} metrics={metrics} metricsLoading={metricsLoading} />}
        {activeTab === "hardware"  && <HardwareTab vm={vm} metrics={metrics} />}
        {activeTab === "network"   && <NetworkTab vm={vm} metrics={metrics} />}
        {activeTab === "storage"   && <StorageTab vm={vm} />}
        {activeTab === "snapshots" && <SnapshotsTab vm={vm} />}
        {activeTab === "console"   && <ConsoleTab vm={vm} />}
        {activeTab === "tasks"     && <TasksTab vmId={vmId} />}
        {activeTab === "activity"  && <ActivityTab vmId={vmId} />}
        {activeTab === "provider"  && <ProviderTab vm={vm} />}
        {activeTab === "tags"      && (
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
            <div className="flex items-center gap-2 mb-5">
              <TagIcon className="w-4 h-4 text-blue-400" />
              <h3 className="text-sm font-semibold text-white">Tag Management</h3>
            </div>
            <TagManager vmId={vmId} />
          </div>
        )}
      </div>
    </div>
  );
}
