"use client";
import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, XCircle, Clock, AlertTriangle, ChevronDown, ChevronUp, MessageSquare, ArrowUpCircle, X, Check, Ban } from "lucide-react";
import { approvalApi } from "@/lib/api/policy";
import { cn } from "@/lib/utils";
import type { ApprovalRequest, ApprovalStatus } from "@/types";

const STATUS_STYLES: Record<ApprovalStatus, string> = {
  pending:   "bg-yellow-900/40 text-yellow-300 border-yellow-700",
  approved:  "bg-green-900/40 text-green-300 border-green-700",
  rejected:  "bg-red-900/40 text-red-300 border-red-700",
  expired:   "bg-gray-800 text-gray-400 border-gray-700",
  escalated: "bg-purple-900/40 text-purple-300 border-purple-700",
  cancelled: "bg-gray-800 text-gray-500 border-gray-700",
};

const STATUS_ICONS: Record<ApprovalStatus, React.ElementType> = {
  pending:   Clock,
  approved:  CheckCircle2,
  rejected:  XCircle,
  expired:   AlertTriangle,
  escalated: ArrowUpCircle,
  cancelled: Ban,
};

type Tab = "inbox" | "all" | "mine";

// ── Comment Modal ─────────────────────────────────────────────────────────────
function CommentModal({ title, confirmLabel, confirmClass, onConfirm, onClose }: {
  title: string; confirmLabel: string; confirmClass: string;
  onConfirm: (comment: string) => void; onClose: () => void;
}) {
  const [comment, setComment] = useState("");
  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between p-5 border-b border-gray-800">
          <h2 className="text-white font-semibold">{title}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X className="w-5 h-5" /></button>
        </div>
        <div className="p-5">
          <label className="text-xs text-gray-400 mb-1 block">Comment (optional)</label>
          <textarea value={comment} onChange={e => setComment(e.target.value)} rows={3} placeholder="Add a comment…"
            className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 resize-none" />
        </div>
        <div className="flex justify-end gap-3 p-5 border-t border-gray-800">
          <button onClick={onClose} className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg">Cancel</button>
          <button onClick={() => onConfirm(comment)} className={cn("px-4 py-2 text-sm text-white rounded-lg", confirmClass)}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  );
}

