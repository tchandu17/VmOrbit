"use client";
import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import {
  Plus, RefreshCw, Plug, Trash2, RotateCcw, Search,
  ChevronLeft, ChevronRight, Edit2, X, Server, CheckCircle2,
  AlertCircle, Loader2, Filter,
} from "lucide-react";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { HypervisorStatusBadge } from "@/components/hypervisors/HypervisorStatusBadge";
import { SyncProgressPanel, useSyncProgress } from "@/components/inventory/SyncProgressPanel";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { relativeTime } from "@/lib/utils";
import type {
  Hypervisor, ProviderType, RegisterHypervisorPayload, UpdateHypervisorPayload,
} from "@/types";

// ── Constants ─────────────────────────────────────────────────────────────────

const PROVIDERS: { value: ProviderType; label: string; defaultPort: number }[] = [
  { value: "vmware",  label: "VMware vCenter", defaultPort: 443  },
  { value: "esxi",    label: "VMware ESXi",    defaultPort: 443  },
  { value: "proxmox", label: "Proxmox VE",     defaultPort: 8006 },
  { value: "kvm",     label: "KVM / QEMU",     defaultPort: 22   },
  { value: "hyperv",  label: "Hyper-V",        defaultPort: 5985 },
];

const PROVIDER_LABELS: Record<ProviderType, string> = {
  vmware:  "VMware vCenter",
  esxi:    "VMware ESXi",
  proxmox: "Proxmox VE",
  kvm:     "KVM / QEMU",
  hyperv:  "Hyper-V",
};

const STATUS_FILTERS = ["all", "connected", "disconnected", "error", "unknown"] as const;
type StatusFilter = (typeof STATUS_FILTERS)[number];

const PAGE_SIZE = 12;

// ── Default form state ────────────────────────────────────────────────────────

function defaultForm(): RegisterHypervisorPayload {
  return {
    name: "", description: "", provider: "vmware",
    host: "", port: 443, username: "", password: "",
    tls_verify: true, tags: [],
    vcenter_url: "", datacenter: "",
    node: "", api_token_id: "", api_token_secret: "",
  };
}
// ── Main page component ───────────────────────────────────────────────────────

