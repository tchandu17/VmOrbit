"use client";
import { useState, useEffect, useCallback, useRef } from "react";
import {
  Maximize2, Minimize2, RefreshCw, Loader2, WifiOff,
  MonitorOff, AlertTriangle, ExternalLink, Monitor,
  CheckCircle2, Clock,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { ConsoleSession } from "@/types";

// ── Types ─────────────────────────────────────────────────────────────────────

type ConnectionState = "idle" | "connecting" | "connected" | "launched" | "expired" | "error";

interface ConsoleViewerProps {
  session: ConsoleSession | null;
  isLoading: boolean;
  error: string | null;
  onRequestSession: () => void;
  className?: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function consoleTypeLabel(type?: string): string {
  switch (type) {
    case "webmks": return "VMware WebMKS";
    case "novnc":  return "Proxmox noVNC";
    case "vnc":    return "VNC";
    case "spice":  return "SPICE";
    default:       return type ? type.toUpperCase() : "Console";
  }
}

function providerLabel(provider?: string): string {
  switch (provider) {
    case "vmware":  return "VMware ESXi / vCenter";
    case "esxi":    return "VMware ESXi";
    case "proxmox": return "Proxmox VE";
    default:        return provider ?? "Unknown";
  }
}

/** Build the absolute WebSocket URL for the backend proxy. */
function buildProxyWsUrl(proxyPath: string, jwtToken: string): string {
  const proto = window.location.protocol === "https:" ? "wss" : "ws";
  const host = window.location.hostname;
  // Use the configured backend WS port (default 8080).
  // In production where frontend and backend are same-origin, set this to "".
  const backendPort = process.env.NEXT_PUBLIC_BACKEND_WS_PORT || "8080";
  const portSuffix = backendPort ? `:${backendPort}` : "";
  return `${proto}://${host}${portSuffix}${proxyPath}?token=${encodeURIComponent(jwtToken)}`;
}

/** Build the noVNC HTML page URL served by our backend proxy. */
function buildNoVNCPageUrl(proxyWsUrl: string): string {
  // We serve a minimal noVNC HTML page from the backend at /novnc.html
  // and pass the WebSocket URL as a query param.
  return `/novnc-console.html?ws=${encodeURIComponent(proxyWsUrl)}`;
}

// ── Expiry countdown ──────────────────────────────────────────────────────────

function ExpiryCountdown({ expiresAt, onExpired }: { expiresAt?: string; onExpired: () => void }) {
  const [label, setLabel] = useState("–");

  useEffect(() => {
    if (!expiresAt) return;
    const tick = () => {
      const diff = Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000);
      if (diff <= 0) { setLabel("Expired"); onExpired(); return; }
      setLabel(diff < 60 ? `${diff}s` : `${Math.floor(diff / 60)}m ${diff % 60}s`);
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [expiresAt, onExpired]);

  return (
    <div className="flex items-center gap-1.5 text-xs text-gray-500">
      <Clock className="w-3 h-3" />
      <span>Expires in <span className="text-gray-300 font-mono">{label}</span></span>
    </div>
  );
}

// ── Connection badge ──────────────────────────────────────────────────────────

function ConnectionBadge({ state }: { state: ConnectionState }) {
  const map: Record<ConnectionState, { label: string; dot: string; text: string }> = {
    idle:       { label: "Not connected",   dot: "bg-gray-500",                 text: "text-gray-400"   },
    connecting: { label: "Connecting…",     dot: "bg-yellow-400 animate-pulse", text: "text-yellow-400" },
    connected:  { label: "Connected",       dot: "bg-green-400",                text: "text-green-400"  },
    launched:   { label: "Console open",    dot: "bg-green-400",                text: "text-green-400"  },
    expired:    { label: "Session expired", dot: "bg-orange-400",               text: "text-orange-400" },
    error:      { label: "Error",           dot: "bg-red-400",                  text: "text-red-400"    },
  };
  const { label, dot, text } = map[state];
  return (
    <div className={cn("flex items-center gap-1.5 text-xs font-medium", text)}>
      <span className={cn("w-2 h-2 rounded-full shrink-0", dot)} />
      {label}
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function ConsoleViewer({ session, isLoading, error, onRequestSession, className }: ConsoleViewerProps) {
  const [connState, setConnState] = useState<ConnectionState>("idle");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [iframeReady, setIframeReady] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  // Derive the iframe src from the session
  const iframeSrc = useCallback((s: ConsoleSession): string | null => {
    const jwtToken = typeof window !== "undefined"
      ? localStorage.getItem("access_token") ?? ""
      : "";

    if (!s.proxy_ws_url) return null;

    // Always use the backend proxy — it handles both Proxmox noVNC (WS→WS)
    // and ESXi MKS (VMRC TCP→WS bridge). The type param tells novnc-console.html
    // which client to use.
    const wsUrl = buildProxyWsUrl(s.proxy_ws_url, jwtToken);
    const type = s.console_type === "webmks" ? "webmks" : "novnc";
    return `/novnc-console.html?ws=${encodeURIComponent(wsUrl)}&type=${type}`;
  }, []);

  // Sync state with session
  useEffect(() => {
    if (isLoading) { setConnState("connecting"); return; }
    if (error)     { setConnState("error");      return; }
    if (!session)  { setConnState("idle");        return; }

    const exp = session.expires_at ? new Date(session.expires_at).getTime() : NaN;
    if (!isNaN(exp) && exp <= Date.now()) { setConnState("expired"); return; }

    setConnState("connecting");
    setIframeReady(false);
  }, [session, isLoading, error]);

  const handleIframeLoad = useCallback(() => {
    setIframeReady(true);
    setConnState("connected");
  }, []);

  const handleExpired = useCallback(() => setConnState("expired"), []);

  // Open in new tab as fallback
  const openInNewTab = useCallback(() => {
    if (!session?.url) return;
    window.open(session.url, "_blank", "noopener,noreferrer,width=1280,height=800");
    setConnState("launched");
  }, [session]);

  // Fullscreen
  const toggleFullscreen = useCallback(async () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      await containerRef.current.requestFullscreen().catch(() => {});
    } else {
      await document.exitFullscreen().catch(() => {});
    }
  }, []);

  useEffect(() => {
    const h = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", h);
    return () => document.removeEventListener("fullscreenchange", h);
  }, []);

  const src = session ? iframeSrc(session) : null;
  const showIframe = !!src && session && connState !== "expired" && connState !== "error";

  return (
    <div
      ref={containerRef}
      className={cn(
        "flex flex-col bg-gray-950 border border-gray-800 rounded-2xl overflow-hidden",
        isFullscreen && "fixed inset-0 z-50 rounded-none border-0",
        className,
      )}
    >
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-gray-900 border-b border-gray-800 shrink-0">
        <div className="flex items-center gap-3">
          <ConnectionBadge state={connState} />
          {session?.console_type && (
            <span className="text-xs text-gray-600 border-l border-gray-700 pl-3">
              {consoleTypeLabel(session.console_type)}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {session && connState !== "expired" && (
            <>
              <button onClick={onRequestSession} title="New session"
                className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-gray-800 rounded-lg transition-colors">
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
              <button onClick={openInNewTab} title="Open direct URL in new tab"
                className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-gray-800 rounded-lg transition-colors">
                <ExternalLink className="w-3.5 h-3.5" />
              </button>
            </>
          )}
          <button onClick={toggleFullscreen} title={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
            className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-gray-800 rounded-lg transition-colors">
            {isFullscreen ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* Console area */}
      <div
        className="relative flex-1"
        style={{ height: isFullscreen ? "calc(100vh - 44px)" : "560px" }}
      >
        {/* Embedded iframe console */}
        {showIframe && (
          <>
            {!iframeReady && (
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-gray-950 z-10">
                <Loader2 className="w-8 h-8 animate-spin text-blue-400" />
                <p className="text-sm text-gray-400">Connecting to console…</p>
              </div>
            )}
            <iframe
              ref={iframeRef}
              src={src}
              className="w-full h-full border-0 bg-black"
              title="VM Console"
              allow="fullscreen"
              onLoad={handleIframeLoad}
            />
          </>
        )}

        {/* Overlay states */}
        {!showIframe && (
          <div className="absolute inset-0 flex items-center justify-center bg-gray-950">
            {isLoading && (
              <div className="flex flex-col items-center gap-3 text-gray-400">
                <Loader2 className="w-10 h-10 animate-spin text-blue-400" />
                <p className="text-sm font-medium">Requesting console session…</p>
                <p className="text-xs text-gray-600">Acquiring ticket from hypervisor</p>
              </div>
            )}

            {!isLoading && error && (
              <div className="flex flex-col items-center gap-3 text-center px-8 max-w-sm">
                <div className="p-3 bg-red-900/30 rounded-full">
                  <AlertTriangle className="w-8 h-8 text-red-400" />
                </div>
                <p className="text-sm font-semibold text-white">Console unavailable</p>
                <p className="text-xs text-red-400">{error}</p>
                <button onClick={onRequestSession}
                  className="mt-1 flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg transition-colors">
                  <RefreshCw className="w-3.5 h-3.5" /> Retry
                </button>
              </div>
            )}

            {!isLoading && !error && connState === "expired" && (
              <div className="flex flex-col items-center gap-3 text-center px-8 max-w-sm">
                <div className="p-3 bg-orange-900/30 rounded-full">
                  <WifiOff className="w-8 h-8 text-orange-400" />
                </div>
                <p className="text-sm font-semibold text-white">Session expired</p>
                <p className="text-xs text-gray-500">Console tickets are short-lived. Request a new session.</p>
                <button onClick={onRequestSession}
                  className="mt-1 flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg transition-colors">
                  <RefreshCw className="w-3.5 h-3.5" /> Reconnect
                </button>
              </div>
            )}

            {!isLoading && !error && !session && (
              <div className="flex flex-col items-center gap-3 text-center px-8">
                <div className="p-4 bg-gray-800/60 rounded-full">
                  <MonitorOff className="w-10 h-10 text-gray-500" />
                </div>
                <p className="text-sm font-semibold text-white">No active console session</p>
                <p className="text-xs text-gray-500">Click Launch Console to start an interactive session.</p>
                <button onClick={onRequestSession}
                  className="mt-1 flex items-center gap-2 px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors">
                  <Monitor className="w-4 h-4" /> Launch Console
                </button>
              </div>
            )}

            {/* Session ready but no proxy_ws_url — fallback to new-tab */}
            {!isLoading && !error && session && !src && connState !== "expired" && (
              <div className="flex flex-col items-center gap-6 px-8 py-6 w-full max-w-md">
                <div className={cn(
                  "w-full rounded-2xl border p-5 flex flex-col gap-4",
                  connState === "launched" ? "bg-green-950/30 border-green-800/40" : "bg-gray-900 border-gray-800",
                )}>
                  <div className="flex items-center gap-3">
                    <div className={cn("p-2.5 rounded-xl", connState === "launched" ? "bg-green-900/40" : "bg-blue-900/30")}>
                      {connState === "launched"
                        ? <CheckCircle2 className="w-6 h-6 text-green-400" />
                        : <Monitor className="w-6 h-6 text-blue-400" />}
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-white">
                        {connState === "launched" ? "Console window opened" : "Session ready"}
                      </p>
                      <p className="text-xs text-gray-500">
                        {connState === "launched" ? "Check your browser for the console window" : "Click Open Console to launch"}
                      </p>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <div className="bg-gray-800/60 rounded-lg px-3 py-2">
                      <p className="text-gray-500 mb-0.5">Type</p>
                      <p className="text-gray-200 font-medium">{consoleTypeLabel(session.console_type)}</p>
                    </div>
                    <div className="bg-gray-800/60 rounded-lg px-3 py-2">
                      <p className="text-gray-500 mb-0.5">Provider</p>
                      <p className="text-gray-200 font-medium">{providerLabel(session.provider)}</p>
                    </div>
                  </div>
                  <ExpiryCountdown expiresAt={session.expires_at} onExpired={handleExpired} />
                </div>
                <div className="flex gap-3 w-full">
                  <button onClick={openInNewTab}
                    className="flex-1 flex items-center justify-center gap-2 px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-xl transition-colors">
                    <ExternalLink className="w-4 h-4" />
                    {connState === "launched" ? "Open Again" : "Open Console"}
                  </button>
                  <button onClick={onRequestSession}
                    className="flex items-center justify-center gap-2 px-4 py-3 bg-gray-800 hover:bg-gray-700 text-gray-300 text-sm rounded-xl transition-colors">
                    <RefreshCw className="w-4 h-4" /> New Session
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Expiry countdown overlay when iframe is showing */}
        {showIframe && session?.expires_at && (
          <div className="absolute bottom-2 right-2 z-20 bg-gray-900/80 backdrop-blur-sm rounded-lg px-2 py-1">
            <ExpiryCountdown expiresAt={session.expires_at} onExpired={handleExpired} />
          </div>
        )}
      </div>
    </div>
  );
}
