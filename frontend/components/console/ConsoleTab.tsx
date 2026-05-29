"use client";
import { useState, useCallback } from "react";
import { Monitor, Info } from "lucide-react";
import { vmApi } from "@/lib/api/vms";
import { ConsoleViewer } from "./ConsoleViewer";
import type { VM, ConsoleSession } from "@/types";

interface ConsoleTabProps {
  vm: VM;
}

export function ConsoleTab({ vm }: ConsoleTabProps) {
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const requestSession = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    setSession(null);
    try {
      const s = await vmApi.getConsole(vm.id);
      setSession(s);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to request console session";
      setError(msg);
    } finally {
      setIsLoading(false);
    }
  }, [vm.id]);

  const isStopped = vm.status === "stopped" || vm.status === "suspended" || vm.status === "paused";

  return (
    <div className="space-y-4">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Monitor className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-semibold text-white">Interactive Console</h3>
        </div>
        <button
          onClick={requestSession}
          disabled={isLoading || isStopped}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm rounded-lg transition-colors"
        >
          <Monitor className="w-3.5 h-3.5" />
          {isLoading ? "Connecting…" : session ? "Reconnect" : "Launch Console"}
        </button>
      </div>

      {/* Stopped VM notice */}
      {isStopped && (
        <div className="flex items-start gap-3 px-4 py-3 bg-yellow-900/20 border border-yellow-800/40 rounded-xl text-sm text-yellow-300">
          <Info className="w-4 h-4 shrink-0 mt-0.5 text-yellow-400" />
          <span>
            Console is only available when the VM is running. Current status:{" "}
            <span className="font-semibold capitalize">{vm.status}</span>.
          </span>
        </div>
      )}

      {/* Console viewer */}
      <ConsoleViewer
        session={session}
        isLoading={isLoading}
        error={error}
        onRequestSession={requestSession}
      />

      {/* Session info */}
      {session && (
        <div className="flex flex-wrap items-center gap-x-6 gap-y-1 text-xs text-gray-500 px-1">
          {session.console_type && (
            <span>
              Type: <span className="text-gray-400 font-medium">{session.console_type.toUpperCase()}</span>
            </span>
          )}
          {session.provider && (
            <span>
              Provider: <span className="text-gray-400 font-medium capitalize">{session.provider}</span>
            </span>
          )}
          {session.session_id && (
            <span>
              Session: <span className="text-gray-400 font-mono">{session.session_id.slice(0, 8)}…</span>
            </span>
          )}
          {session.expires_at && (
            <span>
              Expires: <span className="text-gray-400">{new Date(session.expires_at).toLocaleTimeString()}</span>
            </span>
          )}
        </div>
      )}
    </div>
  );
}
