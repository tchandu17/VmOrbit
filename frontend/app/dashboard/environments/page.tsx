"use client";
import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import {
  Plus, RefreshCw, Layers, Play, Square, RotateCcw, Camera,
  Copy, Trash2, X, AlertTriangle, Loader2, CheckCircle2,
  XCircle, ChevronRight, Server,
} from "lucide-react";
import { environmentApi } from "@/lib/api/environments";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import type {
  Environment, EnvironmentType, EnvironmentStatus,
  CreateEnvironmentPayload,
} from "@/types";

// ── Constants ─────────────────────────────────────────────────────────────────
const ENV_TYPES: { value: EnvironmentType; label: string; color: string }[] = [
  { value: "production",       label: "Production",        color: "#ef4444" },
  { value: "staging",          label: "Staging",           color: "#f97316" },
  { value: "development",      label: "Development",       color: "#3b82f6" },
  { value: "qa",               label: "QA",                color: "#8b5cf6" },
  { value: "disaster_recovery",label: "Disaster Recovery", color: "#6b7280" },
  { value: "custom",           label: "Custom",            color: "#10b981" },
];

const STATUS_CONFIG: Record<EnvironmentStatus, { label: string; cls: string }> = {
  healthy:   { label: "Healthy",   cls: "bg-green-900/40 text-green-400 border-green-800" },
  degraded:  { label: "Degraded",  cls: "bg-yellow-900/40 text-yellow-400 border-yellow-800" },
  unhealthy: { label: "Unhealthy", cls: "bg-red-900/40 text-red-400 border-red-800" },
  unknown:   { label: "Unknown",   cls: "bg-gray-800 text-gray-400 border-gray-700" },
  starting:  { label: "Starting",  cls: "bg-blue-900/40 text-blue-400 border-blue-800" },
  stopping:  { label: "Stopping",  cls: "bg-orange-900/40 text-orange-400 border-orange-800" },
};

