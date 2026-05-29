"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Activity, RefreshCw, Wifi, WifiOff, AlertTriangle,
  CheckCircle2, Clock, Zap, Server, BarChart2, Loader2,
  ChevronDown, ChevronUp,
} from "lucide-react";
import { healthApi } from "@/lib/api/health";
import { cn, relativeTime } from "@/lib/utils";
import type { ProviderHealth, ProviderHealthHistory, HealthStatus } from "@/types";

// ── Status helpers ────────────────────────────────────────────────────────────

const STATUS_CONFIG: Record<HealthStatus, { label: string; dot: string; badge: string; icon: React.ElementType }> = {
  healthy:   { label: "Healthy",   dot: "bg-green-400",  badge: "bg-green-900/30 text-green-400 border-green-500/30",   icon: CheckCircle2   },
  degraded:  { label: "Degraded",  dot: "bg-yellow-400", badge: "bg-yellow-900/30 text-yellow-400 border-yellow-500/30", icon: AlertTriangle  },
  unhealthy: { label: "Unhealthy", dot: "bg-red-400",    badge: "bg-red-900/30 text-red-400 border-red-500/30",         icon: AlertTriangle  },
  unknown:   { label: "Unknown",   dot: "bg-gray-500",   badge: "bg-gray-800 text-gray-400 border-gray-700",            icon: Clock          },
};

function StatusBadge({ status }: { status: HealthStatus }) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.unknown;
  const Icon = cfg.icon;
  return (
    <span className={cn("inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border", cfg.badge)}>
      <Icon className="w-3 h-3" />
      {cfg.label}
    </span>
  );
}

function ScoreRing({ score }: { score: number }) {
  const color = score >= 80 ? "text-green-400" : score >= 50 ? "text-yellow-400" : "text-red-400";
  return (
    <div className={cn("text-3xl font-bold tabular-nums", color)}>
      {score}
      <span className="text-sm font-normal text-gray-500">/100</span>
    </div>
  );
}

// ── Mini latency sparkline ────────────────────────────────────────────────────

