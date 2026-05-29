"use client";
import { useEffect, useState, useCallback } from "react";
import { RefreshCw, CheckCircle2, XCircle, Loader2, ChevronDown, ChevronUp } from "lucide-react";
import { wsClient } from "@/lib/ws/WSClient";
import { cn } from "@/lib/utils";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface SyncEvent {
  hypervisor_id: string;
  hypervisor_name?: string;
  provider?: string;
  vms_updated?: number;
  vms_removed?: number;
  stores_upserted?: number;
  networks_upserted?: number;
  synced_at?: string;
  errors?: string[];
}

export interface SyncProgress {
  taskId: string;
  hypervisorId: string;
  hypervisorName?: string;
  status: "pending" | "running" | "completed" | "failed";
  progress: number;
  message?: string;
  result?: SyncEvent;
  startedAt: Date;
}

// ── Hook ──────────────────────────────────────────────────────────────────────

/**
 * useSyncProgress subscribes to WebSocket events and tracks active inventory
 * sync jobs. Returns the list of recent syncs and a function to start tracking
 * a new sync by task ID.
 */
export function useSyncProgress() {
  const [syncs, setSyncs] = useState<Map<string, SyncProgress>>(new Map());

  const trackSync = useCallback((taskId: string, hypervisorId: string, hypervisorName?: string) => {
    setSyncs((prev) => {
      const next = new Map(prev);
      next.set(taskId, {
        taskId,
        hypervisorId,
        hypervisorName,
        status: "pending",
        progress: 0,
        startedAt: new Date(),
      });
      return next;
    });
  }, []);

  useEffect(() => {
    // Subscribe to task progress events for sync tasks
    const unsubTasks = wsClient.subscribe("tasks", (msg) => {
      const payload = msg.payload as Record<string, unknown>;
      const taskId = payload?.task_id as string;
      const type = msg.type as string;

      if (!taskId) return;

      setSyncs((prev) => {
        const sync = prev.get(taskId);
        if (!sync) return prev; // not a tracked sync

        const next = new Map(prev);

        if (type === "task.progress") {
          next.set(taskId, {
            ...sync,
            status: "running",
            progress: (payload.progress as number) ?? sync.progress,
            message: (payload.message as string) ?? sync.message,
          });
        } else if (type === "task.status_changed") {
          const status = payload.status as string;
          if (status === "completed") {
            next.set(taskId, { ...sync, status: "completed", progress: 100 });
          } else if (status === "failed") {
            next.set(taskId, { ...sync, status: "failed" });
          } else if (status === "running") {
            next.set(taskId, { ...sync, status: "running" });
          }
        }

        return next;
      });
    });

    // Subscribe to inventory.synced events to capture final result
    const unsubInventory = wsClient.subscribe("inventory", (msg) => {
      if (msg.type !== "inventory.synced") return;
      const payload = msg.payload as SyncEvent;

      setSyncs((prev) => {
        // Find the sync for this hypervisor
        const next = new Map(prev);
        for (const [taskId, sync] of next) {
          if (sync.hypervisorId === payload.hypervisor_id && sync.status !== "failed") {
            next.set(taskId, {
              ...sync,
              status: "completed",
              progress: 100,
              result: payload,
            });
          }
        }
        return next;
      });
    });

    return () => {
      unsubTasks();
      unsubInventory();
    };
  }, []);

  // Auto-remove completed/failed syncs after 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      const cutoff = Date.now() - 30_000;
      setSyncs((prev) => {
        let changed = false;
        const next = new Map(prev);
        for (const [id, sync] of next) {
          if (
            (sync.status === "completed" || sync.status === "failed") &&
            sync.startedAt.getTime() < cutoff
          ) {
            next.delete(id);
            changed = true;
          }
        }
        return changed ? next : prev;
      });
    }, 5_000);
    return () => clearInterval(interval);
  }, []);

  return { syncs, trackSync };
}

// ── Component ─────────────────────────────────────────────────────────────────

interface SyncProgressPanelProps {
  syncs: Map<string, SyncProgress>;
  className?: string;
}

/**
 * SyncProgressPanel renders a compact list of active and recently completed
 * inventory sync jobs with real-time progress bars.
 */
