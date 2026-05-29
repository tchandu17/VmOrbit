"use client";
import { useQuery } from "@tanstack/react-query";
import { Shield, AlertOctagon, MessageSquare, CheckCircle2, XCircle, Clock, TrendingUp } from "lucide-react";
import Link from "next/link";
import { policyApi, approvalApi } from "@/lib/api/policy";
import { cn } from "@/lib/utils";

export default function GovernancePage() {
  const { data: policiesData } = useQuery({
    queryKey: ["policies-summary"],
    queryFn: () => policyApi.list({ page_size: 100 }),
  });

  const { data: violationsData } = useQuery({
    queryKey: ["violations-summary"],
    queryFn: () => policyApi.listViolations({ page_size: 5 }),
  });

  const { data: approvalsData } = useQuery({
    queryKey: ["approvals-summary"],
    queryFn: () => approvalApi.list({ page_size: 5 }),
  });

  const { data: pendingData } = useQuery({
    queryKey: ["approvals-pending-count"],
    queryFn: () => approvalApi.getPending({ page_size: 1 }),
    refetchInterval: 30000,
  });

  const policies = policiesData?.data ?? [];
  const violations = violationsData?.data ?? [];
  const approvals = approvalsData?.data ?? [];
  const pendingCount = pendingData?.meta?.total_items ?? 0;

  const enabledPolicies = policies.filter(p => p.enabled).length;
  const blockedViolations = (violationsData?.meta?.total_items ?? 0);
  const totalApprovals = approvalsData?.meta?.total_items ?? 0;

  const effectCounts = policies.reduce<Record<string, number>>((acc, p) => {
    acc[p.effect] = (acc[p.effect] ?? 0) + 1;
    return acc;
  }, {});

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Shield className="w-6 h-6 text-blue-400" />Governance
        </h1>
        <p className="text-gray-400 text-sm mt-0.5">
          Policy enforcement, approval workflows, and compliance overview
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          { label: "Active Policies", value: enabledPolicies, total: policies.length, icon: Shield, color: "text-blue-400", bg: "bg-blue-900/20 border-blue-800/50", href: "/dashboard/governance/policies" },
          { label: "Pending Approvals", value: pendingCount, icon: Clock, color: "text-yellow-400", bg: "bg-yellow-900/20 border-yellow-800/50", href: "/dashboard/governance/approvals" },
          { label: "Total Violations", value: blockedViolations, icon: AlertOctagon, color: "text-red-400", bg: "bg-red-900/20 border-red-800/50", href: "/dashboard/governance/violations" },
          { label: "Total Approvals", value: totalApprovals, icon: MessageSquare, color: "text-purple-400", bg: "bg-purple-900/20 border-purple-800/50", href: "/dashboard/governance/approvals?tab=all" },
        ].map(({ label, value, total, icon: Icon, color, bg, href }) => (
          <Link key={label} href={href}
            className={cn("border rounded-xl p-5 hover:opacity-90 transition-opacity", bg)}>
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-gray-400">{label}</span>
              <Icon className={cn("w-5 h-5", color)} />
            </div>
            <p className="text-3xl font-bold text-white">{value}</p>
            {total !== undefined && (
              <p className="text-xs text-gray-500 mt-1">{total} total configured</p>
            )}
          </Link>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Policy breakdown */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-white font-semibold flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-blue-400" />Policy Effects
            </h2>
            <Link href="/dashboard/governance/policies" className="text-xs text-blue-400 hover:text-blue-300">View all</Link>
          </div>
          {policies.length === 0 ? (
            <p className="text-gray-600 text-sm">No policies configured yet.</p>
          ) : (
            <div className="space-y-3">
              {[
                { effect: "deny", label: "Deny", color: "bg-red-500" },
                { effect: "require_approval", label: "Require Approval", color: "bg-yellow-500" },
                { effect: "require_snapshot", label: "Require Snapshot", color: "bg-blue-500" },
                { effect: "require_justification", label: "Require Justification", color: "bg-purple-500" },
                { effect: "allow", label: "Allow", color: "bg-green-500" },
              ].map(({ effect, label, color }) => {
                const count = effectCounts[effect] ?? 0;
                const pct = policies.length > 0 ? Math.round((count / policies.length) * 100) : 0;
                return (
                  <div key={effect}>
                    <div className="flex items-center justify-between text-xs mb-1">
                      <span className="text-gray-400">{label}</span>
                      <span className="text-gray-300">{count}</span>
                    </div>
                    <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                      <div className={cn("h-full rounded-full", color)} style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Recent violations */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-white font-semibold flex items-center gap-2">
              <AlertOctagon className="w-4 h-4 text-red-400" />Recent Violations
            </h2>
            <Link href="/dashboard/governance/violations" className="text-xs text-blue-400 hover:text-blue-300">View all</Link>
          </div>
          {violations.length === 0 ? (
            <p className="text-gray-600 text-sm">No violations recorded.</p>
          ) : (
            <div className="space-y-2">
              {violations.map(v => (
                <div key={v.id} className="flex items-start gap-3 py-2 border-b border-gray-800/50 last:border-0">
                  <div className={cn("w-2 h-2 rounded-full mt-1.5 shrink-0", v.status === "blocked" ? "bg-red-400" : v.status === "pending_approval" ? "bg-yellow-400" : "bg-green-400")} />
                  <div className="min-w-0">
                    <p className="text-sm text-gray-300 truncate">{v.operation}</p>
                    <p className="text-xs text-gray-500">{v.policy_name} · {v.username}</p>
                  </div>
                  <span className="text-xs text-gray-600 shrink-0">{new Date(v.created_at).toLocaleDateString()}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Recent approvals */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-white font-semibold flex items-center gap-2">
              <MessageSquare className="w-4 h-4 text-purple-400" />Recent Approvals
            </h2>
            <Link href="/dashboard/governance/approvals" className="text-xs text-blue-400 hover:text-blue-300">View all</Link>
          </div>
          {approvals.length === 0 ? (
            <p className="text-gray-600 text-sm">No approval requests yet.</p>
          ) : (
            <div className="space-y-2">
              {approvals.map(a => (
                <div key={a.id} className="flex items-start gap-3 py-2 border-b border-gray-800/50 last:border-0">
                  <div className="shrink-0 mt-0.5">
                    {a.status === "approved" ? <CheckCircle2 className="w-4 h-4 text-green-400" />
                      : a.status === "rejected" ? <XCircle className="w-4 h-4 text-red-400" />
                      : <Clock className="w-4 h-4 text-yellow-400" />}
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm text-gray-300 truncate">{a.operation}</p>
                    <p className="text-xs text-gray-500">{a.requester_name} · {a.resource_name || a.resource_id}</p>
                  </div>
                  <span className={cn("text-xs shrink-0 px-1.5 py-0.5 rounded",
                    a.status === "pending" ? "bg-yellow-900/40 text-yellow-300" :
                    a.status === "approved" ? "bg-green-900/40 text-green-300" :
                    "bg-red-900/40 text-red-300"
                  )}>{a.status}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Quick links */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          { href: "/dashboard/governance/policies", icon: Shield, label: "Manage Policies", desc: "Create, edit, and assign governance policies", color: "text-blue-400" },
          { href: "/dashboard/governance/approvals", icon: MessageSquare, label: "Approval Inbox", desc: "Review and act on pending approval requests", color: "text-yellow-400" },
          { href: "/dashboard/governance/violations", icon: AlertOctagon, label: "Violation Log", desc: "Audit trail of all policy enforcement events", color: "text-red-400" },
        ].map(({ href, icon: Icon, label, desc, color }) => (
          <Link key={href} href={href}
            className="bg-gray-900 border border-gray-800 hover:border-gray-700 rounded-2xl p-5 transition-colors group">
            <Icon className={cn("w-6 h-6 mb-3", color)} />
            <p className="text-white font-medium text-sm group-hover:text-blue-300 transition-colors">{label}</p>
            <p className="text-gray-500 text-xs mt-1">{desc}</p>
          </Link>
        ))}
      </div>
    </div>
  );
}
