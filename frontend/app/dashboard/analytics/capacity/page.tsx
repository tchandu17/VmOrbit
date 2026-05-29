"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { HardDrive, Server, Cpu, Database, TrendingUp, TrendingDown, Minus } from "lucide-react";
import { analyticsApi } from "@/lib/api/analytics";
import type { CapacityTrends, ProviderCapacity } from "@/types";

function fmtGB(gb: number) {
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`;
  return `${gb.toFixed(1)} GB`;
}
function fmtMB(mb: number) {
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(1)} TB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

// ── Simple SVG line chart ─────────────────────────────────────────────────────
function LineChart({
  series, height = 120, color = "#3b82f6", label,
}: {
  series: { timestamp: string; value: number }[];
  height?: number;
  color?: string;
  label?: string;
}) {
  if (!series.length) {
    return (
      <div className="flex items-center justify-center h-32 text-gray-600 text-sm">
        No data available yet
      </div>
    );
  }
  const values = series.map((p) => p.value);
  const max = Math.max(...values, 1);
  const min = Math.min(...values);
  const range = max - min || 1;
  const w = 600;
  const coords = series.map((p, i) => {
    const x = (i / Math.max(series.length - 1, 1)) * w;
    const y = height - ((p.value - min) / range) * (height - 10) - 5;
    return `${x},${y}`;
  });
  const areaCoords = `0,${height} ${coords.join(" ")} ${w},${height}`;
  const last = values[values.length - 1];
  const prev = values[values.length - 2] ?? last;
  const trend = last > prev ? "up" : last < prev ? "down" : "flat";

  return (
    <div>
      {label && (
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-gray-400">{label}</span>
          <div className="flex items-center gap-1 text-xs">
            {trend === "up" && <TrendingUp className="w-3 h-3 text-green-400" />}
            {trend === "down" && <TrendingDown className="w-3 h-3 text-red-400" />}
            {trend === "flat" && <Minus className="w-3 h-3 text-gray-400" />}
            <span className={trend === "up" ? "text-green-400" : trend === "down" ? "text-red-400" : "text-gray-400"}>
              {last.toFixed(1)}
            </span>
          </div>
        </div>
      )}
      <svg viewBox={`0 0 ${w} ${height}`} className="w-full" style={{ height }}>
        <defs>
          <linearGradient id={`grad-${color.replace("#", "")}`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.3" />
            <stop offset="100%" stopColor={color} stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <polygon points={areaCoords} fill={`url(#grad-${color.replace("#", "")})`} />
        <polyline points={coords.join(" ")} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
        {series.length > 0 && (
          <circle
            cx={(series.length - 1) / Math.max(series.length - 1, 1) * w}
            cy={height - ((last - min) / range) * (height - 10) - 5}
            r="3" fill={color}
          />
        )}
      </svg>
      <div className="flex justify-between text-xs text-gray-600 mt-1">
        <span>{series[0] ? new Date(series[0].timestamp).toLocaleDateString() : ""}</span>
        <span>{series[series.length - 1] ? new Date(series[series.length - 1].timestamp).toLocaleDateString() : ""}</span>
      </div>
    </div>
  );
}

