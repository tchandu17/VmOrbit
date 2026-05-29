"use client";
import { create } from "zustand";
import type { Task } from "@/types";

interface TaskStore {
  activeTasks: Map<string, Task>;
  upsertTask: (task: Task) => void;
  upsertPartial: (id: string, patch: Partial<Task>) => void;
  removeTask: (id: string) => void;
  hydrate: (tasks: Task[]) => void;
  getActiveTasks: () => Task[];
}

const TERMINAL = new Set(["completed", "failed", "cancelled", "timed_out"]);

export const useTaskStore = create<TaskStore>((set, get) => ({
  activeTasks: new Map(),

  /** Replace or insert a full task record. */
  upsertTask: (task) =>
    set((s) => {
      const next = new Map(s.activeTasks);
      next.set(task.id, task);
      return { activeTasks: next };
    }),

  /**
   * Merge a partial update (e.g. from a WS status_changed event) into an
   * existing task. If the task isn't in the store yet, the patch is ignored —
   * the next full API refetch will populate it.
   */
  upsertPartial: (id, patch) =>
    set((s) => {
      const existing = s.activeTasks.get(id);
      if (!existing) return s; // not yet loaded — skip
      const next = new Map(s.activeTasks);
      next.set(id, { ...existing, ...patch });
      return { activeTasks: next };
    }),

  removeTask: (id) =>
    set((s) => {
      const next = new Map(s.activeTasks);
      next.delete(id);
      return { activeTasks: next };
    }),

  /** Seed the store from an API response (called on dashboard mount). */
  hydrate: (tasks) =>
    set((s) => {
      const next = new Map(s.activeTasks);
      for (const t of tasks) {
        next.set(t.id, t);
      }
      return { activeTasks: next };
    }),

  /** Returns non-terminal tasks sorted newest-first. */
  getActiveTasks: () => {
    const tasks = Array.from(get().activeTasks.values());
    return tasks
      .filter((t) => !TERMINAL.has(t.status))
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  },
}));
