"use client";
import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import {
  ArrowLeft, Server, Cpu, MemoryStick, HardDrive, Network,
  Activity, Circle, Database, Globe, Layers, RefreshCw,
  Power, AlertCircle, CheckCircle2, Clock,
} from "lucide-react";
import { infrastructureApi } from "@/lib/api/infrastructure";
import { VMStatusBadge } from "@/components/vms/VMStatusBadge";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import type { VM, DataStore, Network as NetworkType } from "@/types";

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatMB(mb: number): string {
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(1)} TB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

function formatGB(gb: number): string {
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`;
  return `${gb.toFixed(1)} GB`;
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function UsageBar({ used, total, label }: { used: number; total: number; label: string }) {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0;
  const color = pct > 85 ? "bg-red-500" : pct > 65 ? "bg-yellow-500" : "bg-blue-500";
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-400">
        <span>{label}</span>
        <span>{pct.toFixed(1)}%</span>
      </div>
      <div className="h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const cfg: Record<string, { color: string; label: string }> = {
    connected:    { color: "bg-green-900/40 text-green-400 border-green-800", label: "Connected" },
    disconnected: { color: "bg-red-900/40 text-red-400 border-red-800", label: "Disconnected" },
    maintenance:  { color: "bg-yellow-900/40 text-yellow-400 border-yellow-800", label: "Maintenance" },
    unknown:      { color: "bg-gray-800 text-gray-400 border-gray-700", label: "Unknown" },
  };
  const c = cfg[status] ?? cfg.unknown;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border text-xs font-medium ${c.color}`}>
      <Circle className="w-2 h-2 fill-current" />
      {c.label}
    </span>
  );
}

