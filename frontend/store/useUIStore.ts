"use client";
import { create } from "zustand";

interface UIStore {
  sidebarOpen: boolean;
  taskDrawerOpen: boolean;
  setSidebarOpen: (v: boolean) => void;
  toggleSidebar: () => void;
  openTaskDrawer: () => void;
  closeTaskDrawer: () => void;
}

export const useUIStore = create<UIStore>((set) => ({
  sidebarOpen: true,
  taskDrawerOpen: false,
  setSidebarOpen: (v) => set({ sidebarOpen: v }),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  openTaskDrawer: () => set({ taskDrawerOpen: true }),
  closeTaskDrawer: () => set({ taskDrawerOpen: false }),
}));
