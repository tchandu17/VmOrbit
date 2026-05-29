"use client";
import { useQuery } from "@tanstack/react-query";
import {
  Server, Cpu, Activity, CheckCircle, ArrowUpRight, RefreshCw,
  BarChart3, Shield, Clock, HelpCircle, Keyboard, Search,
} from "lucide-react";
import Link from "next/link";
import { vmApi } from "@/lib/api/vms";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { taskApi } from "@/lib/api/tasks";
import { wsClient } from "@/lib/ws/WSClient";
import { PageHeader } from "@/components/layout/PageHeader";
import { useUIStore } from "@/store/useUIStore";

function StatCard({ icon: Icon, label, value, sub, color, href }: {
  icon: React.ElementType; label: string; value: string | number; sub?: string; color: string; href: string;
}) {
  return (
    <Link href={href} className="group bg-gray-900 border border-gray-800 hover:border-gray-700 rounded-2xl p-5 transition-colors block">
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm text-gray-400">{label}</span>
        <div className="flex items-center gap-2">
          <ArrowUpRight className="w-3.5 h-3.5 text-gray-600 group-hover:text-gray-400 transition-colors" />
          <div className={`w-9 h-9 rounded-xl flex items-center justify-center ${color}`}>
            <Icon className="w-4 h-4 text-white" />
          </div>
        </div>
      </div>
      <p className="text-3xl font-bold text-white">{value}</p>
      {sub && <p className="text-xs text-gray-500 mt-1">{sub}</p>}
    </Link>
  );
}

function QuickTip({ icon: Icon, label, shortcut }: { icon: React.ElementType; label: string; shortcut: string }) {
  return (
    <div className="flex items-center gap-3 px-3 py-2 rounded-lg bg-gray-800/50">
      <Icon className="w-4 h-4 text-gray-500 shrink-0" />
      <span className="text-sm text-gray-400 flex-1">{label}</span>
      <kbd className="text-[10px] bg-gray-700/50 text-gray-500 px-1.5 py-0.5 rounded border border-gray-700">{shortcut}</kbd>
    </div>
  );
}

export default function DashboardPage() {
  const openCommandPalette = useUIStore((s) => s.openCommandPalette);
  const openHelp = useUIStore((s) => s.openHelpPanel);

  // Adaptive polling: poll every 60s as a fallback when WS is disconnected.
  const wsConnected = wsClient.isConnected();
  const fallbackInterval = wsConnected ? false : (60_000 as number | false);

  const { data: vms } = useQuery({
    queryKey: ["vms", {}],
    queryFn: () => vmApi.list({ page_size: 1 }),
    staleTime: 30_000,
    refetchInterval: fallbackInterval,
  });
  const { data: hypervisors } = useQuery({
    queryKey: ["hypervisors", {}],
    queryFn: () => hypervisorApi.list({ page_size: 1 }),
    staleTime: 60_000,
    refetchInterval: fallbackInterval,
  });
  const { data: tasks } = useQuery({
    queryKey: ["tasks", {}],
    queryFn: () => taskApi.list({ page_size: 100 }),
    staleTime: 15_000,
    refetchInterval: wsConnected ? false : 30_000,
  });

  const activeTasks = tasks?.data?.filter(
    (t) => ["running", "queued", "pending"].includes(t.status)
  ).length ?? 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Overview"
        description="Platform health at a glance"
        helpArticleId="getting-started"
      />

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Server}        label="Total VMs"        value={vms?.meta?.total_items ?? "—"}                                          sub="across all providers" color="bg-blue-600"   href="/dashboard/vms" />
        <StatCard icon={Cpu}           label="Providers"        value={hypervisors?.meta?.total_items ?? "—"}                                   sub="registered"             color="bg-purple-600" href="/dashboard/hypervisors" />
        <StatCard icon={Activity}      label="Active Tasks"     value={activeTasks}                                                             sub="running or queued"      color="bg-orange-600" href="/dashboard/tasks" />
        <StatCard icon={CheckCircle}   label="Completed Tasks"  value={tasks?.data?.filter(t => t.status === "completed").length ?? "—"}        sub="last 100"               color="bg-green-600"  href="/dashboard/tasks" />
      </div>

      {/* Quick Actions */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <h2 className="font-semibold text-white mb-4">Quick Actions</h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            {[
              { label: "Virtual Machines", href: "/dashboard/vms", icon: Server },
              { label: "Providers", href: "/dashboard/hypervisors", icon: Cpu },
              { label: "Tasks", href: "/dashboard/tasks", icon: Activity },
              { label: "Analytics", href: "/dashboard/analytics", icon: BarChart3 },
              { label: "Schedules", href: "/dashboard/schedules", icon: Clock },
              { label: "Governance", href: "/dashboard/governance", icon: Shield },
              { label: "Sync Inventory", href: "/dashboard/hypervisors", icon: RefreshCw },
              { label: "Audit Log", href: "/dashboard/audit", icon: CheckCircle },
            ].map((item) => (
              <Link
                key={item.href + item.label}
                href={item.href}
                className="flex flex-col items-center gap-2 p-4 bg-gray-800/50 hover:bg-gray-800 border border-gray-800 hover:border-gray-700 rounded-xl transition-colors text-center"
              >
                <item.icon className="w-5 h-5 text-blue-400" />
                <span className="text-xs text-gray-300">{item.label}</span>
              </Link>
            ))}
          </div>
        </div>

        {/* Keyboard Shortcuts & Tips */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <h2 className="font-semibold text-white mb-4 flex items-center gap-2">
            <Keyboard className="w-4 h-4 text-gray-500" />
            Quick Tips
          </h2>
          <div className="space-y-2">
            <QuickTip icon={Search} label="Search anything" shortcut="⌘K" />
            <QuickTip icon={HelpCircle} label="Open help" shortcut="⌘/" />
            <QuickTip icon={Keyboard} label="Toggle sidebar" shortcut="⌘B" />
          </div>
          <div className="mt-4 pt-3 border-t border-gray-800">
            <button
              onClick={openHelp}
              className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-lg text-xs text-blue-400 bg-blue-600/10 border border-blue-500/20 hover:bg-blue-600/15 transition-colors"
            >
              <HelpCircle className="w-3.5 h-3.5" />
              View Documentation
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
