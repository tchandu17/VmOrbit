"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, CheckCircle2, XCircle, Clock, MinusCircle, History } from "lucide-react";
import { notificationHistoryApi, type HistoryListParams } from "@/lib/api/notifications";
import { cn, formatDate } from "@/lib/utils";
import type { NotificationHistory, NotificationStatus } from "@/types";

const STATUS_CONFIG: Record<NotificationStatus, { label: string; icon: React.ElementType; color: string }> = {
  delivered: { label: "Delivered", icon: CheckCircle2, color: "text-green-400" },
  failed:    { label: "Failed",    icon: XCircle,      color: "text-red-400"   },
  pending:   { label: "Pending",   icon: Clock,        color: "text-yellow-400"},
  throttled: { label: "Throttled", icon: MinusCircle,  color: "text-gray-400"  },
};

const PAGE_SIZE = 25;

export function HistoryTab() {
  const [page, setPage]     = useState(1);
  const [status, setStatus] = useState("");

  const params: HistoryListParams = {
    page, page_size: PAGE_SIZE,
    ...(status && { status }),
  };

  const { data, isLoading } = useQuery({
    queryKey: ["notification-history", params],
    queryFn: () => notificationHistoryApi.list(params),
  });

  const items: NotificationHistory[] = data?.data ?? [];
  const totalItems = data?.meta?.total_items ?? 0;
  const totalPages = data?.meta?.total_pages ?? 1;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-400">{totalItems.toLocaleString()} delivery records</p>
        <div className="flex gap-2">
          {(["", "delivered", "failed", "throttled"] as const).map((s) => (
            <button key={s} onClick={() => { setStatus(s); setPage(1); }}
              className={cn("px-3 py-1.5 rounded-lg text-xs font-medium transition-colors",
                status === s ? "bg-blue-600 text-white" : "bg-gray-800 text-gray-400 hover:text-white"
              )}>
              {s === "" ? "All" : s.charAt(0).toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800">
                {["Time", "Status", "Rule", "Channel", "Event", "Attempts", "Error"].map((h) => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-gray-800/50">
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><div className="h-4 bg-gray-800 rounded animate-pulse w-16" /></td>
                    ))}
                  </tr>
                ))
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-16 text-center">
                    <History className="w-10 h-10 mx-auto mb-3 text-gray-700" />
                    <p className="text-gray-500">No delivery history yet.</p>
                  </td>
                </tr>
              ) : (
                items.map((h) => <HistoryRow key={h.id} item={h} />)
              )}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <span className="text-xs text-gray-500">Page {page} of {totalPages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors">
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button disabled={page === totalPages} onClick={() => setPage((p) => p + 1)}
                className="p-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-40 text-gray-300 transition-colors">
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function HistoryRow({ item }: { item: NotificationHistory }) {
  const cfg = STATUS_CONFIG[item.status] ?? STATUS_CONFIG.pending;
  const Icon = cfg.icon;
  return (
    <tr className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">{formatDate(item.created_at)}</td>
      <td className="px-4 py-3">
        <span className={cn("flex items-center gap-1 text-xs font-medium", cfg.color)}>
          <Icon className="w-3.5 h-3.5" />{cfg.label}
        </span>
      </td>
      <td className="px-4 py-3 text-sm text-gray-300">{item.rule?.name ?? item.rule_id.slice(0, 8)}</td>
      <td className="px-4 py-3 text-sm text-gray-400">{item.channel?.name ?? item.channel_id.slice(0, 8)}</td>
      <td className="px-4 py-3 text-xs text-gray-500 font-mono">{item.event?.event_type ?? item.event_id.slice(0, 8)}</td>
      <td className="px-4 py-3 text-xs text-gray-500 text-center">{item.attempt_count}</td>
      <td className="px-4 py-3 text-xs text-red-400 max-w-xs">
        {item.error_message ? (
          <span className="truncate block" title={item.error_message}>{item.error_message}</span>
        ) : "—"}
      </td>
    </tr>
  );
}
