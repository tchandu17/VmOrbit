"use client";
import { useState, useEffect, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity, Search, Filter, RefreshCw, ChevronLeft, ChevronRight,
  X, AlertTriangle, Info, Zap, Server, Cpu, CheckCircle2,
} from "lucide-react";
import { eventApi, type EventListParams } from "@/lib/api/events";
import { wsClient } from "@/lib/ws/WSClient";
import { cn, formatDate } from "@/lib/utils";
import type { PlatformEvent, PlatformEventType, PlatformEventSeverity } from "@/types";

// ── Constants ─────────────────────────────────────────────────────────────────

const EVENT_TYPES: { value: PlatformEventType; label: string }[] = [
  { value: "provider_connected",    label: "Provider Connected" },
  { value: "provider_disconnected", label: "Provider Disconnected" },
  { value: "sync_completed",        label: "Sync Completed" },
  { value: "sync_failed",           label: "Sync Failed" },
  { value: "vm_poweron_success",    label: "VM Power On" },
  { value: "vm_poweron_failed",     label: "VM Power On Failed" },
  { value: "vm_poweroff_success",   label: "VM Power Off" },
  { value: "vm_poweroff_failed",    label: "VM Power Off Failed" },
  { value: "vm_reboot_success",     label: "VM Reboot" },
  { value: "vm_reboot_failed",      label: "VM Reboot Failed" },
  { value: "snapshot_created",      label: "Snapshot Created" },
  { value: "snapshot_failed",       label: "Snapshot Failed" },
  { value: "snapshot_deleted",      label: "Snapshot Deleted" },
  { value: "snapshot_reverted",     label: "Snapshot Reverted" },
  { value: "task_failed",           label: "Task Failed" },
  { value: "bulk_operation_failed", label: "Bulk Operation Failed" },
  { value: "login_failed",          label: "Login Failed" },
  { value: "permission_denied",     label: "Permission Denied" },
];

const SEVERITY_CONFIG: Record<PlatformEventSeverity, {
  label: string; bg: string; text: string; border: string; icon: React.ElementType;
}> = {
  info:     { label: "Info",     bg: "bg-blue-900/30",   text: "text-blue-400",   border: "border-blue-500/30",   icon: Info          },
  warning:  { label: "Warning",  bg: "bg-yellow-900/30", text: "text-yellow-400", border: "border-yellow-500/30", icon: AlertTriangle  },
  critical: { label: "Critical", bg: "bg-red-900/30",    text: "text-red-400",    border: "border-red-500/30",    icon: Zap            },
};

const EVENT_TYPE_ICONS: Partial<Record<PlatformEventType, React.ElementType>> = {
  provider_connected:    CheckCircle2,
  provider_disconnected: Server,
  sync_completed:        RefreshCw,
  sync_failed:           RefreshCw,
  vm_poweron_success:    Zap,
  vm_poweron_failed:     Zap,
  snapshot_created:      Activity,
  snapshot_failed:       Activity,
  task_failed:           AlertTriangle,
  bulk_operation_failed: AlertTriangle,
  login_failed:          AlertTriangle,
  permission_denied:     AlertTriangle,
};

const PAGE_SIZE = 30;

// ── Severity Badge ────────────────────────────────────────────────────────────

function SeverityBadge({ severity }: { severity: PlatformEventSeverity }) {
  const cfg = SEVERITY_CONFIG[severity] ?? SEVERITY_CONFIG.info;
  const Icon = cfg.icon;
  return (
    <span className={cn(
      "inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-semibold border",
      cfg.bg, cfg.text, cfg.border
    )}>
      <Icon className="w-3 h-3" />
      {cfg.label}
    </span>
  );
}

// ── Event Row ─────────────────────────────────────────────────────────────────

