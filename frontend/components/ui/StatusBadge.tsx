"use client";
import { cn } from "@/lib/utils";

export type BadgeVariant = "success" | "error" | "warning" | "info" | "neutral" | "purple";

const variantStyles: Record<BadgeVariant, string> = {
  success: "bg-green-500/15 text-green-400 border-green-500/25",
  error: "bg-red-500/15 text-red-400 border-red-500/25",
  warning: "bg-yellow-500/15 text-yellow-400 border-yellow-500/25",
  info: "bg-blue-500/15 text-blue-400 border-blue-500/25",
  neutral: "bg-gray-500/15 text-gray-400 border-gray-500/25",
  purple: "bg-purple-500/15 text-purple-400 border-purple-500/25",
};

interface StatusBadgeProps {
  variant: BadgeVariant;
  children: React.ReactNode;
  dot?: boolean;
  pulse?: boolean;
  className?: string;
}

export function StatusBadge({ variant, children, dot, pulse, className }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium border",
        variantStyles[variant],
        className
      )}
    >
      {dot && (
        <span className={cn("w-1.5 h-1.5 rounded-full bg-current", pulse && "animate-pulse")} />
      )}
      {children}
    </span>
  );
}
