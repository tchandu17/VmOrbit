"use client";
import { useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Server, ChevronDown, Star, Search, Clock } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { usePermissions } from "@/store/usePermissions";
import { cn } from "@/lib/utils";
import { navigationGroups, getAllNavItems, type NavItem } from "@/lib/navigation";

export function Sidebar() {
  const pathname = usePathname();
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const collapsedGroups = useUIStore((s) => s.collapsedGroups);
  const toggleGroup = useUIStore((s) => s.toggleGroup);
  const favorites = useUIStore((s) => s.favorites);
  const recentPages = useUIStore((s) => s.recentPages);
  const addRecentPage = useUIStore((s) => s.addRecentPage);
  const openCommandPalette = useUIStore((s) => s.openCommandPalette);
  const { can } = usePermissions();

  // Track page visits
  useEffect(() => {
    if (pathname) addRecentPage(pathname);
  }, [pathname]);

  const allItems = getAllNavItems();
  const favoriteItems = allItems.filter((item) => favorites.includes(item.href));
  const recentItems = recentPages
    .slice(0, 5)
    .map((href) => allItems.find((item) => item.href === href))
    .filter(Boolean) as (NavItem & { group: string })[];

  function isActive(item: NavItem) {
    return item.exact ? pathname === item.href : pathname === item.href || pathname.startsWith(item.href + "/");
  }

  function canView(item: NavItem) {
    return !item.permission || can(item.permission);
  }

  return (
    <aside
      className={cn(
        "flex flex-col bg-gray-900 border-r border-gray-800 transition-all duration-200 shrink-0",
        sidebarOpen ? "w-60" : "w-16"
      )}
    >
      {/* Brand */}
      <div className="flex items-center gap-3 px-4 py-4 border-b border-gray-800">
        <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center shrink-0">
          <Server className="w-4 h-4 text-white" />
        </div>
        {sidebarOpen && (
          <span className="font-bold text-white text-sm tracking-wide">VMOrbit</span>
        )}
      </div>

      {/* Quick Search Button */}
      {sidebarOpen && (
        <div className="px-3 pt-3 pb-1">
          <button
            onClick={openCommandPalette}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-lg bg-gray-800/50 border border-gray-700/50 text-gray-500 text-xs hover:bg-gray-800 hover:text-gray-400 transition-colors"
          >
            <Search className="w-3.5 h-3.5" />
            <span>Search...</span>
            <kbd className="ml-auto text-[10px] bg-gray-700/50 px-1.5 py-0.5 rounded text-gray-500">⌘K</kbd>
          </button>
        </div>
      )}

      {/* Nav */}
      <nav className="flex-1 py-2 px-2 overflow-y-auto sidebar-scroll">
        {/* Favorites Section */}
        {sidebarOpen && favoriteItems.length > 0 && (
          <div className="mb-3">
            <div className="flex items-center gap-2 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
              <Star className="w-3 h-3" />
              <span>Favorites</span>
            </div>
            {favoriteItems.map((item) => (
              <NavLink key={`fav-${item.id}`} item={item} active={isActive(item)} sidebarOpen={sidebarOpen} />
            ))}
          </div>
        )}

        {/* Recent Pages */}
        {sidebarOpen && recentItems.length > 0 && !favoriteItems.length && (
          <div className="mb-3">
            <div className="flex items-center gap-2 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
              <Clock className="w-3 h-3" />
              <span>Recent</span>
            </div>
            {recentItems.slice(0, 3).map((item) => (
              <NavLink key={`recent-${item.id}`} item={item} active={isActive(item)} sidebarOpen={sidebarOpen} />
            ))}
          </div>
        )}

        {/* Navigation Groups */}
        {navigationGroups.map((group) => {
          const visibleItems = group.items.filter(canView);
          if (visibleItems.length === 0) return null;

          const isCollapsed = collapsedGroups[group.id];
          const hasActiveItem = visibleItems.some(isActive);

          return (
            <div key={group.id} className="mb-1">
              {/* Group Header */}
              {sidebarOpen ? (
                <button
                  onClick={() => toggleGroup(group.id)}
                  className={cn(
                    "w-full flex items-center gap-2 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider rounded-md transition-colors",
                    hasActiveItem ? "text-blue-400" : "text-gray-500 hover:text-gray-300"
                  )}
                >
                  <group.icon className="w-3 h-3" />
                  <span>{group.label}</span>
                  <ChevronDown
                    className={cn(
                      "w-3 h-3 ml-auto transition-transform duration-200",
                      isCollapsed && "-rotate-90"
                    )}
                  />
                </button>
              ) : (
                <div className="flex justify-center py-2">
                  <div className="w-6 h-px bg-gray-800" />
                </div>
              )}

              {/* Group Items */}
              {!isCollapsed && (
                <div className={cn("space-y-0.5", sidebarOpen && "ml-1")}>
                  {visibleItems.map((item) => (
                    <NavLink
                      key={item.id}
                      item={item}
                      active={isActive(item)}
                      sidebarOpen={sidebarOpen}
                    />
                  ))}
                </div>
              )}

              {/* Collapsed: show only icons */}
              {isCollapsed && !sidebarOpen && (
                <div className="space-y-0.5">
                  {visibleItems.map((item) => (
                    <NavLink
                      key={item.id}
                      item={item}
                      active={isActive(item)}
                      sidebarOpen={false}
                    />
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </nav>

      {/* Version */}
      {sidebarOpen && (
        <div className="px-4 py-3 border-t border-gray-800">
          <p className="text-xs text-gray-600">VMOrbit v1.0</p>
        </div>
      )}
    </aside>
  );
}

function NavLink({
  item,
  active,
  sidebarOpen,
}: {
  item: NavItem;
  active: boolean;
  sidebarOpen: boolean;
}) {
  const addFavorite = useUIStore((s) => s.addFavorite);
  const removeFavorite = useUIStore((s) => s.removeFavorite);
  const favorites = useUIStore((s) => s.favorites);
  const isFavorite = favorites.includes(item.href);

  return (
    <div className="group relative">
      <Link
        href={item.href}
        className={cn(
          "flex items-center gap-3 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors",
          active
            ? "bg-blue-600/15 text-blue-400 border border-blue-500/20"
            : "text-gray-400 hover:text-white hover:bg-gray-800/70 border border-transparent"
        )}
        title={!sidebarOpen ? item.label : undefined}
      >
        <item.icon className="w-4 h-4 shrink-0" />
        {sidebarOpen && <span className="truncate">{item.label}</span>}
      </Link>

      {/* Favorite toggle on hover */}
      {sidebarOpen && (
        <button
          onClick={(e) => {
            e.preventDefault();
            isFavorite ? removeFavorite(item.href) : addFavorite(item.href);
          }}
          className={cn(
            "absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-opacity",
            isFavorite
              ? "opacity-100 text-yellow-400"
              : "opacity-0 group-hover:opacity-100 text-gray-600 hover:text-yellow-400"
          )}
        >
          <Star className={cn("w-3 h-3", isFavorite && "fill-current")} />
        </button>
      )}
    </div>
  );
}
