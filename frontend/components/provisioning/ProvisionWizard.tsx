"use client";
import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X, ChevronRight, ChevronLeft, Cpu, HardDrive, MemoryStick, Network, Server, Check, Loader2 } from "lucide-react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { provisioningApi } from "@/lib/api/provisioning";
import { hypervisorApi } from "@/lib/api/hypervisors";
import type { VMTemplate, Hypervisor, ProvisioningJob } from "@/types";

interface Props {
  template: VMTemplate;
  hypervisors: Hypervisor[];
  onClose: () => void;
  onSuccess: (job: ProvisioningJob) => void;
}

type Step = "configure" | "review";

interface FormState {
  name: string;
  cpuCount: number;
  memoryMB: number;
  diskGB: number;
  networkName: string;
  dataStore: string;
  node: string;
  tags: string;
}

export default function ProvisionWizard({ template, hypervisors, onClose, onSuccess }: Props) {
  const hv = hypervisors.find((h) => h.id === template.hypervisor_id);

  const [step, setStep] = useState<Step>("configure");
  const [form, setForm] = useState<FormState>({
    name: `${template.name}-clone`,
    cpuCount: template.cpu_count || 2,
    memoryMB: template.memory_mb || 2048,
    diskGB: template.disk_gb || 20,
    networkName: "",
    dataStore: "",
    node: "",
    tags: "",
  });
  const [errors, setErrors] = useState<Partial<Record<keyof FormState, string>>>({});

  // Fetch datastores and networks for the hypervisor
  const { data: datastoresData } = useQuery({
    queryKey: ["datastores", template.hypervisor_id],
    queryFn: () =>
      fetch(`/api/proxy/api/v1/hypervisors/${template.hypervisor_id}/datastores`, {
        headers: { Authorization: `Bearer ${localStorage.getItem("access_token")}` },
      }).then((r) => r.json()).then((r) => r.data ?? []),
    enabled: !!template.hypervisor_id,
  });

  const { data: networksData } = useQuery({
    queryKey: ["networks", template.hypervisor_id],
    queryFn: () =>
      fetch(`/api/proxy/api/v1/hypervisors/${template.hypervisor_id}/networks`, {
        headers: { Authorization: `Bearer ${localStorage.getItem("access_token")}` },
      }).then((r) => r.json()).then((r) => r.data ?? []),
    enabled: !!template.hypervisor_id,
  });

  const datastores: Array<{ name: string; provider_id: string; free_gb: number }> = datastoresData ?? [];
  const networks: Array<{ name: string; provider_id: string }> = networksData ?? [];

  const provisionMut = useMutation({
    mutationFn: () =>
      provisioningApi.provision({
        template_id: template.id,
        name: form.name,
        cpu_count: form.cpuCount,
        memory_mb: form.memoryMB,
        disk_gb: form.diskGB,
        network_name: form.networkName || undefined,
        data_store: form.dataStore || undefined,
        node: form.node || undefined,
        tags: form.tags ? form.tags.split(",").map((t) => t.trim()).filter(Boolean) : undefined,
      }),
    onSuccess: (job) => onSuccess(job),
  });

  function validate(): boolean {
    const errs: Partial<Record<keyof FormState, string>> = {};
    if (!form.name.trim()) errs.name = "VM name is required";
    if (form.cpuCount < 1) errs.cpuCount = "Must be at least 1";
    if (form.memoryMB < 256) errs.memoryMB = "Must be at least 256 MB";
    if (form.diskGB < 1) errs.diskGB = "Must be at least 1 GB";
    setErrors(errs);
    return Object.keys(errs).length === 0;
  }

  function handleNext() {
    if (validate()) setStep("review");
  }

  const field = (label: string, key: keyof FormState, type: "text" | "number" = "text", placeholder = "") => (
    <div>
      <label className="block text-xs font-medium text-gray-400 mb-1">{label}</label>
      <input
        type={type}
        value={form[key] as string | number}
        onChange={(e) => setForm((f) => ({ ...f, [key]: type === "number" ? Number(e.target.value) : e.target.value }))}
        placeholder={placeholder}
        className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 ${errors[key] ? "border-red-500" : "border-gray-700"}`}
      />
      {errors[key] && <p className="text-xs text-red-400 mt-1">{errors[key]}</p>}
    </div>
  );

  const select = (label: string, key: keyof FormState, options: Array<{ label: string; value: string }>, placeholder = "Auto-select") => (
    <div>
      <label className="block text-xs font-medium text-gray-400 mb-1">{label}</label>
      <select
        value={form[key] as string}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
        className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value="">{placeholder}</option>
        {options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
    </div>
  );

  return (
    <Dialog.Root open onOpenChange={(v) => !v && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" />
        <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-full max-w-lg bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl flex flex-col max-h-[90vh]">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800 shrink-0">
            <div>
              <Dialog.Title className="text-white font-semibold text-base">Provision VM</Dialog.Title>
              <Dialog.Description className="text-gray-400 text-xs mt-0.5">
                From template: <span className="text-blue-400">{template.name}</span>
                {hv && <span className="text-gray-500"> · {hv.name}</span>}
              </Dialog.Description>
            </div>
            <button onClick={onClose} className="text-gray-500 hover:text-gray-300 transition-colors">
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Step indicator */}
          <div className="flex items-center gap-2 px-6 py-3 border-b border-gray-800 shrink-0">
            {(["configure", "review"] as Step[]).map((s, i) => (
              <div key={s} className="flex items-center gap-2">
                {i > 0 && <div className="w-8 h-px bg-gray-700" />}
                <div className={`flex items-center gap-1.5 text-xs font-medium ${step === s ? "text-blue-400" : i < (step === "review" ? 1 : 0) ? "text-green-400" : "text-gray-500"}`}>
                  <div className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ${step === s ? "bg-blue-600 text-white" : i < (step === "review" ? 1 : 0) ? "bg-green-600 text-white" : "bg-gray-700 text-gray-400"}`}>
                    {i < (step === "review" ? 1 : 0) ? <Check className="w-3 h-3" /> : i + 1}
                  </div>
                  {s === "configure" ? "Configure" : "Review"}
                </div>
              </div>
            ))}
          </div>

          {/* Body */}
          <div className="flex-1 overflow-y-auto px-6 py-5">
            {step === "configure" && (
              <div className="space-y-4">
                {field("VM Name *", "name", "text", "my-new-vm")}

                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                      <Cpu className="w-3 h-3 text-blue-400" /> vCPU *
                    </label>
                    <input type="number" min={1} max={64} value={form.cpuCount}
                      onChange={(e) => setForm((f) => ({ ...f, cpuCount: Number(e.target.value) }))}
                      className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${errors.cpuCount ? "border-red-500" : "border-gray-700"}`} />
                    {errors.cpuCount && <p className="text-xs text-red-400 mt-1">{errors.cpuCount}</p>}
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                      <MemoryStick className="w-3 h-3 text-purple-400" /> RAM (MB) *
                    </label>
                    <input type="number" min={256} step={256} value={form.memoryMB}
                      onChange={(e) => setForm((f) => ({ ...f, memoryMB: Number(e.target.value) }))}
                      className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${errors.memoryMB ? "border-red-500" : "border-gray-700"}`} />
                    {errors.memoryMB && <p className="text-xs text-red-400 mt-1">{errors.memoryMB}</p>}
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                      <HardDrive className="w-3 h-3 text-green-400" /> Disk (GB) *
                    </label>
                    <input type="number" min={1} value={form.diskGB}
                      onChange={(e) => setForm((f) => ({ ...f, diskGB: Number(e.target.value) }))}
                      className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${errors.diskGB ? "border-red-500" : "border-gray-700"}`} />
                    {errors.diskGB && <p className="text-xs text-red-400 mt-1">{errors.diskGB}</p>}
                  </div>
                </div>

                {networks.length > 0
                  ? select("Network", "networkName", networks.map((n) => ({ label: n.name, value: n.name })), "Auto-select")
                  : (
                    <div>
                      <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                        <Network className="w-3 h-3 text-yellow-400" /> Network
                      </label>
                      <input type="text" value={form.networkName}
                        onChange={(e) => setForm((f) => ({ ...f, networkName: e.target.value }))}
                        placeholder="e.g. VM Network"
                        className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                  )}

                {datastores.length > 0
                  ? select("Datastore", "dataStore",
                      datastores.map((d) => ({ label: `${d.name}${d.free_gb ? ` (${d.free_gb.toFixed(0)} GB free)` : ""}`, value: d.name })),
                      "Auto-select")
                  : (
                    <div>
                      <label className="block text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
                        <Server className="w-3 h-3 text-orange-400" /> Datastore
                      </label>
                      <input type="text" value={form.dataStore}
                        onChange={(e) => setForm((f) => ({ ...f, dataStore: e.target.value }))}
                        placeholder="e.g. datastore1"
                        className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                  )}

                {hv?.provider === "proxmox" && (
                  <div>
                    <label className="block text-xs font-medium text-gray-400 mb-1">Target Node</label>
                    <input type="text" value={form.node}
                      onChange={(e) => setForm((f) => ({ ...f, node: e.target.value }))}
                      placeholder="e.g. pve1"
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                  </div>
                )}

                <div>
                  <label className="block text-xs font-medium text-gray-400 mb-1">Tags (comma-separated)</label>
                  <input type="text" value={form.tags}
                    onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))}
                    placeholder="production, web, linux"
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
              </div>
            )}

            {step === "review" && (
              <div className="space-y-4">
                <p className="text-sm text-gray-400">Review your configuration before provisioning.</p>
                <div className="bg-gray-800 rounded-xl p-4 space-y-3 text-sm">
                  <Row label="VM Name" value={form.name} />
                  <Row label="Template" value={template.name} />
                  {hv && <Row label="Hypervisor" value={hv.name} />}
                  <Row label="vCPU" value={`${form.cpuCount} cores`} />
                  <Row label="Memory" value={`${form.memoryMB} MB`} />
                  <Row label="Disk" value={`${form.diskGB} GB`} />
                  {form.networkName && <Row label="Network" value={form.networkName} />}
                  {form.dataStore && <Row label="Datastore" value={form.dataStore} />}
                  {form.node && <Row label="Node" value={form.node} />}
                  {form.tags && <Row label="Tags" value={form.tags} />}
                </div>
                {provisionMut.isError && (
                  <div className="bg-red-950 border border-red-800 rounded-lg px-4 py-3 text-sm text-red-300">
                    {(provisionMut.error as Error).message}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between px-6 py-4 border-t border-gray-800 shrink-0">
            <button
              onClick={step === "configure" ? onClose : () => setStep("configure")}
              className="flex items-center gap-2 px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors"
            >
              {step === "configure" ? "Cancel" : <><ChevronLeft className="w-4 h-4" /> Back</>}
            </button>
            {step === "configure" ? (
              <button onClick={handleNext}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors font-medium">
                Review <ChevronRight className="w-4 h-4" />
              </button>
            ) : (
              <button
                onClick={() => provisionMut.mutate()}
                disabled={provisionMut.isPending}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 disabled:opacity-60 text-white rounded-lg transition-colors font-medium"
              >
                {provisionMut.isPending ? <><Loader2 className="w-4 h-4 animate-spin" /> Provisioning…</> : "Provision VM"}
              </button>
            )}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-gray-500 shrink-0">{label}</span>
      <span className="text-white text-right truncate">{value}</span>
    </div>
  );
}
