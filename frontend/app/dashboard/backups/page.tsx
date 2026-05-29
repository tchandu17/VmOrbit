"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Database, Download, RefreshCw, CheckCircle2, AlertTriangle,
  Clock, HardDrive, Trash2, RotateCcw, Plus, Shield,
} from "lucide-react";
import { cn } from "@/lib/utils";
import api from "@/lib/api/client";
import type { ApiResponse } from "@/types";

// ── Types ─────────────────────────────────────────────────────────────────────

interface BackupEntry {
  id: string;
  filename: string;
  size_bytes: number;
  created_at: string;
  label?: string;
  status: "ok" | "corrupt" | "unknown";
}

interface BackupStatus {
  last_backup_at: string | null;
  next_backup_at: string | null;
  backup_count: number;
  retention_days: number;
  total_size_bytes: number;
  backups: BackupEntry[];
}

// ── API ───────────────────────────────────────────────────────────────────────

const backupApi = {
  getStatus: () =>
    api.get<ApiResponse<BackupStatus>>("/v1/system/backups").then((r) => r.data.data),
  triggerBackup: (label?: string) =>
    api.post<ApiResponse<{ message: string }>>("/v1/system/backups/trigger", { label }).then((r) => r.data),
};

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  const hours = Math.floor(mins / 60);
  const days = Math.floor(hours / 24);
  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (mins > 0) return `${mins}m ago`;
  return "just now";
}

