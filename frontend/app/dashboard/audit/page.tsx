"use client";
import { useState, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Search, Filter, Download, RefreshCw, CheckCircle2,
  XCircle, ChevronLeft, ChevronRight, FileText, X,
} from "lucide-react";
import { auditApi, type AuditListParams } from "@/lib/api/audit";
import { cn, formatDate } from "@/lib/utils";
import type { AuditLog, AuditAction } from "@/types";

// ── Constants ─────────────────────────────────────────────────────────────────

const ACTIONS: AuditAction[] = [
  "create", "read", "update", "delete", "login", "logout", "execute",
];

const RESOURCES = [
  "hypervisor", "vm", "snapshot", "task", "user", "role", "permission",
  "tag", "console", "auth",
];

const ACTION_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  create:  { bg: "bg-green-900/30",  text: "text-green-400",  label: "CREATE"  },
  read:    { bg: "bg-gray-800/60",   text: "text-gray-400",   label: "READ"    },
  update:  { bg: "bg-blue-900/30",   text: "text-blue-400",   label: "UPDATE"  },
  delete:  { bg: "bg-red-900/30",    text: "text-red-400",    label: "DELETE"  },
  login:   { bg: "bg-purple-900/30", text: "text-purple-400", label: "LOGIN"   },
  logout:  { bg: "bg-orange-900/30", text: "text-orange-400", label: "LOGOUT"  },
  execute: { bg: "bg-yellow-900/30", text: "text-yellow-400", label: "EXECUTE" },
};

const PAGE_SIZE = 25;

// ── Main page ─────────────────────────────────────────────────────────────────

export default function AuditPage() {
  const [page, setPage]               = useState(1);
  const [search, setSearch]           = useState("");
  const [action, setAction]           = useState<AuditAction | "">("");
  const [resource, setResource]       = useState("");
  const [successFilter, setSuccess]   = useState<"" | "true" | "false">("");
  const [since, setSince]             = useState("");
  const [until, setUntil]             = useState("");
  const [showFilters, setShowFilters] = useState(false);

  const params: AuditListParams = {
    page,
    page_size: PAGE_SIZE,
    ...(search    && { search }),
    ...(action    && { action }),
    ...(resource  && { resource }),
    ...(since     && { since: new Date(since).toISOString() }),
    ...(until     && { until: new Date(until).toISOString() }),
    ...(successFilter !== "" && { success: successFilter === "true" }),
  };

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["audit", params],
    queryFn: () => auditApi.list(params),
  });

  const logs: AuditLog[] = data?.data ?? [];
  const totalItems  = data?.meta?.total_items ?? 0;
  const totalPages  = data?.meta?.total_pages ?? 1;

  const resetFilters = useCallback(() => {
    setSearch(""); setAction(""); setResource("");
    setSuccess(""); setSince(""); setUntil("");
    setPage(1);
  }, []);

  const hasActiveFilters = !!(search || action || resource || successFilter || since || until);

  const exportParams: Omit<AuditListParams, "page" | "page_size"> = {
    ...(search    && { search }),
    ...(action    && { action }),
    ...(resource  && { resource }),
    ...(since     && { since: new Date(since).toISOString() }),
    ...(until     && { until: new Date(until).toISOString() }),
    ...(successFilter !== "" && { success: successFilter === "true" }),
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Audit Log</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {totalItems.toLocaleString()} entries
            {hasActiveFilters && " (filtered)"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="p-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white transition-colors disabled:opacity-50"
            title="Refresh"
          >
            <RefreshCw className={cn("w-4 h-4", isFetching && "animate-spin")} />
          </button>
          <a
            href={auditApi.exportUrl(exportParams)}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
          >
            <Download className="w-4 h-4" /> Export CSV
          </a>
        </div>
      </div>

      {/* Search + filter bar */}
      <div className="flex flex-wrap gap-3">
        <div className="relative flex-1 min-w-[220px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search by user or description…"
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
            <span className="w-4 h-4 rounded-full bg-blue-500 text-white text-[10px] flex items-center justify-center font-bold">
              !
            </span>
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
            <label className="block text-xs text-gray-500 mb-1">Action</label>
            <select
              value={action}
              onChange={(e) => { setAction(e.target.value as AuditAction | ""); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All actions</option>
              {ACTIONS.map((a) => <option key={a} value={a}>{a}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Resource</label>
            <select
              value={resource}
              onChange={(e) => { setResource(e.target.value); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All resources</option>
              {RESOURCES.map((r) => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Status</label>
            <select
              value={successFilter}
              onChange={(e) => { setSuccess(e.target.value as "" | "true" | "false"); setPage(1); }}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-2 py-1.5 focus:outline-none focus:border-blue-500"
            >
              <option value="">All</option>
              <option value="true">Success only</option>
              <option value="false">Failures only</option>
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
                {["Time", "User", "Action", "Resource", "Description", "Status", "IP"].map((h) => (
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
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-4 py-3">
                        <div className="h-4 bg-gray-800 rounded animate-pulse w-20" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-16 text-center">
                    <FileText className="w-10 h-10 mx-auto mb-3 text-gray-700" />
                    <p className="text-gray-500">
                      {hasActiveFilters ? "No entries match your filters." : "No audit entries yet."}
                    </p>
                  </td>
                </tr>
              ) : (
                logs.map((log) => <AuditRow key={log.id} log={log} />)
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

// ── AuditRow ──────────────────────────────────────────────────────────────────

function AuditRow({ log }: { log: AuditLog }) {
  const style = ACTION_STYLES[log.action] ?? ACTION_STYLES.read;

  return (
    <tr className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
        {formatDate(log.created_at)}
      </td>
      <td className="px-4 py-3">
        <span className="text-gray-300 font-medium">{log.username || "system"}</span>
      </td>
      <td className="px-4 py-3">
        <span className={cn("inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold tracking-wide", style.bg, style.text)}>
          {style.label}
        </span>
      </td>
      <td className="px-4 py-3">
        <span className="text-xs text-gray-400 bg-gray-800 px-1.5 py-0.5 rounded">
          {log.resource}
        </span>
      </td>
      <td className="px-4 py-3 text-gray-300 max-w-xs">
        <span className="truncate block" title={log.description}>
          {log.description || "—"}
        </span>
        {log.error_message && (
          <span className="block text-xs text-red-400 truncate mt-0.5" title={log.error_message}>
            {log.error_message}
          </span>
        )}
      </td>
      <td className="px-4 py-3">
        {log.success ? (
          <span title="Success"><CheckCircle2 className="w-4 h-4 text-green-400" /></span>
        ) : (
          <span title="Failed"><XCircle className="w-4 h-4 text-red-400" /></span>
        )}
      </td>
      <td className="px-4 py-3 text-xs text-gray-500 font-mono">
        {log.ip_address || "—"}
      </td>
    </tr>
  );
}
