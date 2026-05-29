"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { scheduleApi, type CreateSchedulePayload } from "@/lib/api/schedules";
import { cn, relativeTime } from "@/lib/utils";
import type { Schedule, ScheduleOperationType, ScheduleTargetType, ScheduleType } from "@/types";
import {
  Plus, Play, Pause, Trash2, Clock, ChevronRight, X, Zap,
  CheckCircle, XCircle, AlertCircle, RefreshCw,
} from "lucide-react";

// ── Helpers ───────────────────────────────────────────────────────────────────

const opLabels: Record<ScheduleOperationType, string> = {
  "inventory.sync":    "Inventory Sync",
  "vm.power_on":       "VM Power On",
  "vm.power_off":      "VM Power Off",
  "vm.reboot":         "VM Reboot",
  "vm.snapshot":       "VM Snapshot",
  "vm.bulk.power_on":  "Bulk Power On",
  "vm.bulk.power_off": "Bulk Power Off",
  "vm.bulk.reboot":    "Bulk Reboot",
  "vm.bulk.snapshot":  "Bulk Snapshot",
};

const opColors: Record<ScheduleOperationType, string> = {
  "inventory.sync":    "bg-blue-900/40 text-blue-300",
  "vm.power_on":       "bg-green-900/40 text-green-300",
  "vm.power_off":      "bg-red-900/40 text-red-300",
  "vm.reboot":         "bg-yellow-900/40 text-yellow-300",
  "vm.snapshot":       "bg-purple-900/40 text-purple-300",
  "vm.bulk.power_on":  "bg-green-900/40 text-green-300",
  "vm.bulk.power_off": "bg-red-900/40 text-red-300",
  "vm.bulk.reboot":    "bg-yellow-900/40 text-yellow-300",
  "vm.bulk.snapshot":  "bg-purple-900/40 text-purple-300",
};

function StatusBadge({ status, enabled }: { status: Schedule["status"]; enabled: boolean }) {
  if (!enabled) return <span className="px-2 py-0.5 rounded-full text-xs bg-gray-700 text-gray-400">Paused</span>;
  const map: Record<string, string> = {
    active:   "bg-green-900/40 text-green-300",
    paused:   "bg-gray-700 text-gray-400",
    disabled: "bg-gray-700 text-gray-500",
    expired:  "bg-orange-900/40 text-orange-300",
  };
  return <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium", map[status] ?? "bg-gray-700 text-gray-400")}>{status}</span>;
}

function LastRunBadge({ status }: { status?: string }) {
  if (!status) return <span className="text-gray-600 text-xs">—</span>;
  if (status === "success") return <span className="flex items-center gap-1 text-green-400 text-xs"><CheckCircle className="w-3 h-3" />Success</span>;
  return <span className="flex items-center gap-1 text-red-400 text-xs"><XCircle className="w-3 h-3" />Failed</span>;
}

// ── Cron Builder ──────────────────────────────────────────────────────────────

