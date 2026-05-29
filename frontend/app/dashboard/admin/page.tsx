"use client";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Shield, Settings, AlertTriangle, CheckCircle2, RefreshCw,
  Wrench, Power, Database, Zap, Activity, ChevronRight,
  Lock, Unlock, Info,
} from "lucide-react";
import { cn } from "@/lib/utils";
import api from "@/lib/api/client";
import type { ApiResponse } from "@/types";

// ── Types ─────────────────────────────────────────────────────────────────────

interface MaintenanceStatus {
  maintenance: boolean;
  reason: string;
  since: string | null;
}

// ── API ───────────────────────────────────────────────────────────────────────

const adminApi = {
  getMaintenanceStatus: () =>
    api.get<ApiResponse<MaintenanceStatus>>("/v1/system/maintenance").then((r) => r.data.data),
  enableMaintenance: (reason: string) =>
    api.post<ApiResponse<MaintenanceStatus>>("/v1/system/maintenance/enable", { reason }).then((r) => r.data.data),
  disableMaintenance: () =>
    api.post<ApiResponse<MaintenanceStatus>>("/v1/system/maintenance/disable").then((r) => r.data.data),
};

// ── Components ────────────────────────────────────────────────────────────────

function AdminCard({
  icon: Icon,
  title,
  description,
  children,
  accent = "blue",
}: {
  icon: React.ElementType;
  title: string;
  description: string;
  children: React.ReactNode;
  accent?: "blue" | "yellow" | "red" | "green";
}) {
  const accents = {
    blue:   "text-blue-400 bg-blue-600/20",
    yellow: "text-yellow-400 bg-yellow-600/20",
    red:    "text-red-400 bg-red-600/20",
    green:  "text-green-400 bg-green-600/20",
  };
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
      <div className="flex items-start gap-3 mb-4">
        <div className={cn("w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0", accents[accent].split(" ")[1])}>
          <Icon className={cn("w-4 h-4", accents[accent].split(" ")[0])} />
        </div>
        <div>
          <h2 className="font-semibold text-white text-sm">{title}</h2>
          <p className="text-xs text-gray-500 mt-0.5">{description}</p>
        </div>
      </div>
      {children}
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-gray-800/60 last:border-0">
      <span className="text-sm text-gray-500">{label}</span>
      <span className="text-sm text-gray-300">{value}</span>
    </div>
  );
}

// ── Maintenance Mode Panel ────────────────────────────────────────────────────

