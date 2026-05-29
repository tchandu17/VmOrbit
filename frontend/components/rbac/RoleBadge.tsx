import { getRoleMeta } from "@/lib/permissions";

interface RoleBadgeProps {
  name: string;
  className?: string;
}

export function RoleBadge({ name, className = "" }: RoleBadgeProps) {
  const meta = getRoleMeta(name);
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium border ${meta.color} ${className}`}
    >
      {meta.label}
    </span>
  );
}
