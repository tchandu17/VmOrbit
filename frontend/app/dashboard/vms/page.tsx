"use client";
import { useState, useEffect, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  Power, RotateCcw, PauseCircle, Monitor, RefreshCw,
  Server, Database, Cpu, X, AlertTriangle, Loader2,
  CheckCircle2, XCircle, ExternalLink,
} from "lucide-react";
import Link from "next/link";
import { vmApi } from "@/lib/api/vms";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { VMStatusBadge } from "@/components/vms/VMStatusBadge";
import { VMToolsBadge } from "@/components/vms/VMToolsBadge";
import { TagBadge } from "@/components/vms/TagBadge";
import { TagFilter } from "@/components/vms/TagFilter";
import { BulkActionsToolbar, type BulkAction } from "@/components/vms/BulkActionsToolbar";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import { SyncProgressPanel, useSyncProgress } from "@/components/inventory/SyncProgressPanel";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { wsClient } from "@/lib/ws/WSClient";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { formatBytes } from "@/lib/utils";
import type { VM, Hypervisor, VMStatus } from "@/types";

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
        <div key={t.id} className={`flex items-start gap-3 px-4 py-3 rounded-xl shadow-lg border text-sm animate-in slide-in-from-right-5 ${
          t.type === "success" ? "bg-green-950 border-green-800 text-green-200"
            : t.type === "error" ? "bg-red-950 border-red-800 text-red-200"
            : "bg-gray-800 border-gray-700 text-gray-200"}`}>
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
            <button onClick={onConfirm} className={`px-4 py-2 text-sm text-white rounded-lg transition-colors ${confirmClass}`}>{confirmLabel}</button>
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

type PowerAction = "power-on" | "power-off" | "reboot" | "suspend";

function allowedActions(status: VMStatus): Set<PowerAction> {
  switch (status) {
    case "running":   return new Set(["power-off", "reboot", "suspend"]);
    case "stopped":   return new Set(["power-on"]);
    case "suspended": return new Set(["power-on", "power-off"]);
    case "paused":    return new Set(["power-on", "power-off"]);
    default:          return new Set(["power-on"]);
  }
}

const ACTION_LABELS: Record<PowerAction, string> = {
  "power-on": "Power On", "power-off": "Power Off", "reboot": "Reboot", "suspend": "Suspend",
};
const CONFIRM_ACTIONS = new Set<PowerAction>(["power-off", "reboot"]);

