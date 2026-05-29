"use client";
import { X, CheckCircle, XCircle, Loader2, Clock, Ban, Activity } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { useTaskStore } from "@/store/useTaskStore";
import { cn } from "@/lib/utils";
import type { Task } from "@/types";

function StatusIcon({ status }: { status: Task["status"] }) {
  if (status === "completed") return <CheckCircle className="w-4 h-4 text-green-400 shrink-0" />;
  if (status === "failed" || status === "timed_out") return <XCircle className="w-4 h-4 text-red-400 shrink-0" />;
  if (status === "running") return <Loader2 className="w-4 h-4 text-blue-400 animate-spin shrink-0" />;
  if (status === "cancelled") return <Ban className="w-4 h-4 text-gray-500 shrink-0" />;
  return <Clock className="w-4 h-4 text-gray-400 shrink-0" />;
}

function progressBarColor(status: Task["status"]) {
  if (status === "completed") return "bg-green-500";
  if (status === "failed" || status === "timed_out") return "bg-red-500";
  if (status === "retrying") return "bg-yellow-500";
  return "bg-blue-500";
}

const statusBadge: Record<string, string> = {
  pending:   "bg-gray-700 text-gray-300",
  queued:    "bg-blue-900/50 text-blue-300",
  running:   "bg-blue-600/20 text-blue-400",
  completed: "bg-green-900/50 text-green-300",
  failed:    "bg-red-900/50 text-red-300",
  cancelled: "bg-gray-700 text-gray-400",
  retrying:  "bg-yellow-900/50 text-yellow-300",
  timed_out: "bg-orange-900/50 text-orange-300",
};

export function TaskDrawer() {
  const taskDrawerOpen = useUIStore((s) => s.taskDrawerOpen);
  const closeTaskDrawer = useUIStore((s) => s.closeTaskDrawer);
  const activeTasks = useTaskStore((s) => s.activeTasks);

  // Show active (non-terminal) tasks first, then recently finished
  const taskList = Array.from(activeTasks.values()).sort((a, b) => {
    const terminalA = ["completed", "failed", "cancelled", "timed_out"].includes(a.status);
    const terminalB = ["completed", "failed", "cancelled", "timed_out"].includes(b.status);
    if (terminalA !== terminalB) return terminalA ? 1 : -1;
    return new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime();
  });

  const activeCount = taskList.filter(
    (t) => !["completed", "failed", "cancelled", "timed_out"].includes(t.status)
  ).length;

  if (!taskDrawerOpen) return null;

  return (
    <>
      <div className="fixed inset-0 bg-black/40 z-40" onClick={closeTaskDrawer} />
      <div className="fixed right-0 top-0 h-full w-96 bg-gray-900 border-l border-gray-800 z-50 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-4 border-b border-gray-800 shrink-0">
          <div className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-blue-400" />
            <h2 className="font-semibold text-white">Active Tasks</h2>
            {activeCount > 0 && (
              <span className="text-xs bg-blue-600/30 text-blue-300 px-2 py-0.5 rounded-full">
                {activeCount} running
              </span>
            )}
          </div>
          <button
            onClick={closeTaskDrawer}
            className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Task list */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {taskList.length === 0 ? (
            <div className="text-center py-12 text-gray-500 text-sm">No tasks yet</div>
          ) : (
            taskList.map((task) => (
              <div key={task.id} className="bg-gray-800 rounded-xl p-4 space-y-3">
                {/* Title row */}
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 min-w-0">
                    <StatusIcon status={task.status} />
                    <span className="text-sm font-medium text-white truncate">
                      {task.type.replace(/\./g, " › ")}
                    </span>
                  </div>
                  <span
                    className={cn(
                      "text-xs px-2 py-0.5 rounded-full shrink-0",
                      statusBadge[task.status] ?? "bg-gray-700 text-gray-300"
                    )}
                  >
                    {task.status}
                  </span>
                </div>

                {/* Progress bar — show for non-terminal states */}
                {!["completed", "failed", "cancelled", "timed_out"].includes(task.status) && (
                  <div>
                    <div className="flex justify-between text-xs text-gray-400 mb-1">
                      <span>Progress</span>
                      <span>{task.progress ?? 0}%</span>
                    </div>
                    <div className="h-1.5 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={cn(
                          "h-full rounded-full transition-all duration-500",
                          progressBarColor(task.status)
                        )}
                        style={{ width: `${task.progress ?? 0}%` }}
                      />
                    </div>
                  </div>
                )}

                {/* Completed progress bar */}
                {task.status === "completed" && (
                  <div className="h-1.5 bg-green-900/40 rounded-full overflow-hidden">
                    <div className="h-full w-full bg-green-500 rounded-full" />
                  </div>
                )}

                {/* Error message */}
                {task.error_message && (
                  <p className="text-xs text-red-400 bg-red-500/10 rounded-lg px-3 py-2 break-words">
                    {task.error_message}
                  </p>
                )}

                <p className="text-xs text-gray-600 font-mono">
                  {task.id?.slice(0, 8)}…
                </p>
              </div>
            ))
          )}
        </div>

        {/* Footer hint */}
        <div className="px-4 py-3 border-t border-gray-800 shrink-0">
          <p className="text-xs text-gray-600 text-center">
            View full history in{" "}
            <a href="/dashboard/tasks" className="text-blue-400 hover:underline" onClick={closeTaskDrawer}>
              Tasks
            </a>
          </p>
        </div>
      </div>
    </>
  );
}
