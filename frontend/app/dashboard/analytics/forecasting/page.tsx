"use client";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, TrendingUp, HardDrive, Server, CheckCircle2, RefreshCw } from "lucide-react";
import { analyticsApi } from "@/lib/api/analytics";
import type { HypervisorForecast, ForecastRisk } from "@/types";

const PROVIDER_COLORS: Record<string, string> = {
  vmware: "text-blue-400", esxi: "text-blue-300",
  proxmox: "text-orange-400", kvm: "text-green-400", hyperv: "text-purple-400",
};

const RISK_CONFIG = {
  critical: { color: "text-red-400", bg: "bg-red-950/30", border: "border-red-800/40", icon: AlertTriangle },
  warning:  { color: "text-yellow-400", bg: "bg-yellow-950/30", border: "border-yellow-800/40", icon: AlertTriangle },
  info:     { color: "text-blue-400", bg: "bg-blue-950/30", border: "border-blue-800/40", icon: TrendingUp },
};

function RiskBadge({ risk }: { risk: ForecastRisk }) {
  const cfg = RISK_CONFIG[risk.severity as keyof typeof RISK_CONFIG] ?? RISK_CONFIG.info;
  const Icon = cfg.icon;
  return (
    <div className={`flex items-start gap-2 p-3 rounded-xl border ${cfg.bg} ${cfg.border}`}>
      <Icon className={`w-4 h-4 shrink-0 mt-0.5 ${cfg.color}`} />
      <div>
        <p className={`text-xs font-medium ${cfg.color}`}>{risk.type.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}</p>
        <p className="text-xs text-gray-400 mt-0.5">{risk.description}</p>
        {risk.days_until !== undefined && (
          <p className={`text-xs font-semibold mt-1 ${cfg.color}`}>~{risk.days_until} days</p>
        )}
      </div>
    </div>
  );
}

