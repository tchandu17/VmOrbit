"use client";
import { useEffect, useRef, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  Search,
  Server,
  Cpu,
  Activity,
  ArrowRight,
  Command,
  HelpCircle,
  Power,
  RefreshCw,
  FileText,
} from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { cn } from "@/lib/utils";
import { searchNavItems, getAllNavItems, type NavItem } from "@/lib/navigation";

interface QuickAction {
  id: string;
  label: string;
  description: string;
  icon: React.ElementType;
  action: () => void;
  keywords: string[];
  category: "navigation" | "action" | "help";
}

export function CommandPalette() {
  const router = useRouter();
  const isOpen = useUIStore((s) => s.commandPaletteOpen);
  const close = useUIStore((s) => s.closeCommandPalette);
  const openHelp = useUIStore((s) => s.openHelpPanel);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Quick actions
  const quickActions: QuickAction[] = [
    {
      id: "search-vms",
      label: "Search Virtual Machines",
      description: "Find VMs by name, UUID, or IP",
      icon: Server,
      action: () => { router.push("/dashboard/vms"); close(); },
      keywords: ["vm", "virtual machine", "search"],
      category: "navigation",
    },
    {
      id: "search-providers",
      label: "Search Providers",
      description: "Find hypervisor providers",
      icon: Cpu,
      action: () => { router.push("/dashboard/hypervisors"); close(); },
      keywords: ["provider", "hypervisor", "vcenter", "proxmox"],
      category: "navigation",
    },
    {
      id: "view-tasks",
      label: "View Running Tasks",
      description: "Check active task queue",
      icon: Activity,
      action: () => { router.push("/dashboard/tasks"); close(); },
      keywords: ["task", "running", "queue", "job"],
      category: "navigation",
    },
    {
      id: "sync-inventory",
      label: "Sync Inventory",
      description: "Trigger inventory sync for a provider",
      icon: RefreshCw,
      action: () => { router.push("/dashboard/hypervisors"); close(); },
      keywords: ["sync", "inventory", "refresh", "discover"],
      category: "action",
    },
    {
      id: "open-help",
      label: "Open Help Center",
      description: "Documentation and guides",
      icon: HelpCircle,
      action: () => { openHelp(); close(); },
      keywords: ["help", "docs", "documentation", "guide"],
      category: "help",
    },
    {
      id: "view-audit",
      label: "View Audit Logs",
      description: "Platform activity history",
      icon: FileText,
      action: () => { router.push("/dashboard/audit"); close(); },
      keywords: ["audit", "log", "history", "activity"],
      category: "navigation",
    },
  ];

  // Compute results
  const navResults = query ? searchNavItems(query).slice(0, 6) : [];
  const actionResults = query
    ? quickActions.filter(
        (a) =>
          a.label.toLowerCase().includes(query.toLowerCase()) ||
          a.keywords.some((k) => k.includes(query.toLowerCase()))
      )
    : quickActions.slice(0, 4);

  const allResults = [
    ...navResults.map((item) => ({
      id: `nav-${item.href}`,
      label: item.label,
      description: item.description || item.group,
      icon: item.icon,
      action: () => { router.push(item.href); close(); },
      category: "navigation" as const,
    })),
    ...actionResults.map((a) => ({
      id: a.id,
      label: a.label,
      description: a.description,
      icon: a.icon,
      action: a.action,
      category: a.category,
    })),
  ];

  // Keyboard handling
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        useUIStore.getState().toggleCommandPalette();
        return;
      }

      if (!isOpen) return;

      if (e.key === "Escape") {
        close();
        return;
      }

      if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex((i) => Math.min(i + 1, allResults.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
      } else if (e.key === "Enter") {
        e.preventDefault();
        allResults[selectedIndex]?.action();
      }
    },
    [isOpen, allResults, selectedIndex, close]
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

  useEffect(() => {
    if (isOpen) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  // Scroll selected item into view
  useEffect(() => {
    const el = listRef.current?.children[selectedIndex] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  if (!isOpen) return null;

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-[100]" onClick={close} />

      {/* Palette */}
      <div className="fixed inset-x-0 top-[15%] mx-auto w-full max-w-xl z-[101]">
        <div className="bg-gray-900 border border-gray-700 rounded-xl shadow-2xl overflow-hidden">
          {/* Search Input */}
          <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-800">
            <Search className="w-5 h-5 text-gray-500 shrink-0" />
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setSelectedIndex(0);
              }}
              placeholder="Search pages, VMs, tasks, or type a command..."
              className="flex-1 bg-transparent text-white text-sm placeholder-gray-500 outline-none"
            />
            <kbd className="text-[10px] bg-gray-800 text-gray-500 px-1.5 py-0.5 rounded border border-gray-700">
              ESC
            </kbd>
          </div>

          {/* Results */}
          <div ref={listRef} className="max-h-80 overflow-y-auto py-2">
            {allResults.length === 0 && query && (
              <div className="px-4 py-8 text-center text-gray-500 text-sm">
                No results for &ldquo;{query}&rdquo;
              </div>
            )}

            {!query && (
              <div className="px-4 py-1.5">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-600">
                  Quick Actions
                </p>
              </div>
            )}

            {allResults.map((result, index) => (
              <button
                key={result.id}
                onClick={result.action}
                onMouseEnter={() => setSelectedIndex(index)}
                className={cn(
                  "w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors",
                  index === selectedIndex
                    ? "bg-blue-600/15 text-white"
                    : "text-gray-400 hover:bg-gray-800/50"
                )}
              >
                <result.icon className="w-4 h-4 shrink-0 text-gray-500" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{result.label}</p>
                  <p className="text-xs text-gray-600 truncate">{result.description}</p>
                </div>
                <ArrowRight className="w-3.5 h-3.5 text-gray-600 shrink-0" />
              </button>
            ))}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between px-4 py-2 border-t border-gray-800 bg-gray-900/50">
            <div className="flex items-center gap-3 text-[10px] text-gray-600">
              <span className="flex items-center gap-1">
                <kbd className="bg-gray-800 px-1 py-0.5 rounded">↑↓</kbd> navigate
              </span>
              <span className="flex items-center gap-1">
                <kbd className="bg-gray-800 px-1 py-0.5 rounded">↵</kbd> select
              </span>
              <span className="flex items-center gap-1">
                <kbd className="bg-gray-800 px-1 py-0.5 rounded">esc</kbd> close
              </span>
            </div>
            <div className="flex items-center gap-1 text-[10px] text-gray-600">
              <Command className="w-3 h-3" />
              <span>K to toggle</span>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
