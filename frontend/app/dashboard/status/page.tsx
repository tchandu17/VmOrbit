"use client";
import { useQuery } from "@tanstack/react-query";
import {
  Activity, CheckCircle2, AlertTriangle, XCircle,
  Server, Database, Zap, Globe, RefreshCw, Clock,
  Cpu, MemoryStick, GitBranch,
} from "lucide-react";
import { cn } from "@/lib/utils";

// ── Types ─────────────────────────────────────────────────────────────────────

interface PlatformStatus {
  service: string;
  version: string;
  status: string;
  uptime: string;
  uptime_sec: number;
  started_at: string;
  time: string;
  dependencies: {
    database: { status: string; latency_ms: number; error?: string };
    redis:    { status: string; latency_ms: number; error?: string };
  };
  runtime: {
    goroutines: number;
    heap_alloc_mb: number;
    num_gc: number;
  };
}

interface ReadinessCheck {
  status: string;
  checks: Record<string, string>;
  time: string;
}

// ── API ───────────────────────────────────────────────────────────────────────

async function fetchStatus(): Promise<PlatformStatus> {
  const r = await fetch("/api/proxy/status");
  if (!r.ok) throw new Error("Failed to fetch status");
  return r.json();
}

async function fetchReadiness(): Promise<ReadinessCheck> {
  const r = await fetch("/api/proxy/ready");
  return r.json(); // always parse even on 503
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const ok = status === "ok" || status === "ready" || status === "running";
  const warn = status === "degraded";
  return (
    <span className={cn(
      "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold",
      ok   ? "bg-green-500/15 text-green-400 border border-green-500/30" :
      warn ? "bg-yellow-500/15 text-yellow-400 border border-yellow-500/30" :
             "bg-red-500/15 text-red-400 border border-red-500/30"
    )}>
      {ok ? <CheckCircle2 className="w-3 h-3" /> :
       warn ? <AlertTriangle className="w-3 h-3" /> :
              <XCircle className="w-3 h-3" />}
      {status}
    </span>
  );
}

function ServiceCard({
  icon: Icon,
  name,
  status,
  detail,
  latency,
  error,
}: {
  icon: React.ElementType;
  name: string;
  status: string;
  detail?: string;
  latency?: number;
  error?: string;
}) {
  const ok = status === "ok";
  return (
    <div className={cn(
      "bg-gray-900 border rounded-2xl p-5 transition-colors",
      ok ? "border-gray-800" : "border-red-800/50"
    )}>
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          <div className={cn(
            "w-10 h-10 rounded-xl flex items-center justify-center",
            ok ? "bg-green-600/20" : "bg-red-600/20"
          )}>
            <Icon className={cn("w-5 h-5", ok ? "text-green-400" : "text-red-400")} />
          </div>
          <div>
            <p className="font-semibold text-white text-sm">{name}</p>
            {detail && <p className="text-xs text-gray-500 mt-0.5">{detail}</p>}
          </div>
        </div>
        <StatusBadge status={status} />
      </div>
      {latency !== undefined && (
        <div className="flex items-center gap-1.5 text-xs text-gray-500 mt-2">
          <Clock className="w-3 h-3" />
          <span>{latency}ms latency</span>
        </div>
      )}
      {error && (
        <p className="text-xs text-red-400 mt-2 font-mono bg-red-900/20 rounded-lg px-3 py-2">
          {error}
        </p>
      )}
    </div>
  );
}

function MetricPill({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="bg-gray-800/60 rounded-xl px-4 py-3 text-center">
      <p className="text-xs text-gray-500 mb-1">{label}</p>
      <p className="text-sm font-bold text-white font-mono">{value}</p>
    </div>
  );
}