// ── Approval Detail Row ───────────────────────────────────────────────────────
function ApprovalRow({ request, showActions, onApprove, onReject, onCancel, onEscalate }: {
  request: ApprovalRequest;
  showActions: boolean;
  onApprove: () => void;
  onReject: () => void;
  onCancel: () => void;
  onEscalate: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const StatusIcon = STATUS_ICONS[request.status];
  const isPending = request.status === "pending";

  return (
    <>
      <tr className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
        <td className="px-4 py-3">
          <div>
            <p className="text-white font-medium text-sm">{request.operation}</p>
            <p className="text-gray-500 text-xs">{request.resource_type}: {request.resource_name || request.resource_id}</p>
          </div>
        </td>
        <td className="px-4 py-3 text-gray-400 text-sm">{request.policy_name}</td>
        <td className="px-4 py-3 text-gray-400 text-sm">{request.requester_name}</td>
        <td className="px-4 py-3">
          <span className={cn("inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded border", STATUS_STYLES[request.status])}>
            <StatusIcon className="w-3 h-3" />{request.status}
          </span>
        </td>
        <td className="px-4 py-3 text-gray-500 text-xs">
          {request.expires_at ? new Date(request.expires_at).toLocaleString() : "—"}
        </td>
        <td className="px-4 py-3">
          <div className="flex items-center gap-1">
            <button onClick={() => setExpanded(p => !p)} className="p-1.5 rounded hover:bg-gray-700 text-gray-400" title="Details">
              {expanded ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
            </button>
            {showActions && isPending && (
              <>
                <button onClick={onApprove} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-green-400" title="Approve"><Check className="w-3.5 h-3.5" /></button>
                <button onClick={onReject} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-red-400" title="Reject"><X className="w-3.5 h-3.5" /></button>
                <button onClick={onEscalate} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-purple-400" title="Escalate"><ArrowUpCircle className="w-3.5 h-3.5" /></button>
              </>
            )}
            {isPending && request.status === "pending" && (
              <button onClick={onCancel} className="p-1.5 rounded hover:bg-gray-700 text-gray-400 hover:text-orange-400" title="Cancel"><Ban className="w-3.5 h-3.5" /></button>
            )}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-gray-800/30 bg-gray-900/50">
          <td colSpan={6} className="px-6 py-4">
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-3">
                {request.justification && (
                  <div>
                    <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">Justification</p>
                    <p className="text-sm text-gray-300 bg-gray-800 rounded-lg px-3 py-2">{request.justification}</p>
                  </div>
                )}
                <div>
                  <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">Approval Steps</p>
                  <div className="space-y-1">
                    {(request.steps ?? []).map(step => (
                      <div key={step.id} className="flex items-center gap-2 text-xs">
                        <span className={cn("w-2 h-2 rounded-full", step.status === "approved" ? "bg-green-400" : step.status === "rejected" ? "bg-red-400" : "bg-yellow-400")} />
                        <span className="text-gray-400">Step {step.step_order}:</span>
                        <span className="text-gray-300">{step.approver_name || step.approver_role || step.approver_id}</span>
                        <span className={cn("px-1.5 py-0.5 rounded text-[10px]", step.status === "approved" ? "bg-green-900/40 text-green-300" : step.status === "rejected" ? "bg-red-900/40 text-red-300" : "bg-yellow-900/40 text-yellow-300")}>{step.status}</span>
                        {step.comment && <span className="text-gray-500 italic">"{step.comment}"</span>}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">History</p>
                <div className="space-y-1 max-h-32 overflow-y-auto">
                  {(request.history ?? []).map(h => (
                    <div key={h.id} className="flex items-start gap-2 text-xs">
                      <span className="text-gray-600 shrink-0">{new Date(h.created_at).toLocaleTimeString()}</span>
                      <span className="text-gray-400">{h.actor_name}</span>
                      <span className="text-gray-300">{h.action}</span>
                      {h.comment && <span className="text-gray-500 italic">"{h.comment}"</span>}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

// ── Escalate Modal ────────────────────────────────────────────────────────────
function EscalateModal({ onConfirm, onClose }: { onConfirm: (to: string, comment: string) => void; onClose: () => void }) {
  const [to, setTo] = useState("");
  const [comment, setComment] = useState("");
  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between p-5 border-b border-gray-800">
          <h2 className="text-white font-semibold">Escalate Approval</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X className="w-5 h-5" /></button>
        </div>
        <div className="p-5 space-y-3">
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Escalate To (User ID)</label>
            <input value={to} onChange={e => setTo(e.target.value)} placeholder="User UUID"
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Reason</label>
            <textarea value={comment} onChange={e => setComment(e.target.value)} rows={2} placeholder="Why are you escalating?"
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 resize-none" />
          </div>
        </div>
        <div className="flex justify-end gap-3 p-5 border-t border-gray-800">
          <button onClick={onClose} className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg">Cancel</button>
          <button onClick={() => onConfirm(to, comment)} disabled={!to} className="px-4 py-2 text-sm bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white rounded-lg">Escalate</button>
        </div>
      </div>
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────
export default function ApprovalsPage() {
  const qc = useQueryClient();
  const [tab, setTab] = useState<Tab>("inbox");
  const [statusFilter, setStatusFilter] = useState("");
  const [actionModal, setActionModal] = useState<{ type: "approve" | "reject"; id: string } | null>(null);
  const [escalateId, setEscalateId] = useState<string | null>(null);
  const [toasts, setToasts] = useState<{ id: number; type: "success" | "error"; text: string }[]>([]);

  const addToast = useCallback((type: "success" | "error", text: string) => {
    const id = Date.now();
    setToasts(p => [...p, { id, type, text }]);
    setTimeout(() => setToasts(p => p.filter(t => t.id !== id)), 4000);
  }, []);

  const { data: inboxData, isLoading: inboxLoading } = useQuery({
    queryKey: ["approvals-inbox"],
    queryFn: () => approvalApi.getPending({ page_size: 50 }),
    enabled: tab === "inbox",
    refetchInterval: 30000,
  });

  const { data: allData, isLoading: allLoading } = useQuery({
    queryKey: ["approvals-all", statusFilter],
    queryFn: () => approvalApi.list({ status: statusFilter || undefined, page_size: 50 }),
    enabled: tab === "all",
  });

  const { data: mineData, isLoading: mineLoading } = useQuery({
    queryKey: ["approvals-mine"],
    queryFn: () => approvalApi.list({ page_size: 50 }),
    enabled: tab === "mine",
  });

  const approveMut = useMutation({
    mutationFn: ({ id, comment }: { id: string; comment: string }) => approvalApi.approve(id, comment),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["approvals"] }); addToast("success", "Request approved"); setActionModal(null); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const rejectMut = useMutation({
    mutationFn: ({ id, comment }: { id: string; comment: string }) => approvalApi.reject(id, comment),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["approvals"] }); addToast("success", "Request rejected"); setActionModal(null); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const cancelMut = useMutation({
    mutationFn: (id: string) => approvalApi.cancel(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["approvals"] }); addToast("success", "Request cancelled"); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const escalateMut = useMutation({
    mutationFn: ({ id, to, comment }: { id: string; to: string; comment: string }) => approvalApi.escalate(id, to, comment),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["approvals"] }); addToast("success", "Request escalated"); setEscalateId(null); },
    onError: (e: Error) => addToast("error", e.message),
  });

  const activeData = tab === "inbox" ? inboxData : tab === "all" ? allData : mineData;
  const isLoading = tab === "inbox" ? inboxLoading : tab === "all" ? allLoading : mineLoading;
  const requests: ApprovalRequest[] = activeData?.data ?? [];
  const inboxCount = inboxData?.meta?.total_items ?? 0;

  return (
    <div className="space-y-5">
      {/* Toasts */}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map(t => (
          <div key={t.id} className={cn("flex items-center gap-2 px-4 py-3 rounded-xl border text-sm shadow-lg", t.type === "success" ? "bg-green-950 border-green-800 text-green-200" : "bg-red-950 border-red-800 text-red-200")}>
            {t.type === "success" ? <Check className="w-4 h-4" /> : <AlertTriangle className="w-4 h-4" />}{t.text}
          </div>
        ))}
      </div>

      {actionModal && (
        <CommentModal
          title={actionModal.type === "approve" ? "Approve Request" : "Reject Request"}
          confirmLabel={actionModal.type === "approve" ? "Approve" : "Reject"}
          confirmClass={actionModal.type === "approve" ? "bg-green-600 hover:bg-green-700" : "bg-red-600 hover:bg-red-700"}
          onClose={() => setActionModal(null)}
          onConfirm={comment => {
            if (actionModal.type === "approve") approveMut.mutate({ id: actionModal.id, comment });
            else rejectMut.mutate({ id: actionModal.id, comment });
          }}
        />
      )}
      {escalateId && (
        <EscalateModal
          onClose={() => setEscalateId(null)}
          onConfirm={(to, comment) => escalateMut.mutate({ id: escalateId, to, comment })}
        />
      )}

      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <MessageSquare className="w-6 h-6 text-yellow-400" />Approval Requests
        </h1>
        <p className="text-gray-400 text-sm mt-0.5">Review and act on pending infrastructure operation approvals</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-900 border border-gray-800 rounded-xl p-1 w-fit">
        {([["inbox", "My Inbox"], ["all", "All Requests"], ["mine", "My Requests"]] as [Tab, string][]).map(([t, label]) => (
          <button key={t} onClick={() => setTab(t)}
            className={cn("px-4 py-2 rounded-lg text-sm font-medium transition-colors flex items-center gap-1.5", tab === t ? "bg-blue-600 text-white" : "text-gray-400 hover:text-white hover:bg-gray-800")}>
            {label}
            {t === "inbox" && inboxCount > 0 && <span className="bg-yellow-500 text-black text-xs font-bold px-1.5 py-0.5 rounded-full">{inboxCount}</span>}
          </button>
        ))}
      </div>

      {tab === "all" && (
        <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-blue-500">
          <option value="">All statuses</option>
          {(["pending","approved","rejected","expired","escalated","cancelled"] as ApprovalStatus[]).map(s => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
      )}

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Operation / Resource", "Policy", "Requested By", "Status", "Expires", "Actions"].map(h => (
                <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? Array.from({ length: 3 }).map((_, i) => (
              <tr key={i} className="border-b border-gray-800/50">
                {Array.from({ length: 6 }).map((_, j) => (
                  <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-24" /></td>
                ))}
              </tr>
            )) : requests.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                {tab === "inbox" ? "No pending approvals requiring your action." : "No approval requests found."}
              </td></tr>
            ) : requests.map(r => (
              <ApprovalRow key={r.id} request={r}
                showActions={tab === "inbox"}
                onApprove={() => setActionModal({ type: "approve", id: r.id })}
                onReject={() => setActionModal({ type: "reject", id: r.id })}
                onCancel={() => { if (confirm("Cancel this approval request?")) cancelMut.mutate(r.id); }}
                onEscalate={() => setEscalateId(r.id)}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