function CronBuilder({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const presets = [
    { label: "Every hour",       cron: "0 * * * *" },
    { label: "Every day at 2am", cron: "0 2 * * *" },
    { label: "Every day at midnight", cron: "0 0 * * *" },
    { label: "Every Sunday 3am", cron: "0 3 * * 0" },
    { label: "Every Monday 6am", cron: "0 6 * * 1" },
    { label: "1st of month midnight", cron: "0 0 1 * *" },
    { label: "Every 15 minutes", cron: "*/15 * * * *" },
    { label: "Every 30 minutes", cron: "*/30 * * * *" },
  ];

  return (
    <div className="space-y-2">
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="* * * * *  (min hour day month weekday)"
        className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white font-mono placeholder-gray-600 focus:outline-none focus:border-blue-500"
      />
      <div className="flex flex-wrap gap-1.5">
        {presets.map((p) => (
          <button
            key={p.cron}
            type="button"
            onClick={() => onChange(p.cron)}
            className={cn(
              "px-2 py-1 rounded text-xs transition-colors",
              value === p.cron
                ? "bg-blue-600 text-white"
                : "bg-gray-800 text-gray-400 hover:text-white hover:bg-gray-700"
            )}
          >
            {p.label}
          </button>
        ))}
      </div>
      <p className="text-xs text-gray-600">Format: minute hour day-of-month month day-of-week</p>
    </div>
  );
}

// ── Create/Edit Modal ─────────────────────────────────────────────────────────

const EMPTY_FORM: CreateSchedulePayload = {
  name: "",
  description: "",
  operation_type: "inventory.sync",
  target_type: "hypervisor",
  target_ids: [],
  schedule_type: "cron",
  cron_expression: "0 2 * * *",
  timezone: "UTC",
  enabled: true,
  max_runs: 0,
};

function ScheduleModal({
  initial,
  onClose,
  onSave,
  saving,
}: {
  initial?: Schedule;
  onClose: () => void;
  onSave: (data: CreateSchedulePayload) => void;
  saving: boolean;
}) {
  const [form, setForm] = useState<CreateSchedulePayload>(
    initial
      ? {
          name: initial.name,
          description: initial.description ?? "",
          operation_type: initial.operation_type,
          target_type: initial.target_type,
          target_ids: initial.target_ids,
          schedule_type: initial.schedule_type,
          cron_expression: initial.cron_expression,
          timezone: initial.timezone,
          enabled: initial.enabled,
          max_runs: initial.max_runs,
          payload: initial.payload,
        }
      : EMPTY_FORM
  );

  const [targetIDsRaw, setTargetIDsRaw] = useState(
    (initial?.target_ids ?? []).join("\n")
  );

  function set<K extends keyof CreateSchedulePayload>(k: K, v: CreateSchedulePayload[K]) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const ids = targetIDsRaw
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    onSave({ ...form, target_ids: ids });
  }

  const scheduleTypes: { value: ScheduleType; label: string }[] = [
    { value: "cron",    label: "Cron expression" },
    { value: "daily",   label: "Daily" },
    { value: "weekly",  label: "Weekly" },
    { value: "monthly", label: "Monthly" },
    { value: "once",    label: "One-time" },
  ];

  const opTypes: { value: ScheduleOperationType; label: string }[] = Object.entries(opLabels).map(
    ([value, label]) => ({ value: value as ScheduleOperationType, label })
  );

  const targetTypes: { value: ScheduleTargetType; label: string }[] = [
    { value: "hypervisor", label: "Hypervisor" },
    { value: "vm",         label: "Virtual Machine" },
    { value: "tag",        label: "Tag" },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <h2 className="text-white font-semibold">{initial ? "Edit Schedule" : "New Schedule"}</h2>
          <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800">
            <X className="w-4 h-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          {/* Name */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Name *</label>
            <input
              required
              value={form.name}
              onChange={(e) => set("name", e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
              placeholder="Nightly snapshot"
            />
          </div>

          {/* Description */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Description</label>
            <input
              value={form.description}
              onChange={(e) => set("description", e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
              placeholder="Optional description"
            />
          </div>

          {/* Operation type */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Operation *</label>
            <select
              value={form.operation_type}
              onChange={(e) => set("operation_type", e.target.value as ScheduleOperationType)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
            >
              {opTypes.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>

          {/* Target type + IDs */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Target type *</label>
              <select
                value={form.target_type}
                onChange={(e) => set("target_type", e.target.value as ScheduleTargetType)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
              >
                {targetTypes.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Timezone</label>
              <input
                value={form.timezone}
                onChange={(e) => set("timezone", e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
                placeholder="UTC"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">Target IDs (one per line) *</label>
            <textarea
              required
              rows={3}
              value={targetIDsRaw}
              onChange={(e) => setTargetIDsRaw(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-blue-500 resize-none"
              placeholder="uuid-1&#10;uuid-2"
            />
          </div>

          {/* Schedule type */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Schedule type *</label>
            <select
              value={form.schedule_type}
              onChange={(e) => set("schedule_type", e.target.value as ScheduleType)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
            >
              {scheduleTypes.map((s) => (
                <option key={s.value} value={s.value}>{s.label}</option>
              ))}
            </select>
          </div>

          {/* Cron expression */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Cron expression *</label>
            <CronBuilder
              value={form.cron_expression ?? ""}
              onChange={(v) => set("cron_expression", v)}
            />
          </div>

          {/* Max runs */}
          <div>
            <label className="block text-xs text-gray-400 mb-1">Max runs (0 = unlimited)</label>
            <input
              type="number"
              min={0}
              value={form.max_runs ?? 0}
              onChange={(e) => set("max_runs", parseInt(e.target.value) || 0)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500"
            />
          </div>

          {/* Enabled */}
          <label className="flex items-center gap-3 cursor-pointer">
            <div
              onClick={() => set("enabled", !form.enabled)}
              className={cn(
                "w-10 h-5 rounded-full transition-colors relative",
                form.enabled ? "bg-blue-600" : "bg-gray-700"
              )}
            >
              <div className={cn(
                "absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform",
                form.enabled ? "translate-x-5" : "translate-x-0.5"
              )} />
            </div>
            <span className="text-sm text-gray-300">Enabled</span>
          </label>

          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="flex-1 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg transition-colors"
            >
              {saving ? "Saving…" : initial ? "Update" : "Create"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Execution History Panel ───────────────────────────────────────────────────

function ExecutionHistory({ schedule, onClose }: { schedule: Schedule; onClose: () => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ["schedule-executions", schedule.id],
    queryFn: () => scheduleApi.listExecutions(schedule.id, { page: 1, page_size: 20 }),
  });

  const executions = data?.data ?? [];

  return (
    <div className="fixed inset-0 z-50 flex">
      <div className="flex-1 bg-black/50" onClick={onClose} />
      <div className="w-[480px] bg-gray-900 border-l border-gray-800 flex flex-col h-full">
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-800">
          <div>
            <p className="text-white font-semibold">{schedule.name}</p>
            <p className="text-xs text-gray-500">Execution history</p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-12 bg-gray-800 rounded-lg animate-pulse" />
              ))}
            </div>
          ) : executions.length === 0 ? (
            <p className="text-gray-500 text-sm text-center py-8">No executions yet</p>
          ) : (
            <div className="space-y-2">
              {executions.map((exec) => (
                <div key={exec.id} className="bg-gray-800/50 rounded-lg px-4 py-3 flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      {exec.status === "triggered" ? (
                        <CheckCircle className="w-3.5 h-3.5 text-green-400" />
                      ) : exec.status === "failed" ? (
                        <XCircle className="w-3.5 h-3.5 text-red-400" />
                      ) : (
                        <AlertCircle className="w-3.5 h-3.5 text-yellow-400" />
                      )}
                      <span className="text-sm text-white capitalize">{exec.status}</span>
                    </div>
                    {exec.error_message && (
                      <p className="text-xs text-red-400 mt-0.5 truncate max-w-[280px]">{exec.error_message}</p>
                    )}
                    {exec.task_id && (
                      <p className="text-xs text-gray-600 font-mono mt-0.5">Task: {exec.task_id.slice(0, 8)}…</p>
                    )}
                  </div>
                  <span className="text-xs text-gray-500 shrink-0">{relativeTime(exec.triggered_at)}</span>
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

export default function SchedulesPage() {
  const qc = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<Schedule | null>(null);
  const [viewHistory, setViewHistory] = useState<Schedule | null>(null);
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ["schedules", { page }],
    queryFn: () => scheduleApi.list({ page, page_size: 20 }),
    refetchInterval: 30_000,
  });

  const createMut = useMutation({
    mutationFn: scheduleApi.create,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["schedules"] }); setShowModal(false); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreateSchedulePayload> }) =>
      scheduleApi.update(id, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["schedules"] }); setEditing(null); },
  });

  const deleteMut = useMutation({
    mutationFn: scheduleApi.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? scheduleApi.disable(id) : scheduleApi.enable(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });

  const triggerMut = useMutation({
    mutationFn: scheduleApi.triggerNow,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });

  const schedules: Schedule[] = data?.data ?? [];

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Schedules</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {data?.meta?.total_items ?? 0} total · automated cron-based operations
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Schedule
        </button>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Name", "Operation", "Schedule", "Status", "Last Run", "Next Run", "Runs", ""].map((h, i) => (
                <th key={i} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 8 }).map((_, j) => (
                      <td key={j} className="px-4 py-3">
                        <div className="h-4 bg-gray-800 rounded animate-pulse w-20" />
                      </td>
                    ))}
                  </tr>
                ))
              : schedules.length === 0
              ? (
                <tr>
                  <td colSpan={8} className="px-4 py-12 text-center text-gray-500">
                    No schedules yet. Create one to automate operations.
                  </td>
                </tr>
              )
              : schedules.map((s) => (
                  <tr key={s.id} className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
                    <td className="px-4 py-3">
                      <p className="text-white font-medium">{s.name}</p>
                      {s.description && <p className="text-xs text-gray-500 truncate max-w-[160px]">{s.description}</p>}
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium", opColors[s.operation_type])}>
                        {opLabels[s.operation_type]}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1.5">
                        <Clock className="w-3 h-3 text-gray-500 shrink-0" />
                        <span className="text-gray-300 font-mono text-xs">{s.cron_expression}</span>
                      </div>
                      <p className="text-xs text-gray-600 mt-0.5">{s.timezone}</p>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={s.status} enabled={s.enabled} />
                    </td>
                    <td className="px-4 py-3">
                      <LastRunBadge status={s.last_run_status} />
                      {s.last_run_at && <p className="text-xs text-gray-600 mt-0.5">{relativeTime(s.last_run_at)}</p>}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-400">
                      {s.next_run_at ? relativeTime(s.next_run_at) : "—"}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-400">
                      {s.run_count}
                      {s.failure_count > 0 && (
                        <span className="text-red-400 ml-1">({s.failure_count} failed)</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        {/* Trigger now */}
                        <button
                          title="Run now"
                          onClick={() => triggerMut.mutate(s.id)}
                          disabled={triggerMut.isPending}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-blue-400 hover:bg-gray-800 transition-colors"
                        >
                          <Zap className="w-3.5 h-3.5" />
                        </button>
                        {/* Toggle enable */}
                        <button
                          title={s.enabled ? "Pause" : "Enable"}
                          onClick={() => toggleMut.mutate({ id: s.id, enabled: s.enabled })}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-yellow-400 hover:bg-gray-800 transition-colors"
                        >
                          {s.enabled ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
                        </button>
                        {/* History */}
                        <button
                          title="Execution history"
                          onClick={() => setViewHistory(s)}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-gray-800 transition-colors"
                        >
                          <ChevronRight className="w-3.5 h-3.5" />
                        </button>
                        {/* Edit */}
                        <button
                          title="Edit"
                          onClick={() => setEditing(s)}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-gray-800 transition-colors"
                        >
                          <RefreshCw className="w-3.5 h-3.5" />
                        </button>
                        {/* Delete */}
                        <button
                          title="Delete"
                          onClick={() => { if (confirm(`Delete schedule "${s.name}"?`)) deleteMut.mutate(s.id); }}
                          className="p-1.5 rounded-lg text-gray-500 hover:text-red-400 hover:bg-gray-800 transition-colors"
                        >
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
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">
                Previous
              </button>
              <button disabled={page === (data.meta?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Modals */}
      {(showModal || editing) && (
        <ScheduleModal
          initial={editing ?? undefined}
          onClose={() => { setShowModal(false); setEditing(null); }}
          onSave={(data) => {
            if (editing) updateMut.mutate({ id: editing.id, data });
            else createMut.mutate(data);
          }}
          saving={createMut.isPending || updateMut.isPending}
        />
      )}

      {viewHistory && (
        <ExecutionHistory schedule={viewHistory} onClose={() => setViewHistory(null)} />
      )}
    </div>
  );
}