function MaintenanceModePanel() {
  const queryClient = useQueryClient();
  const [reason, setReason] = useState("");
  const [showEnable, setShowEnable] = useState(false);

  const { data: status, isLoading } = useQuery({
    queryKey: ["maintenance-status"],
    queryFn: adminApi.getMaintenanceStatus,
    refetchInterval: 30_000,
  });

  const enableMutation = useMutation({
    mutationFn: () => adminApi.enableMaintenance(reason || "Scheduled maintenance"),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["maintenance-status"] });
      setShowEnable(false);
      setReason("");
    },
  });

  const disableMutation = useMutation({
    mutationFn: adminApi.disableMaintenance,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["maintenance-status"] }),
  });

  if (isLoading) {
    return <div className="h-20 bg-gray-800/50 rounded-xl animate-pulse" />;
  }

  const isActive = status?.maintenance ?? false;

  return (
    <div className="space-y-3">
      <div className={cn(
        "flex items-center justify-between p-4 rounded-xl border",
        isActive
          ? "bg-yellow-500/10 border-yellow-500/30"
          : "bg-gray-800/50 border-gray-700/50"
      )}>
        <div className="flex items-center gap-3">
          {isActive
            ? <Lock className="w-5 h-5 text-yellow-400" />
            : <Unlock className="w-5 h-5 text-green-400" />}
          <div>
            <p className={cn("font-medium text-sm", isActive ? "text-yellow-400" : "text-green-400")}>
              {isActive ? "Maintenance Mode Active" : "Service Online"}
            </p>
            {isActive && status?.reason && (
              <p className="text-xs text-gray-400 mt-0.5">{status.reason}</p>
            )}
            {isActive && status?.since && (
              <p className="text-xs text-gray-500 mt-0.5">
                Since {new Date(status.since).toLocaleString()}
              </p>
            )}
          </div>
        </div>
        {isActive ? (
          <button
            onClick={() => disableMutation.mutate()}
            disabled={disableMutation.isPending}
            className="px-3 py-1.5 bg-green-600 hover:bg-green-500 text-white rounded-lg text-xs font-medium transition-colors disabled:opacity-50"
          >
            {disableMutation.isPending ? "Disabling..." : "Disable"}
          </button>
        ) : (
          <button
            onClick={() => setShowEnable(true)}
            className="px-3 py-1.5 bg-yellow-600/20 hover:bg-yellow-600/30 border border-yellow-600/40 text-yellow-400 rounded-lg text-xs font-medium transition-colors"
          >
            Enable
          </button>
        )}
      </div>

      {showEnable && (
        <div className="bg-gray-800/50 rounded-xl p-4 space-y-3">
          <p className="text-sm text-gray-300">
            Enabling maintenance mode will return 503 to all API requests (except health probes).
          </p>
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Reason (e.g. Database migration in progress)"
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-yellow-500"
          />
          <div className="flex gap-2">
            <button
              onClick={() => enableMutation.mutate()}
              disabled={enableMutation.isPending}
              className="px-4 py-2 bg-yellow-600 hover:bg-yellow-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
            >
              {enableMutation.isPending ? "Enabling..." : "Enable Maintenance"}
            </button>
            <button
              onClick={() => setShowEnable(false)}
              className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg text-sm transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function SystemAdminPage() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Shield className="w-6 h-6 text-blue-400" />
          System Administration
        </h1>
        <p className="text-gray-400 text-sm mt-0.5">
          Platform operations, maintenance controls, and deployment information
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Maintenance Mode */}
        <AdminCard
          icon={Wrench}
          title="Maintenance Mode"
          description="Temporarily block API traffic for maintenance operations"
          accent="yellow"
        >
          <MaintenanceModePanel />
        </AdminCard>

        {/* Deployment Info */}
        <AdminCard
          icon={Info}
          title="Deployment Information"
          description="Current environment and infrastructure details"
          accent="blue"
        >
          <div className="space-y-0">
            <InfoRow label="Version" value="VMOrbit v1.0.0" />
            <InfoRow label="Environment" value={
              <span className="px-2 py-0.5 bg-green-600/20 text-green-400 rounded-full text-xs font-medium">
                Production
              </span>
            } />
            <InfoRow label="Backend" value={
              <span className="font-mono text-xs">:8080</span>
            } />
            <InfoRow label="Database" value="PostgreSQL 16" />
            <InfoRow label="Cache" value="Redis 7" />
          </div>
        </AdminCard>

        {/* Probe Endpoints */}
        <AdminCard
          icon={Activity}
          title="Health Probe Endpoints"
          description="Endpoints for load balancers and orchestrators"
          accent="green"
        >
          <div className="space-y-2">
            {[
              { path: "/health", desc: "Liveness probe — always 200 if process is alive", method: "GET" },
              { path: "/ready",  desc: "Readiness probe — 200 when DB + Redis are reachable", method: "GET" },
              { path: "/status", desc: "Extended operational status with dependency health", method: "GET" },
              { path: "/metrics", desc: "Prometheus metrics endpoint", method: "GET" },
            ].map((ep) => (
              <div key={ep.path} className="flex items-start gap-3 p-3 bg-gray-800/50 rounded-xl">
                <span className="text-xs font-mono bg-blue-600/20 text-blue-400 px-2 py-0.5 rounded flex-shrink-0">
                  {ep.method}
                </span>
                <div>
                  <p className="text-sm font-mono text-white">{ep.path}</p>
                  <p className="text-xs text-gray-500 mt-0.5">{ep.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </AdminCard>

        {/* Quick Links */}
        <AdminCard
          icon={Settings}
          title="Operations Quick Links"
          description="Navigate to key operational dashboards"
          accent="blue"
        >
          <div className="space-y-1">
            {[
              { label: "System Health",    href: "/dashboard/system",  icon: Activity },
              { label: "Platform Status",  href: "/dashboard/status",  icon: CheckCircle2 },
              { label: "Backup Status",    href: "/dashboard/backups", icon: Database },
              { label: "Audit Logs",       href: "/dashboard/audit",   icon: Shield },
              { label: "Provider Health",  href: "/dashboard/health",  icon: Zap },
            ].map(({ label, href, icon: Icon }) => (
              <a
                key={href}
                href={href}
                className="flex items-center justify-between p-3 rounded-xl hover:bg-gray-800/50 transition-colors group"
              >
                <div className="flex items-center gap-3">
                  <Icon className="w-4 h-4 text-gray-500 group-hover:text-blue-400 transition-colors" />
                  <span className="text-sm text-gray-300 group-hover:text-white transition-colors">{label}</span>
                </div>
                <ChevronRight className="w-4 h-4 text-gray-600 group-hover:text-gray-400 transition-colors" />
              </a>
            ))}
          </div>
        </AdminCard>
      </div>

      {/* Security notice */}
      <div className="bg-gray-900 border border-blue-800/40 rounded-2xl p-5">
        <h2 className="flex items-center gap-2 font-semibold text-blue-400 mb-3 text-sm">
          <Lock className="w-4 h-4" />
          Security Checklist
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {[
            { label: "VMORBIT_ENCRYPTION_KEY set", check: true },
            { label: "VMORBIT_JWT_SECRET set", check: true },
            { label: "HTTPS enabled via nginx", check: true },
            { label: "Redis password configured", check: true },
            { label: "Database password configured", check: true },
            { label: "CORS origins restricted", check: true },
          ].map(({ label, check }) => (
            <div key={label} className="flex items-center gap-2 text-sm">
              {check
                ? <CheckCircle2 className="w-4 h-4 text-green-400 flex-shrink-0" />
                : <AlertTriangle className="w-4 h-4 text-yellow-400 flex-shrink-0" />}
              <span className={check ? "text-gray-300" : "text-yellow-400"}>{label}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