function SnapshotRiskBadge({ risk }: { risk: string }) {
  const map = {
    low:    { color: "text-green-400", bg: "bg-green-950/30", label: "Low" },
    medium: { color: "text-yellow-400", bg: "bg-yellow-950/30", label: "Medium" },
    high:   { color: "text-red-400", bg: "bg-red-950/30", label: "High" },
  };
  const cfg = map[risk as keyof typeof map] ?? map.low;
  return (
    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${cfg.bg} ${cfg.color}`}>{cfg.label}</span>
  );
}

function StorageGauge({ pct }: { pct: number }) {
  const clamped = Math.min(Math.max(pct, 0), 100);
  const color = clamped > 85 ? "#ef4444" : clamped > 65 ? "#f59e0b" : "#22c55e";
  const r = 28, cx = 36, cy = 36;
  const circumference = 2 * Math.PI * r;
  const dash = (clamped / 100) * circumference;
  return (
    <div className="flex flex-col items-center">
      <svg width="72" height="72" viewBox="0 0 72 72">
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="#1f2937" strokeWidth="6" />
        <circle cx={cx} cy={cy} r={r} fill="none" stroke={color} strokeWidth="6"
          strokeDasharray={`${dash} ${circumference - dash}`}
          strokeLinecap="round"
          transform={`rotate(-90 ${cx} ${cy})`} />
        <text x={cx} y={cy + 5} textAnchor="middle" fill="white" fontSize="11" fontWeight="bold">
          {clamped.toFixed(0)}%
        </text>
      </svg>
      <span className="text-xs text-gray-500 mt-1">Storage</span>
    </div>
  );
}

function ForecastCard({ forecast }: { forecast: HypervisorForecast }) {
  const providerColor = PROVIDER_COLORS[forecast.provider] ?? "text-gray-400";
  const risks = forecast.risks ?? [];
  const hasRisks = risks.length > 0;

  return (
    <div className={`bg-gray-900 border rounded-2xl p-5 space-y-4 ${hasRisks ? "border-yellow-700/40" : "border-gray-800"}`}>
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold text-white">{forecast.hypervisor_name}</h3>
          <span className={`text-xs capitalize ${providerColor}`}>{forecast.provider}</span>
        </div>
        <StorageGauge pct={forecast.current_storage_used_pct} />
      </div>

      {/* Metrics grid */}
      <div className="grid grid-cols-2 gap-3 text-sm">
        <div className="bg-gray-800/50 rounded-xl p-3">
          <div className="flex items-center gap-1.5 text-gray-500 text-xs mb-1">
            <HardDrive className="w-3 h-3" /> Storage growth
          </div>
          <p className="text-white font-semibold">{forecast.storage_growth_rate_gb_day.toFixed(2)} GB/day</p>
          {forecast.storage_exhaustion_days !== undefined && (
            <p className={`text-xs mt-0.5 ${forecast.storage_exhaustion_days < 30 ? "text-red-400" : forecast.storage_exhaustion_days < 90 ? "text-yellow-400" : "text-gray-500"}`}>
              Full in ~{forecast.storage_exhaustion_days}d
            </p>
          )}
        </div>
        <div className="bg-gray-800/50 rounded-xl p-3">
          <div className="flex items-center gap-1.5 text-gray-500 text-xs mb-1">
            <Server className="w-3 h-3" /> VM growth
          </div>
          <p className="text-white font-semibold">{forecast.vm_growth_rate_per_day.toFixed(2)}/day</p>
          <p className="text-xs text-gray-500 mt-0.5">~{forecast.projected_vms_30_days} VMs in 30d</p>
        </div>
        <div className="bg-gray-800/50 rounded-xl p-3 col-span-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5 text-gray-500 text-xs">
              <HardDrive className="w-3 h-3" /> Snapshot growth
            </div>
            <SnapshotRiskBadge risk={forecast.snapshot_risk} />
          </div>
          <p className="text-white font-semibold mt-1">{forecast.snapshot_growth_rate_per_day.toFixed(2)}/day</p>
        </div>
      </div>

      {/* Risks */}
      {risks.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs text-gray-500 uppercase tracking-wide">Predicted Risks</p>
          {risks.map((r, i) => <RiskBadge key={i} risk={r} />)}
        </div>
      )}
    </div>
  );
}

export default function ForecastingPage() {
  const { data: report, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["analytics-forecasts"],
    queryFn: analyticsApi.getForecasts,
    refetchInterval: 300_000,
  });

  const forecasts = report?.forecasts ?? [];
  const globalRisks = report?.global_risks ?? [];

  const criticalForecasts = forecasts.filter((f) => (f.risks ?? []).some((r) => r.severity === "critical"));
  const warningForecasts = forecasts.filter((f) => (f.risks ?? []).some((r) => r.severity === "warning") && !criticalForecasts.includes(f));
  const healthyForecasts = forecasts.filter((f) => (f.risks ?? []).length === 0);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Capacity Forecasting</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {report ? `Generated ${new Date(report.generated_at).toLocaleString()}` : "Trend-based predictions"}
          </p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 text-gray-300 rounded-lg text-sm transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${isFetching ? "animate-spin" : ""}`} />
          Refresh
        </button>
      </div>

      {/* Global risks */}
      {globalRisks.length > 0 && (
        <div className="bg-red-950/20 border border-red-800/40 rounded-2xl p-5 space-y-3">
          <h2 className="text-sm font-semibold text-red-400">Global Infrastructure Risks</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {globalRisks.map((r, i) => <RiskBadge key={i} risk={r} />)}
          </div>
        </div>
      )}

      {/* Summary row */}
      {!isLoading && (
        <div className="grid grid-cols-3 gap-4">
          <div className="bg-red-950/20 border border-red-800/30 rounded-2xl p-4 text-center">
            <p className="text-2xl font-bold text-red-400">{criticalForecasts.length}</p>
            <p className="text-xs text-gray-500 mt-1">Critical Risk</p>
          </div>
          <div className="bg-yellow-950/20 border border-yellow-800/30 rounded-2xl p-4 text-center">
            <p className="text-2xl font-bold text-yellow-400">{warningForecasts.length}</p>
            <p className="text-xs text-gray-500 mt-1">Warning</p>
          </div>
          <div className="bg-green-950/20 border border-green-800/30 rounded-2xl p-4 text-center">
            <p className="text-2xl font-bold text-green-400">{healthyForecasts.length}</p>
            <p className="text-xs text-gray-500 mt-1">Healthy</p>
          </div>
        </div>
      )}

      {/* Forecast cards */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 h-64 animate-pulse" />
          ))}
        </div>
      ) : forecasts.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-12 text-center">
          <CheckCircle2 className="w-10 h-10 text-green-400 mx-auto mb-3 opacity-60" />
          <p className="text-gray-400">No forecast data yet.</p>
          <p className="text-gray-600 text-sm mt-1">Sync hypervisors and wait for the analytics engine to collect history.</p>
        </div>
      ) : (
        <>
          {criticalForecasts.length > 0 && (
            <div>
              <h2 className="text-sm font-semibold text-red-400 mb-3 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" /> Critical Risk Hypervisors
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {criticalForecasts.map((f) => <ForecastCard key={f.hypervisor_id} forecast={f} />)}
              </div>
            </div>
          )}
          {warningForecasts.length > 0 && (
            <div>
              <h2 className="text-sm font-semibold text-yellow-400 mb-3 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" /> Warning
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {warningForecasts.map((f) => <ForecastCard key={f.hypervisor_id} forecast={f} />)}
              </div>
            </div>
          )}
          {healthyForecasts.length > 0 && (
            <div>
              <h2 className="text-sm font-semibold text-green-400 mb-3 flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4" /> Healthy
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {healthyForecasts.map((f) => <ForecastCard key={f.hypervisor_id} forecast={f} />)}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