// ── Capacity Card ─────────────────────────────────────────────────────────────
function CapacityCard({ cap }: { cap: ProviderCapacity }) {
  const name = cap.hypervisor?.name ?? cap.hypervisor_id.slice(0, 8);
  const provider = cap.hypervisor?.provider ?? "unknown";
  const storageRisk = cap.storage_used_pct > 85 ? "critical" : cap.storage_used_pct > 65 ? "warning" : "ok";

  return (
    <div className={`bg-gray-900 border rounded-2xl p-5 space-y-4 ${
      storageRisk === "critical" ? "border-red-700/50" :
      storageRisk === "warning" ? "border-yellow-700/50" : "border-gray-800"
    }`}>
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold text-white">{name}</h3>
          <span className="text-xs text-gray-500 capitalize">{provider}</span>
        </div>
        {storageRisk !== "ok" && (
          <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
            storageRisk === "critical" ? "bg-red-900/40 text-red-400" : "bg-yellow-900/40 text-yellow-400"
          }`}>
            {storageRisk === "critical" ? "Storage Critical" : "Storage Warning"}
          </span>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <div className="flex items-center gap-2 text-gray-300">
          <Server className="w-3.5 h-3.5 text-blue-400 shrink-0" />
          <span>{cap.total_vms} VMs <span className="text-gray-500">({cap.running_vms} on)</span></span>
        </div>
        <div className="flex items-center gap-2 text-gray-300">
          <Cpu className="w-3.5 h-3.5 text-purple-400 shrink-0" />
          <span>{cap.total_cpu_cores} cores</span>
        </div>
        <div className="flex items-center gap-2 text-gray-300">
          <Database className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
          <span>{fmtMB(cap.total_memory_mb)}</span>
        </div>
        <div className="flex items-center gap-2 text-gray-300">
          <HardDrive className="w-3.5 h-3.5 text-green-400 shrink-0" />
          <span>{cap.snapshot_count} snaps</span>
        </div>
      </div>

      {/* Storage bar */}
      <div>
        <div className="flex justify-between text-xs text-gray-400 mb-1">
          <span>Storage</span>
          <span className={storageRisk === "critical" ? "text-red-400" : storageRisk === "warning" ? "text-yellow-400" : "text-gray-300"}>
            {cap.storage_used_pct.toFixed(1)}%
          </span>
        </div>
        <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${
              storageRisk === "critical" ? "bg-red-500" : storageRisk === "warning" ? "bg-yellow-500" : "bg-green-500"
            }`}
            style={{ width: `${Math.min(cap.storage_used_pct, 100)}%` }}
          />
        </div>
        <div className="flex justify-between text-xs text-gray-600 mt-1">
          <span>{fmtGB(cap.used_storage_gb)} used</span>
          <span>{fmtGB(cap.free_storage_gb)} free</span>
        </div>
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────
const RANGES = [
  { label: "7 days", days: 7 },
  { label: "30 days", days: 30 },
  { label: "90 days", days: 90 },
];

export default function CapacityPlanningPage() {
  const [rangeDays, setRangeDays] = useState(30);

  const since = new Date(Date.now() - rangeDays * 24 * 60 * 60 * 1000).toISOString();

  const { data: trends, isLoading: loadingTrends } = useQuery({
    queryKey: ["analytics-trends", rangeDays],
    queryFn: () => analyticsApi.getCapacityTrends({ since, granularity: "day" }),
  });

  const { data: providerCaps, isLoading: loadingCaps } = useQuery({
    queryKey: ["analytics-provider-capacity"],
    queryFn: analyticsApi.getProviderCapacity,
    refetchInterval: 60_000,
  });

  const { data: summary } = useQuery({
    queryKey: ["analytics-capacity"],
    queryFn: analyticsApi.getCapacity,
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Capacity Planning</h1>
          <p className="text-gray-400 text-sm mt-0.5">Historical trends and resource utilisation</p>
        </div>
        <div className="flex items-center gap-1 bg-gray-900 border border-gray-800 rounded-lg p-1">
          {RANGES.map((r) => (
            <button
              key={r.days}
              onClick={() => setRangeDays(r.days)}
              className={`px-3 py-1.5 rounded-md text-sm transition-colors ${
                rangeDays === r.days ? "bg-blue-600 text-white" : "text-gray-400 hover:text-white"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {/* Summary row */}
      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[
            { label: "Total Storage", value: fmtGB(summary.total_storage_gb), sub: `${fmtGB(summary.used_storage_gb)} used`, icon: HardDrive, color: "text-green-400" },
            { label: "Free Storage", value: fmtGB(summary.free_storage_gb), sub: `${summary.storage_utilisation_pct.toFixed(1)}% used`, icon: HardDrive, color: "text-teal-400" },
            { label: "Total VMs", value: summary.total_vms, sub: `${summary.running_vms} running`, icon: Server, color: "text-blue-400" },
            { label: "Total Snapshots", value: summary.total_snapshots, sub: "across all VMs", icon: Database, color: "text-orange-400" },
          ].map(({ label, value, sub, icon: Icon, color }) => (
            <div key={label} className="bg-gray-900 border border-gray-800 rounded-2xl p-4 flex items-start gap-3">
              <div className={`p-2 rounded-xl bg-gray-800 shrink-0 ${color}`}><Icon className="w-4 h-4" /></div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
                <p className="text-xl font-bold text-white">{value}</p>
                <p className="text-xs text-gray-500">{sub}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Trend charts */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {[
          { key: "vm_growth" as keyof CapacityTrends, label: "VM Growth", color: "#3b82f6", unit: "VMs" },
          { key: "storage_trend" as keyof CapacityTrends, label: "Storage Used (GB)", color: "#22c55e", unit: "GB" },
          { key: "snapshot_trend" as keyof CapacityTrends, label: "Snapshot Count", color: "#f97316", unit: "snapshots" },
          { key: "task_trend" as keyof CapacityTrends, label: "Tasks Completed (24h)", color: "#f59e0b", unit: "tasks" },
        ].map(({ key, label, color }) => (
          <div key={key} className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
            <h3 className="text-sm font-semibold text-white mb-4">{label}</h3>
            {loadingTrends ? (
              <div className="h-32 bg-gray-800 rounded animate-pulse" />
            ) : (
              <LineChart
                series={(trends?.[key] as { timestamp: string; value: number }[] | undefined) ?? []}
                color={color}
                height={120}
              />
            )}
          </div>
        ))}
      </div>

      {/* Per-provider capacity cards */}
      <div>
        <h2 className="text-sm font-semibold text-white mb-4">Per-Provider Capacity</h2>
        {loadingCaps ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 h-48 animate-pulse" />
            ))}
          </div>
        ) : !providerCaps?.length ? (
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-8 text-center text-gray-500">
            No capacity data yet. Sync a hypervisor to populate.
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {providerCaps.map((cap) => <CapacityCard key={cap.hypervisor_id} cap={cap} />)}
          </div>
        )}
      </div>
    </div>
  );
}
