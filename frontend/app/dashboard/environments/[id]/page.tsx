"use client";
import { useState, useCallback, use } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import {
  ArrowLeft, Play, Square, RotateCcw, Camera, Copy, RefreshCw,
  Plus, Trash2, X, Loader2, CheckCircle2, XCircle, AlertTriangle,
  Server, GitBranch, Activity, ChevronRight, ArrowRight, Clock,
  Layers,
} from "lucide-react";
import { environmentApi } from "@/lib/api/environments";
import { vmApi } from "@/lib/api/vms";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { VMStatusBadge } from "@/components/vms/VMStatusBadge";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import type {
  Environment, EnvironmentVM, VMDependency, OrchestrationRun,
  OrchestrationStep, VM, DependencyType, SnapshotEnvironmentPayload,
  CloneEnvironmentPayload, AddVMToEnvironmentPayload,
} from "@/types";

// ── Helpers ───────────────────────────────────────────────────────────────────
const STATUS_COLORS: Record<string, string> = {
  healthy:   "text-green-400", degraded: "text-yellow-400",
  unhealthy: "text-red-400",   unknown:  "text-gray-400",
  starting:  "text-blue-400",  stopping: "text-orange-400",
};
const RUN_STATUS_COLORS: Record<string, string> = {
  pending: "text-gray-400", running: "text-blue-400", completed: "text-green-400",
  failed: "text-red-400", cancelled: "text-gray-500", rolling_back: "text-orange-400", rolled_back: "text-gray-400",
};
const STEP_STATUS_COLORS: Record<string, string> = {
  pending: "text-gray-500", running: "text-blue-400", completed: "text-green-400",
  failed: "text-red-400", skipped: "text-gray-500",
};
const DEP_TYPE_LABELS: Record<DependencyType, string> = {
  start_before: "starts before", stop_after: "stops after", requires: "requires",
};

type ToastType = "success" | "error";
interface Toast { id: number; type: ToastType; message: string }
let tc = 0;
function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const add = useCallback((type: ToastType, message: string) => {
    const id = ++tc;
    setToasts((p) => [...p, { id, type, message }]);
    setTimeout(() => setToasts((p) => p.filter((t) => t.id !== id)), 4000);
  }, []);
  return { toasts, add };
}
function Toasts({ toasts }: { toasts: Toast[] }) {
  if (!toasts.length) return null;
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((t) => (
        <div key={t.id} className={`flex items-center gap-3 px-4 py-3 rounded-xl shadow-lg border text-sm ${
          t.type === "success" ? "bg-green-950 border-green-800 text-green-200" : "bg-red-950 border-red-800 text-red-200"}`}>
          {t.type === "success" ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <XCircle className="w-4 h-4 shrink-0" />}
          {t.message}
        </div>
      ))}
    </div>
  );
}

// ── Add VM Modal ──────────────────────────────────────────────────────────────
function AddVMModal({ envId, existingVMIds, onClose, onAdded }: {
  envId: string; existingVMIds: Set<string>; onClose: () => void; onAdded: () => void;
}) {
  const [form, setForm] = useState<AddVMToEnvironmentPayload>({ vm_id: "", start_order: 0, stop_order: 0, role: "" });
  const { data } = useQuery({ queryKey: ["vms-all"], queryFn: () => vmApi.list({ page: 1, page_size: 200 }) });
  const available = (data?.data ?? []).filter((v: VM) => !existingVMIds.has(v.id));
  const mut = useMutation({
    mutationFn: (p: AddVMToEnvironmentPayload) => environmentApi.addVM(envId, p),
    onSuccess: () => { onAdded(); onClose(); },
  });
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-base font-semibold text-white">Add VM to Environment</h2>
          <button onClick={onClose}><X className="w-5 h-5 text-gray-400" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); mut.mutate(form); }} className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Virtual Machine *</label>
            <select value={form.vm_id} onChange={(e) => setForm((f) => ({ ...f, vm_id: e.target.value }))} required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 focus:outline-none focus:border-blue-500">
              <option value="">Select a VM…</option>
              {available.map((v: VM) => <option key={v.id} value={v.id}>{v.name}</option>)}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Start Order</label>
              <input type="number" value={form.start_order} onChange={(e) => setForm((f) => ({ ...f, start_order: +e.target.value }))}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" min={0} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Stop Order</label>
              <input type="number" value={form.stop_order} onChange={(e) => setForm((f) => ({ ...f, stop_order: +e.target.value }))}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" min={0} />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Role (e.g. database, app, lb)</label>
            <input value={form.role ?? ""} onChange={(e) => setForm((f) => ({ ...f, role: e.target.value }))}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              placeholder="app-server" />
          </div>
          {mut.error && <p className="text-sm text-red-400">{(mut.error as Error).message}</p>}
          <div className="flex gap-3">
            <button type="submit" disabled={mut.isPending || !form.vm_id}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
              {mut.isPending && <Loader2 className="w-4 h-4 animate-spin" />}Add VM
            </button>
            <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Snapshot Modal ────────────────────────────────────────────────────────────
