"use client";
import { useQuery } from "@tanstack/react-query";
import {
  Activity, Database, Server, Cpu, Zap, RefreshCw,
  CheckCircle2, AlertTriangle, MemoryStick,
} from "lucide-react";
import { systemApi, type SystemHealthData } from "@/lib/api/system";
import { cn } from "@/lib/utils";

// ── Helpers ───────────────────────────────────────────────────────────────────

function statusColor(status: string) {
  return status === "ok" ? "text-green-400" : "text-red-400";
}

function StatusDot({ status }: { status: string }) {
  return (
    <span
      className={cn(
        "inline-block w-2 h-2 rounded-full",
        status === "ok" ? "bg-green-400" : "bg-red-400"
      )}
    />
  );
}

function fmt(n: number, decimals = 1) {
  return n.toFixed(decimals);
}

function formatUptime(secs: number) {
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

// ── Stat card ─────────────────────────────────────────────────────────────────

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  accent = "blue",
  status,
}: {
  icon: React.ElementType;
  label: string;
  value: string | number;
  sub?: string;
  accent?: "blue" | "green" | "yellow" | "red" | "purple" | "orange";
  status?: string;
}) {
  const accents: Record<string, string> = {
    blue:   "bg-blue-600",
    green:  "bg-green-600",
    yellow: "bg-yellow-600",
    red:    "bg-red-600",
    purple: "bg-purple-600",
    orange: "bg-orange-600",
  };
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-gray-400">{label}</span>
        <div className={cn("w-8 h-8 rounded-xl flex items-center justify-center", accents[accent])}>
          <Icon className="w-4 h-4 text-white" />
        </div>
      </div>
      <div className="flex items-baseline gap-2">
        <p className="text-2xl font-bold text-white">{value}</p>
        {status && <StatusDot status={status} />}
      </div>
      {sub && <p className="text-xs text-gray-500 mt-1">{sub}</p>}
    </div>
  );
}

// ── Section card ──────────────────────────────────────────────────────────────

function Section({ title, icon: Icon, children }: {
  title: string;
  icon: React.ElementType;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-white mb-4">
        <Icon className="w-4 h-4 text-blue-400" />
        {title}
      </h2>
      {children}
    </div>
  );
}

function Row({ label, value, valueClass = "text-gray-300" }: {
  label: string;
  value: React.ReactNode;
  valueClass?: string;
}) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-gray-800/60 last:border-0">
      <span className="text-sm text-gray-500">{label}</span>
      <span className={cn("text-sm font-medium", valueClass)}>{value}</span>
    </div>
  );
}

// ── Queue depth bar ───────────────────────────────────────────────────────────

