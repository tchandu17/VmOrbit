"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertOctagon, ShieldOff, Clock } from "lucide-react";
import { policyApi } from "@/lib/api/policy";
import { cn } from "@/lib/utils";
import type { PolicyViolation, PolicyViolationStatus, PolicyEffect } from "@/types";

const STATUS_STYLES: Record<PolicyViolationStatus, string> = {
  blocked:          "bg-red-900/40 text-red-300 border-red-700",
  overridden:       "bg-green-900/40 text-green-300 border-green-700",
  pending_approval: "bg-yellow-900/40 text-yellow-300 border-yellow-700",
};

const EFFECT_COLORS: Record<PolicyEffect, string> = {
  allow:                  "text-green-400",
  deny:                   "text-red-400",
  require_approval:       "text-yellow-400",
  require_snapshot:       "text-blue-400",
  require_justification:  "text-purple-400",
};

export default function ViolationsPage() {
  const [statusFilter, setStatusFilter] = useState("");
  const [operationFilter, setOperationFilter] = useState("");
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ["policy-violations", { statusFilter, operationFilter, page }],
    queryFn: () => policyApi.listViolations({
      status: statusFilter || undefined,
      operation: operationFilter || undefined,
      page,
      page_size: 25,
    }),
  });

  const violations: PolicyViolation[] = data?.data ?? [];
  const meta = data?.meta;

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <AlertOctagon className="w-6 h-6 text-red-400" />Policy Violations
        </h1>
        <p className="text-gray-400 text-sm mt-0.5">Audit trail of all policy enforcement events</p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        {[
          { label: "Blocked", status: "blocked", icon: ShieldOff, color: "text-red-400", bg: "bg-red-900/20 border-red-800/50" },
          { label: "Pending Approval", status: "pending_approval", icon: Clock, color: "text-yellow-400", bg: "bg-yellow-900/20 border-yellow-800/50" },
          { label: "Overridden", status: "overridden", icon: AlertOctagon, color: "text-green-400", bg: "bg-green-900/20 border-green-800/50" },
        ].map(({ label, status, icon: Icon, color, bg }) => (
          <button key={status} onClick={() => setStatusFilter(s => s === status ? "" : status)}
            className={cn("border rounded-xl p-4 text-left transition-colors hover:opacity-90", bg, statusFilter === status ? "ring-2 ring-blue-500" : "")}>
            <div className="flex items-center gap-2 mb-1">
              <Icon className={cn("w-4 h-4", color)} />
              <span className="text-sm text-gray-300">{label}</span>
            </div>
          </button>
        ))}
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-blue-500">
          <option value="">All statuses</option>
          <option value="blocked">Blocked</option>
          <option value="pending_approval">Pending Approval</option>
          <option value="overridden">Overridden</option>
        </select>
        <input value={operationFilter} onChange={e => setOperationFilter(e.target.value)} placeholder="Filter by operation…"
          className="w-56 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500" />
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Time", "Policy", "Effect", "Operation", "Resource", "User", "Status"].map(h => (
                <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? Array.from({ length: 5 }).map((_, i) => (
              <tr key={i} className="border-b border-gray-800/50">
                {Array.from({ length: 7 }).map((_, j) => (
                  <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-20" /></td>
                ))}
              </tr>
            )) : violations.length === 0 ? (
              <tr><td colSpan={7} className="px-4 py-12 text-center text-gray-500">No violations found.</td></tr>
            ) : violations.map(v => (
              <tr key={v.id} className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
                <td className="px-4 py-3 text-gray-500 text-xs whitespace-nowrap">{new Date(v.created_at).toLocaleString()}</td>
                <td className="px-4 py-3 text-gray-300 text-sm">{v.policy_name}</td>
                <td className="px-4 py-3">
                  <span className={cn("text-xs font-medium", EFFECT_COLORS[v.effect])}>{v.effect.replace(/_/g, " ")}</span>
                </td>
                <td className="px-4 py-3 text-gray-300 text-xs font-mono">{v.operation}</td>
                <td className="px-4 py-3">
                  <div>
                    <p className="text-gray-300 text-xs">{v.resource_name || v.resource_id}</p>
                    <p className="text-gray-600 text-[10px]">{v.resource_type}</p>
                  </div>
                </td>
                <td className="px-4 py-3 text-gray-400 text-sm">{v.username || "—"}</td>
                <td className="px-4 py-3">
                  <span className={cn("text-xs px-2 py-0.5 rounded border", STATUS_STYLES[v.status])}>{v.status.replace(/_/g, " ")}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {meta && meta.total_pages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {meta.total_pages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage(p => p - 1)} className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">Previous</button>
              <button disabled={page === meta.total_pages} onClick={() => setPage(p => p + 1)} className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300">Next</button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