function formatUptime(secs: number) {
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m ${secs % 60}s`;
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function PlatformStatusPage() {
  const { data: status, isLoading: statusLoading, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ["platform-status"],
    queryFn: fetchStatus,
    refetchInterval: 30_000,
    staleTime: 20_000,
    retry: false,
  });

  const { data: readiness } = useQuery({
    queryKey: ["platform-readiness"],
    queryFn: fetchReadiness,
    refetchInterval: 15_000,
    staleTime: 10_000,
    retry: false,
  });

  const overallOk = status?.status === "running" && readiness?.status === "ready";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Globe className="w-6 h-6 text-blue-400" />
            Platform Status
          </h1>
          <p className="text-gray-400 text-sm mt-0.5">
            Live operational status — refreshes every 30 seconds
          </p>
        </div>
        <div className="flex items-center gap-3">
          {dataUpdatedAt > 0 && (
            <span className="text-xs text-gray-600">
              Updated {new Date(dataUpdatedAt).toLocaleTimeString()}
            </span>
          )}
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors disabled:opacity-50"
          >
            <RefreshCw className={cn("w-4 h-4", isFetching && "animate-spin")} />
            Refresh
          </button>
        </div>
      </div>

      {/* Overall status banner */}
      <div className={cn(
        "rounded-2xl border p-5 flex items-center gap-4",
        overallOk
          ? "bg-green-500/5 border-green-500/20"
          : "bg-yellow-500/5 border-yellow-500/20"
      )}>
        {overallOk
          ? <CheckCircle2 className="w-8 h-8 text-green-400 flex-shrink-0" />
          : <AlertTriangle className="w-8 h-8 text-yellow-400 flex-shrink-0" />}
        <div>
          <p className={cn("font-bold text-lg", overallOk ? "text-green-400" : "text-yellow-400")}>
            {overallOk ? "All Systems Operational" : "Service Degraded"}
          </p>
          <p className="text-sm text-gray-400 mt-0.5">
            {status
              ? `VMOrbit ${status.version} · Running since ${new Date(status.started_at).toLocaleString()}`
              : "Connecting to platform..."}
          </p>
        </div>
        {status && (
          <div className="ml-auto">
            <StatusBadge status={status.status} />
          </div>
        )}
      </div>

      {statusLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 animate-pulse h-28" />
          ))}
        </div>
      ) : status ? (
        <>
          {/* Service cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            <ServiceCard
              icon={Server}
              name="API Server"
              status={status.status === "running" ? "ok" : "error"}
              detail={`Port 8080 · ${status.version}`}
            />
            <ServiceCard
              icon={Database}
              name="PostgreSQL"
              status={status.dependencies.database.status}
              detail="Primary database"
              latency={status.dependencies.database.latency_ms}
              error={status.dependencies.database.error}
            />
            <ServiceCard
              icon={Zap}
              name="Redis"
              status={status.dependencies.redis.status}
              detail="Cache & task queue"
              latency={status.dependencies.redis.latency_ms}
              error={status.dependencies.redis.error}
            />
          </div>

          {/* Readiness checks */}
          {readiness && (
            <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-white mb-4">
                <Activity className="w-4 h-4 text-blue-400" />
                Readiness Checks
              </h2>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {Object.entries(readiness.checks).map(([name, result]) => (
                  <div key={name} className="bg-gray-800/50 rounded-xl p-3">
                    <p className="text-xs text-gray-500 mb-1 capitalize">{name}</p>
                    <StatusBadge status={result === "ok" ? "ok" : "error"} />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Runtime metrics */}
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-white mb-4">
              <Cpu className="w-4 h-4 text-blue-400" />
              Runtime Metrics
            </h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <MetricPill label="Uptime" value={formatUptime(status.uptime_sec)} />
              <MetricPill label="Goroutines" value={status.runtime.goroutines} />
              <MetricPill label="Heap" value={`${status.runtime.heap_alloc_mb.toFixed(1)} MB`} />
              <MetricPill label="GC Cycles" value={status.runtime.num_gc} />
            </div>
          </div>

          {/* Version info */}
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-white mb-4">
              <GitBranch className="w-4 h-4 text-blue-400" />
              Version Information
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
              {[
                { label: "Service", value: status.service },
                { label: "Version", value: status.version },
                { label: "Started At", value: new Date(status.started_at).toLocaleString() },
              ].map((item) => (
                <div key={item.label} className="flex flex-col gap-1">
                  <span className="text-xs text-gray-500">{item.label}</span>
                  <span className="text-gray-200 font-mono text-xs">{item.value}</span>
                </div>
              ))}
            </div>
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <XCircle className="w-12 h-12 text-red-500 mb-4" />
          <p className="text-gray-400">Could not reach the platform status endpoint.</p>
          <p className="text-gray-600 text-sm mt-1">Check that the backend is running and accessible.</p>
        </div>
      )}
    </div>
  );
}
