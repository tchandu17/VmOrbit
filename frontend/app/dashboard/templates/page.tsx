"use client";
import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  RefreshCw, Server, Cpu, HardDrive, MemoryStick,
  Search, ChevronLeft, ChevronRight, CheckCircle2,
  XCircle, Loader2, X, Copy,
} from "lucide-react";
import { templateApi } from "@/lib/api/templates";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { formatBytes } from "@/lib/utils";
import type { VMTemplate, Hypervisor } from "@/types";
import ProvisionWizard from "@/components/provisioning/ProvisionWizard";

// ── Toast ─────────────────────────────────────────────────────────────────────
type ToastType = "success" | "error" | "info";
interface Toast { id: number; type: ToastType; message: string }
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

export default function TemplatesPage() {
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, addToast, removeToast } = useToast();

  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [hypervisorFilter, setHypervisorFilter] = useState("");
  const [provisionTarget, setProvisionTarget] = useState<VMTemplate | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["templates", { page, hypervisorFilter }],
    queryFn: () => templateApi.list({ page, page_size: 20, hypervisor_id: hypervisorFilter || undefined }),
  });

  const { data: hypervisorsData } = useQuery({
    queryKey: ["hypervisors-list"],
    queryFn: () => hypervisorApi.list({ page: 1, page_size: 100 }),
  });

  const hypervisors: Hypervisor[] = hypervisorsData?.data ?? [];
  const hypervisorMap = new Map<string, Hypervisor>(hypervisors.map((h) => [h.id, h]));

  const syncMut = useMutation({
    mutationFn: (hypervisorId: string) => templateApi.syncTemplates(hypervisorId),
    onSuccess: (data) => {
      if (data?.task_id) {
        upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "template.sync" } as never);
        openTaskDrawer();
        addToast("success", "Template sync started");
        setTimeout(() => queryClient.invalidateQueries({ queryKey: ["templates"] }), 5000);
      }
    },
    onError: (err: Error) => addToast("error", `Sync failed: ${err.message}`),
  });

  const templates: VMTemplate[] = data?.data ?? [];
  const filtered = search
    ? templates.filter((t) =>
        t.name.toLowerCase().includes(search.toLowerCase()) ||
        t.guest_os?.toLowerCase().includes(search.toLowerCase()) ||
        t.provider_id.toLowerCase().includes(search.toLowerCase())
      )
    : templates;

  return (
    <div className="space-y-5">
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Provision Wizard */}
      {provisionTarget && (
        <ProvisionWizard
          template={provisionTarget}
          hypervisors={hypervisors}
          onClose={() => setProvisionTarget(null)}
          onSuccess={(job) => {
            setProvisionTarget(null);
            addToast("success", `Provisioning "${job.vm_name}" started`);
            if (job.task_id) {
              upsertTask({ id: job.task_id, status: "pending", progress: 0, type: "vm.provision" } as never);
              openTaskDrawer();
            }
          }}
        />
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">VM Templates</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.meta?.total_items ?? 0} templates</p>
        </div>
        <div className="flex items-center gap-2">
          {hypervisors.length > 0 && (
            <select
              defaultValue=""
              onChange={(e) => { const id = e.target.value; if (id) { syncMut.mutate(id); e.target.value = ""; } }}
              className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
            >
              <option value="">Sync templates…</option>
              {hypervisors.map((h) => <option key={h.id} value={h.id}>{h.name} ({h.provider})</option>)}
            </select>
          )}
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ["templates"] })}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
          >
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            type="text"
            placeholder="Search templates…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 pr-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 w-64"
          />
        </div>
        {hypervisors.length > 1 && (
          <select
            value={hypervisorFilter}
            onChange={(e) => { setHypervisorFilter(e.target.value); setPage(1); }}
            className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
          >
            <option value="">All hypervisors</option>
            {hypervisors.map((h) => <option key={h.id} value={h.id}>{h.name}</option>)}
          </select>
        )}
      </div>

      {/* Template Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 space-y-3 animate-pulse">
              <div className="h-5 bg-gray-800 rounded w-3/4" />
              <div className="h-4 bg-gray-800 rounded w-1/2" />
              <div className="h-4 bg-gray-800 rounded w-full" />
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-12 text-center">
          <Server className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400 text-sm">
            {search ? "No templates match your search." : "No templates found. Use the sync button to discover templates from your hypervisors."}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filtered.map((tmpl) => {
            const hv = hypervisorMap.get(tmpl.hypervisor_id);
            return (
              <div key={tmpl.id} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 flex flex-col gap-3 hover:border-gray-700 transition-colors">
                {/* Header */}
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="font-semibold text-white text-sm truncate" title={tmpl.name}>{tmpl.name}</p>
                    {tmpl.description && (
                      <p className="text-xs text-gray-500 truncate mt-0.5" title={tmpl.description}>{tmpl.description}</p>
                    )}
                  </div>
                  {hv && <ProviderBadge provider={hv.provider} />}
                </div>

                {/* OS */}
                {tmpl.guest_os && (
                  <p className="text-xs text-gray-400 truncate">{tmpl.guest_os}</p>
                )}

                {/* Specs */}
                <div className="grid grid-cols-3 gap-2 text-xs text-gray-400">
                  <div className="flex items-center gap-1">
                    <Cpu className="w-3 h-3 text-blue-400 shrink-0" />
                    <span>{tmpl.cpu_count} vCPU</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <MemoryStick className="w-3 h-3 text-purple-400 shrink-0" />
                    <span>{formatBytes(tmpl.memory_mb)}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <HardDrive className="w-3 h-3 text-green-400 shrink-0" />
                    <span>{tmpl.disk_gb > 0 ? `${tmpl.disk_gb} GB` : "—"}</span>
                  </div>
                </div>

                {/* Hypervisor */}
                {hv && (
                  <p className="text-xs text-gray-500 truncate">{hv.name}</p>
                )}

                {/* Tags */}
                {tmpl.tags && tmpl.tags.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {tmpl.tags.slice(0, 3).map((tag) => (
                      <span key={tag} className="px-1.5 py-0.5 bg-gray-800 text-gray-400 rounded text-[10px]">{tag}</span>
                    ))}
                    {tmpl.tags.length > 3 && <span className="text-[10px] text-gray-500">+{tmpl.tags.length - 3}</span>}
                  </div>
                )}

                {/* Action */}
                <button
                  onClick={() => setProvisionTarget(tmpl)}
                  className="mt-auto flex items-center justify-center gap-2 w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
                >
                  <Copy className="w-3.5 h-3.5" /> Provision VM
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination */}
      {data && (data.meta?.total_pages ?? 0) > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-xs text-gray-500">Page {page} of {data.meta?.total_pages}</span>
          <div className="flex gap-2">
            <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
              className="flex items-center gap-1 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
              <ChevronLeft className="w-3.5 h-3.5" /> Previous
            </button>
            <button disabled={page === (data.meta?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
              className="flex items-center gap-1 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
              Next <ChevronRight className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
