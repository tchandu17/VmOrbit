"use client";
import { useQuery } from "@tanstack/react-query";
import { Server, Cpu, Activity, CheckCircle, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { vmApi } from "@/lib/api/vms";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { taskApi } from "@/lib/api/tasks";
import { wsClient } from "@/lib/ws/WSClient";

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

export default function DashboardPage() {
  // Adaptive polling: poll every 60s as a fallback when WS is disconnected.
  // When WS is connected, React Query cache invalidation handles freshness.
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
      <div>
        <h1 className="text-2xl font-bold text-white">Overview</h1>
        <p className="text-gray-400 text-sm mt-1">Platform health at a glance</p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Server}        label="Total VMs"        value={vms?.meta?.total_items ?? "—"}                                          sub="across all hypervisors" color="bg-blue-600"   href="/dashboard/vms" />
        <StatCard icon={Cpu}           label="Hypervisors"      value={hypervisors?.meta?.total_items ?? "—"}                                   sub="registered"             color="bg-purple-600" href="/dashboard/hypervisors" />
        <StatCard icon={Activity}      label="Active Tasks"     value={activeTasks}                                                             sub="running or queued"      color="bg-orange-600" href="/dashboard/tasks" />
        <StatCard icon={CheckCircle}   label="Completed Tasks"  value={tasks?.data?.filter(t => t.status === "completed").length ?? "—"}        sub="last 100"               color="bg-green-600"  href="/dashboard/tasks" />
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
        <h2 className="font-semibold text-white mb-4">Quick Actions</h2>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {[
            { label: "View VMs", href: "/dashboard/vms", icon: Server },
            { label: "Hypervisors", href: "/dashboard/hypervisors", icon: Cpu },
            { label: "Tasks", href: "/dashboard/tasks", icon: Activity },
            { label: "Audit Log", href: "/dashboard/audit", icon: CheckCircle },
          ].map((item) => (
            <a
              key={item.href}
              href={item.href}
              className="flex flex-col items-center gap-2 p-4 bg-gray-800 hover:bg-gray-700 rounded-xl transition-colors text-center"
            >
              <item.icon className="w-5 h-5 text-blue-400" />
              <span className="text-sm text-gray-300">{item.label}</span>
            </a>
          ))}
        </div>
      </div>
    </div>
  );
}
