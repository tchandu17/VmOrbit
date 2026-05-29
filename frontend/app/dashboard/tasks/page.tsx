"use client";
import { useState, useEffect, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { taskApi } from "@/lib/api/tasks";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { wsClient } from "@/lib/ws/WSClient";
import { cn, relativeTime } from "@/lib/utils";
import type { Task, TaskLog, Hypervisor } from "@/types";
import { X, ChevronRight, Loader2, CheckCircle, XCircle, Clock, Ban, Cpu } from "lucide-react";

// ── Status helpers ────────────────────────────────────────────────────────────

const statusColors: Record<string, string> = {
  pending:   "bg-gray-700 text-gray-300",
  queued:    "bg-blue-900/50 text-blue-300",
  running:   "bg-blue-600/20 text-blue-400",
  completed: "bg-green-900/50 text-green-300",
  failed:    "bg-red-900/50 text-red-300",
  cancelled: "bg-gray-700 text-gray-400",
  retrying:  "bg-yellow-900/50 text-yellow-300",
  timed_out: "bg-orange-900/50 text-orange-300",
};

const logLevelColors: Record<string, string> = {
  debug: "text-gray-500",
  info:  "text-gray-300",
  warn:  "text-yellow-400",
  error: "text-red-400",
};

function StatusIcon({ status }: { status: Task["status"] }) {
  if (status === "completed") return <CheckCircle className="w-3.5 h-3.5 text-green-400 shrink-0" />;
  if (status === "failed" || status === "timed_out") return <XCircle className="w-3.5 h-3.5 text-red-400 shrink-0" />;
  if (status === "running") return <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin shrink-0" />;
  if (status === "cancelled") return <Ban className="w-3.5 h-3.5 text-gray-500 shrink-0" />;
  return <Clock className="w-3.5 h-3.5 text-gray-500 shrink-0" />;
}

function duration(task: Task): string {
  if (task.started_at && task.completed_at) {
    const ms = new Date(task.completed_at).getTime() - new Date(task.started_at).getTime();
    return `${Math.round(ms / 1000)}s`;
  }
  if (task.started_at) return "running…";
  return "—";
}

// ── Task detail panel ─────────────────────────────────────────────────────────

function TaskDetail({
  task,
  hvMap,
  onClose,
  onCancel,
}: {
  task: Task;
  hvMap: Record<string, string>;
  onClose: () => void;
  onCancel: (id: string) => void;
}) {
  const [logPage, setLogPage] = useState(1);
  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ["task-logs", task.id, logPage],
    queryFn: () => taskApi.getLogs(task.id, { page: logPage, page_size: 50 }),
    refetchInterval: task.status === "running" ? 2000 : false,
  });

  const logs: TaskLog[] = logsData?.data ?? [];
  const canCancel = ["pending", "queued", "running", "retrying"].includes(task.status);

  return (
    <div className="fixed inset-0 z-50 flex">
      <div className="flex-1 bg-black/50" onClick={onClose} />
      <div className="w-[520px] bg-gray-900 border-l border-gray-800 flex flex-col h-full">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-800 shrink-0">
          <div className="flex items-center gap-2 min-w-0">
            <StatusIcon status={task.status} />
            <span className="font-semibold text-white truncate">{task.type.replace(/\./g, " › ")}</span>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {canCancel && (
              <button
                onClick={() => onCancel(task.id)}
                className="px-3 py-1.5 text-xs bg-red-900/40 hover:bg-red-900/70 text-red-400 rounded-lg transition-colors"
              >
                Cancel
              </button>
            )}
            <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Meta */}
        <div className="px-5 py-4 border-b border-gray-800 shrink-0 space-y-3">
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <p className="text-gray-500 mb-0.5">Status</p>
              <span className={cn("px-2 py-0.5 rounded-full font-medium", statusColors[task.status])}>
                {task.status}
              </span>
            </div>
            <div>
              <p className="text-gray-500 mb-0.5">Priority</p>
              <p className="text-gray-300">{task.priority}</p>
            </div>
            {task.hypervisor_id && hvMap[task.hypervisor_id] && (
              <div className="col-span-2">
                <p className="text-gray-500 mb-0.5">Hypervisor</p>
                <p className="text-blue-400 flex items-center gap-1">
                  <Cpu className="w-3 h-3" />{hvMap[task.hypervisor_id]}
                </p>
              </div>
            )}
            <div>
              <p className="text-gray-500 mb-0.5">Created</p>
              <p className="text-gray-300">{relativeTime(task.created_at)}</p>
            </div>
            <div>
              <p className="text-gray-500 mb-0.5">Duration</p>
              <p className="text-gray-300">{duration(task)}</p>
            </div>
            {task.retry_count > 0 && (
              <div>
                <p className="text-gray-500 mb-0.5">Retries</p>
                <p className="text-gray-300">{task.retry_count} / {task.max_retries}</p>
              </div>
            )}
          </div>

          {/* Progress bar */}
          {["running", "queued", "retrying"].includes(task.status) && (
            <div>
              <div className="flex justify-between text-xs text-gray-400 mb-1">
                <span>Progress</span>
                <span>{task.progress}%</span>
              </div>
              <div className="h-1.5 bg-gray-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-500 rounded-full transition-all duration-500"
                  style={{ width: `${task.progress}%` }}
                />
              </div>
            </div>
          )}

          {task.error_message && (
            <div className="text-xs text-red-400 bg-red-500/10 rounded-lg px-3 py-2">
              {task.error_message}
            </div>
          )}

          <p className="text-xs text-gray-600 font-mono">ID: {task.id}</p>
        </div>

        {/* Logs */}
        <div className="flex-1 overflow-y-auto px-5 py-4">
          <p className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-3">Logs</p>
          {logsLoading ? (
            <div className="flex items-center gap-2 text-gray-500 text-xs">
              <Loader2 className="w-3 h-3 animate-spin" /> Loading logs…
            </div>
          ) : logs.length === 0 ? (
            <p className="text-xs text-gray-600">No logs yet.</p>
          ) : (
            <div className="space-y-1 font-mono text-xs">
              {logs.map((log) => (
                <div key={log.id} className="flex gap-2">
                  <span className="text-gray-600 shrink-0 tabular-nums">
                    {new Date(log.created_at).toLocaleTimeString()}
                  </span>
                  <span className={cn("shrink-0 uppercase w-10", logLevelColors[log.level])}>
                    {log.level}
                  </span>
                  <span className="text-gray-300 break-all">{log.message}</span>
                </div>
              ))}
            </div>
          )}

          {/* Log pagination */}
          {logsData?.meta && logsData.meta.total_pages > 1 && (
            <div className="flex gap-2 mt-4">
              <button
                disabled={logPage === 1}
                onClick={() => setLogPage((p) => p - 1)}
                className="px-3 py-1 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300"
              >
                Prev
              </button>
              <span className="text-xs text-gray-500 self-center">
                {logPage} / {logsData.meta.total_pages}
              </span>
              <button
                disabled={logPage === logsData.meta.total_pages}
                onClick={() => setLogPage((p) => p + 1)}
                className="px-3 py-1 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300"
              >
                Next
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function TasksPage() {
  const [page, setPage] = useState(1);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [cancellingId, setCancellingId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["tasks", { page }],
    queryFn: () => taskApi.list({ page, page_size: 25 }),
    refetchInterval: 10_000,
  });

  // Fetch all hypervisors once to build a name lookup map
  const { data: hvData } = useQuery({
    queryKey: ["hypervisors", { page_size: 100 }],
    queryFn: () => hypervisorApi.list({ page_size: 100 }),
    staleTime: 60_000,
  });
  const hvMap: Record<string, string> = {};
  (hvData?.data ?? []).forEach((h: Hypervisor) => { hvMap[h.id] = h.name; });

  // Merge live WS updates into the query cache so the table updates instantly
  const handleWSMessage = useCallback(
    (msg: { type: string; payload: unknown }) => {
      const payload = msg.payload as Record<string, unknown>;
      const taskId = (payload?.task_id ?? payload?.id) as string | undefined;
      if (!taskId) return;

      // Patch the task in the list cache
      queryClient.setQueryData(
        ["tasks", { page }],
        (old: typeof data) => {
          if (!old) return old;
          return {
            ...old,
            data: old.data.map((t: Task) =>
              t.id === taskId
                ? {
                    ...t,
                    status: (payload.status as Task["status"]) ?? t.status,
                    progress: (payload.progress as number) ?? t.progress,
                    error_message: (payload.error_message as string) ?? t.error_message,
                  }
                : t
            ),
          };
        }
      );

      // Also patch the selected task detail if it's open
      setSelectedTask((prev) => {
        if (!prev || prev.id !== taskId) return prev;
        return {
          ...prev,
          status: (payload.status as Task["status"]) ?? prev.status,
          progress: (payload.progress as number) ?? prev.progress,
          error_message: (payload.error_message as string) ?? prev.error_message,
        };
      });

      // Invalidate logs cache for running tasks so they auto-refresh
      if (msg.type === "task.log_appended") {
        queryClient.invalidateQueries({ queryKey: ["task-logs", taskId] });
      }

      // On terminal state, do a full refetch to get accurate completed_at etc.
      const terminal = ["completed", "failed", "cancelled", "timed_out"];
      if (payload.status && terminal.includes(payload.status as string)) {
        queryClient.invalidateQueries({ queryKey: ["tasks"] });
      }
    },
    [page, queryClient]
  );

  useEffect(() => {
    const unsub = wsClient.subscribe("tasks", handleWSMessage);
    return unsub;
  }, [handleWSMessage]);

  async function handleCancel(id: string) {
    setCancellingId(id);
    try {
      await taskApi.cancel(id);
      // Optimistically update status
      queryClient.setQueryData(
        ["tasks", { page }],
        (old: typeof data) => {
          if (!old) return old;
          return {
            ...old,
            data: old.data.map((t: Task) =>
              t.id === id ? { ...t, status: "cancelled" as Task["status"] } : t
            ),
          };
        }
      );
      setSelectedTask((prev) =>
        prev?.id === id ? { ...prev, status: "cancelled" as Task["status"] } : prev
      );
    } catch {
      // ignore — WS will correct the state
    } finally {
      setCancellingId(null);
    }
  }

  const tasks: Task[] = data?.data ?? [];

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold text-white">Tasks</h1>
        <p className="text-gray-400 text-sm mt-0.5">
          {data?.meta?.total_items ?? 0} total · real-time via WebSocket
        </p>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Type", "Status", "Progress", "Created", "Duration", ""].map((h, i) => (
                <th
                  key={i}
                  className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b border-gray-800/50">
                  {Array.from({ length: 6 }).map((_, j) => (
                    <td key={j} className="px-4 py-3">
                      <div className="h-4 bg-gray-800 rounded animate-pulse w-24" />
                    </td>
                  ))}
                </tr>
              ))
            ) : tasks.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                  No tasks yet
                </td>
              </tr>
            ) : (
              tasks.map((task: Task) => {
                const canCancel = ["pending", "queued", "running", "retrying"].includes(task.status);
                return (
                  <tr
                    key={task.id}
                    className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors cursor-pointer"
                    onClick={() => setSelectedTask(task)}
                  >
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <StatusIcon status={task.status} />
                        <div>
                          <p className="text-white font-medium">
                            {task.type.replace(/\./g, " › ")}
                          </p>
                          {task.hypervisor_id && hvMap[task.hypervisor_id] ? (
                            <p className="text-xs text-blue-400/80 flex items-center gap-1 mt-0.5">
                              <Cpu className="w-3 h-3" />{hvMap[task.hypervisor_id]}
                            </p>
                          ) : (
                            <p className="text-xs text-gray-500">{task.id.slice(0, 8)}…</p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={cn(
                          "text-xs px-2 py-1 rounded-full font-medium",
                          statusColors[task.status] ?? "bg-gray-700 text-gray-300"
                        )}
                      >
                        {task.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="w-20 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-blue-500 rounded-full transition-all duration-500"
                            style={{ width: `${task.progress}%` }}
                          />
                        </div>
                        <span className="text-xs text-gray-400">{task.progress}%</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-xs">
                      {relativeTime(task.created_at)}
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-xs">{duration(task)}</td>
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      <div className="flex items-center gap-1">
                        {canCancel && (
                          <button
                            disabled={cancellingId === task.id}
                            onClick={() => handleCancel(task.id)}
                            className="px-2 py-1 text-xs bg-red-900/30 hover:bg-red-900/60 text-red-400 rounded-lg transition-colors disabled:opacity-40"
                          >
                            {cancellingId === task.id ? "…" : "Cancel"}
                          </button>
                        )}
                        <button
                          onClick={() => setSelectedTask(task)}
                          className="p-1 text-gray-500 hover:text-gray-300 rounded"
                        >
                          <ChevronRight className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>

        {data && (data.meta?.total_pages ?? 0) > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">
              Page {page} of {data.meta?.total_pages}
            </span>
            <div className="flex gap-2">
              <button
                disabled={page === 1}
                onClick={() => setPage((p) => p - 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300"
              >
                Previous
              </button>
              <button
                disabled={page === (data.meta?.total_pages ?? 1)}
                onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Task detail panel */}
      {selectedTask && (
        <TaskDetail
          task={selectedTask}
          hvMap={hvMap}
          onClose={() => setSelectedTask(null)}
          onCancel={handleCancel}
        />
      )}
    </div>
  );
}