function QueueBar({ label, count, max }: { label: string; count: number; max: number }) {
  const pct = max > 0 ? Math.min((count / max) * 100, 100) : 0;
  const color = pct > 80 ? "bg-red-500" : pct > 50 ? "bg-yellow-500" : "bg-blue-500";
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs">
        <span className="text-gray-500">{label}</span>
        <span className="text-gray-300 font-mono">{count}</span>
      </div>
      <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
        <div className={cn("h-full rounded-full transition-all", color)} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function SystemHealthPage() {
  const { data, isLoading, isFetching, refetch, dataUpdatedAt } = useQuery({
    queryKey: ["system-health"],
    queryFn: systemApi.getHealth,
    refetchInterval: 15_000, // auto-refresh every 15s
    staleTime: 10_000,
  });

  const h = data as SystemHealthData | undefined;

  const totalQueued = h?.tasks.total_queued ?? 0;
  const maxQueueDepth = 1000; // matches config queue_size

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Activity className="w-6 h-6 text-blue-400" />
            System Health
          </h1>
          <p className="text-gray-400 text-sm mt-0.5">
            Platform infrastructure metrics — refreshes every 15 seconds
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

      {isLoading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 animate-pulse h-28" />
          ))}
        </div>
      ) : !h ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <AlertTriangle className="w-12 h-12 text-yellow-600 mb-4" />
          <p className="text-gray-400">Could not load system health data.</p>
          <p className="text-gray-600 text-sm mt-1">Check that the backend is running.</p>
        </div>
      ) : (
        <>
          {/* Top stat cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              icon={Server}
              label="API Status"
              value={h.api.status === "ok" ? "Healthy" : "Degraded"}
              sub={`Uptime ${formatUptime(h.uptime_secs)}`}
              accent={h.api.status === "ok" ? "green" : "red"}
              status={h.api.status}
            />
            <StatCard
              icon={Database}
              label="Database"
              value={h.database.status === "ok" ? "Connected" : "Error"}
              sub={`${fmt(h.database.latency_ms)} ms latency`}
              accent={h.database.status === "ok" ? "green" : "red"}
              status={h.database.status}
            />
            <StatCard
              icon={Zap}
              label="Redis Cache"
              value={h.cache.status === "ok" ? "Connected" : "Error"}
              sub={`${fmt(h.cache.hit_rate_percent)}% hit rate`}
              accent={h.cache.status === "ok" ? "green" : "red"}
              status={h.cache.status}
            />
            <StatCard
              icon={Activity}
              label="Task Queue"
              value={totalQueued}
              sub={`${h.tasks.running_tasks} running · ${h.tasks.pending_tasks} pending`}
              accent={totalQueued > 800 ? "red" : totalQueued > 400 ? "yellow" : "blue"}
            />
          </div>

          {/* Detail sections */}
          <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">

            {/* Database */}
            <Section title="Database" icon={Database}>
              <Row
                label="Status"
                value={
                  <span className={cn("flex items-center gap-1.5", statusColor(h.database.status))}>
                    {h.database.status === "ok"
                      ? <CheckCircle2 className="w-3.5 h-3.5" />
                      : <AlertTriangle className="w-3.5 h-3.5" />}
                    {h.database.status}
                  </span>
                }
              />
              <Row label="Latency" value={`${fmt(h.database.latency_ms)} ms`} />
              <Row label="Open connections" value={h.database.open_connections} />
              <Row label="In use" value={h.database.in_use_connections} />
              <Row label="Idle" value={h.database.idle_connections} />
              <Row label="Wait count" value={h.database.wait_count} />
            </Section>

            {/* Redis */}
            <Section title="Redis Cache" icon={Zap}>
              <Row
                label="Status"
                value={
                  <span className={cn("flex items-center gap-1.5", statusColor(h.cache.status))}>
                    {h.cache.status === "ok"
                      ? <CheckCircle2 className="w-3.5 h-3.5" />
                      : <AlertTriangle className="w-3.5 h-3.5" />}
                    {h.cache.status}
                  </span>
                }
              />
              <Row label="Latency" value={`${fmt(h.cache.latency_ms)} ms`} />
              <Row label="Used memory" value={`${fmt(h.cache.used_memory_mb)} MB`} />
              <Row
                label="Hit rate"
                value={`${fmt(h.cache.hit_rate_percent)}%`}
                valueClass={h.cache.hit_rate_percent > 80 ? "text-green-400" : "text-yellow-400"}
              />
            </Section>

            {/* Go runtime */}
            <Section title="Go Runtime" icon={Cpu}>
              <Row label="Goroutines" value={h.runtime.goroutines} />
              <Row label="Heap allocated" value={`${fmt(h.runtime.heap_alloc_mb)} MB`} />
              <Row label="Heap system" value={`${fmt(h.runtime.heap_sys_mb)} MB`} />
              <Row label="GC pause (last)" value={`${fmt(h.runtime.gc_pause_ms, 2)} ms`} />
              <Row label="GC cycles" value={h.runtime.num_gc} />
            </Section>

            {/* Task queues */}
            <Section title="Task Queue Depths" icon={Activity}>
              <div className="space-y-3">
                <QueueBar label="Total queued" count={totalQueued} max={maxQueueDepth} />
                {Object.entries(h.tasks.queue_depths).map(([priority, count]) => (
                  <QueueBar
                    key={priority}
                    label={`Priority ${priority}`}
                    count={count}
                    max={maxQueueDepth}
                  />
                ))}
                {Object.keys(h.tasks.queue_depths).length === 0 && (
                  <p className="text-sm text-gray-600 text-center py-2">All queues empty</p>
                )}
              </div>
              <div className="mt-4 pt-3 border-t border-gray-800 grid grid-cols-2 gap-3">
                <div className="bg-gray-800/50 rounded-xl p-3 text-center">
                  <p className="text-xs text-gray-500 mb-1">Running</p>
                  <p className="text-xl font-bold text-blue-400">{h.tasks.running_tasks}</p>
                </div>
                <div className="bg-gray-800/50 rounded-xl p-3 text-center">
                  <p className="text-xs text-gray-500 mb-1">Pending</p>
                  <p className="text-xl font-bold text-yellow-400">{h.tasks.pending_tasks}</p>
                </div>
              </div>
            </Section>

            {/* Memory */}
            <Section title="Memory Usage" icon={MemoryStick}>
              <div className="space-y-3">
                <div>
                  <div className="flex justify-between text-xs mb-1">
                    <span className="text-gray-500">Heap allocated</span>
                    <span className="text-gray-300">{fmt(h.runtime.heap_alloc_mb)} MB</span>
                  </div>
                  <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-blue-500 rounded-full"
                      style={{
                        width: `${Math.min((h.runtime.heap_alloc_mb / h.runtime.heap_sys_mb) * 100, 100)}%`,
                      }}
                    />
                  </div>
                  <div className="flex justify-between text-xs mt-0.5">
                    <span className="text-gray-600">0 MB</span>
                    <span className="text-gray-600">{fmt(h.runtime.heap_sys_mb)} MB reserved</span>
                  </div>
                </div>
              </div>
            </Section>

            {/* Server info */}
            <Section title="Server Info" icon={Server}>
              <Row label="Uptime" value={formatUptime(h.uptime_secs)} />
              <Row label="Snapshot time" value={new Date(h.timestamp).toLocaleTimeString()} />
              <Row
                label="API"
                value={
                  <span className={cn("flex items-center gap-1.5", statusColor(h.api.status))}>
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    {h.api.status}
                  </span>
                }
              />
            </Section>
          </div>
        </>
      )}
    </div>
  );
}
