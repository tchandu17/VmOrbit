"use client";
import { Menu, Bell, LogOut, Activity, Search, HelpCircle } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { useTaskStore } from "@/store/useTaskStore";
import { useAuthStore } from "@/store/useAuthStore";
import { useRouter } from "next/navigation";

export function TopBar() {
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
  const openCommandPalette = useUIStore((s) => s.openCommandPalette);
  const openHelpPanel = useUIStore((s) => s.openHelpPanel);
  const getActiveTasks = useTaskStore((s) => s.getActiveTasks);
  const activeCount = getActiveTasks().length;
  const { user, clear } = useAuthStore();
  const router = useRouter();

  function handleLogout() {
    clear();
    router.push("/login");
  }

  return (
    <header className="h-14 bg-gray-900 border-b border-gray-800 flex items-center justify-between px-4 shrink-0">
      <div className="flex items-center gap-2">
        <button onClick={toggleSidebar} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors">
          <Menu className="w-5 h-5" />
        </button>

        {/* Global Search Trigger */}
        <button
          onClick={openCommandPalette}
          className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800/50 border border-gray-700/50 text-gray-500 text-xs hover:bg-gray-800 hover:text-gray-400 hover:border-gray-600 transition-colors ml-2"
        >
          <Search className="w-3.5 h-3.5" />
          <span>Search platform...</span>
          <kbd className="ml-4 text-[10px] bg-gray-700/50 px-1.5 py-0.5 rounded text-gray-500 border border-gray-700">⌘K</kbd>
        </button>
      </div>

      <div className="flex items-center gap-1">
        {/* Active tasks button */}
        <button
          onClick={openTaskDrawer}
          className="relative flex items-center gap-2 px-3 py-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors text-sm"
        >
          <Activity className="w-4 h-4" />
          {activeCount > 0 && (
            <>
              <span className="text-xs hidden md:inline">{activeCount} running</span>
              <span className="absolute top-1 right-1 w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
            </>
          )}
        </button>

        {/* Help button */}
        <button
          onClick={openHelpPanel}
          className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
          title="Help & Documentation"
        >
          <HelpCircle className="w-4 h-4" />
        </button>

        <button className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors">
          <Bell className="w-4 h-4" />
        </button>

        <div className="flex items-center gap-2 pl-2 ml-1 border-l border-gray-800">
          <div className="w-7 h-7 rounded-full bg-blue-600 flex items-center justify-center text-xs font-medium text-white">
            {user?.username?.[0]?.toUpperCase() ?? "U"}
          </div>
          <span className="text-sm text-gray-300 hidden sm:block">{user?.username ?? "User"}</span>
          <button onClick={handleLogout} className="p-1.5 rounded-lg text-gray-400 hover:text-red-400 hover:bg-gray-800 transition-colors ml-1">
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>
  );
}