function SnapshotModal({ onClose, onSubmit, isPending }: {
  onClose: () => void; onSubmit: (p: SnapshotEnvironmentPayload) => void; isPending: boolean;
}) {
  const [form, setForm] = useState<SnapshotEnvironmentPayload>({ snapshot_name: "", description: "", memory: false });
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-base font-semibold text-white">Snapshot Environment</h2>
          <button onClick={onClose}><X className="w-5 h-5 text-gray-400" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Snapshot Name *</label>
            <input value={form.snapshot_name} onChange={(e) => setForm((f) => ({ ...f, snapshot_name: e.target.value }))} required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              placeholder="pre-deploy-2024" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
            <input value={form.description ?? ""} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500" />
          </div>
          <label className="flex items-center gap-3 cursor-pointer">
            <input type="checkbox" checked={form.memory} onChange={(e) => setForm((f) => ({ ...f, memory: e.target.checked }))}
              className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500" />
            <span className="text-sm text-gray-300">Include memory state</span>
          </label>
          <div className="flex gap-3">
            <button type="submit" disabled={isPending}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
              {isPending && <Loader2 className="w-4 h-4 animate-spin" />}Snapshot All VMs
            </button>
            <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Clone Modal ───────────────────────────────────────────────────────────────
function CloneModal({ onClose, onSubmit, isPending }: {
  onClose: () => void; onSubmit: (p: CloneEnvironmentPayload) => void; isPending: boolean;
}) {
  const [form, setForm] = useState<CloneEnvironmentPayload>({ new_environment_name: "", name_suffix: "-clone" });
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-base font-semibold text-white">Clone Environment</h2>
          <button onClick={onClose}><X className="w-5 h-5 text-gray-400" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">New Environment Name *</label>
            <input value={form.new_environment_name} onChange={(e) => setForm((f) => ({ ...f, new_environment_name: e.target.value }))} required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              placeholder="Production-Clone" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">VM Name Suffix</label>
            <input value={form.name_suffix ?? ""} onChange={(e) => setForm((f) => ({ ...f, name_suffix: e.target.value }))}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              placeholder="-clone" />
          </div>
          <div className="flex gap-3">
            <button type="submit" disabled={isPending}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
              {isPending && <Loader2 className="w-4 h-4 animate-spin" />}Clone Environment
            </button>
            <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Dependency Graph (visual topology) ───────────────────────────────────────
function DependencyGraph({ envVMs, deps }: { envVMs: EnvironmentVM[]; deps: VMDependency[] }) {
  if (envVMs.length === 0) return <p className="text-gray-500 text-sm">No VMs in this environment.</p>;
  return (
    <div className="space-y-3">
      {envVMs.map((ev, i) => {
        const vmDeps = deps.filter((d) => d.source_vm_id === ev.vm_id);
        return (
          <div key={ev.id} className="flex items-start gap-3">
            <div className="flex flex-col items-center">
              <div className="w-8 h-8 rounded-full bg-blue-900/40 border border-blue-700 flex items-center justify-center text-xs font-bold text-blue-400">
                {i + 1}
              </div>
              {i < envVMs.length - 1 && <div className="w-px h-4 bg-gray-700 mt-1" />}
            </div>
            <div className="flex-1 pb-2">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-medium text-white text-sm">{ev.vm?.name ?? ev.vm_id.slice(0, 8)}</span>
                {ev.role && <span className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 text-xs">{ev.role}</span>}
                {ev.vm && <VMStatusBadge status={ev.vm.status} />}
                <span className="text-xs text-gray-600">start:{ev.start_order} stop:{ev.stop_order}</span>
              </div>
              {vmDeps.length > 0 && (
                <div className="mt-1.5 flex flex-wrap gap-2">
                  {vmDeps.map((d) => {
                    const target = envVMs.find((e) => e.vm_id === d.target_vm_id);
                    return (
                      <span key={d.id} className="flex items-center gap-1 text-xs text-gray-500 bg-gray-800/60 px-2 py-0.5 rounded-md">
                        <ArrowRight className="w-3 h-3 text-blue-500" />
                        {DEP_TYPE_LABELS[d.type]} <span className="text-gray-300">{target?.vm?.name ?? d.target_vm_id.slice(0, 8)}</span>
                        {d.delay_seconds > 0 && <span className="text-gray-600">(+{d.delay_seconds}s)</span>}
                      </span>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── Orchestration Run Panel ───────────────────────────────────────────────────
function RunPanel({ run, steps }: { run: OrchestrationRun; steps: OrchestrationStep[] }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-white capitalize">{run.operation}</span>
          <span className={`text-xs font-medium ${RUN_STATUS_COLORS[run.status] ?? "text-gray-400"}`}>{run.status}</span>
        </div>
        <span className="text-xs text-gray-500">{new Date(run.created_at).toLocaleString()}</span>
      </div>
      <div className="w-full bg-gray-800 rounded-full h-1.5">
        <div className="bg-blue-500 h-1.5 rounded-full transition-all" style={{ width: `${run.progress}%` }} />
      </div>
      <div className="flex items-center gap-4 text-xs text-gray-500">
        <span className="text-green-400">{run.completed_vms} done</span>
        <span className="text-red-400">{run.failed_vms} failed</span>
        <span>{run.total_vms} total</span>
      </div>
      {run.error_message && <p className="text-xs text-red-400 bg-red-900/20 rounded px-2 py-1">{run.error_message}</p>}
      {steps.length > 0 && (
        <div className="space-y-1 pt-1">
          {steps.map((s) => (
            <div key={s.id} className="flex items-center gap-2 text-xs">
              <span className={`w-2 h-2 rounded-full shrink-0 ${
                s.status === "completed" ? "bg-green-500" : s.status === "failed" ? "bg-red-500"
                : s.status === "running" ? "bg-blue-500 animate-pulse" : "bg-gray-600"}`} />
              <span className="text-gray-300 flex-1 truncate">{s.vm?.name ?? s.vm_id.slice(0, 8)}</span>
              <span className={STEP_STATUS_COLORS[s.status] ?? "text-gray-500"}>{s.status}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────
export default function EnvironmentDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, add: addToast } = useToast();
  const [tab, setTab] = useState<"vms" | "topology" | "runs">("vms");
  const [showAddVM, setShowAddVM] = useState(false);
  const [showSnapshot, setShowSnapshot] = useState(false);
  const [showClone, setShowClone] = useState(false);
  const [isActing, setIsActing] = useState(false);
  const [selectedRun, setSelectedRun] = useState<string | null>(null);

  const { data: env, isLoading } = useQuery({
    queryKey: ["environment", id],
    queryFn: () => environmentApi.get(id),
    refetchInterval: 10000,
  });

  const { data: envVMs = [] } = useQuery({
    queryKey: ["environment-vms", id],
    queryFn: () => environmentApi.listVMs(id),
  });

  const { data: deps = [] } = useQuery({
    queryKey: ["environment-deps", id],
    queryFn: () => environmentApi.listDependencies(id),
  });

  const { data: runsData } = useQuery({
    queryKey: ["environment-runs", id],
    queryFn: () => environmentApi.listRuns(id, { page: 1, page_size: 10 }),
    refetchInterval: 5000,
  });
  const runs: OrchestrationRun[] = runsData?.data ?? [];

  const { data: runSteps = [] } = useQuery({
    queryKey: ["run-steps", selectedRun],
    queryFn: () => selectedRun ? environmentApi.getRunSteps(id, selectedRun) : Promise.resolve([]),
    enabled: !!selectedRun,
    refetchInterval: selectedRun ? 3000 : false,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["environment", id] });
    queryClient.invalidateQueries({ queryKey: ["environment-vms", id] });
    queryClient.invalidateQueries({ queryKey: ["environment-runs", id] });
  };

  const removeVMMut = useMutation({
    mutationFn: (vmId: string) => environmentApi.removeVM(id, vmId),
    onSuccess: () => { invalidate(); addToast("success", "VM removed"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const removeDep = useMutation({
    mutationFn: (depId: string) => environmentApi.removeDependency(id, depId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["environment-deps", id] }); addToast("success", "Dependency removed"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const snapshotMut = useMutation({
    mutationFn: (p: SnapshotEnvironmentPayload) => environmentApi.snapshot(id, p),
    onSuccess: (data) => {
      if (data?.task_id) { upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "env.snapshot" } as never); openTaskDrawer(); }
      setShowSnapshot(false); invalidate(); addToast("success", "Snapshot queued");
    },
    onError: (e: Error) => addToast("error", e.message),
  });

  const cloneMut = useMutation({
    mutationFn: (p: CloneEnvironmentPayload) => environmentApi.clone(id, p),
    onSuccess: (data) => {
      if (data?.task_id) { upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "env.clone" } as never); openTaskDrawer(); }
      setShowClone(false); addToast("success", "Clone queued");
    },
    onError: (e: Error) => addToast("error", e.message),
  });

  const orchestrate = useCallback(async (op: "start" | "stop" | "restart") => {
    setIsActing(true);
    try {
      let result: { task_id: string } | undefined;
      if (op === "start")   result = await environmentApi.start(id);
      if (op === "stop")    result = await environmentApi.stop(id);
      if (op === "restart") result = await environmentApi.restart(id);
      if (result?.task_id) {
        upsertTask({ id: result.task_id, status: "pending", progress: 0, type: `env.${op}` } as never);
        openTaskDrawer();
        addToast("success", `Environment ${op} queued`);
        invalidate();
      }
    } catch (e) { addToast("error", `${op} failed: ${(e as Error).message}`); }
    finally { setIsActing(false); }
  }, [id, upsertTask, openTaskDrawer, addToast]);

  const existingVMIds = new Set(envVMs.map((ev: EnvironmentVM) => ev.vm_id));

  if (isLoading) return (
    <div className="flex items-center justify-center h-64">
      <Loader2 className="w-8 h-8 animate-spin text-blue-400" />
    </div>
  );
  if (!env) return <div className="text-gray-400 p-8">Environment not found.</div>;

  const activeRun = runs.find((r) => r.status === "running" || r.status === "pending");

  return (
    <div className="space-y-5">
      <Toasts toasts={toasts} />
      {showAddVM && <AddVMModal envId={id} existingVMIds={existingVMIds} onClose={() => setShowAddVM(false)} onAdded={invalidate} />}
      {showSnapshot && <SnapshotModal onClose={() => setShowSnapshot(false)} onSubmit={snapshotMut.mutate} isPending={snapshotMut.isPending} />}
      {showClone && <CloneModal onClose={() => setShowClone(false)} onSubmit={cloneMut.mutate} isPending={cloneMut.isPending} />}

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <Link href="/dashboard/environments" className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white">
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <div className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: env.color || "#3b82f6" }} />
          <div>
            <h1 className="text-2xl font-bold text-white">{env.name}</h1>
            <p className={`text-sm font-medium ${STATUS_COLORS[env.status] ?? "text-gray-400"}`}>{env.status}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap justify-end">
          <button onClick={() => orchestrate("start")} disabled={isActing}
            className="flex items-center gap-1.5 px-3 py-2 bg-green-700 hover:bg-green-600 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
            {isActing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}Start
          </button>
          <button onClick={() => orchestrate("stop")} disabled={isActing}
            className="flex items-center gap-1.5 px-3 py-2 bg-red-700 hover:bg-red-600 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
            <Square className="w-4 h-4" />Stop
          </button>
          <button onClick={() => orchestrate("restart")} disabled={isActing}
            className="flex items-center gap-1.5 px-3 py-2 bg-yellow-700 hover:bg-yellow-600 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
            <RotateCcw className="w-4 h-4" />Restart
          </button>
          <button onClick={() => setShowSnapshot(true)}
            className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">
            <Camera className="w-4 h-4" />Snapshot
          </button>
          <button onClick={() => setShowClone(true)}
            className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">
            <Copy className="w-4 h-4" />Clone
          </button>
          <button onClick={() => { environmentApi.refreshStatus(id).then(() => invalidate()); }}
            className="p-2 bg-gray-800 hover:bg-gray-700 text-gray-400 rounded-lg">
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Active run banner */}
      {activeRun && (
        <div className="bg-blue-950/40 border border-blue-800 rounded-xl px-4 py-3 flex items-center gap-3">
          <Loader2 className="w-4 h-4 animate-spin text-blue-400 shrink-0" />
          <div className="flex-1">
            <span className="text-sm text-blue-300 font-medium capitalize">{activeRun.operation} in progress</span>
            <div className="w-full bg-blue-900/40 rounded-full h-1 mt-1.5">
              <div className="bg-blue-400 h-1 rounded-full transition-all" style={{ width: `${activeRun.progress}%` }} />
            </div>
          </div>
          <span className="text-xs text-blue-400">{activeRun.completed_vms}/{activeRun.total_vms} VMs</span>
          <button onClick={() => { setTab("runs"); setSelectedRun(activeRun.id); }}
            className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1">
            Details <ChevronRight className="w-3 h-3" />
          </button>
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-800">
        {(["vms", "topology", "runs"] as const).map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium capitalize transition-colors border-b-2 -mb-px ${
              tab === t ? "border-blue-500 text-blue-400" : "border-transparent text-gray-400 hover:text-white"}`}>
            {t === "vms" ? `VMs (${envVMs.length})` : t === "topology" ? "Topology" : `Runs (${runs.length})`}
          </button>
        ))}
      </div>

      {/* VMs Tab */}
      {tab === "vms" && (
        <div className="space-y-3">
          <div className="flex justify-end">
            <button onClick={() => setShowAddVM(true)}
              className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium">
              <Plus className="w-4 h-4" />Add VM
            </button>
          </div>
          {envVMs.length === 0 ? (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-10 text-center">
              <Server className="w-10 h-10 mx-auto mb-3 text-gray-700" />
              <p className="text-gray-400">No VMs in this environment yet.</p>
              <button onClick={() => setShowAddVM(true)} className="mt-3 flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm mx-auto">
                <Plus className="w-4 h-4" />Add First VM
              </button>
            </div>
          ) : (
            <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-800">
                    {["Order", "VM", "Provider", "Status", "Role", "Actions"].map((h) => (
                      <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {[...envVMs].sort((a, b) => a.start_order - b.start_order).map((ev: EnvironmentVM) => (
                    <tr key={ev.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                      <td className="px-4 py-3">
                        <div className="flex flex-col gap-0.5">
                          <span className="text-xs text-gray-400">▲ {ev.start_order}</span>
                          <span className="text-xs text-gray-600">▼ {ev.stop_order}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <Link href={`/dashboard/vms/${ev.vm_id}`} className="font-medium text-white hover:text-blue-400">
                          {ev.vm?.name ?? ev.vm_id.slice(0, 8)}
                        </Link>
                      </td>
                      <td className="px-4 py-3">
                        {ev.vm?.hypervisor && <ProviderBadge provider={ev.vm.hypervisor.provider} />}
                      </td>
                      <td className="px-4 py-3">{ev.vm && <VMStatusBadge status={ev.vm.status} />}</td>
                      <td className="px-4 py-3">
                        {ev.role ? <span className="px-2 py-0.5 rounded bg-gray-800 text-gray-300 text-xs">{ev.role}</span> : <span className="text-gray-600">—</span>}
                      </td>
                      <td className="px-4 py-3">
                        <button onClick={() => { if (confirm("Remove VM from environment?")) removeVMMut.mutate(ev.vm_id); }}
                          className="p-1.5 rounded-lg hover:bg-red-900/30 text-gray-400 hover:text-red-400">
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Topology Tab */}
      {tab === "topology" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <h3 className="text-sm font-semibold text-white mb-4 flex items-center gap-2">
              <GitBranch className="w-4 h-4 text-blue-400" />Startup Order & Dependencies
            </h3>
            <DependencyGraph envVMs={[...envVMs].sort((a, b) => a.start_order - b.start_order)} deps={deps} />
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                <ArrowRight className="w-4 h-4 text-purple-400" />Dependencies ({deps.length})
              </h3>
            </div>
            {deps.length === 0 ? (
              <p className="text-gray-500 text-sm">No dependencies defined. Add dependencies to control startup/shutdown order.</p>
            ) : (
              <div className="space-y-2">
                {deps.map((d: VMDependency) => {
                  const src = envVMs.find((e) => e.vm_id === d.source_vm_id);
                  const tgt = envVMs.find((e) => e.vm_id === d.target_vm_id);
                  return (
                    <div key={d.id} className="flex items-center gap-2 bg-gray-800/50 rounded-lg px-3 py-2">
                      <span className="text-sm text-white font-medium">{src?.vm?.name ?? d.source_vm_id.slice(0, 8)}</span>
                      <span className="text-xs text-gray-500">{DEP_TYPE_LABELS[d.type]}</span>
                      <span className="text-sm text-white font-medium">{tgt?.vm?.name ?? d.target_vm_id.slice(0, 8)}</span>
                      {d.delay_seconds > 0 && <span className="text-xs text-gray-500 flex items-center gap-1"><Clock className="w-3 h-3" />+{d.delay_seconds}s</span>}
                      <button onClick={() => removeDep.mutate(d.id)} className="ml-auto p-1 hover:text-red-400 text-gray-500">
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Runs Tab */}
      {tab === "runs" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <div className="space-y-3">
            <h3 className="text-sm font-semibold text-white flex items-center gap-2">
              <Activity className="w-4 h-4 text-blue-400" />Recent Runs
            </h3>
            {runs.length === 0 ? (
              <p className="text-gray-500 text-sm">No orchestration runs yet.</p>
            ) : (
              runs.map((r) => (
                <button key={r.id} onClick={() => setSelectedRun(r.id === selectedRun ? null : r.id)}
                  className={`w-full text-left bg-gray-900 border rounded-xl p-4 transition-colors ${
                    selectedRun === r.id ? "border-blue-600" : "border-gray-800 hover:border-gray-700"}`}>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-white capitalize">{r.operation}</span>
                    <span className={`text-xs font-medium ${RUN_STATUS_COLORS[r.status] ?? "text-gray-400"}`}>{r.status}</span>
                  </div>
                  <div className="w-full bg-gray-800 rounded-full h-1 mt-2">
                    <div className="bg-blue-500 h-1 rounded-full" style={{ width: `${r.progress}%` }} />
                  </div>
                  <div className="flex items-center gap-3 mt-1.5 text-xs text-gray-500">
                    <span className="text-green-400">{r.completed_vms} done</span>
                    {r.failed_vms > 0 && <span className="text-red-400">{r.failed_vms} failed</span>}
                    <span>{r.total_vms} total</span>
                    <span className="ml-auto">{new Date(r.created_at).toLocaleString()}</span>
                  </div>
                </button>
              ))
            )}
          </div>
          {selectedRun && (
            <div>
              <h3 className="text-sm font-semibold text-white mb-3 flex items-center gap-2">
                <Layers className="w-4 h-4 text-purple-400" />Step Details
              </h3>
              {(() => {
                const run = runs.find((r) => r.id === selectedRun);
                return run ? <RunPanel run={run} steps={runSteps} /> : null;
              })()}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
