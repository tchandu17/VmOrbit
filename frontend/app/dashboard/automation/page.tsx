"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { workflowApi, type CreateWorkflowPayload, type WorkflowActionPayload } from "@/lib/api/workflows";
import { cn, relativeTime } from "@/lib/utils";
import type { Workflow, WorkflowRun, WorkflowTriggerType, WorkflowActionType } from "@/types";
import {
  Plus, Play, Pause, Trash2, ChevronRight, X, Zap,
  CheckCircle, XCircle, Loader2, Clock, GitBranch,
} from "lucide-react";

// ── Helpers ───────────────────────────────────────────────────────────────────

const triggerLabels: Record<WorkflowTriggerType, string> = {
  schedule:             "Scheduled",
  provider_disconnected:"Provider Disconnected",
  sync_failure:         "Sync Failure",
  task_failure:         "Task Failure",
  vm_state_change:      "VM State Change",
  manual:               "Manual",
};

const triggerColors: Record<WorkflowTriggerType, string> = {
  schedule:             "bg-blue-900/40 text-blue-300",
  provider_disconnected:"bg-red-900/40 text-red-300",
  sync_failure:         "bg-orange-900/40 text-orange-300",
  task_failure:         "bg-red-900/40 text-red-300",
  vm_state_change:      "bg-purple-900/40 text-purple-300",
  manual:               "bg-gray-700 text-gray-300",
};

const actionLabels: Record<WorkflowActionType, string> = {
  create_snapshot:   "Create Snapshot",
  power_on:          "Power On",
  power_off:         "Power Off",
  reboot:            "Reboot",
  send_notification: "Send Notification",
  trigger_sync:      "Trigger Sync",
  webhook:           "Webhook",
  delay:             "Delay",
};

function RunStatusIcon({ status }: { status: WorkflowRun["status"] }) {
  if (status === "completed") return <CheckCircle className="w-3.5 h-3.5 text-green-400" />;
  if (status === "failed")    return <XCircle className="w-3.5 h-3.5 text-red-400" />;
  if (status === "running")   return <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin" />;
  return <Clock className="w-3.5 h-3.5 text-gray-500" />;
}

const runStatusColors: Record<WorkflowRun["status"], string> = {
  pending:   "bg-gray-700 text-gray-300",
  running:   "bg-blue-900/40 text-blue-300",
  completed: "bg-green-900/40 text-green-300",
  failed:    "bg-red-900/40 text-red-300",
  cancelled: "bg-gray-700 text-gray-400",
};

// ── Action Builder ────────────────────────────────────────────────────────────

