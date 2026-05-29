"use client";
import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import {
  ChevronRight, ChevronDown, Server, Cpu, Database, Network,
  RefreshCw, Search, Circle, HardDrive, MemoryStick, Activity,
  Layers, Globe, AlertCircle,
} from "lucide-react";
import { infrastructureApi } from "@/lib/api/infrastructure";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import type { InfraTreeNode, Hypervisor } from "@/types";

// ── Status dot ────────────────────────────────────────────────────────────────
function StatusDot({ status }: { status: string }) {
  const color =
    status === "connected" || status === "active"
      ? "bg-green-400"
      : status === "disconnected" || status === "error"
      ? "bg-red-400"
      : status === "maintenance"
      ? "bg-yellow-400"
      : "bg-gray-500";
  return <span className={`inline-block w-2 h-2 rounded-full ${color} shrink-0`} />;
}

// ── Tree node component ───────────────────────────────────────────────────────
interface TreeNodeProps {
  node: InfraTreeNode;
  depth?: number;
  selectedId: string | null;
  onSelect: (node: InfraTreeNode) => void;
  searchQuery: string;
}

function TreeNode({ node, depth = 0, selectedId, onSelect, searchQuery }: TreeNodeProps) {
  const [expanded, setExpanded] = useState(depth < 2);
  const hasChildren = (node.children?.length ?? 0) > 0;

  // Highlight match
  const matchesSearch =
    !searchQuery || node.name.toLowerCase().includes(searchQuery.toLowerCase());

  // If searching, auto-expand to show matches
  const shouldShow = !searchQuery || matchesSearch || nodeContainsMatch(node, searchQuery);
  if (!shouldShow) return null;

  const Icon =
    node.type === "provider"
      ? Globe
      : node.type === "cluster"
      ? Layers
      : node.type === "host"
      ? Server
      : Cpu;

  const isSelected = selectedId === node.id;

  return (
    <div>
      <div
        className={`flex items-center gap-2 px-3 py-1.5 rounded-lg cursor-pointer transition-colors group
          ${isSelected ? "bg-blue-600/20 text-blue-300" : "hover:bg-gray-800 text-gray-300"}`}
        style={{ paddingLeft: `${12 + depth * 16}px` }}
        onClick={() => {
          onSelect(node);
          if (hasChildren) setExpanded((e) => !e);
        }}
      >
        {/* Expand toggle */}
        <span className="w-4 shrink-0">
          {hasChildren ? (
            expanded ? (
              <ChevronDown className="w-3.5 h-3.5 text-gray-500" />
            ) : (
              <ChevronRight className="w-3.5 h-3.5 text-gray-500" />
            )
          ) : null}
        </span>

        <StatusDot status={node.status} />
        <Icon className={`w-3.5 h-3.5 shrink-0 ${
          node.type === "provider" ? "text-blue-400"
          : node.type === "cluster" ? "text-purple-400"
          : node.type === "host" ? "text-green-400"
          : "text-gray-400"
        }`} />

        <span className={`text-sm flex-1 truncate ${
          searchQuery && matchesSearch ? "text-yellow-300 font-medium" : ""
        }`}>
          {node.name}
        </span>

        {node.vm_count > 0 && (
          <span className="text-[10px] text-gray-500 bg-gray-800 px-1.5 py-0.5 rounded-full shrink-0">
            {node.vm_count} VM{node.vm_count !== 1 ? "s" : ""}
          </span>
        )}
      </div>

      {expanded && hasChildren && (
        <div>
          {node.children!.map((child) => (
            <TreeNode
              key={child.id}
              node={child}
              depth={depth + 1}
              selectedId={selectedId}
              onSelect={onSelect}
              searchQuery={searchQuery}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function nodeContainsMatch(node: InfraTreeNode, query: string): boolean {
  if (node.name.toLowerCase().includes(query.toLowerCase())) return true;
  return (node.children ?? []).some((c) => nodeContainsMatch(c, query));
}

// ── Detail panel ──────────────────────────────────────────────────────────────
function DetailPanel({
  node,
  hypervisors,
}: {
  node: InfraTreeNode | null;
  hypervisors: Hypervisor[];
}) {
  if (!node) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-gray-600 gap-3">
        <Server className="w-12 h-12 opacity-30" />
        <p className="text-sm">Select a node from the tree to view details</p>
      </div>
    );
  }

  if (node.type === "provider") {
    const hv = hypervisors.find((h) => h.id === node.id);
    return <ProviderDetail node={node} hypervisor={hv} />;
  }

  if (node.type === "cluster") {
    return <ClusterDetail node={node} />;
  }

  if (node.type === "host") {
    return <HostDetailPanel node={node} />;
  }

  return (
    <div className="p-4 text-gray-400 text-sm">
      No detail view for type: {node.type}
    </div>
  );
}

// ── Provider detail ───────────────────────────────────────────────────────────
function ProviderDetail({ node, hypervisor }: { node: InfraTreeNode; hypervisor?: Hypervisor }) {
  const clusterCount = (node.children ?? []).filter((c) => c.type === "cluster").length;
  const hostCount = countNodes(node, "host");

  return (
    <div className="p-5 space-y-5">
      <div className="flex items-start gap-3">
        <div className="p-2.5 bg-blue-900/30 rounded-xl">
          <Globe className="w-6 h-6 text-blue-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-white">{node.name}</h2>
          <div className="flex items-center gap-2 mt-1">
            <StatusDot status={node.status} />
            <span className="text-sm text-gray-400 capitalize">{node.status}</span>
            {node.provider_type && (
              <ProviderBadge provider={node.provider_type as never} />
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <StatCard label="Clusters" value={clusterCount} icon={Layers} color="text-purple-400" />
        <StatCard label="Hosts" value={hostCount} icon={Server} color="text-green-400" />
        <StatCard label="VMs" value={node.vm_count} icon={Cpu} color="text-blue-400" />
      </div>

      {hypervisor && (
        <div className="bg-gray-800/50 rounded-xl p-4 space-y-2">
          <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Connection</h3>
          <InfoRow label="Host" value={`${hypervisor.host}:${hypervisor.port}`} mono />
          <InfoRow label="Provider" value={hypervisor.provider} />
          <InfoRow label="TLS Verify" value={hypervisor.tls_verify ? "Yes" : "No"} />
          {hypervisor.last_checked_at && (
            <InfoRow label="Last Checked" value={new Date(hypervisor.last_checked_at).toLocaleString()} />
          )}
        </div>
      )}
    </div>
  );
}

// ── Cluster detail ────────────────────────────────────────────────────────────
function ClusterDetail({ node }: { node: InfraTreeNode }) {
  const hostCount = (node.children ?? []).filter((c) => c.type === "host").length;
  const meta = node.metadata ?? {};

  return (
    <div className="p-5 space-y-5">
      <div className="flex items-start gap-3">
        <div className="p-2.5 bg-purple-900/30 rounded-xl">
          <Layers className="w-6 h-6 text-purple-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-white">{node.name}</h2>
          <p className="text-sm text-gray-400 mt-0.5">Cluster</p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <StatCard label="Hosts" value={hostCount} icon={Server} color="text-green-400" />
        <StatCard label="VMs" value={node.vm_count} icon={Cpu} color="text-blue-400" />
      </div>

      {(meta.total_cpu || meta.total_memory_mb) && (
        <div className="bg-gray-800/50 rounded-xl p-4 space-y-2">
          <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Resources</h3>
          {meta.total_cpu ? <InfoRow label="Total CPU" value={`${meta.total_cpu} cores`} /> : null}
          {meta.total_memory_mb ? (
            <InfoRow label="Total Memory" value={formatMB(meta.total_memory_mb as number)} />
          ) : null}
        </div>
      )}

      {/* Host list */}
      {(node.children ?? []).length > 0 && (
        <div>
          <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Hosts</h3>
          <div className="space-y-2">
            {node.children!.map((host) => (
              <div key={host.id} className="flex items-center gap-3 p-3 bg-gray-800/50 rounded-lg">
                <StatusDot status={host.status} />
                <Server className="w-4 h-4 text-green-400 shrink-0" />
                <span className="text-sm text-gray-200 flex-1">{host.name}</span>
                <span className="text-xs text-gray-500">{host.vm_count} VMs</span>
                <Link
                  href={`/dashboard/infrastructure/hosts/${host.id}`}
                  className="text-xs text-blue-400 hover:text-blue-300"
                >
                  Details →
                </Link>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Host detail panel (summary) ───────────────────────────────────────────────
function HostDetailPanel({ node }: { node: InfraTreeNode }) {
  const meta = node.metadata ?? {};

  return (
    <div className="p-5 space-y-5">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <div className="p-2.5 bg-green-900/30 rounded-xl">
            <Server className="w-6 h-6 text-green-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">{node.name}</h2>
            <div className="flex items-center gap-2 mt-1">
              <StatusDot status={node.status} />
              <span className="text-sm text-gray-400 capitalize">{node.status}</span>
            </div>
          </div>
        </div>
        <Link
          href={`/dashboard/infrastructure/hosts/${node.id}`}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs rounded-lg transition-colors"
        >
          Full Details →
        </Link>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <StatCard label="VMs" value={node.vm_count} icon={Cpu} color="text-blue-400" />
        {meta.cpu_cores ? (
          <StatCard label="CPU Cores" value={meta.cpu_cores as number} icon={Activity} color="text-orange-400" />
        ) : null}
      </div>

      <div className="bg-gray-800/50 rounded-xl p-4 space-y-2">
        <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">Compute</h3>
        {meta.cpu_model ? <InfoRow label="CPU Model" value={meta.cpu_model as string} /> : null}
        {meta.cpu_sockets ? <InfoRow label="Sockets" value={String(meta.cpu_sockets)} /> : null}
        {meta.cpu_cores ? <InfoRow label="Cores" value={String(meta.cpu_cores)} /> : null}
        {meta.total_memory_mb ? (
          <InfoRow label="Total RAM" value={formatMB(meta.total_memory_mb as number)} />
        ) : null}
        {meta.used_memory_mb ? (
          <InfoRow label="Used RAM" value={formatMB(meta.used_memory_mb as number)} />
        ) : null}
        {meta.hypervisor_version ? (
          <InfoRow label="Version" value={meta.hypervisor_version as string} />
        ) : null}
        {meta.uptime_seconds ? (
          <InfoRow label="Uptime" value={formatUptime(meta.uptime_seconds as number)} />
        ) : null}
      </div>
    </div>
  );
}

// ── Shared helpers ────────────────────────────────────────────────────────────
function StatCard({
  label, value, icon: Icon, color,
}: { label: string; value: number; icon: React.ElementType; color: string }) {
  return (
    <div className="bg-gray-800/50 rounded-xl p-3 flex items-center gap-3">
      <Icon className={`w-5 h-5 ${color} shrink-0`} />
      <div>
        <p className="text-lg font-bold text-white">{value}</p>
        <p className="text-xs text-gray-500">{label}</p>
      </div>
    </div>
  );
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="text-xs text-gray-500 shrink-0">{label}</span>
      <span className={`text-xs text-gray-300 text-right ${mono ? "font-mono" : ""}`}>{value}</span>
    </div>
  );
}

function countNodes(node: InfraTreeNode, type: string): number {
  let count = 0;
  for (const child of node.children ?? []) {
    if (child.type === type) count++;
    count += countNodes(child, type);
  }
  return count;
}

function formatMB(mb: number): string {
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(1)} TB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

// ── Main page ─────────────────────────────────────────────────────────────────
export default function InfrastructurePage() {
  const [selectedNode, setSelectedNode] = useState<InfraTreeNode | null>(null);
  const [search, setSearch] = useState("");
  const [hypervisorFilter, setHypervisorFilter] = useState("");

  const { data: tree = [], isLoading, refetch } = useQuery({
    queryKey: ["infra-tree", hypervisorFilter],
    queryFn: () => infrastructureApi.getTree(hypervisorFilter || undefined),
    staleTime: 30_000,
  });

  const { data: hypervisorsData } = useQuery({
    queryKey: ["hypervisors-list"],
    queryFn: () => hypervisorApi.list({ page: 1, page_size: 100 }),
    staleTime: 60_000,
  });
  const hypervisors: Hypervisor[] = hypervisorsData?.data ?? [];

  // Summary stats
  const stats = useMemo(() => {
    let providers = 0, clusters = 0, hosts = 0, vms = 0;
    function walk(nodes: InfraTreeNode[]) {
      for (const n of nodes) {
        if (n.type === "provider") providers++;
        else if (n.type === "cluster") clusters++;
        else if (n.type === "host") { hosts++; vms += n.vm_count; }
        walk(n.children ?? []);
      }
    }
    walk(tree);
    return { providers, clusters, hosts, vms };
  }, [tree]);

  return (
    <div className="flex flex-col h-full gap-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Infrastructure Explorer</h1>
          <p className="text-gray-400 text-sm mt-0.5">
            {stats.providers} providers · {stats.clusters} clusters · {stats.hosts} hosts · {stats.vms} VMs
          </p>
        </div>
        <div className="flex items-center gap-2">
          {hypervisors.length > 1 && (
            <select
              value={hypervisorFilter}
              onChange={(e) => setHypervisorFilter(e.target.value)}
              className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500"
            >
              <option value="">All providers</option>
              {hypervisors.map((h) => (
                <option key={h.id} value={h.id}>{h.name}</option>
              ))}
            </select>
          )}
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
          >
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: "Providers", value: stats.providers, icon: Globe, color: "text-blue-400", bg: "bg-blue-900/20" },
          { label: "Clusters", value: stats.clusters, icon: Layers, color: "text-purple-400", bg: "bg-purple-900/20" },
          { label: "Hosts", value: stats.hosts, icon: Server, color: "text-green-400", bg: "bg-green-900/20" },
          { label: "VMs", value: stats.vms, icon: Cpu, color: "text-orange-400", bg: "bg-orange-900/20" },
        ].map((s) => (
          <div key={s.label} className={`${s.bg} border border-gray-800 rounded-xl p-4 flex items-center gap-3`}>
            <s.icon className={`w-6 h-6 ${s.color} shrink-0`} />
            <div>
              <p className="text-xl font-bold text-white">{s.value}</p>
              <p className="text-xs text-gray-500">{s.label}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Main split panel */}
      <div className="flex gap-4 flex-1 min-h-0">
        {/* Tree panel */}
        <div className="w-72 shrink-0 bg-gray-900 border border-gray-800 rounded-2xl flex flex-col overflow-hidden">
          <div className="p-3 border-b border-gray-800">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search infrastructure…"
                className="w-full pl-8 pr-3 py-1.5 bg-gray-800 border border-gray-700 rounded-lg text-xs text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div className="flex-1 overflow-y-auto py-2">
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <div className="w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : tree.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
                <AlertCircle className="w-8 h-8 text-gray-600 mb-2" />
                <p className="text-xs text-gray-500">
                  No infrastructure found. Add a hypervisor and sync inventory to populate the tree.
                </p>
              </div>
            ) : (
              tree.map((node) => (
                <TreeNode
                  key={node.id}
                  node={node}
                  selectedId={selectedNode?.id ?? null}
                  onSelect={setSelectedNode}
                  searchQuery={search}
                />
              ))
            )}
          </div>
        </div>

        {/* Detail panel */}
        <div className="flex-1 bg-gray-900 border border-gray-800 rounded-2xl overflow-y-auto">
          <DetailPanel node={selectedNode} hypervisors={hypervisors} />
        </div>
      </div>
    </div>
  );
}