function EventRow({ event }: { event: PlatformEvent }) {
  const Icon = EVENT_TYPE_ICONS[event.event_type] ?? Activity;
  const sevCfg = SEVERITY_CONFIG[event.severity] ?? SEVERITY_CONFIG.info;

  return (
    <tr className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
        {formatDate(event.created_at)}
      </td>
      <td className="px-4 py-3">
        <SeverityBadge severity={event.severity} />
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className={cn("w-6 h-6 rounded flex items-center justify-center shrink-0", sevCfg.bg)}>
            <Icon className={cn("w-3.5 h-3.5", sevCfg.text)} />
          </div>
          <span className="text-xs text-gray-300 font-mono">{event.event_type}</span>
        </div>
      </td>
      <td className="px-4 py-3 text-gray-300 max-w-sm">
        <span className="truncate block text-sm" title={event.message}>
          {event.message}
        </span>
      </td>
      <td className="px-4 py-3">
        {event.provider ? (
          <span className="text-xs bg-gray-800 text-gray-400 px-1.5 py-0.5 rounded font-mono">
            {event.provider}
          </span>
        ) : "—"}
      </td>
      <td className="px-4 py-3">
        {event.resource_type ? (
          <span className="text-xs text-gray-500">{event.resource_type}</span>
        ) : "—"}
      </td>
    </tr>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export default function EventsPage() {
  const queryClient = useQueryClient();
  const [page, setPage]               = useState(1);
  const [search, setSearch]           = useState("");
  const [eventType, setEventType]     = useState<PlatformEventType | "">("");
  const [severity, setSeverity]       = useState<PlatformEventSeverity | "">("");
  const [provider, setProvider]       = useState("");
  const [since, setSince]             = useState("");
  const [until, setUntil]             = useState("");
  const [showFilters, setShowFilters] = useState(false);

  const params: EventListParams = {
    page,
    page_size: PAGE_SIZE,
    ...(search    && { search }),
    ...(eventType && { event_type: eventType }),
    ...(severity  && { severity }),
    ...(provider  && { provider }),
    ...(since     && { since: new Date(since).toISOString() }),
    ...(until     && { until: new Date(until).toISOString() }),
  };

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["platform-events", params],
    queryFn: () => eventApi.list(params),
  });

  const events: PlatformEvent[] = data?.data ?? [];
  const totalItems = data?.meta?.total_items ?? 0;
  const totalPages = data?.meta?.total_pages ?? 1;

  // Real-time: subscribe to the "events" WebSocket room
  useEffect(() => {
    const unsub = wsClient.subscribe("events", () => {
      queryClient.invalidateQueries({ queryKey: ["platform-events"] });
    });
    return unsub;
  }, [queryClient]);

  const resetFilters = useCallback(() => {
    setSearch(""); setEventType(""); setSeverity("");
    setProvider(""); setSince(""); setUntil("");
    setPage(1);
  }, []);

  const hasActiveFilters = !!(search || eventType || severity || provider || since || until);

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Activity className="w-6 h-6 text-blue-400" />
            Event Activity
          </h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {totalItems.toLocaleString()} events
            {hasActiveFilters && " (filtered)"}
          </p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="p-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white transition-colors disabled:opacity-50"
          title="Refresh"
        >
          <RefreshCw className={cn("w-4 h-4", isFetching && "animate-spin")} />
        </button>
      </div>

      {/* Severity summary tiles */}
      <div className="grid grid-cols-3 gap-3">
        {(["critical", "warning", "info"] as PlatformEventSeverity[]).map((sev) => {
          const cfg = SEVERITY_CONFIG[sev];
          const Icon = cfg.icon;
          return (
            <button
              key={sev}
              onClick={() => { setSeverity(severity === sev ? "" : sev); setPage(1); }}
              className={cn(
                "flex items-center gap-3 p-4 rounded-xl border transition-all text-left",
                severity === sev
                  ? cn(cfg.bg, cfg.border, "ring-1 ring-inset", cfg.text.replace("text-", "ring-"))
                  : "bg-gray-900 border-gray-800 hover:border-gray-700"
              )}
            >
              <div className={cn("w-8 h-8 rounded-lg flex items-center justify-center", cfg.bg)}>
                <Icon className={cn("w-4 h-4", cfg.text)} />
              </div>
              <div>
                <p className={cn("text-sm font-semibold", cfg.text)}>{cfg.label}</p>
                <p className="text-xs text-gray-500">Click to filter</p>
              </div>
            </button>
          );
        })}
      </div>

      {/* Search + filter bar */}
      <div className="flex flex-wrap gap-3">
        <div className="relative flex-1 min-w-[220px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search event messages…"
            className="w-full pl-9 pr-4 py-2 bg-gray-900 border border-gray-800 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
          />
        </div>
        <button
          onClick={() => setShowFilters((v) => !v)}
          className={cn(
            "flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors",
            showFilters || hasActiveFilters
              ? "bg-blue-600/20 border border-blue-500/40 text-blue-400"
              : "bg-gray-900 border border-gray-800 text-gray-400 hover:text-white"
          )}
        >
          <Filter className="w-4 h-4" />
          Filters
          {hasActiveFilters && (
            <span className="w-4 h-4 rounded-full bg-blue-500 text-white text-[10px] flex items-center justify-center font-bold">!</span>
          )}
        </button>
        {hasActiveFilters && (
          <button
            onClick={resetFilters}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm bg-gray-900 border border-gray-800 text-gray-400 hover:text-red-400 transition-colors"
          >
            <X className="w-3.5 h-3.5" /> Clear
          </button>
        )}
      </div>

      {/* Expanded filters */}
      {showFilters && (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 p-4 bg-gray-900 border border-gray-800 rounded-xl">
          <div>
            <label className="block text-xs text-gray-500 mb-1">Event Type</label>
            <select
              value={eventType}
              onChange={(e) => { setEventType(e.target.value as PlatformEventType | ""); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All types</option>
              {EVENT_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Severity</label>
            <select
              value={severity}
              onChange={(e) => { setSeverity(e.target.value as PlatformEventSeverity | ""); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All</option>
              <option value="critical">Critical</option>
              <option value="warning">Warning</option>
              <option value="info">Info</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Provider</label>
            <select
              value={provider}
              onChange={(e) => { setProvider(e.target.value); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All providers</option>
              <option value="vmware">VMware</option>
              <option value="proxmox">Proxmox</option>
              <option value="esxi">ESXi</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">From</label>
            <input
              type="datetime-local"
              value={since}
              onChange={(e) => { setSince(e.target.value); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">To</label>
            <input
              type="datetime-local"
              value={until}
              onChange={(e) => { setUntil(e.target.value); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>
      )}

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800">
                {["Time", "Severity", "Event Type", "Message", "Provider", "Resource"].map((h) => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide whitespace-nowrap">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 6 }).map((_, j) => (
                      <td key={j} className="px-4 py-3">
                        <div className="h-4 bg-gray-800 rounded animate-pulse w-20" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : events.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-16 text-center">
                    <Activity className="w-10 h-10 mx-auto mb-3 text-gray-700" />
                    <p className="text-gray-500">
                      {hasActiveFilters ? "No events match your filters." : "No platform events yet."}
                    </p>
                    {!hasActiveFilters && (
                      <p className="text-gray-600 text-xs mt-1">
                        Events are generated automatically as the platform operates.
                      </p>
                    )}
                  </td>
                </tr>
              ) : (
                events.map((event) => <EventRow key={event.id} event={event} />)
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">
              Page {page} of {totalPages} · {totalItems.toLocaleString()} total
            </span>
            <div className="flex gap-2">
              <button
                disabled={page === 1}
                onClick={() => setPage((p) => p - 1)}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={page === totalPages}
                onClick={() => setPage((p) => p + 1)}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
