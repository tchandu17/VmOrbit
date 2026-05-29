"use client";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  RefreshCw, Copy, Server, CheckCircle2, XCircle,
  Clock, Loader2, ChevronLeft, ChevronRight, ExternalLink,
} from "lucide-react";
import Link from "next/link";
import { provisioningApi } from "@/lib/api/provisioning";
import { hypervisorApi } from "@/lib/api/hypervisors";
import { ProviderBadge } from "@/components/hypervisors/ProviderBadge";
import type { ProvisioningJob, ProvisioningJobStatus, Hypervisor } from "@/types";

function StatusBadge({ status }: { status: ProvisioningJobStatus }) {
  const cfg: Record<ProvisioningJobStatus, { label: string; cls: string; icon: React.ReactNode }> = {
    pending:   { label: "Pending",   cls: "bg-yellow-900/40 text-yellow-300 border-yellow-800", icon: <Clock className="w-3 h-3" /> },
    running:   { label: "Running",   cls: "bg-blue-900/40 text-blue-300 border-blue-800",       icon: <Loader2 className="w-3 h-3 animate-spin" /> },
    completed: { label: "Completed", cls: "bg-green-900/40 text-green-300 border-green-800",    icon: <CheckCircle2 className="w-3 h-3" /> },
    failed:    { label: "Failed",    cls: "bg-red-900/40 text-red-300 border-red-800",          icon: <XCircle className="w-3 h-3" /> },
    cancelled: { label: "Cancelled", cls: "bg-gray-800 text-gray-400 border-gray-700",          icon: <XCircle className="w-3 h-3" /> },
  };
  const { label, cls, icon } = cfg[status] ?? cfg.pending;
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border ${cls}`}>
      {icon}{label}
    </span>
  );
}

function TypeBadge({ type }: { type: "clone" | "provision" }) {
  return type === "clone"
    ? <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-purple-900/40 text-purple-300 border border-purple-800"><Copy className="w-3 h-3" />Clone</span>
    : <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-900/40 text-blue-300 border border-blue-800"><Server className="w-3 h-3" />Provision</span>;
}

export default function ProvisioningPage() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [hypervisorFilter, setHypervisorFilter] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["provisioning-jobs", { page, typeFilter, statusFilter, hypervisorFilter }],
    queryFn: () =>
      provisioningApi.listJobs({
        page,
        page_size: 20,
        type: typeFilter || undefined,
        status: statusFilter || undefined,
        hypervisor_id: hypervisorFilter || undefined,
      }),
    refetchInterval: 5000, // auto-refresh for running jobs
  });

  const { data: hypervisorsData } = useQuery({
    queryKey: ["hypervisors-list"],
    queryFn: () => hypervisorApi.list({ page: 1, page_size: 100 }),
  });

  const hypervisors: Hypervisor[] = hypervisorsData?.data ?? [];
  const hypervisorMap = new Map<string, Hypervisor>(hypervisors.map((h) => [h.id, h]));
  const jobs: ProvisioningJob[] = data?.data ?? [];

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Provisioning</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.meta?.total_items ?? 0} jobs</p>
        </div>
        <div className="flex items-center gap-2">
          <Link href="/dashboard/templates"
            className="flex items-center gap-2 px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors">
            <Server className="w-4 h-4" /> Browse Templates
          </Link>
          <button onClick={() => queryClient.invalidateQueries({ queryKey: ["provisioning-jobs"] })}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors">
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 flex-wrap">
        <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); }}
          className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="">All types</option>
          <option value="clone">Clone</option>
          <option value="provision">Provision</option>
        </select>
        <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
          <option value="">All statuses</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
        {hypervisors.length > 1 && (
          <select value={hypervisorFilter} onChange={(e) => { setHypervisorFilter(e.target.value); setPage(1); }}
            className="bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 px-3 py-2 focus:outline-none focus:border-blue-500">
            <option value="">All hypervisors</option>
            {hypervisors.map((h) => <option key={h.id} value={h.id}>{h.name}</option>)}
          </select>
        )}
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800">
                {["VM Name", "Type", "Status", "Provider", "Template / Source", "CPU", "RAM", "Disk", "Task", "Created"].map((h) => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 10 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-20" /></td>
                    ))}
                  </tr>
                ))
              ) : jobs.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-4 py-12 text-center text-gray-500">
                    No provisioning jobs yet. Go to <Link href="/dashboard/templates" className="text-blue-400 hover:underline">Templates</Link> to provision a VM.
                  </td>
                </tr>
              ) : (
                jobs.map((job) => {
                  const hv = hypervisorMap.get(job.hypervisor_id);
                  const sourceName = job.template?.name ?? job.source_vm?.name ?? "—";
                  return (
                    <tr key={job.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-white">{job.vm_name}</span>
                          {job.result_vm_id && (
                            <Link href={`/dashboard/vms/${job.result_vm_id}`} title="View VM">
                              <ExternalLink className="w-3.5 h-3.5 text-blue-400 hover:text-blue-300" />
                            </Link>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3"><TypeBadge type={job.type} /></td>
                      <td className="px-4 py-3"><StatusBadge status={job.status} /></td>
                      <td className="px-4 py-3">{hv ? <ProviderBadge provider={hv.provider} /> : <span className="text-gray-600 text-xs">—</span>}</td>
                      <td className="px-4 py-3 text-gray-400 text-xs max-w-[160px] truncate" title={sourceName}>{sourceName}</td>
                      <td className="px-4 py-3 text-gray-400">{job.cpu_count > 0 ? `${job.cpu_count} vCPU` : "—"}</td>
                      <td className="px-4 py-3 text-gray-400">{job.memory_mb > 0 ? `${job.memory_mb} MB` : "—"}</td>
                      <td className="px-4 py-3 text-gray-400">{job.disk_gb > 0 ? `${job.disk_gb} GB` : "—"}</td>
                      <td className="px-4 py-3">
                        {job.task_id ? (
                          <Link href={`/dashboard/tasks`} className="text-xs text-blue-400 hover:underline font-mono">
                            {job.task_id.slice(0, 8)}…
                          </Link>
                        ) : <span className="text-gray-600 text-xs">—</span>}
                      </td>
                      <td className="px-4 py-3 text-gray-500 text-xs whitespace-nowrap">
                        {new Date(job.created_at).toLocaleString()}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {data && (data.meta?.total_pages ?? 0) > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {data.meta?.total_pages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
                <ChevronLeft className="w-3.5 h-3.5" /> Previous
              </button>
              <button disabled={page === (data.meta?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
                className="flex items-center gap-1 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 disabled:opacity-40 rounded-lg text-gray-300 transition-colors">
                Next <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
