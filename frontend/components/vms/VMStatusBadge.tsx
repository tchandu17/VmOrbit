import { cn } from "@/lib/utils";
import type { VMStatus } from "@/types";

const config: Record<VMStatus, { label: string; dot: string; text: string }> = {
  running:      { label: "Running",      dot: "bg-green-400",  text: "text-green-400" },
  stopped:      { label: "Stopped",      dot: "bg-gray-400",   text: "text-gray-400" },
  suspended:    { label: "Suspended",    dot: "bg-yellow-400", text: "text-yellow-400" },
  paused:       { label: "Paused",       dot: "bg-orange-400", text: "text-orange-400" },
  unknown:      { label: "Unknown",      dot: "bg-gray-500",   text: "text-gray-500" },
  provisioning: { label: "Provisioning", dot: "bg-blue-400",   text: "text-blue-400" },
  deleting:     { label: "Deleting",     dot: "bg-red-400",    text: "text-red-400" },
  error:        { label: "Error",        dot: "bg-red-500",    text: "text-red-500" },
};

export function VMStatusBadge({ status }: { status: VMStatus }) {
  const c = config[status] ?? config.unknown;
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs font-medium", c.text)}>
      <span className={cn("w-1.5 h-1.5 rounded-full", c.dot, status === "running" && "animate-pulse")} />
      {c.label}
    </span>
  );
}