// ── ActionBtn ─────────────────────────────────────────────────────────────────
function ActionBtn({ icon: Icon, title, onClick, color, disabled = false, spinning = false }: {
  icon: React.ElementType; title: string; onClick: () => void;
  color: string; disabled?: boolean; spinning?: boolean;
}) {
  return (
    <button title={title} onClick={onClick} disabled={disabled}
      className={`p-1.5 rounded-lg transition-colors ${disabled ? "opacity-30 cursor-not-allowed text-gray-500" : `hover:bg-gray-700 ${color}`}`}>
      <Icon className={`w-3.5 h-3.5 ${spinning ? "animate-spin" : ""}`} />
    </button>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────
export default function VMsPage() {
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [hypervisorFilter, setHypervisorFilter] = useState("");
  const [tagFilter, setTagFilter] = useState<string[]>([]);
  const [selectedVMIds, setSelectedVMIds] = useState<Set<string>>(new Set());
  const [pendingActions, setPendingActions] = useState<Record<string, PowerAction>>({});
  const [confirm, setConfirm] = useState<{ vmId: string; vmName: string; action: PowerAction } | null>(null);
  const { toasts, addToast, removeToast } = useToast();
  const { syncs, trackSync } = useSyncProgress();

  // Debounce search input — only fires query after 350ms of inactivity
  const debouncedSearch = useDebounce(search, 350);

  // Reset to page 1 when filters change
  useEffect(() => { setPage(1); }, [debouncedSearch, hypervisorFilter, tagFilter]);

  // WebSocket: invalidate VM list on inventory sync or VM state change.
  // Using a ref-based guard to avoid re-subscribing on every render.
  useEffect(() => {
    const unsubInventory = wsClient.subscribe("inventory", (msg) => {
      if (msg.type === "inventory.synced") {
        queryClient.invalidateQueries({ queryKey: ["vms"] });
      }
    });
    const unsubVMs = wsClient.subscribe("vms", () => {
      // Invalidate with a short stale time so the refetch is batched
      queryClient.invalidateQueries({ queryKey: ["vms"] });
    });
    return () => { unsubInventory(); unsubVMs(); };
  }, [queryClient]);

  const { data, isLoading } = useQuery({
    queryKey: ["vms", { page, hypervisorFilter, tagFilter, search: debouncedSearch }],
    queryFn: () => vmApi.list({
      page,
      page_size: 20,
      hypervisor_id: hypervisorFilter || undefined,
      tag_ids: tagFilter.length ? tagFilter.join(",") : undefined,
      search: debouncedSearch || undefined,
    } as Parameters<typeof vmApi.list>[0]),
    // Keep previous data visible while fetching next page (no flash)
    placeholderData: (prev) => prev,
    // Cache for 30s — WS events will invalidate sooner if needed
    staleTime: 30_000,
  });

  const { data: hypervisorsData } = useQuery({
    queryKey: ["hypervisors-list"],
    queryFn: () => hypervisorApi.list({ page: 1, page_size: 100 }),
    staleTime: 60_000, // hypervisor list changes rarely
  });

  const hypervisorMap = new Map<string, Hypervisor>(
    (hypervisorsData?.data ?? []).map((h: Hypervisor) => [h.id, h])
  );

  // ── Single VM action mutation ─────────────────────────────────────────────
  const action = useMutation({
    mutationFn: ({ id, act }: { id: string; act: PowerAction }) => {
      if (act === "power-on") return vmApi.powerOn(id);
      if (act === "power-off") return vmApi.powerOff(id);
      if (act === "reboot") return vmApi.reboot(id);
      return vmApi.suspend(id);
    },
    onSuccess: (data, { id, act }) => {
      upsertTask({ id: data.task_id, status: "pending", progress: 0, type: `vm.${act.replace(/-/g, "_")}` } as never);
      openTaskDrawer();
      addToast("success", `${ACTION_LABELS[act]} task queued`);
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ["vms"] });
        setPendingActions((prev) => { const n = { ...prev }; delete n[id]; return n; });
      }, 4000);
    },
    onError: (err: Error, { id, act }) => {
      setPendingActions((prev) => { const n = { ...prev }; delete n[id]; return n; });
      addToast("error", `${ACTION_LABELS[act]} failed: ${err.message}`);
    },
  });

  // ── Bulk action mutation ──────────────────────────────────────────────────
  const [bulkPending, setBulkPending] = useState(false);

  const handleBulkAction = useCallback(async (bulkAction: BulkAction, snapshotName?: string) => {
    const ids = Array.from(selectedVMIds);
    if (!ids.length) return;
    setBulkPending(true);
    try {
      let result: { task_id: string; vm_count: number } | undefined;
      if (bulkAction === "power-on")  result = await vmApi.bulkPowerOn(ids);
      if (bulkAction === "power-off") result = await vmApi.bulkPowerOff(ids);
      if (bulkAction === "reboot")    result = await vmApi.bulkReboot(ids);
      if (bulkAction === "snapshot" && snapshotName) result = await vmApi.bulkSnapshot(ids, snapshotName);
      if (result) {
        upsertTask({ id: result.task_id, status: "pending", progress: 0, type: `vm.bulk.${bulkAction.replace(/-/g, "_")}` } as never);
        openTaskDrawer();
        addToast("success", `Bulk ${bulkAction} queued for ${result.vm_count} VMs`);
        setSelectedVMIds(new Set());
        setTimeout(() => queryClient.invalidateQueries({ queryKey: ["vms"] }), 3000);
      }
    } catch (err) {
      addToast("error", `Bulk ${bulkAction} failed: ${(err as Error).message}`);
    } finally {
      setBulkPending(false);
    }
  }, [selectedVMIds, upsertTask, openTaskDrawer, addToast, queryClient]);

  // ── Sync mutation ─────────────────────────────────────────────────────────
  const syncMut = useMutation({
    mutationFn: (hypervisorId: string) => hypervisorApi.syncInventory(hypervisorId),
    onSuccess: (data, hypervisorId) => {
      if (data?.task_id) {
        const hv = hypervisorsData?.data?.find((h: Hypervisor) => h.id === hypervisorId);
        trackSync(data.task_id, hypervisorId, hv?.name);
        upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "inventory.sync" } as never);
        openTaskDrawer();
      }
    },
  });

  const triggerAction = useCallback((vm: VM, act: PowerAction) => {
    if (CONFIRM_ACTIONS.has(act)) {
      setConfirm({ vmId: vm.id, vmName: vm.name, action: act });
    } else {
      setPendingActions((prev) => ({ ...prev, [vm.id]: act }));
      action.mutate({ id: vm.id, act });
    }
  }, [action]);

  const handleConfirm = useCallback(() => {
    if (!confirm) return;
    const { vmId, action: act } = confirm;
    setConfirm(null);
    setPendingActions((prev) => ({ ...prev, [vmId]: act }));
    action.mutate({ id: vmId, act });
  }, [confirm, action]);

  // ── Selection helpers ─────────────────────────────────────────────────────
  const vms = data?.data ?? [];
  const hypervisors: Hypervisor[] = hypervisorsData?.data ?? [];
  // Search is now server-side — no client-side filtering needed
  const filtered = vms;

  const allSelected = filtered.length > 0 && filtered.every((v: VM) => selectedVMIds.has(v.id));
  const someSelected = filtered.some((v: VM) => selectedVMIds.has(v.id));

  const toggleAll = () => {
    if (allSelected) {
      setSelectedVMIds((prev) => { const n = new Set(prev); filtered.forEach((v: VM) => n.delete(v.id)); return n; });
    } else {
      setSelectedVMIds((prev) => { const n = new Set(prev); filtered.forEach((v: VM) => n.add(v.id)); return n; });
    }
  };

  const toggleVM = (id: string) => {
    setSelectedVMIds((prev) => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });
  };

  return (
    <div className="space-y-5">
      {confirm && (
        <ConfirmDialog open title={`${ACTION_LABELS[confirm.action]} — ${confirm.vmName}`}
          description={confirm.action === "power-off"
            ? `This will hard-stop "${confirm.vmName}". Any unsaved work in the guest OS will be lost.`
            : `This will reboot "${confirm.vmName}". The guest OS will receive a graceful reboot signal.`}
          confirmLabel={ACTION_LABELS[confirm.action]}
          confirmClass={confirm.action === "power-off" ? "bg-red-600 hover:bg-red-700" : "bg-yellow-600 hover:bg-yellow-700"}
          onConfirm={handleConfirm} onCancel={() => setConfirm(null)} />
      )}
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Virtual Machines</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.meta?.total_items ?? 0} total</p>
        </div>
        <div className="flex items-center gap-2">
          {hypervisors.length > 0 && (
            <select defaultValue="" onChange={(e) => { const id = e.target.value; if (id) { syncMut.mutate(id); e.target.value = ""; } }}
              className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
              <option value="">Sync inventory…</option>
              {hypervisors.map((h) => <option key={h.id} value={h.id}>{h.name} ({h.provider})</option>)}
            </select>
          )}
          <button onClick={() => queryClient.invalidateQueries({ queryKey: ["vms"] })}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            <RefreshCw className="w-4 h-4" />Refresh
          </button>
        </div>
      </div>

      <SyncProgressPanel syncs={syncs} />

      {/* Bulk toolbar */}
      <BulkActionsToolbar
        selectedCount={selectedVMIds.size}
        onClearSelection={() => setSelectedVMIds(new Set())}
        onAction={handleBulkAction}
        isPending={bulkPending}
      />

      {/* Filters */}
      <div className="space-y-2">
        <div className="flex items-center gap-3 flex-wrap">
          <input type="text" placeholder="Search by name, MOR, or UUID…" value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full max-w-sm px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
          {hypervisors.length > 1 && (
            <select value={hypervisorFilter} onChange={(e) => { setHypervisorFilter(e.target.value); setPage(1); }}
              className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
              <option value="">All hypervisors</option>
              {hypervisors.map((h) => <option key={h.id} value={h.id}>{h.name}</option>)}
            </select>
          )}
        </div>
        <TagFilter selectedTagIds={tagFilter} onChange={(ids) => { setTagFilter(ids); setPage(1); }} />
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm min-w-[1200px]">
            <thead>
              <tr className="border-b border-gray-800">
                {/* Select-all checkbox */}
                <th className="px-4 py-3 w-10">
                  <input type="checkbox" checked={allSelected} ref={(el) => { if (el) el.indeterminate = someSelected && !allSelected; }}
                    onChange={toggleAll}
                    className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500 focus:ring-blue-500 focus:ring-offset-gray-900 cursor-pointer" />
                </th>
                {["Name / UUID", "Provider", "Status", "vCPU", "RAM", "Disk", "IP", "Cluster / Host", "Datastore", "OS", "Tools", "Tags", "Actions"].map((h) => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 14 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-20" /></td>
                    ))}
                  </tr>
                ))
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={14} className="px-4 py-12 text-center text-gray-500">
                    {search || tagFilter.length ? "No VMs match your filters" : "No VMs found. Add a hypervisor and sync inventory to populate this list."}
                  </td>
                </tr>
              ) : (
                filtered.map((vm: VM) => {
                  const hv = hypervisorMap.get(vm.hypervisor_id);
                  const esxiHost = metaStr(vm, "esxi_host");
                  const cluster = metaStr(vm, "cluster");
                  const datastore = metaStr(vm, "datastore");
                  const proxmoxNode = metaStr(vm, "node");
                  const uuid = metaStr(vm, "uuid");
                  const allowed = allowedActions(vm.status);
                  const busyAction = pendingActions[vm.id];
                  const isBusy = !!busyAction;
                  const isSelected = selectedVMIds.has(vm.id);

                  return (
                    <tr key={vm.id} className={`border-b border-gray-800/50 transition-colors ${isSelected ? "bg-blue-950/20" : "hover:bg-gray-800/30"}`}>
                      {/* Checkbox */}
                      <td className="px-4 py-3">
                        <input type="checkbox" checked={isSelected} onChange={() => toggleVM(vm.id)}
                          className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500 focus:ring-blue-500 focus:ring-offset-gray-900 cursor-pointer" />
                      </td>

                      {/* Name / UUID */}
                      <td className="px-4 py-3">
                        <div>
                          <p className="font-medium text-white">{vm.name}</p>
                          {uuid ? (
                            <p className="text-[10px] text-gray-500 font-mono truncate max-w-[160px]" title={uuid}>{uuid}</p>
                          ) : proxmoxNode ? (
                            <p className="text-[10px] text-gray-500 font-mono">vmid:{vm.metadata?.vmid ?? vm.provider_vm_id}</p>
                          ) : (
                            <p className="text-[10px] text-gray-600 font-mono">{vm.provider_vm_id}</p>
                          )}
                        </div>
                      </td>
                      {/* Provider */}
                      <td className="px-4 py-3">{hv ? <ProviderBadge provider={hv.provider} /> : <span className="text-gray-600 text-xs">—</span>}</td>
                      {/* Status */}
                      <td className="px-4 py-3"><VMStatusBadge status={vm.status} /></td>
                      {/* vCPU */}
                      <td className="px-4 py-3 text-gray-300"><span className="flex items-center gap-1"><Cpu className="w-3 h-3 text-gray-500" />{vm.cpu_count}</span></td>
                      {/* RAM */}
                      <td className="px-4 py-3 text-gray-300">{formatBytes(vm.memory_mb)}</td>
                      {/* Disk */}
                      <td className="px-4 py-3 text-gray-300">{vm.disk_gb > 0 ? `${vm.disk_gb} GB` : "—"}</td>
                      {/* IP */}
                      <td className="px-4 py-3 text-gray-300 font-mono text-xs">
                        {vm.ip_addresses?.[0] ?? "—"}
                        {vm.ip_addresses && vm.ip_addresses.length > 1 && <span className="text-gray-500 ml-1">+{vm.ip_addresses.length - 1}</span>}
                      </td>
                      {/* Cluster / Host */}
                      <td className="px-4 py-3">
                        <div className="space-y-0.5">
                          {cluster && <div className="flex items-center gap-1 text-xs text-gray-300"><Server className="w-3 h-3 text-blue-400 shrink-0" /><span className="truncate max-w-[120px]" title={cluster}>{cluster}</span></div>}
                          {esxiHost && <div className="flex items-center gap-1 text-xs text-gray-500"><Database className="w-3 h-3 shrink-0" /><span className="truncate max-w-[120px]" title={esxiHost}>{esxiHost}</span></div>}
                          {proxmoxNode && !cluster && !esxiHost && <div className="flex items-center gap-1 text-xs text-gray-300"><Server className="w-3 h-3 text-orange-400 shrink-0" /><span className="truncate max-w-[120px]">{proxmoxNode}</span></div>}
                          {!cluster && !esxiHost && !proxmoxNode && <span className="text-gray-600 text-xs">—</span>}
                        </div>
                      </td>
                      {/* Datastore */}
                      <td className="px-4 py-3 text-gray-400 text-xs truncate max-w-[120px]" title={datastore}>{datastore || "—"}</td>
                      {/* OS */}
                      <td className="px-4 py-3 text-gray-400 text-xs truncate max-w-[120px]" title={vm.guest_os}>{vm.guest_os || "—"}</td>
                      {/* Tools */}
                      <td className="px-4 py-3"><VMToolsBadge status={vm.tools_status} /></td>
                      {/* Tags */}
                      <td className="px-4 py-3">
                        <div className="flex flex-wrap gap-1 max-w-[160px]">
                          {(vm.tag_objects ?? []).slice(0, 3).map((tag) => (
                            <TagBadge key={tag.id} tag={tag} />
                          ))}
                          {(vm.tag_objects ?? []).length > 3 && (
                            <span className="text-[10px] text-gray-500">+{(vm.tag_objects ?? []).length - 3}</span>
                          )}
                          {!(vm.tag_objects ?? []).length && <span className="text-gray-600 text-xs">—</span>}
                        </div>
                      </td>

                      {/* Actions */}
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1">
                          <Link href={`/dashboard/vms/${vm.id}`} title="View Details"
                            className="p-1.5 rounded-lg transition-colors hover:bg-gray-700 text-gray-400 hover:text-white">
                            <ExternalLink className="w-3.5 h-3.5" />
                          </Link>
                          <ActionBtn icon={isBusy && busyAction === "power-on" ? Loader2 : Power}
                            title={allowed.has("power-on") ? "Power On" : "VM is already running"}
                            onClick={() => triggerAction(vm, "power-on")} color="text-green-400"
                            disabled={!allowed.has("power-on") || isBusy} spinning={isBusy && busyAction === "power-on"} />
                          <ActionBtn icon={isBusy && busyAction === "power-off" ? Loader2 : Power}
                            title={allowed.has("power-off") ? "Power Off" : "VM is already stopped"}
                            onClick={() => triggerAction(vm, "power-off")} color="text-red-400"
                            disabled={!allowed.has("power-off") || isBusy} spinning={isBusy && busyAction === "power-off"} />
                          <ActionBtn icon={isBusy && busyAction === "reboot" ? Loader2 : RotateCcw}
                            title={allowed.has("reboot") ? "Reboot" : "VM must be running to reboot"}
                            onClick={() => triggerAction(vm, "reboot")} color="text-yellow-400"
                            disabled={!allowed.has("reboot") || isBusy} spinning={isBusy && busyAction === "reboot"} />
                          <ActionBtn icon={isBusy && busyAction === "suspend" ? Loader2 : PauseCircle}
                            title={allowed.has("suspend") ? "Suspend" : "VM must be running to suspend"}
                            onClick={() => triggerAction(vm, "suspend")} color="text-orange-400"
                            disabled={!allowed.has("suspend") || isBusy} spinning={isBusy && busyAction === "suspend"} />
                          <ActionBtn icon={Monitor} title="Open Console"
                            onClick={async () => { try { const s = await vmApi.getConsole(vm.id); window.open(s.url, "_blank"); } catch { addToast("error", "Failed to open console session"); } }}
                            color="text-blue-400" disabled={vm.status !== "running"} />
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {data && (data.meta?.total_pages ?? 0) > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {data.meta?.total_pages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Previous</button>
              <button disabled={page === (data.meta?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Next</button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
