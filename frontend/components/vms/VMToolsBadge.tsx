import { cn } from "@/lib/utils";

interface VMToolsBadgeProps {
  status: string;
  className?: string;
}

type ToolsState = "ok" | "outdated" | "notInstalled" | "unknown";

function classify(status: string): ToolsState {
  const s = (status ?? "").toLowerCase();
  if (s === "toolsok" || s === "ok") return "ok";
  if (s.includes("old") || s.includes("outdated")) return "outdated";
  if (s.includes("notinstalled") || s.includes("not_installed") || s === "toolsnotinstalled") return "notInstalled";
  return "unknown";
}

const STATE_CONFIG: Record<ToolsState, { label: string; className: string }> = {
  ok:           { label: "Tools OK",      className: "bg-green-500/15 text-green-400" },
  outdated:     { label: "Outdated",      className: "bg-yellow-500/15 text-yellow-400" },
  notInstalled: { label: "No Tools",      className: "bg-gray-500/15 text-gray-500" },
  unknown:      { label: "—",             className: "text-gray-600" },
};

export function VMToolsBadge({ status, className }: VMToolsBadgeProps) {
  if (!status) return <span className="text-gray-600 text-xs">—</span>;
  const state = classify(status);
  const { label, className: stateClass } = STATE_CONFIG[state];

  if (state === "unknown") {
    return <span className={cn("text-xs text-gray-600", className)}>—</span>;
  }

  return (
    <span
      className={cn(
        "inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium",
        stateClass,
        className
      )}
    >
      {label}
    </span>
  );
}
