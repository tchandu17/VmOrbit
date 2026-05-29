"use client";
import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X, Loader2, Copy, Server } from "lucide-react";
import { useMutation } from "@tanstack/react-query";
import { provisioningApi } from "@/lib/api/provisioning";
import type { VM, ProvisioningJob } from "@/types";

interface Props {
  vm: VM;
  onClose: () => void;
  onSuccess: (job: ProvisioningJob) => void;
}

export default function CloneVMDialog({ vm, onClose, onSuccess }: Props) {
  const [name, setName] = useState(`${vm.name}-clone`);
  const [dataStore, setDataStore] = useState("");
  const [node, setNode] = useState(
    typeof vm.metadata?.node === "string" ? vm.metadata.node : ""
  );
  const [linked, setLinked] = useState(false);
  const [nameError, setNameError] = useState("");

  const cloneMut = useMutation({
    mutationFn: () =>
      provisioningApi.clone({
        source_vm_id: vm.id,
        name: name.trim(),
        data_store: dataStore || undefined,
        node: node || undefined,
        linked,
      }),
    onSuccess: (job) => onSuccess(job),
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) { setNameError("Clone name is required"); return; }
    setNameError("");
    cloneMut.mutate();
  }

  return (
    <Dialog.Root open onOpenChange={(v) => !v && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" />
        <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-full max-w-md bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-900/40 rounded-lg">
                <Copy className="w-4 h-4 text-blue-400" />
              </div>
              <div>
                <Dialog.Title className="text-white font-semibold text-base">Clone VM</Dialog.Title>
                <Dialog.Description className="text-gray-400 text-xs mt-0.5">
                  Source: <span className="text-blue-400">{vm.name}</span>
                </Dialog.Description>
              </div>
            </div>
            <button onClick={onClose} className="text-gray-500 hover:text-gray-300 transition-colors">
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
            {/* Clone name */}
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1">Clone Name *</label>
              <input
                type="text"
                value={name}
                onChange={(e) => { setName(e.target.value); setNameError(""); }}
                placeholder="my-vm-clone"
                className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 ${nameError ? "border-red-500" : "border-gray-700"}`}
              />
              {nameError && <p className="text-xs text-red-400 mt-1">{nameError}</p>}
            </div>

            {/* Datastore */}
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1">Target Datastore</label>
              <input
                type="text"
                value={dataStore}
                onChange={(e) => setDataStore(e.target.value)}
                placeholder="Same as source"
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            {/* Node (Proxmox) */}
            {vm.metadata?.node && (
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                  <Server className="w-3 h-3 text-orange-400" /> Target Node
                </label>
                <input
                  type="text"
                  value={node}
                  onChange={(e) => setNode(e.target.value)}
                  placeholder="Same as source"
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            )}

            {/* Linked clone toggle */}
            <div className="flex items-center justify-between py-2 px-3 bg-gray-800 rounded-lg">
              <div>
                <p className="text-sm text-white font-medium">Linked Clone</p>
                <p className="text-xs text-gray-500 mt-0.5">Faster creation, shares base disk with source. Requires a snapshot.</p>
              </div>
              <button
                type="button"
                onClick={() => setLinked((v) => !v)}
                className={`relative w-10 h-5 rounded-full transition-colors ${linked ? "bg-blue-600" : "bg-gray-600"}`}
              >
                <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${linked ? "translate-x-5" : "translate-x-0.5"}`} />
              </button>
            </div>

            {/* Error */}
            {cloneMut.isError && (
              <div className="bg-red-950 border border-red-800 rounded-lg px-4 py-3 text-sm text-red-300">
                {(cloneMut.error as Error).message}
              </div>
            )}

            {/* Actions */}
            <div className="flex justify-end gap-3 pt-2">
              <button type="button" onClick={onClose}
                className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors">
                Cancel
              </button>
              <button type="submit" disabled={cloneMut.isPending}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 disabled:opacity-60 text-white rounded-lg transition-colors font-medium">
                {cloneMut.isPending ? <><Loader2 className="w-4 h-4 animate-spin" /> Cloning…</> : <><Copy className="w-4 h-4" /> Clone VM</>}
              </button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