function SectionCard({ title, icon: Icon, children }: {
  title: string; icon: React.ElementType; children: React.ReactNode;
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
      <div className="flex items-center gap-2 px-5 py-3.5 border-b border-gray-800">
        <Icon className="w-4 h-4 text-gray-400" />
        <h3 className="text-sm font-semibold text-white">{title}</h3>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 border-b border-gray-800/50 last:border-0">
      <span className="text-xs text-gray-500 shrink-0">{label}</span>
      <span className={`text-xs text-gray-200 text-right ${mono ? "font-mono" : ""}`}>{value || "—"}</span>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export default function HostDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);

  const { data: detail, isLoading, refetch } = useQuery({
    queryKey: ["host-detail", id],
    queryFn: () => infrastructureApi.getHost(id),
    staleTime: 30_000,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3">
        <AlertCircle className="w-10 h-10 text-gray-600" />
        <p className="text-gray-400">Host not found</p>
        <Link href="/dashboard/infrastructure" className="text-blue-400 text-sm hover:underline">
          ← Back to Infrastructure
        </Link>
      </div>
    );
  }

  const { host, vms, datastores, networks } = detail;
  const runningVMs = vms.filter((v: VM) => v.status === "running").length;
  const stoppedVMs = vms.filter((v: VM) => v.status === "stopped").length;
  const memUsedPct = host.total_memory_mb > 0
    ? (host.used_memory_mb / host.total_memory_mb) * 100
    : 0;
  const totalDsGB = datastores.reduce((s: number, d: DataStore) => s + d.capacity_gb, 0);
  const usedDsGB = datastores.reduce((s: number, d: DataStore) => s + d.used_gb, 0);

  return (
    <div className="space-y-5">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-gray-500">
        <Link href="/dashboard/infrastructure" className="hover:text-gray-300 flex items-center gap-1">
          <ArrowLeft className="w-3.5 h-3.5" /> Infrastructure
        </Link>
        <span>/</span>
        <span className="text-gray-300">{host.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4">
          <div className="p-3 bg-green-900/30 rounded-xl">
            <Server className="w-7 h-7 text-green-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-white">{host.name}</h1>
            <div className="flex items-center gap-3 mt-1.5 flex-wrap">
              <StatusBadge status={host.status} />
              {host.hypervisor && (
                <ProviderBadge provider={host.hypervisor.provider} />
              )}
              {host.cluster && (
                <span className="flex items-center gap-1.5 text-xs text-purple-400 bg-purple-900/20 px-2 py-1 rounded-lg border border-purple-800">
                  <Layers className="w-3 h-3" />
                  {host.cluster.name}
                </span>
              )}
              {host.hypervisor_version && (
                <span className="text-xs text-gray-500">{host.hypervisor_version}</span>
              )}
            </div>
          </div>
        </div>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
        >
          <RefreshCw className="w-4 h-4" /> Refresh
        </button>
      </div>

      {/* Quick stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {[
          { label: "Total VMs", value: vms.length, icon: Cpu, color: "text-blue-400", bg: "bg-blue-900/20" },
          { label: "Running", value: runningVMs, icon: Power, color: "text-green-400", bg: "bg-green-900/20" },
          { label: "Stopped", value: stoppedVMs, icon: Power, color: "text-red-400", bg: "bg-red-900/20" },
          { label: "Uptime", value: host.uptime_seconds ? formatUptime(host.uptime_seconds) : "—", icon: Clock, color: "text-orange-400", bg: "bg-orange-900/20", isStr: true },
        ].map((s) => (
          <div key={s.label} className={`${s.bg} border border-gray-800 rounded-xl p-4 flex items-center gap-3`}>
            <s.icon className={`w-5 h-5 ${s.color} shrink-0`} />
            <div>
              <p className="text-xl font-bold text-white">{s.value}</p>
              <p className="text-xs text-gray-500">{s.label}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Main grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* General info */}
        <SectionCard title="General" icon={Globe}>
          <div className="space-y-0">
            <InfoRow label="Hostname" value={host.name} mono />
            <InfoRow label="Provider ID" value={host.provider_id} mono />
            <InfoRow label="Status" value={host.status} />
            {host.hypervisor && <InfoRow label="Hypervisor" value={host.hypervisor.name} />}
            {host.cluster && <InfoRow label="Cluster" value={host.cluster.name} />}
            {host.hypervisor_version && <InfoRow label="Version" value={host.hypervisor_version} />}
            {host.uptime_seconds > 0 && <InfoRow label="Uptime" value={formatUptime(host.uptime_seconds)} />}
          </div>
        </SectionCard>

        {/* Compute */}
        <SectionCard title="Compute" icon={Cpu}>
          <div className="space-y-3">
            {host.cpu_model && <InfoRow label="CPU Model" value={host.cpu_model} />}
            <div className="grid grid-cols-3 gap-3 py-2">
              {[
                { label: "Sockets", value: host.cpu_sockets },
                { label: "Cores", value: host.cpu_cores },
                { label: "Threads", value: host.cpu_threads },
              ].map((c) => c.value > 0 ? (
                <div key={c.label} className="text-center bg-gray-800/50 rounded-lg p-2">
                  <p className="text-lg font-bold text-white">{c.value}</p>
                  <p className="text-xs text-gray-500">{c.label}</p>
                </div>
              ) : null)}
            </div>
            {host.total_memory_mb > 0 && (
              <div className="space-y-2 pt-1">
                <UsageBar
                  used={host.used_memory_mb}
                  total={host.total_memory_mb}
                  label={`Memory: ${formatMB(host.used_memory_mb)} / ${formatMB(host.total_memory_mb)}`}
                />
              </div>
            )}
          </div>
        </SectionCard>

        {/* Storage */}
        <SectionCard title={`Storage (${datastores.length} datastores)`} icon={HardDrive}>
          {datastores.length === 0 ? (
            <p className="text-sm text-gray-500">No datastores found</p>
          ) : (
            <div className="space-y-3">
              {totalDsGB > 0 && (
                <UsageBar
                  used={usedDsGB}
                  total={totalDsGB}
                  label={`Total: ${formatGB(usedDsGB)} / ${formatGB(totalDsGB)}`}
                />
              )}
              <div className="space-y-2 mt-3">
                {datastores.map((ds: DataStore) => (
                  <div key={ds.id} className="flex items-center gap-3 p-2.5 bg-gray-800/50 rounded-lg">
                    <Database className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-xs text-gray-200 truncate">{ds.name}</p>
                      <p className="text-[10px] text-gray-500">{ds.type}</p>
                    </div>
                    <div className="text-right shrink-0">
                      <p className="text-xs text-gray-300">{formatGB(ds.free_gb)} free</p>
                      <p className="text-[10px] text-gray-500">{formatGB(ds.capacity_gb)} total</p>
                    </div>
                    <div className={`w-2 h-2 rounded-full shrink-0 ${ds.accessible ? "bg-green-400" : "bg-red-400"}`} />
                  </div>
                ))}
              </div>
            </div>
          )}
        </SectionCard>

        {/* Networks */}
        <SectionCard title={`Networks (${networks.length})`} icon={Network}>
          {networks.length === 0 ? (
            <p className="text-sm text-gray-500">No networks found</p>
          ) : (
            <div className="space-y-2">
              {networks.map((net: NetworkType) => (
                <div key={net.id} className="flex items-center gap-3 p-2.5 bg-gray-800/50 rounded-lg">
                  <Network className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-gray-200 truncate">{net.name}</p>
                    <p className="text-[10px] text-gray-500">{net.type}{net.vlan > 0 ? ` · VLAN ${net.vlan}` : ""}</p>
                  </div>
                  <div className={`w-2 h-2 rounded-full shrink-0 ${net.accessible ? "bg-green-400" : "bg-red-400"}`} />
                </div>
              ))}
            </div>
          )}
        </SectionCard>
      </div>

      {/* VMs table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-gray-800">
          <div className="flex items-center gap-2">
            <Cpu className="w-4 h-4 text-gray-400" />
            <h3 className="text-sm font-semibold text-white">
              Virtual Machines ({vms.length})
            </h3>
          </div>
          <div className="flex items-center gap-3 text-xs text-gray-500">
            <span className="flex items-center gap-1">
              <CheckCircle2 className="w-3 h-3 text-green-400" /> {runningVMs} running
            </span>
            <span className="flex items-center gap-1">
              <Circle className="w-3 h-3 text-gray-500" /> {stoppedVMs} stopped
            </span>
          </div>
        </div>

        {vms.length === 0 ? (
          <div className="px-5 py-10 text-center text-gray-500 text-sm">
            No VMs hosted on this node
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800">
                  {["Name", "Status", "vCPU", "RAM", "Disk", "IP", "OS"].map((h) => (
                    <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">
                      {h}
                    </th>
                  ))}
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {vms.map((vm: VM) => (
                  <tr key={vm.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                    <td className="px-4 py-3">
                      <p className="font-medium text-white">{vm.name}</p>
                      <p className="text-[10px] text-gray-500 font-mono">{vm.provider_vm_id}</p>
                    </td>
                    <td className="px-4 py-3"><VMStatusBadge status={vm.status} /></td>
                    <td className="px-4 py-3 text-gray-300">{vm.cpu_count}</td>
                    <td className="px-4 py-3 text-gray-300">{formatMB(vm.memory_mb)}</td>
                    <td className="px-4 py-3 text-gray-300">{vm.disk_gb > 0 ? `${vm.disk_gb} GB` : "—"}</td>
                    <td className="px-4 py-3 text-gray-300 font-mono text-xs">
                      {vm.ip_addresses?.[0] ?? "—"}
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-xs truncate max-w-[120px]">
                      {vm.guest_os || "—"}
                    </td>
                    <td className="px-4 py-3">
                      <Link
                        href={`/dashboard/vms/${vm.id}`}
                        className="text-xs text-blue-400 hover:text-blue-300"
                      >
                        Details →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
