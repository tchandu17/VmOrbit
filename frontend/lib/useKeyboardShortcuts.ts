"use client";
import { useEffect } from "react";
import { useUIStore } from "@/store/useUIStore";

/**
 * Global keyboard shortcuts for the platform.
 * Mount this once in the dashboard layout.
 */
export function useKeyboardShortcuts() {
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const isInput =
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        (e.target as HTMLElement)?.isContentEditable;

      // Ctrl/Cmd + K — Command Palette (handled in CommandPalette component)
      // Already handled there, but we add it here as backup
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        useUIStore.getState().toggleCommandPalette();
        return;
      }

      // Ctrl/Cmd + / — Toggle Help Panel
      if ((e.metaKey || e.ctrlKey) && e.key === "/") {
        e.preventDefault();
        useUIStore.getState().toggleHelpPanel();
        return;
      }

      // Ctrl/Cmd + B — Toggle Sidebar
      if ((e.metaKey || e.ctrlKey) && e.key === "b") {
        e.preventDefault();
        useUIStore.getState().toggleSidebar();
        return;
      }

      // Escape — Close any open overlay
      if (e.key === "Escape") {
        const state = useUIStore.getState();
        if (state.commandPaletteOpen) {
          state.closeCommandPalette();
          return;
        }
        if (state.helpPanelOpen) {
          state.closeHelpPanel();
          return;
        }
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);
}
