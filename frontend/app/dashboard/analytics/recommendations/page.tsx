"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Info, X, Check, Server, Cpu, HardDrive, Clock } from "lucide-react";
import { analyticsApi } from "@/lib/api/analytics";
import type { OptimizationRecommendation, RecommendationType, RecommendationSeverity } from "@/types";

const SEVERITY_CONFIG: Record<RecommendationSeverity, { label: string; color: string; bg: string; border: string; icon: React.ElementType }> = {
  critical: { label: "Critical", color: "text-red-400", bg: "bg-red-950/30", border: "border-red-800/40", icon: AlertTriangle },
  warning:  { label: "Warning",  color: "text-yellow-400", bg: "bg-yellow-950/30", border: "border-yellow-800/40", icon: AlertTriangle },
  info:     { label: "Info",     color: "text-blue-400", bg: "bg-blue-950/30", border: "border-blue-800/40", icon: Info },
};

const TYPE_LABELS: Record<RecommendationType, string> = {
  oversized_vm:        "Oversized VM",
  idle_vm:             "Idle VM",
  stale_snapshot:      "Stale Snapshots",
  underutilized_host:  "Underutilised Host",
  overcommitted_host:  "Overcommitted Host",
  powered_off_stale_vm:"Powered-Off Stale VM",
  orphaned_resource:   "Orphaned Resource",
  snapshot_growth:     "Snapshot Growth",
  storage_exhaustion:  "Storage Exhaustion",
};

const TYPE_ICONS: Record<RecommendationType, React.ElementType> = {
  oversized_vm: Cpu, idle_vm: Clock, stale_snapshot: HardDrive,
  underutilized_host: Server, overcommitted_host: Server,
  powered_off_stale_vm: Clock, orphaned_resource: HardDrive,
  snapshot_growth: HardDrive, storage_exhaustion: HardDrive,
};

