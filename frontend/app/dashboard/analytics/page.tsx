"use client";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Server, Cpu, HardDrive, Activity, TrendingUp, AlertTriangle,
  CheckCircle2, RefreshCw, BarChart3, Zap, Database, Layers,
} from "lucide-react";
import { analyticsApi } from "@/lib/api/analytics";
import Link from "next/link";

// ── Helpers ───────────────────────────────────────────────────────────────────
function fmtGB(gb: number) {
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`;
  return `${gb.toFixed(0)} GB`;
}
function fmtMB(mb: number) {
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(1)} TB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}
function pct(v: number) { return `${v.toFixed(1)}%`; }

// ── Stat Card ─────────────────────────────────────────────────────────────────
function StatCard({
  label, value, sub, icon: Icon, color = "text-blue-400", warn,
}: {
  label: string; value: string | number; sub?: string;
  icon: React.ElementType; color?: string; warn?: boolean;
}) {
  return (
    <div className={`bg-gray-900 border rounded-2xl p-5 flex items-start gap-4 ${warn ? "border-yellow-700/50" : "border-gray-800"}`}>
      <div className={`p-2.5 rounded-xl bg-gray-800 shrink-0 ${color}`}>
        <Icon className="w-5 h-5" />
      </div>
      <div className="min-w-0">
        <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">{label}</p>
        <p className="text-2xl font-bold text-white">{value}</p>
        {sub && <p className="text-xs text-gray-500 mt-0.5">{sub}</p>}
      </div>
    </div>
  );
}

// ── Utilisation Bar ───────────────────────────────────────────────────────────
function UtilBar({ label, pct: p, color }: { label: string; pct: number; color: string }) {
  const clamped = Math.min(Math.max(p, 0), 100);
  const barColor = clamped > 85 ? "bg-red-500" : clamped > 65 ? "bg-yellow-500" : color;
  return (
    <div>
      <div className="flex justify-between text-xs text-gray-400 mb-1">
        <span>{label}</span>
        <span className={clamped > 85 ? "text-red-400" : clamped > 65 ? "text-yellow-400" : "text-gray-300"}>
          {pct(clamped)}
        </span>
      </div>
      <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
        <div className={`h-full rounded-full transition-all ${barColor}`} style={{ width: `${clamped}%` }} />
      </div>
    </div>
  );
}

// ── Mini Trend Sparkline (SVG) ────────────────────────────────────────────────
function Sparkline({ points, color = "#3b82f6" }: { points: number[]; color?: string }) {
  if (!points.length) return <div className="h-10 bg-gray-800 rounded animate-pulse" />;
  const max = Math.max(...points, 1);
  const min = Math.min(...points);
  const range = max - min || 1;
  const w = 120, h = 40;
  const coords = points.map((v, i) => {
    const x = (i / (points.length - 1)) * w;
    const y = h - ((v - min) / range) * (h - 4) - 2;
    return `${x},${y}`;
  });
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="overflow-visible">
      <polyline points={coords.join(" ")} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
    </svg>
  );
}

// ── Provider Capacity Row ─────────────────────────────────────────────────────
function ProviderRow({ cap }: { cap: import("@/types").ProviderCapacity }) {
  const name = cap.hypervisor?.name ?? cap.hypervisor_id.slice(0, 8);
  const provider = cap.hypervisor?.provider ?? "unknown";
  const providerColor: Record<string, string> = {
    vmware: "text-blue-400", esxi: "text-blue-300",
    proxmox: "text-orange-400", kvm: "text-green-400", hyperv: "text-purple-400",
  };
  return (
    <tr className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
      <td className="px-4 py-3">
        <div className="font-medium text-white text-sm">{name}</div>
        <div className={`text-xs ${providerColor[provider] ?? "text-gray-400"}`}>{provider}</div>
      </td>
      <td className="px-4 py-3 text-sm text-gray-300">{cap.total_vms} <span className="text-gray-500 text-xs">({cap.running_vms} running)</span></td>
      <td className="px-4 py-3 text-sm text-gray-300">{cap.total_cpu_cores}</td>
      <td className="px-4 py-3 text-sm text-gray-300">{fmtMB(cap.total_memory_mb)}</td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="flex-1 h-1.5 bg-gray-800 rounded-full overflow-hidden min-w-[60px]">
            <div
              className={`h-full rounded-full ${cap.storage_used_pct > 85 ? "bg-red-500" : cap.storage_used_pct > 65 ? "bg-yellow-500" : "bg-blue-500"}`}
              style={{ width: `${Math.min(cap.storage_used_pct, 100)}%` }}
            />
          </div>
          <span className="text-xs text-gray-400 whitespace-nowrap">{pct(cap.storage_used_pct)}</span>
        </div>
        <div className="text-xs text-gray-500 mt-0.5">{fmtGB(cap.used_storage_gb)} / {fmtGB(cap.total_storage_gb)}</div>
      </td>
    </tr>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────
export default function AnalyticsDashboardPage() {
  const qc = useQueryClient();

  const { data: summary, isLoading: loadingSummary } = useQuery({
    queryKey: ["analytics-capacity"],
    queryFn: analyticsApi.getCapacity,
    refetchInterval: 60_000,
  });

  const { data: trends } = useQuery({
    queryKey: ["analytics-trends"],
    queryFn: () => analyticsApi.getCapacityTrends({ granularity: "day" }),
    refetchInterval: 300_000,
  });

  const { data: providerCaps } = useQuery({
    queryKey: ["analytics-provider-capacity"],
    queryFn: analyticsApi.getProviderCapacity,
    refetchInterval: 60_000,
  });

  const { data: recSummary } = useQuery({
    queryKey: ["analytics-rec-summary"],
    queryFn: analyticsApi.getRecommendationSummary,
    refetchInterval: 60_000,
  });

  const collectMut = useMutation({
    mutationFn: analyticsApi.triggerCollection,
    onSuccess: () => {
      setTimeout(() => {
        qc.invalidateQueries({ queryKey: ["analytics-capacity"] });
        qc.invalidateQueries({ queryKey: ["analytics-provider-capacity"] });
        qc.invalidateQueries({ queryKey: ["analytics-rec-summary"] });
      }, 3000);
    },
  });

  const vmTrend = (trends?.vm_growth ?? []).map((p) => p.value);
  const storageTrend = (trends?.storage_trend ?? []).map((p) => p.value);
  const taskTrend = (trends?.task_trend ?? []).map((p) => p.value);
  const snapTrend = (trends?.snapshot_trend ?? []).map((p) => p.value);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Infrastructure Analytics</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {summary ? `Last collected ${new Date(summary.collected_at).toLocaleTimeString()}` : "Loading…"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Link href="/dashboard/analytics/capacity" className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            Capacity Planning
          </Link>
          <Link href="/dashboard/analytics/recommendations" className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            Recommendations
          </Link>
          <Link href="/dashboard/analytics/forecasting" className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            Forecasting
          </Link>
          <button
            onClick={() => collectMut.mutate()}
            disabled={collectMut.isPending}
            className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${collectMut.isPending ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>
      </div>

      {/* Top stat cards */}
      {loadingSummary ? (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 h-24 animate-pulse" />
          ))}
        </div>
      ) : summary ? (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard label="Total VMs" value={summary.total_vms} sub={`${summary.running_vms} running`} icon={Server} color="text-blue-400" />
          <StatCard label="Hypervisors" value={summary.total_hypervisors} sub={`${summary.total_environments} environments`} icon={Cpu} color="text-purple-400" />
          <StatCard label="Total Storage" value={fmtGB(summary.total_storage_gb)} sub={`${fmtGB(summary.free_storage_gb)} free`} icon={HardDrive} color="text-green-400" warn={summary.storage_utilisation_pct > 80} />
          <StatCard label="Tasks (24h)" value={summary.tasks_completed_24h} sub={`${summary.tasks_failed_24h} failed`} icon={Activity} color="text-yellow-400" />
          <StatCard label="CPU Cores" value={summary.total_cpu_cores.toLocaleString()} sub="allocated" icon={Cpu} color="text-blue-300" />
          <StatCard label="Total Memory" value={fmtMB(summary.total_memory_mb)} sub="allocated" icon={Database} color="text-indigo-400" />
          <StatCard label="Snapshots" value={summary.total_snapshots} sub="across all VMs" icon={Layers} color="text-orange-400" />
          <StatCard label="VM Density" value={summary.vm_density.toFixed(1)} sub="running VMs / hypervisor" icon={BarChart3} color="text-teal-400" />
        </div>
      ) : null}

      {/* Utilisation + Recommendations */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Utilisation */}
        <div className="lg:col-span-2 bg-gray-900 border border-gray-800 rounded-2xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-white">Resource Utilisation</h2>
          {summary ? (
            <div className="space-y-4">
              <UtilBar label="CPU Utilisation" pct={summary.cpu_utilisation_pct} color="bg-blue-500" />
              <UtilBar label="Memory Utilisation" pct={summary.memory_utilisation_pct} color="bg-purple-500" />
              <UtilBar label="Storage Utilisation" pct={summary.storage_utilisation_pct} color="bg-green-500" />
            </div>
          ) : (
            <div className="space-y-4">
              {[1, 2, 3].map((i) => <div key={i} className="h-6 bg-gray-800 rounded animate-pulse" />)}
            </div>
          )}
        </div>

        {/* Recommendations summary */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold text-white">Optimization Alerts</h2>
            <Link href="/dashboard/analytics/recommendations" className="text-xs text-blue-400 hover:text-blue-300">View all →</Link>
          </div>
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 bg-red-950/30 border border-red-800/30 rounded-xl">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-red-400" />
                <span className="text-sm text-red-300">Critical</span>
              </div>
              <span className="text-lg font-bold text-red-400">{recSummary?.by_severity?.critical ?? summary?.critical_recommendations ?? 0}</span>
            </div>
            <div className="flex items-center justify-between p-3 bg-yellow-950/30 border border-yellow-800/30 rounded-xl">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-yellow-400" />
                <span className="text-sm text-yellow-300">Warning</span>
              </div>
              <span className="text-lg font-bold text-yellow-400">{recSummary?.by_severity?.warning ?? summary?.warning_recommendations ?? 0}</span>
            </div>
            <div className="flex items-center justify-between p-3 bg-blue-950/30 border border-blue-800/30 rounded-xl">
              <div className="flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4 text-blue-400" />
                <span className="text-sm text-blue-300">Info</span>
              </div>
              <span className="text-lg font-bold text-blue-400">{recSummary?.by_severity?.info ?? summary?.info_recommendations ?? 0}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Trend sparklines */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: "VM Growth", points: vmTrend, color: "#3b82f6", icon: Server },
          { label: "Storage Used", points: storageTrend, color: "#22c55e", icon: HardDrive },
          { label: "Tasks Completed", points: taskTrend, color: "#f59e0b", icon: Zap },
          { label: "Snapshots", points: snapTrend, color: "#f97316", icon: Layers },
        ].map(({ label, points, color, icon: Icon }) => (
          <div key={label} className="bg-gray-900 border border-gray-800 rounded-2xl p-4">
            <div className="flex items-center gap-2 mb-3">
              <Icon className="w-4 h-4" style={{ color }} />
              <span className="text-xs text-gray-400">{label}</span>
            </div>
            <Sparkline points={points} color={color} />
            {points.length > 0 && (
              <div className="flex items-center gap-1 mt-2">
                <TrendingUp className="w-3 h-3 text-gray-500" />
                <span className="text-xs text-gray-500">
                  {points[points.length - 1]?.toFixed(0) ?? "—"} current
                </span>
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Provider capacity table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-white">Provider Capacity</h2>
          <Link href="/dashboard/analytics/capacity" className="text-xs text-blue-400 hover:text-blue-300">Full report →</Link>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase tracking-wide">
                <th className="text-left px-4 py-3">Hypervisor</th>
                <th className="text-left px-4 py-3">VMs</th>
                <th className="text-left px-4 py-3">CPU Cores</th>
                <th className="text-left px-4 py-3">Memory</th>
                <th className="text-left px-4 py-3">Storage</th>
              </tr>
            </thead>
            <tbody>
              {!providerCaps ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 5 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-20" /></td>
                    ))}
                  </tr>
                ))
              ) : providerCaps.length === 0 ? (
                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-500">No capacity data yet. Sync a hypervisor to populate.</td></tr>
              ) : (
                providerCaps.map((cap) => <ProviderRow key={cap.hypervisor_id} cap={cap} />)
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