export default function HypervisorsPage() {
  const queryClient = useQueryClient();
  const upsertTask  = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { syncs, trackSync } = useSyncProgress();

  // ── UI state ────────────────────────────────────────────────────────────────
  const [showAddModal,    setShowAddModal]    = useState(false);
  const [editTarget,      setEditTarget]      = useState<Hypervisor | null>(null);
  const [deleteTarget,    setDeleteTarget]    = useState<Hypervisor | null>(null);
  const [testingId,       setTestingId]       = useState<string | null>(null);
  const [testResults,     setTestResults]     = useState<Record<string, { ok: boolean; msg?: string }>>({});
  const [search,          setSearch]          = useState("");
  const [statusFilter,    setStatusFilter]    = useState<StatusFilter>("all");
  const [providerFilter,  setProviderFilter]  = useState<ProviderType | "all">("all");
  const [page,            setPage]            = useState(1);

  // ── Data fetching ────────────────────────────────────────────────────────────
  const { data: apiData, isLoading } = useQuery({
    queryKey: ["hypervisors", page],
    queryFn: () => hypervisorApi.list({ page, page_size: PAGE_SIZE }),
  });

  // Backend shape: { success, data: Hypervisor[], meta: { total_items, total_pages, ... } }
  const allHypervisors: Hypervisor[] = apiData?.data ?? [];
  const totalItems  = apiData?.meta?.total_items ?? 0;
  const totalPages  = apiData?.meta?.total_pages ?? 1;

  // Client-side filtering (search + status + provider)
  const hypervisors = useMemo(() => {
    let list = allHypervisors;
    if (search.trim()) {
      const q = search.toLowerCase();
      list = list.filter(
        (h) =>
          h.name.toLowerCase().includes(q) ||
          h.host.toLowerCase().includes(q) ||
          h.provider.toLowerCase().includes(q)
      );
    }
    if (statusFilter !== "all")   list = list.filter((h) => h.connection_status === statusFilter);
    if (providerFilter !== "all") list = list.filter((h) => h.provider === providerFilter);
    return list;
  }, [allHypervisors, search, statusFilter, providerFilter]);

  // ── Mutations ────────────────────────────────────────────────────────────────
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["hypervisors"] });

  const registerMut = useMutation({
    mutationFn: hypervisorApi.register,
    onSuccess: () => { invalidate(); setShowAddModal(false); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateHypervisorPayload }) =>
      hypervisorApi.update(id, payload),
    onSuccess: () => { invalidate(); setEditTarget(null); },
  });

  const deleteMut = useMutation({
    mutationFn: hypervisorApi.delete,
    onSuccess: () => { invalidate(); setDeleteTarget(null); },
  });

  const syncMut = useMutation({
    mutationFn: hypervisorApi.syncInventory,
    onSuccess: (data, hypervisorId) => {
      if (data?.task_id) {
        const hv = allHypervisors.find((h) => h.id === hypervisorId);
        trackSync(data.task_id, hypervisorId, hv?.name);
        upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "inventory.sync" } as never);
        openTaskDrawer();
      }
    },
  });

  const handleTest = async (id: string) => {
    setTestingId(id);
    try {
      const res = await hypervisorApi.testConnection(id);
      setTestResults((prev) => ({
        ...prev,
        [id]: { ok: res!.connected, msg: res!.error },
      }));
      invalidate();
    } catch {
      setTestResults((prev) => ({ ...prev, [id]: { ok: false, msg: "Request failed" } }));
    } finally {
      setTestingId(null);
    }
  };

  // ── Render ───────────────────────────────────────────────────────────────────
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Hypervisors</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {totalItems} registered · {hypervisors.length} shown          </p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
        >
          <Plus className="w-4 h-4" /> Add Hypervisor
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3">        <div className="relative flex-1 min-w-[200px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search by name, host, provider…"
            className="w-full pl-9 pr-4 py-2 bg-gray-900 border border-gray-800 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
          />
        </div>
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-gray-500" />
          <select
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value as StatusFilter); setPage(1); }}
            className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
          >
            {STATUS_FILTERS.map((s) => (
              <option key={s} value={s}>{s === "all" ? "All statuses" : s.charAt(0).toUpperCase() + s.slice(1)}</option>
            ))}
          </select>
          <select
            value={providerFilter}
            onChange={(e) => { setProviderFilter(e.target.value as ProviderType | "all"); setPage(1); }}
            className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
          >
            <option value="all">All providers</option>
            {PROVIDERS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
          </select>
        </div>
      </div>

      {/* Sync progress panel */}
      <SyncProgressPanel syncs={syncs} />

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase tracking-wider">
                <th className="text-left px-5 py-3 font-medium">Name</th>
                <th className="text-left px-5 py-3 font-medium">Provider</th>
                <th className="text-left px-5 py-3 font-medium">Host</th>
                <th className="text-left px-5 py-3 font-medium">Status</th>
                <th className="text-left px-5 py-3 font-medium">Last Checked</th>
                <th className="text-left px-5 py-3 font-medium">Tags</th>
                <th className="text-right px-5 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-5 py-4">
                        <div className="h-4 bg-gray-800 rounded animate-pulse w-24" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : hypervisors.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-16 text-center text-gray-500">
                    <Server className="w-10 h-10 mx-auto mb-3 opacity-30" />
                    {search || statusFilter !== "all" || providerFilter !== "all"
                      ? "No hypervisors match your filters."
                      : 'No hypervisors registered yet. Click "Add Hypervisor" to get started.'}
                  </td>
                </tr>
              ) : (
                hypervisors.map((h) => (
                  <HypervisorRow
                    key={h.id}
                    hypervisor={h}
                    testResult={testResults[h.id]}
                    isTesting={testingId === h.id}
                    isSyncing={syncMut.isPending && syncMut.variables === h.id}
                    onTest={() => handleTest(h.id)}
                    onSync={() => syncMut.mutate(h.id)}
                    onEdit={() => setEditTarget(h)}
                    onDelete={() => setDeleteTarget(h)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-5 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">
              Page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Modals */}
      {showAddModal && (
        <HypervisorFormModal
          title="Add Hypervisor"
          submitLabel="Register"
          onClose={() => setShowAddModal(false)}
          onSubmit={(payload) => registerMut.mutate(payload)}
          isPending={registerMut.isPending}
          error={registerMut.error ? String((registerMut.error as Error).message) : undefined}
        />
      )}
      {editTarget && (
        <HypervisorFormModal
          title="Edit Hypervisor"
          submitLabel="Save Changes"
          initial={editTarget}
          onClose={() => setEditTarget(null)}
          onSubmit={(payload) => updateMut.mutate({ id: editTarget.id, payload })}
          isPending={updateMut.isPending}
          error={updateMut.error ? String((updateMut.error as Error).message) : undefined}
        />
      )}
      {deleteTarget && (
        <DeleteConfirmModal
          hypervisor={deleteTarget}
          isPending={deleteMut.isPending}
          error={deleteMut.error ? String((deleteMut.error as Error).message) : undefined}
          onConfirm={() => deleteMut.mutate(deleteTarget.id)}
          onClose={() => { setDeleteTarget(null); deleteMut.reset(); }}
        />
      )}
    </div>
  );
}

// ── HypervisorRow ─────────────────────────────────────────────────────────────

interface RowProps {
  hypervisor: Hypervisor;
  testResult?: { ok: boolean; msg?: string };
  isTesting: boolean;
  isSyncing: boolean;
  onTest: () => void;
  onSync: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

function HypervisorRow({ hypervisor: h, testResult, isTesting, isSyncing, onTest, onSync, onEdit, onDelete }: RowProps) {
  return (
    <tr className="hover:bg-gray-800/40 transition-colors group">
      <td className="px-5 py-4">
        <Link href={`/dashboard/hypervisors/${h.id}`} className="font-medium text-white hover:text-blue-400 transition-colors">
          {h.name}
        </Link>
        {h.description && (
          <div className="text-xs text-gray-500 mt-0.5 truncate max-w-[180px]">{h.description}</div>
        )}
      </td>
      <td className="px-5 py-4">
        <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-gray-800 text-xs text-gray-300 font-medium">
          {PROVIDER_LABELS[h.provider] ?? h.provider}
        </span>
      </td>
      <td className="px-5 py-4 text-gray-300 font-mono text-xs">
        {h.host}:{h.port}
      </td>
      <td className="px-5 py-4">
        <div className="flex items-center gap-2">
          <HypervisorStatusBadge status={h.connection_status} />
          {testResult && (
            testResult.ok
              ? <span title="Connection OK"><CheckCircle2 className="w-3.5 h-3.5 text-green-400" /></span>
              : <span title={testResult.msg ?? "Failed"}><AlertCircle className="w-3.5 h-3.5 text-red-400" /></span>
          )}
        </div>
      </td>
      <td className="px-5 py-4 text-xs text-gray-500">
        {h.last_checked_at ? relativeTime(h.last_checked_at) : "—"}
      </td>
      <td className="px-5 py-4">
        <div className="flex flex-wrap gap-1">
          {(h.tags ?? []).slice(0, 3).map((tag) => (
            <span key={tag} className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 text-xs">{tag}</span>
          ))}
          {(h.tags ?? []).length > 3 && (
            <span className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-500 text-xs">+{h.tags.length - 3}</span>
          )}
        </div>
      </td>
      <td className="px-5 py-4">
        <div className="flex items-center justify-end gap-1">
          <ActionBtn onClick={onTest} disabled={isTesting} title="Test connection">
            {isTesting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Plug className="w-3.5 h-3.5" />}
          </ActionBtn>
          <ActionBtn onClick={onSync} disabled={isSyncing} title="Sync inventory">
            <RotateCcw className={`w-3.5 h-3.5 ${isSyncing ? "animate-spin" : ""}`} />
          </ActionBtn>
          <ActionBtn onClick={onEdit} title="Edit">
            <Edit2 className="w-3.5 h-3.5" />
          </ActionBtn>
          <ActionBtn onClick={onDelete} title="Delete" danger>
            <Trash2 className="w-3.5 h-3.5" />
          </ActionBtn>
        </div>
      </td>
    </tr>
  );
}

function ActionBtn({ onClick, disabled, title, danger, children }: {
  onClick: () => void; disabled?: boolean; title?: string; danger?: boolean; children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={title}
      className={`p-1.5 rounded-lg transition-colors disabled:opacity-40 ${
        danger
          ? "hover:bg-red-900/40 text-gray-400 hover:text-red-400"
          : "hover:bg-gray-700 text-gray-400 hover:text-gray-200"
      }`}
    >
      {children}
    </button>
  );
}

// ── HypervisorFormModal ───────────────────────────────────────────────────────

interface FormModalProps {
  title: string;
  submitLabel: string;
  initial?: Hypervisor;
  onClose: () => void;
  onSubmit: (payload: RegisterHypervisorPayload) => void;
  isPending: boolean;
  error?: string;
}

function HypervisorFormModal({ title, submitLabel, initial, onClose, onSubmit, isPending, error }: FormModalProps) {
  const [form, setForm] = useState<RegisterHypervisorPayload>(() => {
    if (initial) {
      return {
        name:             initial.name,
        description:      initial.description ?? "",
        provider:         initial.provider,
        host:             initial.host,
        port:             initial.port,
        username:         initial.username ?? "",
        password:         "",
        tls_verify:       initial.tls_verify,
        tags:             initial.tags ?? [],
        vcenter_url:      String(initial.metadata?.vcenter_url ?? ""),
        datacenter:       String(initial.metadata?.datacenter ?? ""),
        node:             String(initial.metadata?.node ?? ""),
        api_token_id:     String(initial.metadata?.api_token_id ?? ""),
        api_token_secret: "",
      };
    }
    return defaultForm();
  });

  const [tagInput, setTagInput] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const setField = <K extends keyof RegisterHypervisorPayload>(k: K, v: RegisterHypervisorPayload[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors((e) => { const n = { ...e }; delete n[k as string]; return n; });
  };

  const handleProviderChange = (p: ProviderType) => {
    const def = PROVIDERS.find((x) => x.value === p);
    // On-prem hypervisors (Proxmox, ESXi, KVM) typically use self-signed certs
    const tlsVerify = p === "vmware" || p === "hyperv";
    setForm((f) => ({ ...f, provider: p, port: def?.defaultPort ?? f.port, tls_verify: tlsVerify }));
  };

  const addTag = () => {
    const t = tagInput.trim();
    if (t && !(form.tags ?? []).includes(t)) {
      setField("tags", [...(form.tags ?? []), t]);
    }
    setTagInput("");
  };

  const removeTag = (t: string) => setField("tags", (form.tags ?? []).filter((x) => x !== t));

  const validate = (): boolean => {
    const e: Record<string, string> = {};
    if (!form.name.trim())  e.name = "Name is required";
    if (!form.host.trim())  e.host = "Host / IP is required";
    if (!form.port || form.port < 1 || form.port > 65535) e.port = "Valid port required (1–65535)";
    if (form.provider === "vmware" && !form.username?.trim()) e.username = "Username is required for VMware";
    if (!initial && form.provider === "vmware" && !form.password?.trim()) e.password = "Password is required";
    if (!initial && form.provider === "esxi" && !form.password?.trim()) e.password = "Password is required";
    if (!initial && form.provider === "proxmox" && !form.api_token_id?.trim()) e.api_token_id = "API Token ID is required";
    if (!initial && form.provider === "proxmox" && !form.api_token_secret?.trim()) e.api_token_secret = "API Token Secret is required";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (validate()) onSubmit(form);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-lg font-semibold text-white">{title}</h2>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {/* Common fields */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField label="Hypervisor Name *" error={errors.name}>
              <input value={form.name} onChange={(e) => setField("name", e.target.value)}
                className={input(errors.name)} placeholder="prod-vcenter-01" />
            </FormField>
            <FormField label="Provider *">
              <select value={form.provider} onChange={(e) => handleProviderChange(e.target.value as ProviderType)}
                className={input()}>
                {PROVIDERS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
              </select>
            </FormField>
            <FormField label="Host / IP *" error={errors.host}>
              <input value={form.host} onChange={(e) => setField("host", e.target.value)}
                className={input(errors.host)} placeholder="192.168.1.10" />
            </FormField>
            <FormField label="Port *" error={errors.port}>
              <input type="number" value={form.port} onChange={(e) => setField("port", +e.target.value)}
                className={input(errors.port)} min={1} max={65535} />
            </FormField>
            <FormField label="Username" error={errors.username} className="sm:col-span-2">
              <input value={form.username ?? ""} onChange={(e) => setField("username", e.target.value)}
                className={input(errors.username)} placeholder={form.provider === "vmware" ? "administrator@vsphere.local" : "root@pam"} />
            </FormField>
          </div>

          {/* VMware-specific fields */}
          {(form.provider === "vmware" || form.provider === "esxi") && (
            <div className="space-y-4 p-4 bg-gray-800/40 rounded-xl border border-gray-700/50">
              <p className="text-xs font-semibold text-blue-400 uppercase tracking-wider">
                {form.provider === "esxi" ? "VMware ESXi" : "VMware vCenter"}
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <FormField label={`Password${initial ? " (leave blank to keep)" : " *"}`} error={errors.password}>
                  <input type="password" value={form.password ?? ""} onChange={(e) => setField("password", e.target.value)}
                    className={input(errors.password)} placeholder="••••••••" autoComplete="new-password" />
                </FormField>
                {form.provider === "vmware" && (
                  <>
                    <FormField label="vCenter URL">
                      <input value={form.vcenter_url ?? ""} onChange={(e) => setField("vcenter_url", e.target.value)}
                        className={input()} placeholder="https://vcenter.example.com" />
                    </FormField>
                    <FormField label="Datacenter (optional)">
                      <input value={form.datacenter ?? ""} onChange={(e) => setField("datacenter", e.target.value)}
                        className={input()} placeholder="DC-East" />
                    </FormField>
                  </>
                )}
              </div>
            </div>
          )}

          {/* Proxmox-specific fields */}
          {form.provider === "proxmox" && (
            <div className="space-y-4 p-4 bg-gray-800/40 rounded-xl border border-gray-700/50">
              <p className="text-xs font-semibold text-orange-400 uppercase tracking-wider">Proxmox VE</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <FormField label="Node Name">
                  <input value={form.node ?? ""} onChange={(e) => setField("node", e.target.value)}
                    className={input()} placeholder="pve" />
                </FormField>
                <FormField label={`API Token ID${initial ? "" : " *"}`} error={errors.api_token_id}>
                  <input value={form.api_token_id ?? ""} onChange={(e) => setField("api_token_id", e.target.value)}
                    className={input(errors.api_token_id)} placeholder="root@pam!vmorbit" />
                </FormField>
                <FormField label={`API Token Secret${initial ? " (leave blank to keep)" : " *"}`} error={errors.api_token_secret} className="sm:col-span-2">
                  <input type="password" value={form.api_token_secret ?? ""} onChange={(e) => setField("api_token_secret", e.target.value)}
                    className={input(errors.api_token_secret)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" autoComplete="new-password" />
                </FormField>
              </div>
            </div>
          )}

          {/* KVM / Hyper-V password */}
          {(form.provider === "kvm" || form.provider === "hyperv") && (
            <FormField label={`Password${initial ? " (leave blank to keep)" : ""}`} error={errors.password}>
              <input type="password" value={form.password ?? ""} onChange={(e) => setField("password", e.target.value)}
                className={input(errors.password)} placeholder="••••••••" autoComplete="new-password" />
            </FormField>
          )}

          {/* Description */}
          <FormField label="Description">
            <textarea value={form.description ?? ""} onChange={(e) => setField("description", e.target.value)}
              className={`${input()} resize-none`} rows={2} placeholder="Optional description…" />
          </FormField>

          {/* TLS Verify */}
          <label className="flex items-center gap-3 cursor-pointer">
            <div className="relative">
              <input type="checkbox" checked={form.tls_verify} onChange={(e) => setField("tls_verify", e.target.checked)}
                className="sr-only peer" />
              <div className="w-9 h-5 bg-gray-700 peer-checked:bg-blue-600 rounded-full transition-colors" />
              <div className="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-4" />
            </div>
            <span className="text-sm text-gray-300">Verify TLS certificate</span>
          </label>

          {/* Tags */}
          <FormField label="Tags">
            <div className="flex gap-2">
              <input value={tagInput} onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }}
                className={`${input()} flex-1`} placeholder="Add tag and press Enter" />
              <button type="button" onClick={addTag}
                className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
                Add
              </button>
            </div>
            {(form.tags ?? []).length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {(form.tags ?? []).map((t) => (
                  <span key={t} className="inline-flex items-center gap-1 px-2 py-0.5 bg-gray-800 text-gray-300 rounded-md text-xs">
                    {t}
                    <button type="button" onClick={() => removeTag(t)} className="hover:text-red-400 transition-colors">
                      <X className="w-3 h-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}
          </FormField>

          {/* Error */}
          {error && (
            <div className="flex items-start gap-2 p-3 bg-red-900/20 border border-red-800/50 rounded-lg">
              <AlertCircle className="w-4 h-4 text-red-400 mt-0.5 shrink-0" />
              <p className="text-sm text-red-400">{error}</p>
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-1">
            <button type="submit" disabled={isPending}
              className="flex items-center gap-2 px-5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors">
              {isPending && <Loader2 className="w-4 h-4 animate-spin" />}
              {isPending ? "Saving…" : submitLabel}
            </button>
            <button type="button" onClick={onClose}
              className="px-5 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── DeleteConfirmModal ────────────────────────────────────────────────────────

function DeleteConfirmModal({ hypervisor, isPending, onConfirm, onClose, error }: {
  hypervisor: Hypervisor; isPending: boolean; onConfirm: () => void; onClose: () => void; error?: string;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md shadow-2xl p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-full bg-red-900/30 flex items-center justify-center">
            <Trash2 className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Delete Hypervisor</h2>
            <p className="text-sm text-gray-400">This action cannot be undone.</p>
          </div>
        </div>
        <p className="text-sm text-gray-300 mb-6">
          Are you sure you want to delete <span className="font-semibold text-white">{hypervisor.name}</span>?
          All associated VMs, datastores, and network records will be removed.
        </p>
        {error && (
          <div className="flex items-start gap-2 p-3 mb-4 bg-red-900/20 border border-red-800/50 rounded-lg">
            <AlertCircle className="w-4 h-4 text-red-400 mt-0.5 shrink-0" />
            <p className="text-sm text-red-400">{error}</p>
          </div>
        )}
        <div className="flex gap-3">
          <button onClick={onConfirm} disabled={isPending}
            className="flex items-center gap-2 px-4 py-2 bg-red-600 hover:bg-red-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors">
            {isPending && <Loader2 className="w-4 h-4 animate-spin" />}
            {isPending ? "Deleting…" : "Delete"}
          </button>
          <button onClick={onClose}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Shared UI helpers ─────────────────────────────────────────────────────────

function FormField({ label, error, className, children }: {
  label: string; error?: string; className?: string; children: React.ReactNode;
}) {
  return (
    <div className={className}>
      <label className="block text-xs font-medium text-gray-400 mb-1.5">{label}</label>
      {children}
      {error && <p className="mt-1 text-xs text-red-400">{error}</p>}
    </div>
  );
}

function input(error?: string) {
  return `w-full px-3 py-2 bg-gray-800 border ${
    error ? "border-red-500" : "border-gray-700"
  } rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 transition-colors`;
}
