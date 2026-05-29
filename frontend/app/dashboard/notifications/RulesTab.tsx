"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2, Pencil, ToggleLeft, ToggleRight, ShieldAlert } from "lucide-react";
import { notificationRuleApi, notificationChannelApi, type CreateRuleRequest } from "@/lib/api/notifications";
import { cn, formatDate } from "@/lib/utils";
import type { NotificationRule, NotificationChannel } from "@/types";

const ALL_EVENT_TYPES = [
  "provider_connected", "provider_disconnected", "sync_completed", "sync_failed",
  "vm_poweron_success", "vm_poweron_failed", "vm_poweroff_success", "vm_poweroff_failed",
  "vm_reboot_success", "vm_reboot_failed", "snapshot_created", "snapshot_failed",
  "snapshot_deleted", "snapshot_reverted", "task_failed", "bulk_operation_failed",
  "login_failed", "permission_denied",
];
const ALL_SEVERITIES = ["info", "warning", "critical"];
const ALL_PROVIDERS  = ["vmware", "proxmox", "esxi"];

export function RulesTab() {
  const qc = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<NotificationRule | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["notification-rules"],
    queryFn: () => notificationRuleApi.list(),
  });
  const rules: NotificationRule[] = data?.data ?? [];

  const deleteMut = useMutation({
    mutationFn: (id: string) => notificationRuleApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notification-rules"] }),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      notificationRuleApi.update(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notification-rules"] }),
  });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-400">{rules.length} rule{rules.length !== 1 ? "s" : ""}</p>
        <button
          onClick={() => { setEditing(null); setShowForm(true); }}
          className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
        >
          <Plus className="w-4 h-4" /> Add Rule
        </button>
      </div>

      {(showForm || editing) && (
        <RuleForm
          initial={editing}
          onClose={() => { setShowForm(false); setEditing(null); }}
          onSaved={() => { setShowForm(false); setEditing(null); qc.invalidateQueries({ queryKey: ["notification-rules"] }); }}
        />
      )}

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-24 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
          ))}
        </div>
      ) : rules.length === 0 ? (
        <div className="text-center py-16 bg-gray-900 border border-gray-800 rounded-2xl">
          <ShieldAlert className="w-10 h-10 mx-auto mb-3 text-gray-700" />
          <p className="text-gray-500">No notification rules yet.</p>
          <p className="text-gray-600 text-xs mt-1">Create a rule to route events to a channel.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {rules.map((rule) => (
            <div key={rule.id} className="p-4 bg-gray-900 border border-gray-800 rounded-xl">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium text-white">{rule.name}</span>
                    {!rule.enabled && <span className="text-xs text-gray-600 bg-gray-800 px-1.5 py-0.5 rounded">Disabled</span>}
                    {rule.channel && (
                      <span className="text-xs text-blue-400 bg-blue-900/20 border border-blue-500/20 px-1.5 py-0.5 rounded">
                        → {rule.channel.name}
                      </span>
                    )}
                  </div>
                  {rule.description && <p className="text-xs text-gray-500 mt-0.5">{rule.description}</p>}
                  <div className="flex flex-wrap gap-1.5 mt-2">
                    {rule.event_types?.length > 0 && rule.event_types.map((et) => (
                      <span key={et} className="text-[10px] bg-gray-800 text-gray-400 px-1.5 py-0.5 rounded font-mono">{et}</span>
                    ))}
                    {rule.severities?.length > 0 && rule.severities.map((s) => (
                      <span key={s} className={cn("text-[10px] px-1.5 py-0.5 rounded font-semibold",
                        s === "critical" ? "bg-red-900/30 text-red-400" :
                        s === "warning"  ? "bg-yellow-900/30 text-yellow-400" :
                                           "bg-blue-900/30 text-blue-400"
                      )}>{s}</span>
                    ))}
                    {rule.providers?.length > 0 && rule.providers.map((p) => (
                      <span key={p} className="text-[10px] bg-gray-800 text-gray-400 px-1.5 py-0.5 rounded">{p}</span>
                    ))}
                    {rule.throttle_seconds > 0 && (
                      <span className="text-[10px] bg-gray-800 text-gray-500 px-1.5 py-0.5 rounded">
                        throttle {rule.throttle_seconds}s
                      </span>
                    )}
                  </div>
                  {rule.last_triggered_at && (
                    <p className="text-[10px] text-gray-600 mt-1">Last triggered: {formatDate(rule.last_triggered_at)}</p>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={() => toggleMut.mutate({ id: rule.id, enabled: !rule.enabled })}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition-colors">
                    {rule.enabled ? <ToggleRight className="w-4 h-4 text-green-400" /> : <ToggleLeft className="w-4 h-4" />}
                  </button>
                  <button onClick={() => { setEditing(rule); setShowForm(false); }}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition-colors">
                    <Pencil className="w-4 h-4" />
                  </button>
                  <button onClick={() => { if (confirm(`Delete rule "${rule.name}"?`)) deleteMut.mutate(rule.id); }}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-red-400 transition-colors">
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Rule Form ─────────────────────────────────────────────────────────────────

function MultiSelect({ label, options, value, onChange }: {
  label: string; options: string[]; value: string[]; onChange: (v: string[]) => void;
}) {
  function toggle(opt: string) {
    onChange(value.includes(opt) ? value.filter((v) => v !== opt) : [...value, opt]);
  }
  return (
    <div>
      <label className="block text-xs text-gray-400 mb-1">{label} <span className="text-gray-600">(empty = all)</span></label>
      <div className="flex flex-wrap gap-1.5">
        {options.map((opt) => (
          <button key={opt} type="button" onClick={() => toggle(opt)}
            className={cn("text-xs px-2 py-1 rounded border transition-colors font-mono",
              value.includes(opt)
                ? "bg-blue-600/20 border-blue-500/40 text-blue-400"
                : "bg-gray-800 border-gray-700 text-gray-400 hover:text-white"
            )}>
            {opt}
          </button>
        ))}
      </div>
    </div>
  );
}

function RuleForm({ initial, onClose, onSaved }: {
  initial: NotificationRule | null; onClose: () => void; onSaved: () => void;
}) {
  const [name, setName]               = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [channelID, setChannelID]     = useState(initial?.channel_id ?? "");
  const [eventTypes, setEventTypes]   = useState<string[]>(initial?.event_types ?? []);
  const [severities, setSeverities]   = useState<string[]>(initial?.severities ?? []);
  const [providers, setProviders]     = useState<string[]>(initial?.providers ?? []);
  const [throttle, setThrottle]       = useState(String(initial?.throttle_seconds ?? 0));
  const [enabled, setEnabled]         = useState(initial?.enabled ?? true);
  const [error, setError]             = useState("");
  const [saving, setSaving]           = useState(false);

  const { data: chData } = useQuery({
    queryKey: ["notification-channels"],
    queryFn: () => notificationChannelApi.list(),
  });
  const channels: NotificationChannel[] = chData?.data ?? [];

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!channelID) { setError("Please select a channel"); return; }
    setSaving(true);
    try {
      const payload: CreateRuleRequest = {
        name, description, channel_id: channelID,
        event_types: eventTypes, severities, providers,
        throttle_seconds: parseInt(throttle) || 0, enabled,
      };
      if (initial) {
        await notificationRuleApi.update(initial.id, payload);
      } else {
        await notificationRuleApi.create(payload);
      }
      onSaved();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save rule");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="bg-gray-900 border border-blue-500/30 rounded-xl p-5 space-y-4">
      <h3 className="font-semibold text-white">{initial ? "Edit Rule" : "New Rule"}</h3>
      {error && <p className="text-sm text-red-400 bg-red-900/20 border border-red-800/40 rounded-lg px-3 py-2">{error}</p>}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs text-gray-400 mb-1">Name *</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Channel *</label>
          <select value={channelID} onChange={(e) => setChannelID(e.target.value)} required
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 focus:outline-none focus:border-blue-500">
            <option value="">Select channel…</option>
            {channels.map((ch) => <option key={ch.id} value={ch.id}>{ch.name} ({ch.type})</option>)}
          </select>
        </div>
      </div>
      <div>
        <label className="block text-xs text-gray-400 mb-1">Description</label>
        <input value={description} onChange={(e) => setDescription(e.target.value)}
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" />
      </div>
      <MultiSelect label="Event Types" options={ALL_EVENT_TYPES} value={eventTypes} onChange={setEventTypes} />
      <MultiSelect label="Severities" options={ALL_SEVERITIES} value={severities} onChange={setSeverities} />
      <MultiSelect label="Providers" options={ALL_PROVIDERS} value={providers} onChange={setProviders} />
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs text-gray-400 mb-1">Throttle (seconds)</label>
          <input type="number" min="0" value={throttle} onChange={(e) => setThrottle(e.target.value)}
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" />
        </div>
        <div className="flex items-end pb-2">
          <label className="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)}
              className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500" />
            <span className="text-sm text-gray-300">Enabled</span>
          </label>
        </div>
      </div>
      <div className="flex gap-3 justify-end">
        <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">Cancel</button>
        <button type="submit" disabled={saving} className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50">
          {saving ? "Saving…" : "Save Rule"}
        </button>
      </div>
    </form>
  );
}
