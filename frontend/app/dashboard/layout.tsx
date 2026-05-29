"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { TopBar } from "@/components/layout/TopBar";
import { TaskDrawer } from "@/components/tasks/TaskDrawer";
import { useAuthStore } from "@/store/useAuthStore";
import { wsClient } from "@/lib/ws/WSClient";
import { useTaskStore } from "@/store/useTaskStore";
import { taskApi } from "@/lib/api/tasks";
import { authApi } from "@/lib/api/auth";
import type { Task } from "@/types";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const { hydrate: hydrateAuth, accessToken, isAuthenticated, setUser } = useAuthStore();
  const { upsertPartial, hydrate: hydrateStore } = useTaskStore();

  // Step 1: hydrate auth from localStorage on client only
  useEffect(() => {
    hydrateAuth();
    setMounted(true);
  }, []);

  // Step 2: after hydration, check auth, seed store, connect WS
  useEffect(() => {
    if (!mounted) return;

    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }

    if (!accessToken) return;

    // Refresh user profile to get latest roles/permissions
    authApi.me().then(setUser).catch(() => {
      // Non-fatal — cached user from localStorage is still valid
    });

    // Seed the task store with recent active tasks so the TopBar badge and
    // TaskDrawer are populated immediately without waiting for a WS event.
    taskApi
      .list({ page: 1, page_size: 50 })
      .then((res) => {
        if (res.data) hydrateStore(res.data);
      })
      .catch(() => {
        // Non-fatal — WS events will populate the store as they arrive
      });

    // Connect WebSocket
    wsClient.connect(accessToken);

    // Subscribe to the tasks room for real-time status/progress updates
    const unsub = wsClient.subscribe("tasks", (msg) => {
      const payload = msg.payload as Record<string, unknown>;
      const taskId = (payload?.task_id ?? payload?.id) as string | undefined;
      if (!taskId) return;

      const type = msg.type;

      if (type === "task.status_changed" || type === "task.progress") {
        const patch: Partial<Task> = {};
        if (payload.status !== undefined) patch.status = payload.status as Task["status"];
        if (payload.progress !== undefined) patch.progress = payload.progress as number;
        if (payload.error_message !== undefined) patch.error_message = payload.error_message as string;
        if (payload.type !== undefined) patch.type = payload.type as Task["type"];

        upsertPartial(taskId, patch);
      }

      if (type === "task.cancelled") {
        upsertPartial(taskId, { status: "cancelled" });
      }
    });

    return () => {
      unsub();
      wsClient.disconnect();
    };
  }, [mounted, accessToken]);

  // Don't render anything until we've read localStorage
  if (!mounted) {
    return (
      <div className="flex h-screen bg-gray-950 items-center justify-center">
        <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!isAuthenticated()) return null;

  return (
    <div className="flex h-screen bg-gray-950 overflow-hidden">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0">
        <TopBar />
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
      <TaskDrawer />
    </div>
  );
}
