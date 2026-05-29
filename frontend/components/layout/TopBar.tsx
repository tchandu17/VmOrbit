"use client";
import { Menu, Bell, LogOut, Activity } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { useTaskStore } from "@/store/useTaskStore";
import { useAuthStore } from "@/store/useAuthStore";
import { useRouter } from "next/navigation";

export function TopBar() {
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);
  const openTaskDrawer = useUIStore((s) => s.openTaskDrawer);
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
      <button onClick={toggleSidebar} className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors">
        <Menu className="w-5 h-5" />
      </button>

      <div className="flex items-center gap-2">
        {/* Active tasks button */}
        <button
          onClick={openTaskDrawer}
          className="relative flex items-center gap-2 px-3 py-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors text-sm"
        >
          <Activity className="w-4 h-4" />
          {activeCount > 0 && (
            <>
              <span className="text-xs">{activeCount} running</span>
              <span className="absolute top-1 right-1 w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
            </>
          )}
        </button>

        <button className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors">
          <Bell className="w-4 h-4" />
        </button>

        <div className="flex items-center gap-2 pl-2 border-l border-gray-800">
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
