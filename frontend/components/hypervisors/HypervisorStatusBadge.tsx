import { cn } from "@/lib/utils";
import type { ConnectionStatus } from "@/types";

const config: Record<ConnectionStatus, { label: string; dot: string; text: string }> = {
  connected:    { label: "Connected",    dot: "bg-green-400",  text: "text-green-400" },
  disconnected: { label: "Disconnected", dot: "bg-gray-400",   text: "text-gray-400" },
  error:        { label: "Error",        dot: "bg-red-500",    text: "text-red-500" },
  unknown:      { label: "Unknown",      dot: "bg-gray-500",   text: "text-gray-500" },
};

export function HypervisorStatusBadge({ status }: { status: ConnectionStatus }) {
  const c = config[status] ?? config.unknown;
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs font-medium", c.text)}>
      <span className={cn("w-1.5 h-1.5 rounded-full", c.dot)} />
      {c.label}
    </span>
  );
}