function Sparkline({ data }: { data: ProviderHealthHistory[] }) {
  if (!data || data.length < 2) {
    return <div className="h-10 flex items-center text-xs text-gray-600">No history</div>;
  }

  const sorted = [...data].reverse(); // oldest first
  const values = sorted.map((d) => d.latency_ms);
  const max = Math.max(...values, 1);
  const W = 120;
  const H = 32;
  const step = W / (values.length - 1);

  const points = values
    .map((v, i) => `${i * step},${H - (v / max) * H}`)
    .join(" ");

  return (
    <svg width={W} height={H} className="overflow-visible">
      <polyline
        points={points}
        fill="none"
        stroke="#3b82f6"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}

// ── Provider Health Card ──────────────────────────────────────────────────────

function HealthCard({ snap }: { snap: ProviderHealth }) {
  const [expanded, setExpanded] = useState(false);
  const queryClient = useQueryClient();

  const { data: history } = useQuery({
    queryKey: ["health-history", snap.hypervisor_id],
    queryFn: () => healthApi.getHistory(snap.hypervisor_id, 30),
    enabled: expanded,
    staleTime: 30_000,
  });

  const checkMut = useMutation({
    mutationFn: () => healthApi.triggerCheck(snap.hypervisor_id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-health"] });
      queryClient.invalidateQueries({ queryKey: ["health-history", snap.hypervisor_id] });
    },
  });

  const hv = snap.hypervisor;
  const statusCfg = STATUS_CONFIG[snap.status] ?? STATUS_CONFIG.unknown;

  return (
    <div className={cn(
      "bg-gray-900 border rounded-2xl overflow-hidden transition-all",
      snap.status === "unhealthy" ? "border-red-800/50" :
      snap.status === "degraded"  ? "border-yellow-800/50" :
                                    "border-gray-800"
    )}>
      {/* Card header */}
      <div className="p-5">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            {/* Online indicator */}
            <div className="relative shrink-0">
              <div className={cn(
                "w-10 h-10 rounded-xl flex items-center justify-center",
                snap.online ? "bg-green-900/30" : "bg-red-900/30"
              )}>
                {snap.online
                  ? <Wifi className="w-5 h-5 text-green-400" />
                  : <WifiOff className="w-5 h-5 text-red-400" />
                }
              </div>
              <span className={cn(
                "absolute -top-0.5 -right-0.5 w-3 h-3 rounded-full border-2 border-gray-900",
                statusCfg.dot
              )} />
            </div>
            <div className="min-w-0">
              <div className="font-semibold text-white truncate">
                {hv?.name ?? snap.hypervisor_id.slice(0, 8)}
              </div>
              <div className="text-xs text-gray-500 mt-0.5 font-mono truncate">
                {hv?.host}
                {hv?.provider && (
                  <span className="ml-2 text-gray-600 font-sans">{hv.provider}</span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <StatusBadge status={snap.status} />
            <button
              onClick={() => checkMut.mutate()}
              disabled={checkMut.isPending}
              title="Run health check now"
              className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white transition-colors disabled:opacity-50"
            >
              {checkMut.isPending
                ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
                : <RefreshCw className="w-3.5 h-3.5" />
              }
            </button>
          </div>
        </div>

        {/* Score + key metrics */}
        <div className="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-3">
          <MetricTile label="Health Score">
            <ScoreRing score={snap.health_score} />
          </MetricTile>
          <MetricTile label="Avg Latency">
            <span className={cn(
              "text-2xl font-bold tabular-nums",
              snap.avg_latency_ms > 1000 ? "text-red-400" :
              snap.avg_latency_ms > 500  ? "text-yellow-400" : "text-white"
            )}>
              {snap.avg_latency_ms > 0 ? `${Math.round(snap.avg_latency_ms)}` : "—"}
              <span className="text-sm font-normal text-gray-500"> ms</span>
            </span>
          </MetricTile>
          <MetricTile label="VMs">
            <span className="text-2xl font-bold text-white tabular-nums">{snap.vm_count}</span>
          </MetricTile>
          <MetricTile label="Task Failures (24h)">
            <span className={cn(
              "text-2xl font-bold tabular-nums",
              snap.tasks_failed_24h > 0 ? "text-red-400" : "text-white"
            )}>
              {snap.tasks_failed_24h}
              <span className="text-sm font-normal text-gray-500">/{snap.tasks_total_24h}</span>
            </span>
          </MetricTile>
        </div>

        {/* Alert banners */}
        <div className="mt-3 space-y-1.5">
          {snap.inventory_stale && (
            <AlertBanner icon={Clock} color="yellow">
              Inventory stale
              {snap.inventory_age_minutes > 0
                ? ` — last synced ${snap.inventory_age_minutes}m ago`
                : " — never synced"}
            </AlertBanner>
          )}
          {snap.consecutive_fails >= 3 && (
            <AlertBanner icon={WifiOff} color="red">
              {snap.consecutive_fails} consecutive connectivity failures
            </AlertBanner>
          )}
          {snap.auth_failures_24h > 3 && (
            <AlertBanner icon={AlertTriangle} color="orange">
              {snap.auth_failures_24h} auth failures in the last 24 h
            </AlertBanner>
          )}
          {snap.sync_failures_24h > 0 && (
            <AlertBanner icon={RefreshCw} color="yellow">
              {snap.sync_failures_24h} sync failure{snap.sync_failures_24h > 1 ? "s" : ""} in the last 24 h
            </AlertBanner>
          )}
        </div>
      </div>

      {/* Expand toggle */}
      <button
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center justify-between px-5 py-2.5 border-t border-gray-800 text-xs text-gray-500 hover:text-gray-300 hover:bg-gray-800/40 transition-colors"
      >
        <span className="flex items-center gap-1.5">
          <BarChart2 className="w-3.5 h-3.5" />
          {expanded ? "Hide details" : "Show latency history & details"}
        </span>
        {expanded ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
      </button>

      {/* Expanded details */}
      {expanded && (
        <div className="px-5 pb-5 pt-3 border-t border-gray-800 space-y-4">
          {/* Latency sparkline */}
          <div>
            <p className="text-xs text-gray-500 mb-2 font-medium uppercase tracking-wide">
              Latency (last 30 checks)
            </p>
            <div className="flex items-end gap-4">
              <Sparkline data={history ?? []} />
              <div className="text-xs text-gray-500 space-y-0.5">
                <div>Peak: <span className="text-white">{Math.round(snap.peak_latency_ms)} ms</span></div>
                <div>Avg: <span className="text-white">{Math.round(snap.avg_latency_ms)} ms</span></div>
                <div>Last: <span className="text-white">{Math.round(snap.latency_ms)} ms</span></div>
              </div>
            </div>
          </div>

          {/* Detail grid */}
          <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-xs">
            <DetailRow label="Last check">
              {snap.last_check_at ? relativeTime(snap.last_check_at) : "—"}
            </DetailRow>
            <DetailRow label="Last seen online">
              {snap.last_seen_at ? relativeTime(snap.last_seen_at) : "Never"}
            </DetailRow>
            <DetailRow label="Last sync">
              {snap.last_sync_at ? relativeTime(snap.last_sync_at) : "Never"}
            </DetailRow>
            <DetailRow label="Last sync status">
              <span className={cn(
                snap.last_sync_status === "success" ? "text-green-400" :
                snap.last_sync_status === "failed"  ? "text-red-400"   : "text-gray-400"
              )}>
                {snap.last_sync_status || "—"}
              </span>
            </DetailRow>
            <DetailRow label="Task failure rate">
              {(snap.task_failure_rate * 100).toFixed(1)}%
            </DetailRow>
            <DetailRow label="Auth failures (24h)">
              <span className={snap.auth_failures_24h > 0 ? "text-red-400" : "text-gray-300"}>
                {snap.auth_failures_24h}
              </span>
            </DetailRow>
            <DetailRow label="Inventory age">
              {snap.inventory_age_minutes >= 0 ? `${snap.inventory_age_minutes}m` : "Unknown"}
            </DetailRow>
            <DetailRow label="Consecutive fails">
              <span className={snap.consecutive_fails > 0 ? "text-red-400" : "text-gray-300"}>
                {snap.consecutive_fails}
              </span>
            </DetailRow>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Small helpers ─────────────────────────────────────────────────────────────

function MetricTile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="bg-gray-800/50 rounded-xl p-3">
      <p className="text-[11px] text-gray-500 uppercase tracking-wide mb-1">{label}</p>
      {children}
    </div>
  );
}

function AlertBanner({
  icon: Icon, color, children,
}: {
  icon: React.ElementType;
  color: "red" | "yellow" | "orange";
  children: React.ReactNode;
}) {
  const styles = {
    red:    "bg-red-900/20 border-red-800/40 text-red-400",
    yellow: "bg-yellow-900/20 border-yellow-800/40 text-yellow-400",
    orange: "bg-orange-900/20 border-orange-800/40 text-orange-400",
  };
  return (
    <div className={cn("flex items-center gap-2 px-3 py-1.5 rounded-lg border text-xs", styles[color])}>
      <Icon className="w-3.5 h-3.5 shrink-0" />
      {children}
    </div>
  );
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-1 border-b border-gray-800/50">
      <span className="text-gray-500">{label}</span>
      <span className="text-gray-300">{children}</span>
    </div>
  );
}

// ── Summary bar ───────────────────────────────────────────────────────────────

function SummaryBar({ items }: { items: ProviderHealth[] }) {
  const healthy   = items.filter((i) => i.status === "healthy").length;
  const degraded  = items.filter((i) => i.status === "degraded").length;
  const unhealthy = items.filter((i) => i.status === "unhealthy").length;
  const unknown   = items.filter((i) => i.status === "unknown").length;
  const online    = items.filter((i) => i.online).length;

  return (
    <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
      {[
        { label: "Total",     value: items.length, color: "text-white",        bg: "bg-gray-800/60" },
        { label: "Online",    value: online,        color: "text-green-400",    bg: "bg-green-900/20" },
        { label: "Healthy",   value: healthy,       color: "text-green-400",    bg: "bg-green-900/20" },
        { label: "Degraded",  value: degraded,      color: "text-yellow-400",   bg: "bg-yellow-900/20" },
        { label: "Unhealthy", value: unhealthy,     color: "text-red-400",      bg: "bg-red-900/20" },
      ].map(({ label, value, color, bg }) => (
        <div key={label} className={cn("rounded-xl p-4 border border-gray-800", bg)}>
          <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
          <p className={cn("text-3xl font-bold mt-1 tabular-nums", color)}>{value}</p>
        </div>
      ))}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function HealthPage() {
  const [statusFilter, setStatusFilter] = useState<HealthStatus | "all">("all");
  const queryClient = useQueryClient();

  const { data, isLoading, isFetching, refetch, error } = useQuery({
    queryKey: ["provider-health"],
    queryFn: () => healthApi.listAll(),
    refetchInterval: 60_000, // auto-refresh every 60 s
  });

  const items: ProviderHealth[] = data?.data ?? [];

  const filtered = statusFilter === "all"
    ? items
    : items.filter((i) => i.status === statusFilter);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Activity className="w-6 h-6 text-blue-400" />
            Provider Health
          </h1>
          <p className="text-gray-400 text-sm mt-0.5">
            Real-time connectivity and operational health for all hypervisors
          </p>
        </div>
        <button
          onClick={() => {
            refetch();
            queryClient.invalidateQueries({ queryKey: ["provider-health"] });
          }}
          disabled={isFetching}
          className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors disabled:opacity-50"
        >
          <RefreshCw className={cn("w-4 h-4", isFetching && "animate-spin")} />
          Refresh
        </button>
      </div>

      {/* Summary */}
      {!isLoading && items.length > 0 && <SummaryBar items={items} />}

      {/* Status filter tabs */}
      {items.length > 0 && (
        <div className="flex gap-2 flex-wrap">
          {(["all", "healthy", "degraded", "unhealthy", "unknown"] as const).map((s) => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={cn(
                "px-3 py-1.5 rounded-lg text-sm font-medium transition-colors",
                statusFilter === s
                  ? "bg-blue-600 text-white"
                  : "bg-gray-800 text-gray-400 hover:text-white hover:bg-gray-700"
              )}
            >
              {s === "all" ? "All" : s.charAt(0).toUpperCase() + s.slice(1)}
              <span className="ml-1.5 text-xs opacity-70">
                {s === "all" ? items.length : items.filter((i) => i.status === s).length}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* Cards */}
      {isLoading ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 animate-pulse">
              <div className="flex gap-3 mb-4">
                <div className="w-10 h-10 bg-gray-800 rounded-xl" />
                <div className="flex-1 space-y-2">
                  <div className="h-4 bg-gray-800 rounded w-32" />
                  <div className="h-3 bg-gray-800 rounded w-24" />
                </div>
              </div>
              <div className="grid grid-cols-4 gap-3">
                {Array.from({ length: 4 }).map((_, j) => (
                  <div key={j} className="h-16 bg-gray-800 rounded-xl" />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <Server className="w-12 h-12 text-gray-700 mb-4" />
          <p className="text-gray-400 font-medium">
            {items.length === 0
              ? "No hypervisors registered yet."
              : "No providers match the selected filter."}
          </p>
          {items.length === 0 && (
            <p className="text-gray-600 text-sm mt-1">
              Register a hypervisor to start monitoring its health.
            </p>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {filtered.map((snap) => (
            <HealthCard key={snap.hypervisor_id} snap={snap} />
          ))}
        </div>
      )}
    </div>
  );
}
