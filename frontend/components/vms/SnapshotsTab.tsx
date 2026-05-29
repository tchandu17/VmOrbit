"use client";
import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  Camera, Plus, RotateCcw, Trash2, Loader2, CheckCircle2,
  XCircle, AlertTriangle, X, Clock, ChevronRight, Info,
} from "lucide-react";
import { vmApi } from "@/lib/api/vms";
import { useTaskStore } from "@/store/useTaskStore";
import { useUIStore } from "@/store/useUIStore";
import { cn, relativeTime, formatDate } from "@/lib/utils";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import type { VM, Snapshot } from "@/types";
import type { ProviderType } from "@/types/hypervisor";

// ── Types ─────────────────────────────────────────────────────────────────────

interface Props {
  vm: VM;
}

type ToastType = "success" | "error" | "info";
interface Toast { id: number; type: ToastType; message: string; }
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

// ── Toast ─────────────────────────────────────────────────────────────────────

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

// ── Create Snapshot Modal ─────────────────────────────────────────────────────

function CreateSnapshotModal({ open, vmName, provider, onClose, onSubmit, isLoading }: {
  open: boolean; vmName: string; provider?: ProviderType;
  onClose: () => void; onSubmit: (data: { name: string; description: string; memory: boolean }) => void;
  isLoading: boolean;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [memory, setMemory] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    onSubmit({ name: name.trim(), description: description.trim(), memory });
  };

  const handleClose = () => {
    setName(""); setDescription(""); setMemory(false);
    onClose();
  };

  const isVMware = provider === "vmware" || provider === "esxi";

  return (
    <Dialog.Root open={open} onOpenChange={(v) => !v && handleClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" />
        <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-full max-w-lg bg-gray-900 border border-gray-700 rounded-2xl p-6 shadow-2xl">
          <div className="flex items-center justify-between mb-5">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-900/40 rounded-lg">
                <Camera className="w-4 h-4 text-blue-400" />
              </div>
              <div>
                <Dialog.Title className="text-white font-semibold text-base">Create Snapshot</Dialog.Title>
                <Dialog.Description className="text-gray-500 text-xs mt-0.5">{vmName}</Dialog.Description>
              </div>
            </div>
            <button onClick={handleClose} className="text-gray-500 hover:text-gray-300 transition-colors">
              <X className="w-4 h-4" />
            </button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Snapshot Name <span className="text-red-400">*</span></label>
              <input
                type="text" value={name} onChange={(e) => setName(e.target.value)}
                placeholder="e.g. before-upgrade-2024"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 transition-colors"
                autoFocus
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
              <textarea
                value={description} onChange={(e) => setDescription(e.target.value)}
                placeholder="Optional description..."
                rows={2}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 transition-colors resize-none"
              />
            </div>
            {isVMware && (
              <label className="flex items-center gap-3 p-3 bg-gray-800/60 rounded-xl cursor-pointer hover:bg-gray-800 transition-colors">
                <input type="checkbox" checked={memory} onChange={(e) => setMemory(e.target.checked)}
                  className="w-4 h-4 rounded border-gray-600 bg-gray-700 text-blue-500 focus:ring-blue-500 focus:ring-offset-gray-900" />
                <div>
                  <p className="text-sm text-white font-medium">Include memory state</p>
                  <p className="text-xs text-gray-500">Captures RAM contents — allows restoring to exact running state</p>
                </div>
              </label>
            )}
            <div className="flex justify-end gap-3 pt-2">
              <button type="button" onClick={handleClose}
                className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors">
                Cancel
              </button>
              <button type="submit" disabled={!name.trim() || isLoading}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors">
                {isLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Camera className="w-3.5 h-3.5" />}
                {isLoading ? "Creating…" : "Create Snapshot"}
              </button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

// ── Snapshot Row ──────────────────────────────────────────────────────────────

function SnapshotRow({ snap, provider, onRestore, onDelete, isBusy }: {
  snap: Snapshot; provider?: ProviderType;
  onRestore: (snap: Snapshot) => void;
  onDelete: (snap: Snapshot) => void;
  isBusy: boolean;
}) {
  return (
    <div className={cn(
      "flex items-start justify-between gap-4 px-5 py-4 hover:bg-gray-800/30 transition-colors",
      snap.is_current && "bg-blue-950/20 border-l-2 border-blue-500"
    )}>
      <div className="flex items-start gap-3 min-w-0">
        <div className={cn(
          "mt-0.5 w-8 h-8 rounded-lg flex items-center justify-center shrink-0",
          snap.is_current ? "bg-blue-900/50" : "bg-gray-800"
        )}>
          <Camera className={cn("w-4 h-4", snap.is_current ? "text-blue-400" : "text-gray-500")} />
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium text-white">{snap.name}</span>
            {snap.is_current && (
              <span className="text-[10px] font-semibold bg-blue-900/60 text-blue-300 px-1.5 py-0.5 rounded uppercase tracking-wide">
                Current
              </span>
            )}
            {provider && <ProviderBadge provider={provider} />}
          </div>
          {snap.description && (
            <p className="text-xs text-gray-500 mt-0.5 truncate">{snap.description}</p>
          )}
          <div className="flex items-center gap-3 mt-1 text-xs text-gray-600">
            <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{relativeTime(snap.created_at)}</span>
            <span className="font-mono">{snap.provider_id}</span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <button
          onClick={() => onRestore(snap)}
          disabled={isBusy || snap.is_current}
          title={snap.is_current ? "Already the current snapshot" : "Revert to this snapshot"}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-yellow-600/20 hover:bg-yellow-600/30 text-yellow-400 border border-yellow-600/40 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <RotateCcw className="w-3 h-3" />Restore
        </button>
        <button
          onClick={() => onDelete(snap)}
          disabled={isBusy}
          title="Delete snapshot"
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-red-600/20 hover:bg-red-600/30 text-red-400 border border-red-600/40 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Trash2 className="w-3 h-3" />Delete
        </button>
      </div>
    </div>
  );
}

// ── Main SnapshotsTab ─────────────────────────────────────────────────────────

export function SnapshotsTab({ vm }: Props) {
  const queryClient = useQueryClient();
  const upsertTask = useTaskStore((s) => s.upsertTask);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const { toasts, add: addToast, remove: removeToast } = useToast();

  const [showCreate, setShowCreate] = useState(false);
  const [confirmRestore, setConfirmRestore] = useState<Snapshot | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Snapshot | null>(null);
  const [busySnapshotId, setBusySnapshotId] = useState<string | null>(null);

  const provider = vm.hypervisor?.provider as ProviderType | undefined;

  // Fetch snapshots from the dedicated endpoint
  const { data: snapshots = [], isLoading, refetch } = useQuery({
    queryKey: ["vm-snapshots", vm.id],
    queryFn: () => vmApi.listSnapshots(vm.id),
    refetchInterval: 30_000,
  });

  const invalidate = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ["vm-snapshots", vm.id] });
    queryClient.invalidateQueries({ queryKey: ["vm", vm.id] });
  }, [queryClient, vm.id]);

  // Create snapshot
  const createMut = useMutation({
    mutationFn: (data: { name: string; description: string; memory: boolean }) =>
      vmApi.createSnapshot(vm.id, data),
    onSuccess: (data) => {
      upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "vm.snapshot" } as never);
      openTaskDrawer();
      setShowCreate(false);
      addToast("success", "Snapshot task queued — it will appear here once complete");
      setTimeout(invalidate, 5000);
    },
    onError: (err: Error) => addToast("error", `Create failed: ${err.message}`),
  });

  // Restore snapshot
  const restoreMut = useMutation({
    mutationFn: (snap: Snapshot) => vmApi.revertSnapshot(vm.id, snap.id),
    onSuccess: (data, snap) => {
      upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "vm.restore" } as never);
      openTaskDrawer();
      setBusySnapshotId(null);
      setConfirmRestore(null);
      addToast("success", `Restoring to "${snap.name}"…`);
      setTimeout(invalidate, 5000);
    },
    onError: (err: Error, snap) => {
      setBusySnapshotId(null);
      addToast("error", `Restore failed for "${snap.name}": ${err.message}`);
    },
  });

  // Delete snapshot
  const deleteMut = useMutation({
    mutationFn: (snap: Snapshot) => vmApi.deleteSnapshot(vm.id, snap.id),
    onSuccess: (data, snap) => {
      upsertTask({ id: data.task_id, status: "pending", progress: 0, type: "vm.snapshot.delete" } as never);
      openTaskDrawer();
      setBusySnapshotId(null);
      setConfirmDelete(null);
      addToast("success", `Deleting snapshot "${snap.name}"…`);
      setTimeout(invalidate, 5000);
    },
    onError: (err: Error, snap) => {
      setBusySnapshotId(null);
      addToast("error", `Delete failed for "${snap.name}": ${err.message}`);
    },
  });

  const handleRestore = (snap: Snapshot) => {
    setBusySnapshotId(snap.id);
    restoreMut.mutate(snap);
  };

  const handleDelete = (snap: Snapshot) => {
    setBusySnapshotId(snap.id);
    deleteMut.mutate(snap);
  };

  const isBusy = createMut.isPending || restoreMut.isPending || deleteMut.isPending;

  return (
    <>
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Restore confirm */}
      {confirmRestore && (
        <ConfirmDialog
          open={true}
          title={`Restore to "${confirmRestore.name}"`}
          description={`This will revert "${vm.name}" to the state captured in this snapshot. The VM will be powered off if running. This cannot be undone.`}
          confirmLabel="Restore"
          confirmClass="bg-yellow-600 hover:bg-yellow-700"
          onConfirm={() => handleRestore(confirmRestore)}
          onCancel={() => { setConfirmRestore(null); setBusySnapshotId(null); }}
        />
      )}

      {/* Delete confirm */}
      {confirmDelete && (
        <ConfirmDialog
          open={true}
          title={`Delete "${confirmDelete.name}"`}
          description={`This will permanently delete the snapshot "${confirmDelete.name}". This action cannot be undone.`}
          confirmLabel="Delete Snapshot"
          onConfirm={() => handleDelete(confirmDelete)}
          onCancel={() => { setConfirmDelete(null); setBusySnapshotId(null); }}
        />
      )}

      {/* Create modal */}
      <CreateSnapshotModal
        open={showCreate}
        vmName={vm.name}
        provider={provider}
        onClose={() => setShowCreate(false)}
        onSubmit={(data) => createMut.mutate(data)}
        isLoading={createMut.isPending}
      />

      <div className="space-y-4">
        {/* Header */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
          <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Camera className="w-4 h-4 text-blue-400" />
              <h3 className="text-sm font-semibold text-white">Snapshots</h3>
              {snapshots.length > 0 && (
                <span className="text-xs bg-gray-800 text-gray-400 px-2 py-0.5 rounded-full">{snapshots.length}</span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <button onClick={() => refetch()}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 text-gray-400 rounded-lg transition-colors">
                Refresh
              </button>
              <button onClick={() => setShowCreate(true)} disabled={isBusy}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg transition-colors">
                <Plus className="w-3.5 h-3.5" />New Snapshot
              </button>
            </div>
          </div>

          {isLoading ? (
            <div className="p-8 flex items-center justify-center gap-3 text-gray-500">
              <Loader2 className="w-5 h-5 animate-spin" />
              <span className="text-sm">Loading snapshots…</span>
            </div>
          ) : snapshots.length === 0 ? (
            <div className="py-16 flex flex-col items-center gap-3 text-center px-6">
              <div className="w-12 h-12 rounded-2xl bg-gray-800 flex items-center justify-center">
                <Camera className="w-6 h-6 text-gray-600" />
              </div>
              <div>
                <p className="text-sm font-medium text-gray-300">No snapshots yet</p>
                <p className="text-xs text-gray-500 mt-1">Create a snapshot to capture the current state of this VM</p>
              </div>
              <button onClick={() => setShowCreate(true)}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors mt-1">
                <Plus className="w-3.5 h-3.5" />Create First Snapshot
              </button>
            </div>
          ) : (
            <div className="divide-y divide-gray-800/60">
              {snapshots.map((snap) => (
                <SnapshotRow
                  key={snap.id}
                  snap={snap}
                  provider={provider}
                  onRestore={(s) => { setBusySnapshotId(s.id); setConfirmRestore(s); }}
                  onDelete={(s) => { setBusySnapshotId(s.id); setConfirmDelete(s); }}
                  isBusy={isBusy && busySnapshotId !== snap.id ? false : isBusy && busySnapshotId === snap.id}
                />
              ))}
            </div>
          )}
        </div>

        {/* Info card */}
        <div className="bg-gray-900/50 border border-gray-800/60 rounded-xl p-4 flex items-start gap-3">
          <Info className="w-4 h-4 text-gray-500 shrink-0 mt-0.5" />
          <div className="text-xs text-gray-500 space-y-1">
            <p>Snapshot operations run as async tasks — check the Tasks tab for progress.</p>
            {(provider === "vmware" || provider === "esxi") && (
              <p>VMware snapshots with memory state capture RAM contents and allow restoring to the exact running state.</p>
            )}
            {provider === "proxmox" && (
              <p>Proxmox snapshots are stored on the same storage as the VM disk.</p>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