function RecommendationCard({ rec, onDismiss, onResolve }: {
  rec: OptimizationRecommendation;
  onDismiss: (id: string) => void;
  onResolve: (id: string) => void;
}) {
  const cfg = SEVERITY_CONFIG[rec.severity];
  const Icon = TYPE_ICONS[rec.type] ?? Info;
  const [showDismiss, setShowDismiss] = useState(false);
  const [note, setNote] = useState("");

  return (
    <div className={`border rounded-2xl p-5 space-y-3 ${cfg.bg} ${cfg.border}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3 min-w-0">
          <div className={`p-2 rounded-xl bg-gray-900/60 shrink-0 ${cfg.color}`}>
            <Icon className="w-4 h-4" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${cfg.bg} ${cfg.color} border ${cfg.border}`}>
                {cfg.label}
              </span>
              <span className="text-xs text-gray-500">{TYPE_LABELS[rec.type] ?? rec.type}</span>
              <span className="text-xs text-gray-600">Score: {rec.score}</span>
            </div>
            <h3 className="font-semibold text-white mt-1 text-sm">{rec.title}</h3>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            onClick={() => onResolve(rec.id)}
            title="Mark resolved"
            className="p-1.5 rounded-lg hover:bg-green-900/40 text-gray-500 hover:text-green-400 transition-colors"
          >
            <Check className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => setShowDismiss(!showDismiss)}
            title="Dismiss"
            className="p-1.5 rounded-lg hover:bg-gray-700 text-gray-500 hover:text-gray-300 transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <p className="text-sm text-gray-400">{rec.description}</p>

      <div className="p-3 bg-gray-900/60 rounded-xl border border-gray-700/40">
        <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">Suggested Action</p>
        <p className="text-sm text-gray-300">{rec.action}</p>
      </div>

      {/* Savings */}
      {(rec.estimated_savings_gb > 0 || rec.estimated_savings_cpu > 0 || rec.estimated_savings_mb > 0) && (
        <div className="flex flex-wrap gap-3 text-xs text-gray-500">
          {rec.estimated_savings_gb > 0 && (
            <span className="flex items-center gap-1"><HardDrive className="w-3 h-3" /> ~{rec.estimated_savings_gb.toFixed(1)} GB recoverable</span>
          )}
          {rec.estimated_savings_cpu > 0 && (
            <span className="flex items-center gap-1"><Cpu className="w-3 h-3" /> ~{rec.estimated_savings_cpu} vCPU recoverable</span>
          )}
          {rec.estimated_savings_mb > 0 && (
            <span className="flex items-center gap-1"><Server className="w-3 h-3" /> ~{(rec.estimated_savings_mb / 1024).toFixed(1)} GB RAM recoverable</span>
          )}
        </div>
      )}

      {/* Resource links */}
      {(rec.hypervisor || rec.vm) && (
        <div className="text-xs text-gray-600">
          {rec.hypervisor && <span>Hypervisor: <span className="text-gray-400">{rec.hypervisor.name}</span></span>}
          {rec.vm && <span className="ml-3">VM: <span className="text-gray-400">{rec.vm.name}</span></span>}
        </div>
      )}

      {/* Dismiss form */}
      {showDismiss && (
        <div className="pt-2 border-t border-gray-700/40 space-y-2">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Optional note (reason for dismissal)…"
            className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-600 focus:outline-none focus:border-blue-500"
          />
          <div className="flex gap-2">
            <button
              onClick={() => { onDismiss(rec.id); setShowDismiss(false); }}
              className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg text-xs transition-colors"
            >
              Confirm Dismiss
            </button>
            <button onClick={() => setShowDismiss(false)} className="px-3 py-1.5 text-gray-500 hover:text-gray-300 text-xs transition-colors">
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function RecommendationsPage() {
  const qc = useQueryClient();
  const [severityFilter, setSeverityFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("active");
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ["analytics-recommendations", severityFilter, typeFilter, statusFilter, page],
    queryFn: () => analyticsApi.getRecommendations({
      severity: severityFilter || undefined,
      type: typeFilter || undefined,
      status: statusFilter || undefined,
      page, page_size: 20,
    }),
  });

  const { data: summary } = useQuery({
    queryKey: ["analytics-rec-summary"],
    queryFn: analyticsApi.getRecommendationSummary,
  });

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["analytics-recommendations"] });
    qc.invalidateQueries({ queryKey: ["analytics-rec-summary"] });
  };

  const dismissMut = useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) => analyticsApi.dismissRecommendation(id, note),
    onSuccess: invalidate,
  });

  const resolveMut = useMutation({
    mutationFn: (id: string) => analyticsApi.resolveRecommendation(id),
    onSuccess: invalidate,
  });

  const recs: OptimizationRecommendation[] = data?.data ?? [];
  const totalPages = data?.meta?.total_pages ?? 1;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Optimization Recommendations</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.meta?.total_items ?? 0} recommendations</p>
        </div>
      </div>

      {/* Summary badges */}
      {summary && (
        <div className="flex flex-wrap gap-3">
          {[
            { label: "Active", count: summary.total_active, color: "text-white bg-gray-800 border-gray-700" },
            { label: "Critical", count: summary.by_severity?.critical ?? 0, color: "text-red-400 bg-red-950/30 border-red-800/40" },
            { label: "Warning", count: summary.by_severity?.warning ?? 0, color: "text-yellow-400 bg-yellow-950/30 border-yellow-800/40" },
            { label: "Dismissed", count: summary.total_dismissed, color: "text-gray-400 bg-gray-800 border-gray-700" },
            { label: "Resolved", count: summary.total_resolved, color: "text-green-400 bg-green-950/30 border-green-800/40" },
          ].map(({ label, count, color }) => (
            <div key={label} className={`px-3 py-1.5 rounded-xl border text-sm font-medium ${color}`}>
              {label}: {count}
            </div>
          ))}
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="active">Active</option>
          <option value="dismissed">Dismissed</option>
          <option value="resolved">Resolved</option>
          <option value="">All statuses</option>
        </select>
        <select value={severityFilter} onChange={(e) => { setSeverityFilter(e.target.value); setPage(1); }}
          className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="">All severities</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
        <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); }}
          className="bg-gray-900 border border-gray-800 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="">All types</option>
          {Object.entries(TYPE_LABELS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
        </select>
      </div>

      {/* List */}
      {isLoading ? (
        <div className="space-y-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 h-36 animate-pulse" />
          ))}
        </div>
      ) : recs.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-12 text-center">
          <CheckCircle2 className="w-10 h-10 text-green-400 mx-auto mb-3 opacity-60" />
          <p className="text-gray-400">No recommendations found for the selected filters.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {recs.map((rec) => (
            <RecommendationCard
              key={rec.id}
              rec={rec}
              onDismiss={(id) => dismissMut.mutate({ id })}
              onResolve={(id) => resolveMut.mutate(id)}
            />
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-xs text-gray-500">Page {page} of {totalPages}</span>
          <div className="flex gap-2">
            <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Previous</button>
            <button disabled={page === totalPages} onClick={() => setPage((p) => p + 1)}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">Next</button>
          </div>
        </div>
      )}
    </div>
  );
}