function ActionRow({
  action, index, onChange, onRemove,
}: {
  action: WorkflowActionPayload;
  index: number;
  onChange: (a: WorkflowActionPayload) => void;
  onRemove: () => void;
}) {
  const actionTypes: WorkflowActionType[] = [
    "create_snapshot","power_on","power_off","reboot",
    "send_notification","trigger_sync","webhook","delay",
  ];

  return (
    <div className="bg-gray-800/60 rounded-xl p-4 space-y-3 border border-gray-700/50">
      <div className="flex items-center justify-between">
        <span className="text-xs text-gray-500 font-medium">Step {index + 1}</span>
        <button type="button" onClick={onRemove} className="p-1 text-gray-600 hover:text-red-400 rounded">
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Action type</label>
          <select
            value={action.action_type}
            onChange={(e) => onChange({ ...action, action_type: e.target.value as WorkflowActionType })}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-blue-500"
          >
            {actionTypes.map((t) => (
              <option key={t} value={t}>{actionLabels[t]}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Name</label>
          <input
            value={action.name ?? ""}
            onChange={(e) => onChange({ ...action, name: e.target.value })}
            placeholder={actionLabels[action.action_type]}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>
      {/* Config JSON */}
      <div>
        <label className="block text-xs text-gray-500 mb-1">Config (JSON)</label>
        <textarea
          rows={3}
          value={action.config ? JSON.stringify(action.config, null, 2) : ""}
          onChange={(e) => {
            try { onChange({ ...action, config: JSON.parse(e.target.value) }); } catch { /* ignore */ }
          }}
          className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white font-mono focus:outline-none focus:border-blue-500 resize-none"
          placeholder='{"target_type":"vm","target_ids":["..."]}'
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Retry count</label>
          <input type="number" min={0} max={5}
            value={action.retry_count ?? 0}
            onChange={(e) => onChange({ ...action, retry_count: parseInt(e.target.value) || 0 })}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-blue-500"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Timeout (seconds)</label>
          <input type="number" min={10}
            value={action.timeout_seconds ?? 300}
            onChange={(e) => onChange({ ...action, timeout_seconds: parseInt(e.target.value) || 300 })}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>
    </div>
  );
}

// ── Workflow Modal ────────────────────────────────────────────────────────────

const EMPTY_WF: CreateWorkflowPayload = {
  name: "", description: "", enabled: true,
  trigger_type: "manual", continue_on_error: false,
  max_concurrent_runs: 1, actions: [],
};

function WorkflowModal({
  initial, onClose, onSave, saving,
}: {
  initial?: Workflow; onClose: () => void;
  onSave: (d: CreateWorkflowPayload) => void; saving: boolean;
}) {
  const [form, setForm] = useState<CreateWorkflowPayload>(
    initial ? {
      name: initial.name, description: initial.description ?? "",
      enabled: initial.enabled, trigger_type: initial.trigger_type,
      trigger_config: initial.trigger_config, conditions: initial.conditions,
      continue_on_error: initial.continue_on_error,
      max_concurrent_runs: initial.max_concurrent_runs,
      actions: (initial.actions ?? []).map((a) => ({
        order: a.order, action_type: a.action_type, name: a.name,
        description: a.description, config: a.config,
        retry_count: a.retry_count, timeout_seconds: a.timeout_seconds,
        continue_on_error: a.continue_on_error,
      })),
    } : EMPTY_WF
  );

  function setF<K extends keyof CreateWorkflowPayload>(k: K, v: CreateWorkflowPayload[K]) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  function addAction() {
    setF("actions", [...(form.actions ?? []), {
      order: (form.actions?.length ?? 0) + 1,
      action_type: "create_snapshot", retry_count: 0, timeout_seconds: 300,
    }]);
  }

  function updateAction(i: number, a: WorkflowActionPayload) {
    const acts = [...(form.actions ?? [])];
    acts[i] = a;
    setF("actions", acts);
  }

  function removeAction(i: number) {
    setF("actions", (form.actions ?? []).filter((_, idx) => idx !== i));
  }

  const triggerTypes: WorkflowTriggerType[] = [
    "manual","schedule","provider_disconnected","sync_failure","task_failure","vm_state_change",
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800 sticky top-0 bg-gray-900 z-10">
          <h2 className="text-white font-semibold">{initial ? "Edit Workflow" : "New Workflow"}</h2>
          <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800">
            <X className="w-4 h-4" />
          </button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); onSave(form); }} className="px-6 py-5 space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Name *</label>
              <input required value={form.name} onChange={(e) => setF("name", e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                placeholder="Auto snapshot production VMs" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Trigger *</label>
              <select value={form.trigger_type}
                onChange={(e) => setF("trigger_type", e.target.value as WorkflowTriggerType)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500">
                {triggerTypes.map((t) => <option key={t} value={t}>{triggerLabels[t]}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Description</label>
            <input value={form.description} onChange={(e) => setF("description", e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
              placeholder="Optional description" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Max concurrent runs</label>
              <input type="number" min={1} value={form.max_concurrent_runs ?? 1}
                onChange={(e) => setF("max_concurrent_runs", parseInt(e.target.value) || 1)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            <div className="flex items-end gap-4 pb-1">
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={form.continue_on_error}
                  onChange={(e) => setF("continue_on_error", e.target.checked)}
                  className="rounded border-gray-600 bg-gray-800 text-blue-500" />
                <span className="text-sm text-gray-300">Continue on error</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={form.enabled}
                  onChange={(e) => setF("enabled", e.target.checked)}
                  className="rounded border-gray-600 bg-gray-800 text-blue-500" />
                <span className="text-sm text-gray-300">Enabled</span>
              </label>
            </div>
          </div>

          {/* Actions */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <label className="text-xs text-gray-400 font-medium uppercase tracking-wide">Actions</label>
              <button type="button" onClick={addAction}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors">
                <Plus className="w-3 h-3" /> Add step
              </button>
            </div>
            {(form.actions ?? []).length === 0 ? (
              <p className="text-xs text-gray-600 text-center py-4 border border-dashed border-gray-800 rounded-xl">
                No actions yet. Add a step to define what this workflow does.
              </p>
            ) : (
              <div className="space-y-3">
                {(form.actions ?? []).map((a, i) => (
                  <ActionRow key={i} action={a} index={i}
                    onChange={(updated) => updateAction(i, updated)}
                    onRemove={() => removeAction(i)} />
                ))}
              </div>
            )}
          </div>

          <div className="flex gap-3 pt-2">
            <button type="button" onClick={onClose}
              className="flex-1 px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={saving}
              className="flex-1 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg transition-colors">
              {saving ? "Saving…" : initial ? "Update" : "Create"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Run History Panel ─────────────────────────────────────────────────────────

function RunHistory({ workflow, onClose }: { workflow: Workflow; onClose: () => void }) {
  const [selectedRun, setSelectedRun] = useState<WorkflowRun | null>(null);
  const { data, isLoading } = useQuery({
    queryKey: ["workflow-runs", workflow.id],
    queryFn: () => workflowApi.listRuns(workflow.id, { page: 1, page_size: 20 }),
  });
  const runs: WorkflowRun[] = data?.data ?? [];

  return (
    <div className="fixed inset-0 z-50 flex">
      <div className="flex-1 bg-black/50" onClick={onClose} />
      <div className="w-[520px] bg-gray-900 border-l border-gray-800 flex flex-col h-full">
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-800">
          <div>
            <p className="text-white font-semibold">{workflow.name}</p>
            <p className="text-xs text-gray-500">Run history</p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">
          {isLoading ? (
            <div className="space-y-2">{Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-14 bg-gray-800 rounded-lg animate-pulse" />
            ))}</div>
          ) : runs.length === 0 ? (
            <p className="text-gray-500 text-sm text-center py-8">No runs yet</p>
          ) : (
            <div className="space-y-2">
              {runs.map((run) => (
                <div key={run.id}
                  className="bg-gray-800/50 rounded-lg px-4 py-3 cursor-pointer hover:bg-gray-800 transition-colors"
                  onClick={() => setSelectedRun(selectedRun?.id === run.id ? null : run)}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <RunStatusIcon status={run.status} />
                      <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium", runStatusColors[run.status])}>
                        {run.status}
                      </span>
                      <span className="text-xs text-gray-500">{triggerLabels[run.trigger_type]}</span>
                    </div>
                    <span className="text-xs text-gray-500">{relativeTime(run.created_at)}</span>
                  </div>
                  <div className="flex gap-4 mt-1.5 text-xs text-gray-500">
                    <span>{run.actions_run} actions</span>
                    {run.actions_failed > 0 && <span className="text-red-400">{run.actions_failed} failed</span>}
                  </div>
                  {run.error_message && (
                    <p className="text-xs text-red-400 mt-1 truncate">{run.error_message}</p>
                  )}
                  {/* Expanded logs */}
                  {selectedRun?.id === run.id && run.logs?.entries && (
                    <div className="mt-3 space-y-1.5 border-t border-gray-700 pt-3">
                      {run.logs.entries.map((entry, i) => (
                        <div key={i} className="flex items-start gap-2 text-xs">
                          {entry.status === "completed"
                            ? <CheckCircle className="w-3 h-3 text-green-400 mt-0.5 shrink-0" />
                            : <XCircle className="w-3 h-3 text-red-400 mt-0.5 shrink-0" />}
                          <div>
                            <span className="text-gray-300">{entry.action || entry.action_type}</span>
                            {entry.error && <p className="text-red-400">{entry.error}</p>}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export default function AutomationPage() {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Workflow | null>(null);
  const [viewRuns, setViewRuns] = useState<Workflow | null>(null);
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ["workflows", { page }],
    queryFn: () => workflowApi.list({ page, page_size: 20 }),
    refetchInterval: 30_000,
  });

  const createMut = useMutation({
    mutationFn: workflowApi.create,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["workflows"] }); setShowModal(false); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreateWorkflowPayload> }) =>
      workflowApi.update(id, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["workflows"] }); setEditing(null); },
  });

  const deleteMut = useMutation({
    mutationFn: workflowApi.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflows"] }),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? workflowApi.disable(id) : workflowApi.enable(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflows"] }),
  });

  const triggerMut = useMutation({
    mutationFn: (id: string) => workflowApi.triggerNow(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflows"] }),
  });

  const workflows: Workflow[] = data?.data ?? [];

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Automation</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {data?.meta?.total_items ?? 0} workflows · event-driven infrastructure automation
          </p>
        </div>
        <button onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded-lg transition-colors">
          <Plus className="w-4 h-4" /> New Workflow
        </button>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Workflow", "Trigger", "Actions", "Status", "Last Run", "Runs", ""].map((h, i) => (
                <th key={i} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-20" /></td>
                    ))}
                  </tr>
                ))
              : workflows.length === 0
              ? (
                <tr>
                  <td colSpan={7} className="px-4 py-12 text-center text-gray-500">
                    No workflows yet. Create one to automate your infrastructure.
                  </td>
                </tr>
              )
              : workflows.map((w) => (
                  <tr key={w.id} className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <GitBranch className="w-4 h-4 text-gray-500 shrink-0" />
                        <div>
                          <p className="text-white font-medium">{w.name}</p>
                          {w.description && <p className="text-xs text-gray-500 truncate max-w-[180px]">{w.description}</p>}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium", triggerColors[w.trigger_type])}>
                        {triggerLabels[w.trigger_type]}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-xs">
                      {(w.actions?.length ?? 0)} steps
                    </td>
                    <td className="px-4 py-3">
                      {w.enabled
                        ? <span className="px-2 py-0.5 rounded-full text-xs bg-green-900/40 text-green-300">Active</span>
                        : <span className="px-2 py-0.5 rounded-full text-xs bg-gray-700 text-gray-400">Paused</span>}
                    </td>
                    <td className="px-4 py-3 text-xs">
                      {w.last_run_status === "success"
                        ? <span className="text-green-400 flex items-center gap-1"><CheckCircle className="w-3 h-3" />Success</span>
                        : w.last_run_status === "failed"
                        ? <span className="text-red-400 flex items-center gap-1"><XCircle className="w-3 h-3" />Failed</span>
                        : <span className="text-gray-600">—</span>}
                      {w.last_run_at && <p className="text-gray-600 mt-0.5">{relativeTime(w.last_run_at)}</p>}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-400">
                      {w.run_count}
                      {w.failure_count > 0 && <span className="text-red-400 ml-1">({w.failure_count} failed)</span>}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <button title="Run now" onClick={() => triggerMut.mutate(w.id)}
                          disabled={!w.enabled || triggerMut.isPending}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-blue-400 hover:bg-gray-800 transition-colors disabled:opacity-40">
                          <Zap className="w-3.5 h-3.5" />
                        </button>
                        <button title={w.enabled ? "Pause" : "Enable"}
                          onClick={() => toggleMut.mutate({ id: w.id, enabled: w.enabled })}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-yellow-400 hover:bg-gray-800 transition-colors">
                          {w.enabled ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
                        </button>
                        <button title="Run history" onClick={() => setViewRuns(w)}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-gray-800 transition-colors">
                          <ChevronRight className="w-3.5 h-3.5" />
                        </button>
                        <button title="Edit" onClick={() => setEditing(w)}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-gray-800 transition-colors">
                          <GitBranch className="w-3.5 h-3.5" />
                        </button>
                        <button title="Delete"
                          onClick={() => { if (confirm(`Delete workflow "${w.name}"?`)) deleteMut.mutate(w.id); }}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-red-400 hover:bg-gray-800 transition-colors">
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
          </tbody>
        </table>

        {data && (data.meta?.total_pages ?? 0) > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {data.meta?.total_pages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">Previous</button>
              <button disabled={page === (data.meta?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">Next</button>
            </div>
          </div>
        )}
      </div>

      {(showModal || editing) && (
        <WorkflowModal
          initial={editing ?? undefined}
          onClose={() => { setShowModal(false); setEditing(null); }}
          onSave={(d) => { if (editing) updateMut.mutate({ id: editing.id, data: d }); else createMut.mutate(d); }}
          saving={createMut.isPending || updateMut.isPending}
        />
      )}
      {viewRuns && <RunHistory workflow={viewRuns} onClose={() => setViewRuns(null)} />}
    </div>
  );
}
