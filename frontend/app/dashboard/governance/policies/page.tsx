"use client";
import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Shield, Plus, Trash2, ToggleLeft, ToggleRight, Edit2, ChevronDown, ChevronUp, X, Check, AlertTriangle } from "lucide-react";
import { policyApi } from "@/lib/api/policy";
import { cn } from "@/lib/utils";
import type { Policy, PolicyEffect, PolicyType, PolicyConditionType, PolicyConditionOperator, PolicyAssignmentTargetType } from "@/types";

const EFFECT_COLORS: Record<PolicyEffect, string> = {
  allow: "bg-green-900/40 text-green-300 border-green-700",
  deny: "bg-red-900/40 text-red-300 border-red-700",
  require_approval: "bg-yellow-900/40 text-yellow-300 border-yellow-700",
  require_snapshot: "bg-blue-900/40 text-blue-300 border-blue-700",
  require_justification: "bg-purple-900/40 text-purple-300 border-purple-700",
};

const EFFECT_LABELS: Record<PolicyEffect, string> = {
  allow: "Allow",
  deny: "Deny",
  require_approval: "Require Approval",
  require_snapshot: "Require Snapshot",
  require_justification: "Require Justification",
};

const POLICY_TYPES: PolicyType[] = ["vm", "environment", "provider", "task", "user"];
const POLICY_EFFECTS: PolicyEffect[] = ["allow", "deny", "require_approval", "require_snapshot", "require_justification"];
const CONDITION_TYPES: PolicyConditionType[] = ["vm_tag", "environment", "provider", "user_role", "operation", "vm_name", "hypervisor", "bulk_size", "maintenance_window", "time_schedule"];
const CONDITION_OPERATORS: PolicyConditionOperator[] = ["equals", "not_equals", "contains", "in", "not_in", "greater_than", "less_than", "matches"];
const ASSIGNMENT_TARGETS: PolicyAssignmentTargetType[] = ["global", "hypervisor", "environment", "vm", "tag", "role"];

const COMMON_OPERATIONS = [
  "vm.power_off", "vm.power_on", "vm.reboot", "vm.suspend", "vm.delete",
  "vm.snapshot", "vm.snapshot.delete", "vm.restore", "vm.clone", "vm.provision",
  "vm.bulk.power_off", "vm.bulk.power_on", "vm.bulk.reboot", "vm.bulk.snapshot",
  "inventory.sync", "*",
];

// ── Toast ─────────────────────────────────────────────────────────────────────
let toastId = 0;
type ToastMsg = { id: number; type: "success" | "error"; text: string };
function useToast() {
  const [toasts, setToasts] = useState<ToastMsg[]>([]);
  const add = useCallback((type: "success" | "error", text: string) => {
    const id = ++toastId;
    setToasts(p => [...p, { id, type, text }]);
    setTimeout(() => setToasts(p => p.filter(t => t.id !== id)), 4000);
  }, []);
  return { toasts, add };
}

// ── Policy Form Modal ─────────────────────────────────────────────────────────
interface PolicyFormProps {
  initial?: Policy | null;
  onClose: () => void;
  onSaved: () => void;
}

