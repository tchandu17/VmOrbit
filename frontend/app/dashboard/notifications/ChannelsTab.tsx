"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2, Pencil, Send, Mail, Hash, Globe, ToggleLeft, ToggleRight } from "lucide-react";
import { notificationChannelApi, type CreateChannelRequest } from "@/lib/api/notifications";
import { cn } from "@/lib/utils";
import type { NotificationChannel, NotificationChannelType } from "@/types";

const CHANNEL_ICONS: Record<NotificationChannelType, React.ElementType> = {
  email: Mail, slack: Hash, webhook: Globe,
};
const CHANNEL_COLORS: Record<NotificationChannelType, string> = {
  email: "text-blue-400 bg-blue-900/30",
  slack: "text-purple-400 bg-purple-900/30",
  webhook: "text-green-400 bg-green-900/30",
};

export function ChannelsTab() {
  const qc = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<NotificationChannel | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["notification-channels"],
    queryFn: () => notificationChannelApi.list(),
  });
  const channels: NotificationChannel[] = data?.data ?? [];

  const deleteMut = useMutation({
    mutationFn: (id: string) => notificationChannelApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notification-channels"] }),
  });

  const testMut = useMutation({
    mutationFn: (id: string) => notificationChannelApi.test(id),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      notificationChannelApi.update(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notification-channels"] }),
  });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-400">{channels.length} channel{channels.length !== 1 ? "s" : ""} configured</p>
        <button
          onClick={() => { setEditing(null); setShowForm(true); }}
          className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
        >
          <Plus className="w-4 h-4" /> Add Channel
        </button>
      </div>

      {(showForm || editing) && (
        <ChannelForm
          initial={editing}
          onClose={() => { setShowForm(false); setEditing(null); }}
          onSaved={() => { setShowForm(false); setEditing(null); qc.invalidateQueries({ queryKey: ["notification-channels"] }); }}
        />
      )}

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-20 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
          ))}
        </div>
      ) : channels.length === 0 ? (
        <div className="text-center py-16 bg-gray-900 border border-gray-800 rounded-2xl">
          <Mail className="w-10 h-10 mx-auto mb-3 text-gray-700" />
          <p className="text-gray-500">No channels configured yet.</p>
          <p className="text-gray-600 text-xs mt-1">Add a channel to start receiving notifications.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {channels.map((ch) => {
            const Icon = CHANNEL_ICONS[ch.type] ?? Globe;
            const color = CHANNEL_COLORS[ch.type] ?? "text-gray-400 bg-gray-800";
            return (
              <div key={ch.id} className="flex items-center gap-4 p-4 bg-gray-900 border border-gray-800 rounded-xl">
                <div className={cn("w-10 h-10 rounded-xl flex items-center justify-center shrink-0", color)}>
                  <Icon className="w-5 h-5" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-white">{ch.name}</span>
                    <span className="text-xs text-gray-500 bg-gray-800 px-1.5 py-0.5 rounded capitalize">{ch.type}</span>
                    {!ch.enabled && <span className="text-xs text-gray-600 bg-gray-800 px-1.5 py-0.5 rounded">Disabled</span>}
                  </div>
                  {ch.description && <p className="text-xs text-gray-500 mt-0.5 truncate">{ch.description}</p>}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <button
                    onClick={() => toggleMut.mutate({ id: ch.id, enabled: !ch.enabled })}
                    title={ch.enabled ? "Disable" : "Enable"}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition-colors"
                  >
                    {ch.enabled ? <ToggleRight className="w-4 h-4 text-green-400" /> : <ToggleLeft className="w-4 h-4" />}
                  </button>
                  <button
                    onClick={() => testMut.mutate(ch.id)}
                    disabled={testMut.isPending}
                    title="Send test notification"
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-blue-400 transition-colors"
                  >
                    <Send className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => { setEditing(ch); setShowForm(false); }}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition-colors"
                  >
                    <Pencil className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => { if (confirm(`Delete channel "${ch.name}"?`)) deleteMut.mutate(ch.id); }}
                    className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-red-400 transition-colors"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ── Channel Form ──────────────────────────────────────────────────────────────

function ChannelForm({
  initial, onClose, onSaved,
}: {
  initial: NotificationChannel | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [type, setType] = useState<NotificationChannelType>(initial?.type ?? "slack");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [config, setConfig] = useState(
    initial?.config ? JSON.stringify(initial.config, null, 2) : getDefaultConfig("slack")
  );
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  function getDefaultConfig(t: NotificationChannelType): string {
    if (t === "slack")   return JSON.stringify({ webhook_url: "", channel: "", username: "VMOrbit" }, null, 2);
    if (t === "email")   return JSON.stringify({ host: "", port: 587, username: "", password: "", from: "", to: [], tls: false }, null, 2);
    if (t === "webhook") return JSON.stringify({ url: "", method: "POST", headers: {}, secret: "" }, null, 2);
    return "{}";
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    let parsedConfig: Record<string, unknown>;
    try { parsedConfig = JSON.parse(config); } catch { setError("Config must be valid JSON"); return; }
    setSaving(true);
    try {
      if (initial) {
        await notificationChannelApi.update(initial.id, { name, description, enabled, config: parsedConfig });
      } else {
        await notificationChannelApi.create({ name, type, description, enabled, config: parsedConfig } as CreateChannelRequest);
      }
      onSaved();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save channel");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="bg-gray-900 border border-blue-500/30 rounded-xl p-5 space-y-4">
      <h3 className="font-semibold text-white">{initial ? "Edit Channel" : "New Channel"}</h3>
      {error && <p className="text-sm text-red-400 bg-red-900/20 border border-red-800/40 rounded-lg px-3 py-2">{error}</p>}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs text-gray-400 mb-1">Name *</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Type *</label>
          <select value={type} onChange={(e) => { setType(e.target.value as NotificationChannelType); setConfig(getDefaultConfig(e.target.value as NotificationChannelType)); }}
            disabled={!!initial}
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 focus:outline-none focus:border-blue-500 disabled:opacity-50">
            <option value="slack">Slack</option>
            <option value="email">Email</option>
            <option value="webhook">Webhook</option>
          </select>
        </div>
      </div>
      <div>
        <label className="block text-xs text-gray-400 mb-1">Description</label>
        <input value={description} onChange={(e) => setDescription(e.target.value)}
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500" />
      </div>
      <div>
        <label className="block text-xs text-gray-400 mb-1">Configuration (JSON) *</label>
        <textarea value={config} onChange={(e) => setConfig(e.target.value)} rows={6} required
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white font-mono focus:outline-none focus:border-blue-500 resize-none" />
      </div>
      <div className="flex items-center gap-2">
        <input type="checkbox" id="ch-enabled" checked={enabled} onChange={(e) => setEnabled(e.target.checked)}
          className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500" />
        <label htmlFor="ch-enabled" className="text-sm text-gray-300">Enabled</label>
      </div>
      <div className="flex gap-3 justify-end">
        <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">Cancel</button>
        <button type="submit" disabled={saving} className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50">
          {saving ? "Saving…" : "Save Channel"}
        </button>
      </div>
    </form>
  );
}
