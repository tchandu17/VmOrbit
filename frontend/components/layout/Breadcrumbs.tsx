"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronRight, Home } from "lucide-react";
import { getAllNavItems } from "@/lib/navigation";

const labelMap: Record<string, string> = {
  dashboard: "Dashboard",
  vms: "Virtual Machines",
  hypervisors: "Providers",
  infrastructure: "Infrastructure",
  environments: "Environments",
  templates: "Templates",
  provisioning: "Provisioning",
  health: "Provider Health",
  analytics: "Analytics",
  capacity: "Capacity Planning",
  recommendations: "Optimization",
  forecasting: "Forecasting",
  tasks: "Tasks",
  schedules: "Schedules",
  automation: "Workflows",
  governance: "Governance",
  policies: "Policies",
  approvals: "Approvals",
  violations: "Violations",
  events: "Events",
  notifications: "Notifications",
  audit: "Audit Logs",
  users: "Users",
  roles: "Roles",
  system: "System Health",
  status: "Platform Status",
  backups: "Backup & Recovery",
  admin: "Administration",
  settings: "Settings",
  hosts: "Hosts",
};

export function Breadcrumbs() {
  const pathname = usePathname();
  const segments = pathname.split("/").filter(Boolean);

  // Don't show breadcrumbs on the root dashboard page
  if (segments.length <= 1) return null;

  const crumbs = segments.map((segment, index) => {
    const href = "/" + segments.slice(0, index + 1).join("/");
    const label = labelMap[segment] || segment.charAt(0).toUpperCase() + segment.slice(1);
    const isLast = index === segments.length - 1;
    // Skip UUID-like segments in display
    const isId = /^[0-9a-f-]{8,}$/i.test(segment);

    return { href, label: isId ? "Details" : label, isLast, isId };
  });

  return (
    <nav className="flex items-center gap-1 text-xs text-gray-500 mb-4" aria-label="Breadcrumb">
      <Link href="/dashboard" className="hover:text-gray-300 transition-colors">
        <Home className="w-3.5 h-3.5" />
      </Link>
      {crumbs.slice(1).map((crumb, i) => (
        <span key={crumb.href} className="flex items-center gap-1">
          <ChevronRight className="w-3 h-3 text-gray-700" />
          {crumb.isLast ? (
            <span className="text-gray-300 font-medium">{crumb.label}</span>
          ) : (
            <Link href={crumb.href} className="hover:text-gray-300 transition-colors">
              {crumb.label}
            </Link>
          )}
        </span>
      ))}
    </nav>
  );
}
