"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface UIStore {
  sidebarOpen: boolean;
  taskDrawerOpen: boolean;
  helpPanelOpen: boolean;
  commandPaletteOpen: boolean;
  onboardingComplete: boolean;
  onboardingStep: number;
  collapsedGroups: Record<string, boolean>;
  favorites: string[];
  recentPages: string[];
  setSidebarOpen: (v: boolean) => void;
  toggleSidebar: () => void;
  openTaskDrawer: () => void;
  closeTaskDrawer: () => void;
  openHelpPanel: () => void;
  closeHelpPanel: () => void;
  toggleHelpPanel: () => void;
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  toggleCommandPalette: () => void;
  toggleGroup: (group: string) => void;
  setGroupCollapsed: (group: string, collapsed: boolean) => void;
  addFavorite: (path: string) => void;
  removeFavorite: (path: string) => void;
  addRecentPage: (path: string) => void;
  completeOnboarding: () => void;
  setOnboardingStep: (step: number) => void;
  resetOnboarding: () => void;
}

export const useUIStore = create<UIStore>()(
  persist(
    (set, get) => ({
      sidebarOpen: true,
      taskDrawerOpen: false,
      helpPanelOpen: false,
      commandPaletteOpen: false,
      onboardingComplete: false,
      onboardingStep: 0,
      collapsedGroups: {},
      favorites: [],
      recentPages: [],

      setSidebarOpen: (v) => set({ sidebarOpen: v }),
      toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
      openTaskDrawer: () => set({ taskDrawerOpen: true }),
      closeTaskDrawer: () => set({ taskDrawerOpen: false }),
      openHelpPanel: () => set({ helpPanelOpen: true }),
      closeHelpPanel: () => set({ helpPanelOpen: false }),
      toggleHelpPanel: () => set((s) => ({ helpPanelOpen: !s.helpPanelOpen })),
      openCommandPalette: () => set({ commandPaletteOpen: true }),
      closeCommandPalette: () => set({ commandPaletteOpen: false }),
      toggleCommandPalette: () => set((s) => ({ commandPaletteOpen: !s.commandPaletteOpen })),

      toggleGroup: (group) =>
        set((s) => ({
          collapsedGroups: {
            ...s.collapsedGroups,
            [group]: !s.collapsedGroups[group],
          },
        })),

      setGroupCollapsed: (group, collapsed) =>
        set((s) => ({
          collapsedGroups: { ...s.collapsedGroups, [group]: collapsed },
        })),

      addFavorite: (path) =>
        set((s) => ({
          favorites: s.favorites.includes(path) ? s.favorites : [...s.favorites, path],
        })),

      removeFavorite: (path) =>
        set((s) => ({
          favorites: s.favorites.filter((f) => f !== path),
        })),

      addRecentPage: (path) =>
        set((s) => {
          const filtered = s.recentPages.filter((p) => p !== path);
          return { recentPages: [path, ...filtered].slice(0, 10) };
        }),

      completeOnboarding: () => set({ onboardingComplete: true }),
      setOnboardingStep: (step) => set({ onboardingStep: step }),
      resetOnboarding: () => set({ onboardingComplete: false, onboardingStep: 0 }),
    }),
    {
      name: "vmorbit-ui",
      partialize: (state) => ({
        sidebarOpen: state.sidebarOpen,
        collapsedGroups: state.collapsedGroups,
        favorites: state.favorites,
        recentPages: state.recentPages,
        onboardingComplete: state.onboardingComplete,
      }),
    }
  )
);