function StatusBadge({ status }: { status: EnvironmentStatus }) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.unknown;
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-md border text-xs font-medium ${cfg.cls}`}>
      {cfg.label}
    </span>
  );
}

function TypeBadge({ type }: { type: EnvironmentType }) {
  const cfg = ENV_TYPES.find((t) => t.value === type) ?? ENV_TYPES[ENV_TYPES.length - 1];
  return (
    <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-gray-800 text-xs text-gray-300 font-medium">
      <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: cfg.color }} />
      {cfg.label}
    </span>
  );
}

// ── Toast ─────────────────────────────────────────────────────────────────────
type ToastType = "success" | "error" | "info";
interface Toast { id: number; type: ToastType; message: string }
let toastCounter = 0;
function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const add = useCallback((type: ToastType, message: string) => {
    const id = ++toastCounter;
    setToasts((p) => [...p, { id, type, message }]);
    setTimeout(() => setToasts((p) => p.filter((t) => t.id !== id)), 4000);
  }, []);
  const remove = useCallback((id: number) => setToasts((p) => p.filter((t) => t.id !== id)), []);
  return { toasts, add, remove };
}
function ToastContainer({ toasts, onRemove }: { toasts: Toast[]; onRemove: (id: number) => void }) {
  if (!toasts.length) return null;
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((t) => (
        <div key={t.id} className={`flex items-start gap-3 px-4 py-3 rounded-xl shadow-lg border text-sm ${
          t.type === "success" ? "bg-green-950 border-green-800 text-green-200"
          : t.type === "error" ? "bg-red-950 border-red-800 text-red-200"
          : "bg-gray-800 border-gray-700 text-gray-200"}`}>
          {t.type === "success" ? <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0 mt-0.5" />
            : t.type === "error" ? <XCircle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" /> : null}
          <span className="flex-1">{t.message}</span>
          <button onClick={() => onRemove(t.id)}><X className="w-3.5 h-3.5 text-gray-500 hover:text-gray-300" /></button>
        </div>
      ))}
    </div>
  );
}

// ── Create Modal ──────────────────────────────────────────────────────────────
function CreateModal({ onClose, onSubmit, isPending, error }: {
  onClose: () => void; onSubmit: (p: CreateEnvironmentPayload) => void;
  isPending: boolean; error?: string;
}) {
  const [form, setForm] = useState<CreateEnvironmentPayload>({
    name: "", description: "", type: "custom", color: "#3b82f6", tags: [],
  });
  const [tagInput, setTagInput] = useState("");
  const set = <K extends keyof CreateEnvironmentPayload>(k: K, v: CreateEnvironmentPayload[K]) =>
    setForm((f) => ({ ...f, [k]: v }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-lg shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-lg font-semibold text-white">Create Environment</h2>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400"><X className="w-5 h-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Name *</label>
            <input value={form.name} onChange={(e) => set("name", e.target.value)} required
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              placeholder="Production" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Type</label>
              <select value={form.type} onChange={(e) => set("type", e.target.value as EnvironmentType)}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 focus:outline-none focus:border-blue-500">
                {ENV_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Color</label>
              <div className="flex items-center gap-2">
                <input type="color" value={form.color ?? "#3b82f6"} onChange={(e) => set("color", e.target.value)}
                  className="w-10 h-9 rounded-lg border border-gray-700 bg-gray-800 cursor-pointer p-0.5" />
                <input value={form.color ?? ""} onChange={(e) => set("color", e.target.value)}
                  className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500"
                  placeholder="#3b82f6" />
              </div>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
            <textarea value={form.description ?? ""} onChange={(e) => set("description", e.target.value)} rows={2}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 resize-none"
              placeholder="Optional description…" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Tags</label>
            <div className="flex gap-2">
              <input value={tagInput} onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); if (tagInput.trim()) { set("tags", [...(form.tags ?? []), tagInput.trim()]); setTagInput(""); } } }}
                className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
                placeholder="Add tag and press Enter" />
            </div>
            {(form.tags ?? []).length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {(form.tags ?? []).map((t) => (
                  <span key={t} className="inline-flex items-center gap-1 px-2 py-0.5 bg-gray-800 text-gray-300 rounded-md text-xs">
                    {t}<button type="button" onClick={() => set("tags", (form.tags ?? []).filter((x) => x !== t))}><X className="w-3 h-3" /></button>
                  </span>
                ))}
              </div>
            )}
          </div>
          {error && <p className="text-sm text-red-400 bg-red-900/20 border border-red-800/50 rounded-lg px-3 py-2">{error}</p>}
          <div className="flex gap-3 pt-1">
            <button type="submit" disabled={isPending}
              className="flex items-center gap-2 px-5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium">
              {isPending && <Loader2 className="w-4 h-4 animate-spin" />}
              {isPending ? "Creating…" : "Create Environment"}
            </button>
            <button type="button" onClick={onClose} className="px-5 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Environment Card ──────────────────────────────────────────────────────────
function EnvironmentCard({ env, onStart, onStop, onRestart, onDelete, isActing }: {
  env: Environment;
  onStart: () => void; onStop: () => void; onRestart: () => void; onDelete: () => void;
  isActing: boolean;
}) {
  const vmCount = env.vms?.length ?? 0;
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5 hover:border-gray-700 transition-colors group">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: env.color || "#3b82f6" }} />
          <div className="min-w-0">
            <h3 className="font-semibold text-white truncate">{env.name}</h3>
            {env.description && <p className="text-xs text-gray-500 truncate mt-0.5">{env.description}</p>}
          </div>
        </div>
        <StatusBadge status={env.status} />
      </div>

      <div className="flex items-center gap-2 mb-4">
        <TypeBadge type={env.type} />
        <span className="text-xs text-gray-500 flex items-center gap-1">
          <Server className="w-3 h-3" />{vmCount} VM{vmCount !== 1 ? "s" : ""}
        </span>
      </div>

      {(env.tags ?? []).length > 0 && (
        <div className="flex flex-wrap gap-1 mb-4">
          {env.tags.slice(0, 4).map((t) => (
            <span key={t} className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 text-xs">{t}</span>
          ))}
          {env.tags.length > 4 && <span className="text-xs text-gray-500">+{env.tags.length - 4}</span>}
        </div>
      )}

      <div className="flex items-center justify-between pt-3 border-t border-gray-800">
        <div className="flex items-center gap-1">
          <button onClick={onStart} disabled={isActing} title="Start all VMs"
            className="p-1.5 rounded-lg hover:bg-green-900/30 text-gray-400 hover:text-green-400 disabled:opacity-40 transition-colors">
            {isActing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
          </button>
          <button onClick={onStop} disabled={isActing} title="Stop all VMs"
            className="p-1.5 rounded-lg hover:bg-red-900/30 text-gray-400 hover:text-red-400 disabled:opacity-40 transition-colors">
            <Square className="w-4 h-4" />
          </button>
          <button onClick={onRestart} disabled={isActing} title="Restart all VMs"
            className="p-1.5 rounded-lg hover:bg-yellow-900/30 text-gray-400 hover:text-yellow-400 disabled:opacity-40 transition-colors">
            <RotateCcw className="w-4 h-4" />
          </button>
          <button onClick={onDelete} disabled={isActing} title="Delete environment"
            className="p-1.5 rounded-lg hover:bg-red-900/30 text-gray-400 hover:text-red-400 disabled:opacity-40 transition-colors">
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
        <Link href={`/dashboard/environments/${env.id}`}
          className="flex items-center gap-1 text-xs text-gray-400 hover:text-blue-400 transition-colors">
          Manage <ChevronRight className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────
export default function EnvironmentsPage() {
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, add: addToast, remove: removeToast } = useToast();
  const [showCreate, setShowCreate] = useState(false);
  const [actingIds, setActingIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["environments", { search, typeFilter }],
    queryFn: () => environmentApi.list({ search: search || undefined, type: typeFilter || undefined, page: 1, page_size: 50 }),
  });

  const environments: Environment[] = data?.data ?? [];
  const filtered = environments.filter((e) =>
    (!search || e.name.toLowerCase().includes(search.toLowerCase())) &&
    (!typeFilter || e.type === typeFilter)
  );

  const createMut = useMutation({
    mutationFn: environmentApi.create,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["environments"] }); setShowCreate(false); addToast("success", "Environment created"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const deleteMut = useMutation({
    mutationFn: environmentApi.delete,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["environments"] }); addToast("success", "Environment deleted"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const orchestrate = useCallback(async (envId: string, op: "start" | "stop" | "restart") => {
    setActingIds((s) => new Set(s).add(envId));
    try {
      let result: { task_id: string } | undefined;
      if (op === "start")   result = await environmentApi.start(envId);
      if (op === "stop")    result = await environmentApi.stop(envId);
      if (op === "restart") result = await environmentApi.restart(envId);
      if (result?.task_id) {
        upsertTask({ id: result.task_id, status: "pending", progress: 0, type: `env.${op}` } as never);
        openTaskDrawer();
        addToast("success", `Environment ${op} queued`);
        setTimeout(() => queryClient.invalidateQueries({ queryKey: ["environments"] }), 3000);
      }
    } catch (e) {
      addToast("error", `${op} failed: ${(e as Error).message}`);
    } finally {
      setActingIds((s) => { const n = new Set(s); n.delete(envId); return n; });
    }
  }, [upsertTask, openTaskDrawer, addToast, queryClient]);

  return (
    <div className="space-y-5">
      <ToastContainer toasts={toasts} onRemove={removeToast} />
      {showCreate && (
        <CreateModal onClose={() => setShowCreate(false)} onSubmit={createMut.mutate}
          isPending={createMut.isPending} error={createMut.error?.message} />
      )}

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2"><Layers className="w-6 h-6 text-blue-400" />Environments</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.meta?.total_items ?? 0} environments</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => queryClient.invalidateQueries({ queryKey: ["environments"] })}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm">
            <RefreshCw className="w-4 h-4" />Refresh
          </button>
          <button onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium">
            <Plus className="w-4 h-4" />New Environment
          </button>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search environments…"
          className="w-full max-w-sm px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
        <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="">All types</option>
          {ENV_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
        </select>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 h-44 animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-16 text-center">
          <Layers className="w-12 h-12 mx-auto mb-3 text-gray-700" />
          <p className="text-gray-400 font-medium">No environments yet</p>
          <p className="text-gray-600 text-sm mt-1">Create an environment to group and orchestrate your VMs as application stacks.</p>
          <button onClick={() => setShowCreate(true)} className="mt-4 flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium mx-auto">
            <Plus className="w-4 h-4" />Create First Environment
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((env) => (
            <EnvironmentCard key={env.id} env={env}
              isActing={actingIds.has(env.id)}
              onStart={() => orchestrate(env.id, "start")}
              onStop={() => orchestrate(env.id, "stop")}
              onRestart={() => orchestrate(env.id, "restart")}
              onDelete={() => { if (confirm(`Delete environment "${env.name}"?`)) deleteMut.mutate(env.id); }}
            />
          ))}
        </div>
      )}
    </div>
  );
}