function PolicyForm({ initial, onClose, onSaved }: PolicyFormProps) {
  const qc = useQueryClient();
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [type, setType] = useState<PolicyType>(initial?.type ?? "vm");
  const [effect, setEffect] = useState<PolicyEffect>(initial?.effect ?? "deny");
  const [priority, setPriority] = useState(initial?.priority ?? 100);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [operations, setOperations] = useState<string[]>(initial?.operations ?? []);
  const [customOp, setCustomOp] = useState("");
  const [conditions, setConditions] = useState(
    initial?.conditions?.map(c => ({ type: c.type, operator: c.operator, value: c.value, negate: c.negate })) ?? []
  );
  const [approverRole, setApproverRole] = useState(
    (initial?.approval_config?.approver_role as string) ?? ""
  );
  const [expiryHours, setExpiryHours] = useState(
    (initial?.approval_config?.expiry_hours as number) ?? 24
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const addCondition = () =>
    setConditions(p => [...p, { type: "vm_tag", operator: "equals", value: "", negate: false }]);
  const removeCondition = (i: number) => setConditions(p => p.filter((_, idx) => idx !== i));
  const updateCondition = (i: number, field: string, val: unknown) =>
    setConditions(p => p.map((c, idx) => idx === i ? { ...c, [field]: val } : c));

  const toggleOp = (op: string) =>
    setOperations(p => p.includes(op) ? p.filter(o => o !== op) : [...p, op]);

  const addCustomOp = () => {
    if (customOp.trim() && !operations.includes(customOp.trim())) {
      setOperations(p => [...p, customOp.trim()]);
      setCustomOp("");
    }
  };

  const handleSave = async () => {
    if (!name.trim()) { setError("Name is required"); return; }
    if (operations.length === 0) { setError("Select at least one operation"); return; }
    setSaving(true); setError("");
    try {
      const approvalConfig = effect === "require_approval"
        ? { approver_role: approverRole, expiry_hours: expiryHours }
        : undefined;
      if (initial) {
        await policyApi.update(initial.id, { name, description, effect, priority, enabled, operations, conditions, approval_config: approvalConfig });
      } else {
        await policyApi.create({ name, description, type, effect, priority, enabled, operations, conditions, approval_config: approvalConfig });
      }
      qc.invalidateQueries({ queryKey: ["policies"] });
      onSaved();
    } catch (e) { setError((e as Error).message); }
    finally { setSaving(false); }
  };

  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto shadow-2xl">
        <div className="flex items-center justify-between p-6 border-b border-gray-800">
          <h2 className="text-white font-semibold text-lg">{initial ? "Edit Policy" : "Create Policy"}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X className="w-5 h-5" /></button>
        </div>
        <div className="p-6 space-y-4">
          {error && <div className="bg-red-950 border border-red-800 text-red-300 text-sm px-4 py-2 rounded-lg">{error}</div>}
          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2">
              <label className="text-xs text-gray-400 mb-1 block">Name *</label>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Block production VM deletion"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            <div className="col-span-2">
              <label className="text-xs text-gray-400 mb-1 block">Description</label>
              <input value={description} onChange={e => setDescription(e.target.value)} placeholder="Optional description"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            {!initial && (
              <div>
                <label className="text-xs text-gray-400 mb-1 block">Type</label>
                <select value={type} onChange={e => setType(e.target.value as PolicyType)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500">
                  {POLICY_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                </select>
              </div>
            )}
            <div>
              <label className="text-xs text-gray-400 mb-1 block">Effect</label>
              <select value={effect} onChange={e => setEffect(e.target.value as PolicyEffect)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500">
                {POLICY_EFFECTS.map(e => <option key={e} value={e}>{EFFECT_LABELS[e]}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs text-gray-400 mb-1 block">Priority (lower = higher)</label>
              <input type="number" value={priority} onChange={e => setPriority(Number(e.target.value))} min={1} max={1000}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-xs text-gray-400">Enabled</label>
              <button onClick={() => setEnabled(p => !p)} className={cn("w-10 h-5 rounded-full transition-colors", enabled ? "bg-blue-600" : "bg-gray-700")}>
                <div className={cn("w-4 h-4 bg-white rounded-full transition-transform mx-0.5", enabled ? "translate-x-5" : "translate-x-0")} />
              </button>
            </div>
          </div>

          {/* Operations */}
          <div>
            <label className="text-xs text-gray-400 mb-2 block">Operations *</label>
            <div className="flex flex-wrap gap-2 mb-2">
              {COMMON_OPERATIONS.map(op => (
                <button key={op} onClick={() => toggleOp(op)}
                  className={cn("px-2 py-1 rounded text-xs border transition-colors", operations.includes(op) ? "bg-blue-600 border-blue-500 text-white" : "bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-500")}>
                  {op}
                </button>
              ))}
            </div>
            <div className="flex gap-2">
              <input value={customOp} onChange={e => setCustomOp(e.target.value)} onKeyDown={e => e.key === "Enter" && addCustomOp()}
                placeholder="Custom operation…" className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white focus:outline-none focus:border-blue-500" />
              <button onClick={addCustomOp} className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white text-xs rounded-lg">Add</button>
            </div>
          </div>

          {/* Approval config */}
          {effect === "require_approval" && (
            <div className="bg-yellow-950/30 border border-yellow-800/50 rounded-xl p-4 space-y-3">
              <p className="text-xs text-yellow-400 font-medium">Approval Configuration</p>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-400 mb-1 block">Approver Role</label>
                  <input value={approverRole} onChange={e => setApproverRole(e.target.value)} placeholder="e.g. admin"
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
                </div>
                <div>
                  <label className="text-xs text-gray-400 mb-1 block">Expiry (hours)</label>
                  <input type="number" value={expiryHours} onChange={e => setExpiryHours(Number(e.target.value))} min={1}
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
                </div>
              </div>
            </div>
          )}

          {/* Conditions */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-xs text-gray-400">Conditions (all must match)</label>
              <button onClick={addCondition} className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1"><Plus className="w-3 h-3" />Add</button>
            </div>
            <div className="space-y-2">
              {conditions.map((c, i) => (
                <div key={i} className="flex items-center gap-2 bg-gray-800 rounded-lg p-2">
                  <select value={c.type} onChange={e => updateCondition(i, "type", e.target.value)}
                    className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-xs text-white focus:outline-none">
                    {CONDITION_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                  </select>
                  <select value={c.operator} onChange={e => updateCondition(i, "operator", e.target.value)}
                    className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-xs text-white focus:outline-none">
                    {CONDITION_OPERATORS.map(o => <option key={o} value={o}>{o}</option>)}
                  </select>
                  <input value={c.value} onChange={e => updateCondition(i, "value", e.target.value)} placeholder="value"
                    className="flex-1 bg-gray-700 border border-gray-600 rounded px-2 py-1 text-xs text-white focus:outline-none" />
                  <label className="flex items-center gap-1 text-xs text-gray-400 cursor-pointer">
                    <input type="checkbox" checked={c.negate} onChange={e => updateCondition(i, "negate", e.target.checked)} className="w-3 h-3" />NOT
                  </label>
                  <button onClick={() => removeCondition(i)} className="text-gray-500 hover:text-red-400"><X className="w-3.5 h-3.5" /></button>
                </div>
              ))}
              {conditions.length === 0 && <p className="text-xs text-gray-600 italic">No conditions — policy applies to all matching operations</p>}
            </div>
          </div>
        </div>
        <div className="flex justify-end gap-3 p-6 border-t border-gray-800">
          <button onClick={onClose} className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg">Cancel</button>
          <button onClick={handleSave} disabled={saving} className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg">
            {saving ? "Saving…" : initial ? "Update Policy" : "Create Policy"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Assignment Modal ──────────────────────────────────────────────────────────
function AssignModal({ policy, onClose }: { policy: Policy; onClose: () => void }) {
  const qc = useQueryClient();
  const [targetType, setTargetType] = useState<PolicyAssignmentTargetType>("global");
  const [targetId, setTargetId] = useState("");
  const [saving, setSaving] = useState(false);

  const { data: assignments } = useQuery({
    queryKey: ["policy-assignments", policy.id],
    queryFn: () => policyApi.listAssignments(policy.id),
  });

  const handleAssign = async () => {
    setSaving(true);
    try {
      await policyApi.assign(policy.id, { target_type: targetType, target_id: targetType !== "global" ? targetId : undefined });
      qc.invalidateQueries({ queryKey: ["policy-assignments", policy.id] });
      setTargetId("");
    } catch { /* ignore */ }
    finally { setSaving(false); }
  };

  const handleUnassign = async (assignmentId: string) => {
    await policyApi.unassign(policy.id, assignmentId);
    qc.invalidateQueries({ queryKey: ["policy-assignments", policy.id] });
  };

  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl w-full max-w-lg shadow-2xl">
        <div className="flex items-center justify-between p-5 border-b border-gray-800">
          <h2 className="text-white font-semibold">Assign Policy: {policy.name}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X className="w-5 h-5" /></button>
        </div>
        <div className="p-5 space-y-4">
          <div className="flex gap-2">
            <select value={targetType} onChange={e => setTargetType(e.target.value as PolicyAssignmentTargetType)}
              className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500">
              {ASSIGNMENT_TARGETS.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
            {targetType !== "global" && (
              <input value={targetId} onChange={e => setTargetId(e.target.value)} placeholder="Target ID (UUID)"
                className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
            )}
            <button onClick={handleAssign} disabled={saving || (targetType !== "global" && !targetId)}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded-lg">
              {saving ? "…" : "Assign"}
            </button>
          </div>
          <div className="space-y-2">
            <p className="text-xs text-gray-500 uppercase tracking-wide">Current Assignments</p>
            {(assignments ?? []).length === 0 && <p className="text-sm text-gray-600 italic">No assignments yet</p>}
            {(assignments ?? []).map(a => (
              <div key={a.id} className="flex items-center justify-between bg-gray-800 rounded-lg px-3 py-2">
                <span className="text-sm text-gray-300">{a.target_type}{a.target_id ? ` → ${a.target_id}` : ""}</span>
                <button onClick={() => handleUnassign(a.id)} className="text-gray-500 hover:text-red-400"><X className="w-3.5 h-3.5" /></button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Policy Row ────────────────────────────────────────────────────────────────
function PolicyRow({ policy, onEdit, onAssign, onDelete, onToggle }: {
  policy: Policy;
  onEdit: () => void;
  onAssign: () => void;
  onDelete: () => void;
  onToggle: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <>
      <tr className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
        <td className="px-4 py-3">
          <div>
            <p className="text-white font-medium text-sm">{policy.name}</p>
            {policy.description && <p className="text-gray-500 text-xs mt-0.5">{policy.description}</p>}
          </div>
        </td>
        <td className="px-4 py-3"><span className="text-xs text-gray-400 bg-gray-800 px-2 py-0.5 rounded">{policy.type}</span></td>
        <td className="px-4 py-3">
          <span className={cn("text-xs px-2 py-0.5 rounded border", EFFECT_COLORS[policy.effect])}>{EFFECT_LABELS[policy.effect]}</span>
        </td>
        <td className="px-4 py-3 text-gray-400 text-sm">{policy.priority}</td>
        <td className="px-4 py-3">
          <button onClick={onToggle} className={cn("w-8 h-4 rounded-full transition-colors relative", policy.enabled ? "bg-blue-600" : "bg-gray-700")}>
            <div className={cn("absolute top-0.5 w-3 h-3 bg-white rounded-full transition-transform", policy.enabled ? "translate-x-4" : "translate-x-0.5")} />
          </button>
        </td>
        <td className="px-4 py-3">
          <div className="flex items-center gap-1">
            <button onClick={() => setExpanded(p => !p)} className="p-1.5 rounded hover:bg-gray-700 text-gray-400" title="Details">
              {expanded ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
            </button>
            <button onClick={onEdit} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-blue-400" title="Edit"><Edit2 className="w-3.5 h-3.5" /></button>
            <button onClick={onAssign} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-green-400" title="Assign"><Shield className="w-3.5 h-3.5" /></button>
            <button onClick={onDelete} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-red-400" title="Delete"><Trash2 className="w-3.5 h-3.5" /></button>
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-gray-800/30 bg-gray-900/50">
          <td colSpan={6} className="px-6 py-3">
            <div className="space-y-2">
              <div>
                <span className="text-xs text-gray-500 uppercase tracking-wide">Operations: </span>
                <span className="text-xs text-gray-300">{(policy.operations ?? []).join(", ") || "—"}</span>
              </div>
              {(policy.conditions ?? []).length > 0 && (
                <div>
                  <span className="text-xs text-gray-500 uppercase tracking-wide">Conditions:</span>
                  <div className="flex flex-wrap gap-1 mt-1">
                    {policy.conditions!.map((c, i) => (
                      <span key={i} className="text-xs bg-gray-800 text-gray-300 px-2 py-0.5 rounded border border-gray-700">
                        {c.negate ? "NOT " : ""}{c.type} {c.operator} {c.value}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────
export default function PoliciesPage() {
  const qc = useQueryClient();
  const { toasts, add: addToast } = useToast();
  const [showForm, setShowForm] = useState(false);
  const [editPolicy, setEditPolicy] = useState<Policy | null>(null);
  const [assignPolicy, setAssignPolicy] = useState<Policy | null>(null);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [effectFilter, setEffectFilter] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["policies", { search, typeFilter, effectFilter }],
    queryFn: () => policyApi.list({ search: search || undefined, type: typeFilter || undefined, effect: effectFilter || undefined, page_size: 50 }),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => policyApi.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["policies"] }); addToast("success", "Policy deleted"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? policyApi.disable(id) : policyApi.enable(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["policies"] }),
    onError: (e: Error) => addToast("error", e.message),
  });

  const policies = data?.data ?? [];

  return (
    <div className="space-y-5">
      {/* Toasts */}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map(t => (
          <div key={t.id} className={cn("flex items-center gap-2 px-4 py-3 rounded-xl border text-sm shadow-lg", t.type === "success" ? "bg-green-950 border-green-800 text-green-200" : "bg-red-950 border-red-800 text-red-200")}>
            {t.type === "success" ? <Check className="w-4 h-4" /> : <AlertTriangle className="w-4 h-4" />}
            {t.text}
          </div>
        ))}
      </div>

      {showForm && <PolicyForm onClose={() => setShowForm(false)} onSaved={() => { setShowForm(false); addToast("success", "Policy created"); }} />}
      {editPolicy && <PolicyForm initial={editPolicy} onClose={() => setEditPolicy(null)} onSaved={() => { setEditPolicy(null); addToast("success", "Policy updated"); }} />}
      {assignPolicy && <AssignModal policy={assignPolicy} onClose={() => setAssignPolicy(null)} />}

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2"><Shield className="w-6 h-6 text-blue-400" />Policies</h1>
          <p className="text-gray-400 text-sm mt-0.5">Governance rules that control infrastructure operations</p>
        </div>
        <button onClick={() => setShowForm(true)} className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg transition-colors">
          <Plus className="w-4 h-4" />New Policy
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 flex-wrap">
        <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search policies…"
          className="w-64 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
        <select value={typeFilter} onChange={e => setTypeFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-blue-500">
          <option value="">All types</option>
          {POLICY_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
        </select>
        <select value={effectFilter} onChange={e => setEffectFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-blue-500">
          <option value="">All effects</option>
          {POLICY_EFFECTS.map(e => <option key={e} value={e}>{EFFECT_LABELS[e]}</option>)}
        </select>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Policy", "Type", "Effect", "Priority", "Enabled", "Actions"].map(h => (
                <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? Array.from({ length: 4 }).map((_, i) => (
              <tr key={i} className="border-b border-gray-800/50">
                {Array.from({ length: 6 }).map((_, j) => (
                  <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-24" /></td>
                ))}
              </tr>
            )) : policies.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-12 text-center text-gray-500">No policies found. Create one to start governing operations.</td></tr>
            ) : policies.map(p => (
              <PolicyRow key={p.id} policy={p}
                onEdit={() => setEditPolicy(p)}
                onAssign={() => setAssignPolicy(p)}
                onDelete={() => { if (confirm(`Delete policy "${p.name}"?`)) deleteMut.mutate(p.id); }}
                onToggle={() => toggleMut.mutate({ id: p.id, enabled: p.enabled })}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