export function SyncProgressPanel({ syncs, className }: SyncProgressPanelProps) {
  const [collapsed, setCollapsed] = useState(false);
  const list = Array.from(syncs.values()).sort(
    (a, b) => b.startedAt.getTime() - a.startedAt.getTime()
  );

  if (list.length === 0) return null;

  const activeCount = list.filter((s) => s.status === "running" || s.status === "pending").length;

  return (
    <div className={cn("bg-gray-900 border border-gray-800 rounded-xl overflow-hidden", className)}>
      {/* Header */}
      <button
        onClick={() => setCollapsed((c) => !c)}
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-800/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          {activeCount > 0 ? (
            <Loader2 className="w-4 h-4 text-blue-400 animate-spin" />
          ) : (
            <RefreshCw className="w-4 h-4 text-gray-400" />
          )}
          <span className="text-sm font-medium text-white">
            Inventory Sync
            {activeCount > 0 && (
              <span className="ml-2 text-xs text-blue-400">({activeCount} active)</span>
            )}
          </span>
        </div>
        {collapsed ? (
          <ChevronDown className="w-4 h-4 text-gray-500" />
        ) : (
          <ChevronUp className="w-4 h-4 text-gray-500" />
        )}
      </button>

      {/* Sync list */}
      {!collapsed && (
        <div className="divide-y divide-gray-800">
          {list.map((sync) => (
            <SyncItem key={sync.taskId} sync={sync} />
          ))}
        </div>
      )}
    </div>
  );
}

function SyncItem({ sync }: { sync: SyncProgress }) {
  const isActive = sync.status === "running" || sync.status === "pending";

  return (
    <div className="px-4 py-3 space-y-2">
      {/* Row 1: name + status icon */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          {sync.status === "completed" ? (
            <CheckCircle2 className="w-3.5 h-3.5 text-green-400 shrink-0" />
          ) : sync.status === "failed" ? (
            <XCircle className="w-3.5 h-3.5 text-red-400 shrink-0" />
          ) : (
            <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin shrink-0" />
          )}
          <span className="text-sm text-white truncate">
            {sync.hypervisorName ?? sync.hypervisorId.slice(0, 8)}
          </span>
        </div>
        <span
          className={cn("text-xs px-1.5 py-0.5 rounded-full font-medium", {
            "bg-blue-500/20 text-blue-400": isActive,
            "bg-green-500/20 text-green-400": sync.status === "completed",
            "bg-red-500/20 text-red-400": sync.status === "failed",
          })}
        >
          {sync.status}
        </span>
      </div>

      {/* Progress bar */}
      {(isActive || sync.status === "completed") && (
        <div>
          <div className="h-1 bg-gray-800 rounded-full overflow-hidden">
            <div
              className={cn("h-full rounded-full transition-all duration-500", {
                "bg-blue-500": isActive,
                "bg-green-500": sync.status === "completed",
              })}
              style={{ width: `${sync.progress}%` }}
            />
          </div>
          {sync.message && (
            <p className="text-xs text-gray-500 mt-1 truncate">{sync.message}</p>
          )}
        </div>
      )}

      {/* Result summary */}
      {sync.status === "completed" && sync.result && (
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-gray-400">
          {sync.result.vms_updated !== undefined && (
            <span>{sync.result.vms_updated} VMs synced</span>
          )}
          {sync.result.vms_removed !== undefined && sync.result.vms_removed > 0 && (
            <span>{sync.result.vms_removed} removed</span>
          )}
          {sync.result.stores_upserted !== undefined && sync.result.stores_upserted > 0 && (
            <span>{sync.result.stores_upserted} datastores</span>
          )}
          {sync.result.networks_upserted !== undefined && sync.result.networks_upserted > 0 && (
            <span>{sync.result.networks_upserted} networks</span>
          )}
        </div>
      )}

      {/* Errors */}
      {sync.result?.errors && sync.result.errors.length > 0 && (
        <div className="text-xs text-red-400 bg-red-500/10 rounded px-2 py-1">
          {sync.result.errors[0]}
          {sync.result.errors.length > 1 && ` (+${sync.result.errors.length - 1} more)`}
        </div>
      )}
    </div>
  );
}