// ── Components ────────────────────────────────────────────────────────────────

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  accent = "blue",
}: {
  icon: React.ElementType;
  label: string;
  value: string | number;
  sub?: string;
  accent?: "blue" | "green" | "yellow" | "red";
}) {
  const accents = {
    blue: "bg-blue-600",
    green: "bg-green-600",
    yellow: "bg-yellow-600",
    red: "bg-red-600",
  };
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-gray-400">{label}</span>
        <div className={cn("w-8 h-8 rounded-xl flex items-center justify-center", accents[accent])}>
          <Icon className="w-4 h-4 text-white" />
        </div>
      </div>
      <p className="text-2xl font-bold text-white">{value}</p>
      {sub && <p className="text-xs text-gray-500 mt-1">{sub}</p>}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function BackupsPage() {
  const queryClient = useQueryClient();
  const [triggerLabel, setTriggerLabel] = useState("");
  const [showTrigger, setShowTrigger] = useState(false);

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["backup-status"],
    queryFn: backupApi.getStatus,
    refetchInterval: 60_000,
    staleTime: 30_000,
    retry: false,
  });

  const triggerMutation = useMutation({
    mutationFn: () => backupApi.triggerBackup(triggerLabel || undefined),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["backup-status"] });
      setShowTrigger(false);
      setTriggerLabel("");
    },
  });

  const backups = data?.backups ?? [];
  const totalSize = data?.total_size_bytes ?? 0;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Database className="w-6 h-6 text-blue-400" />
            Backup Status
          </h1>
          <p className="text-gray-400 text-sm mt-0.5">
            PostgreSQL backup management and restore procedures
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors disabled:opacity-50"
          >
            <RefreshCw className={cn("w-4 h-4", isFetching && "animate-spin")} />
            Refresh
          </button>
          <button
            onClick={() => setShowTrigger(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
          >
            <Plus className="w-4 h-4" />
            Trigger Backup
          </button>
        </div>
      </div>

      {/* Trigger backup dialog */}
      {showTrigger && (
        <div className="bg-gray-900 border border-blue-500/30 rounded-2xl p-5">
          <h2 className="font-semibold text-white mb-3">Trigger Manual Backup</h2>
          <div className="flex items-center gap-3">
            <input
              value={triggerLabel}
              onChange={(e) => setTriggerLabel(e.target.value)}
              placeholder="Optional label (e.g. pre-upgrade)"
              className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              onClick={() => triggerMutation.mutate()}
              disabled={triggerMutation.isPending}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
            >
              {triggerMutation.isPending ? "Running..." : "Run Backup"}
            </button>
            <button
              onClick={() => setShowTrigger(false)}
              className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
            >
              Cancel
            </button>
          </div>
          {triggerMutation.isError && (
            <p className="text-red-400 text-sm mt-2">
              Backup trigger failed. Check that the backup service is running.
            </p>
          )}
        </div>
      )}

      {/* Stat cards */}
      {isLoading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 animate-pulse h-28" />
          ))}
        </div>
      ) : data ? (
        <>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              icon={Database}
              label="Total Backups"
              value={data.backup_count}
              sub={`${data.retention_days} day retention`}
              accent="blue"
            />
            <StatCard
              icon={HardDrive}
              label="Total Size"
              value={formatBytes(totalSize)}
              sub="Compressed SQL dumps"
              accent="green"
            />
            <StatCard
              icon={Clock}
              label="Last Backup"
              value={data.last_backup_at ? timeAgo(data.last_backup_at) : "Never"}
              sub={data.last_backup_at ? new Date(data.last_backup_at).toLocaleString() : "No backups yet"}
              accent={data.last_backup_at ? "green" : "yellow"}
            />
            <StatCard
              icon={Shield}
              label="Next Backup"
              value={data.next_backup_at ? timeAgo(data.next_backup_at) : "Unknown"}
              sub="Daily at 02:00 UTC"
              accent="blue"
            />
          </div>

          {/* Backup list */}
          <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
              <h2 className="font-semibold text-white text-sm">Backup History</h2>
              <span className="text-xs text-gray-500">{backups.length} backup(s)</span>
            </div>

            {backups.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16 text-center">
                <Database className="w-10 h-10 text-gray-700 mb-3" />
                <p className="text-gray-500 text-sm">No backups found</p>
                <p className="text-gray-600 text-xs mt-1">
                  Backups run daily at 02:00 UTC or can be triggered manually
                </p>
              </div>
            ) : (
              <div className="divide-y divide-gray-800">
                {backups.map((backup) => (
                  <div key={backup.id} className="flex items-center gap-4 px-5 py-4 hover:bg-gray-800/30 transition-colors">
                    <div className={cn(
                      "w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0",
                      backup.status === "ok" ? "bg-green-600/20" : "bg-red-600/20"
                    )}>
                      {backup.status === "ok"
                        ? <CheckCircle2 className="w-4 h-4 text-green-400" />
                        : <AlertTriangle className="w-4 h-4 text-red-400" />}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-white truncate font-mono">
                        {backup.filename}
                      </p>
                      <div className="flex items-center gap-3 mt-0.5">
                        <span className="text-xs text-gray-500">
                          {new Date(backup.created_at).toLocaleString()}
                        </span>
                        {backup.label && (
                          <span className="text-xs bg-blue-600/20 text-blue-400 px-2 py-0.5 rounded-full">
                            {backup.label}
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="text-right flex-shrink-0">
                      <p className="text-sm text-gray-300 font-mono">{formatBytes(backup.size_bytes)}</p>
                      <p className="text-xs text-gray-600 mt-0.5">{timeAgo(backup.created_at)}</p>
                    </div>
                    <div className="flex items-center gap-1 flex-shrink-0">
                      <button
                        title="Download backup"
                        className="p-2 text-gray-500 hover:text-blue-400 hover:bg-blue-400/10 rounded-lg transition-colors"
                      >
                        <Download className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Restore instructions */}
          <div className="bg-gray-900 border border-yellow-800/40 rounded-2xl p-5">
            <h2 className="flex items-center gap-2 font-semibold text-yellow-400 mb-3 text-sm">
              <RotateCcw className="w-4 h-4" />
              Restore Procedure
            </h2>
            <div className="space-y-2 text-sm text-gray-400">
              <p>To restore from a backup, run the following on the server:</p>
              <pre className="bg-gray-800 rounded-lg p-3 text-xs font-mono text-gray-300 overflow-x-auto">
{`# Stop the backend service first
docker compose -f docker-compose.production.yml stop backend

# Restore from a specific backup file
./scripts/backup.sh --restore ./backups/vmorbit_YYYYMMDD_HHMMSS.sql.gz

# Restart the backend
docker compose -f docker-compose.production.yml start backend`}
              </pre>
              <p className="text-yellow-400/80 text-xs">
                ⚠️ Restore will drop and recreate the database. Ensure all services are stopped before restoring.
              </p>
            </div>
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <AlertTriangle className="w-12 h-12 text-yellow-600 mb-4" />
          <p className="text-gray-400">Backup status unavailable</p>
          <p className="text-gray-600 text-sm mt-1">
            The backup API endpoint is not yet configured. See the deployment guide.
          </p>
        </div>
      )}
    </div>
  );
}
