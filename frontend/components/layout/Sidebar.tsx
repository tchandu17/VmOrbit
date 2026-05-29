"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Server, Cpu, Activity, FileText, Settings, Users, LayoutDashboard, Shield, HeartPulse, Bell, Zap, Clock, GitBranch, Copy, PackagePlus, Layers, BarChart3, TrendingUp, Lightbulb, MessageSquare, AlertOctagon, MonitorDot, Globe, Database, ShieldCheck, Network } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { usePermissions } from "@/store/usePermissions";
import { Permissions } from "@/lib/permissions";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  exact?: boolean;
  permission?: string;
  section?: string;
}

const navItems: NavItem[] = [
  { label: "Overview",         href: "/dashboard",                icon: LayoutDashboard, exact: true },
  { label: "Virtual Machines", href: "/dashboard/vms",            icon: Server,          permission: Permissions.VM_READ },
  { label: "Infrastructure",   href: "/dashboard/infrastructure",  icon: Network,         permission: Permissions.HYPERVISOR_READ },
  { label: "Environments",     href: "/dashboard/environments",   icon: Layers,          permission: Permissions.VM_READ },
  { label: "Templates",        href: "/dashboard/templates",      icon: Copy,            permission: Permissions.VM_READ },
  { label: "Provisioning",     href: "/dashboard/provisioning",   icon: PackagePlus,     permission: Permissions.VM_READ },
  { label: "Hypervisors",      href: "/dashboard/hypervisors",    icon: Cpu,             permission: Permissions.HYPERVISOR_READ },
  { label: "Provider Health",  href: "/dashboard/health",         icon: HeartPulse,      permission: Permissions.HYPERVISOR_READ },
  { label: "Analytics",        href: "/dashboard/analytics",      icon: BarChart3,       permission: Permissions.HYPERVISOR_READ },
  { label: "Capacity",         href: "/dashboard/analytics/capacity",       icon: TrendingUp,  permission: Permissions.HYPERVISOR_READ },
  { label: "Optimization",     href: "/dashboard/analytics/recommendations", icon: Lightbulb,  permission: Permissions.HYPERVISOR_READ },
  { label: "Forecasting",      href: "/dashboard/analytics/forecasting",    icon: TrendingUp,  permission: Permissions.HYPERVISOR_READ },
  { label: "Tasks",            href: "/dashboard/tasks",          icon: Activity,        permission: Permissions.TASK_READ },
  { label: "Schedules",        href: "/dashboard/schedules",      icon: Clock,           permission: Permissions.TASK_READ },
  { label: "Automation",       href: "/dashboard/automation",     icon: GitBranch,       permission: Permissions.TASK_READ },
  { label: "Governance",       href: "/dashboard/governance",     icon: Shield,          permission: Permissions.POLICY_READ },
  { label: "Policies",         href: "/dashboard/governance/policies",   icon: Shield,         permission: Permissions.POLICY_READ },
  { label: "Approvals",        href: "/dashboard/governance/approvals",  icon: MessageSquare,  permission: Permissions.APPROVAL_READ },
  { label: "Violations",       href: "/dashboard/governance/violations", icon: AlertOctagon,   permission: Permissions.POLICY_READ },
  { label: "Event Activity",   href: "/dashboard/events",         icon: Zap,             permission: Permissions.AUDIT_READ },
  { label: "Notifications",    href: "/dashboard/notifications",  icon: Bell,            permission: Permissions.AUDIT_READ },
  { label: "Audit Log",        href: "/dashboard/audit",          icon: FileText,        permission: Permissions.AUDIT_READ },
  { label: "Users",            href: "/dashboard/users",          icon: Users,           permission: Permissions.USER_READ },
  { label: "Roles",            href: "/dashboard/roles",          icon: Shield,          permission: Permissions.ROLE_READ },
  // ── Operations ──────────────────────────────────────────────────────────────
  { label: "System Health",    href: "/dashboard/system",         icon: MonitorDot,      permission: Permissions.HYPERVISOR_READ },
  { label: "Platform Status",  href: "/dashboard/status",         icon: Globe,           permission: Permissions.HYPERVISOR_READ },
  { label: "Backup Status",    href: "/dashboard/backups",        icon: Database,        permission: Permissions.HYPERVISOR_READ },
  { label: "Administration",   href: "/dashboard/admin",          icon: ShieldCheck,     permission: Permissions.HYPERVISOR_READ },
  { label: "Settings",         href: "/dashboard/settings",       icon: Settings },
];

export function Sidebar() {
  const pathname = usePathname();
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const { can } = usePermissions();

  const visibleItems = navItems.filter((item) =>
    !item.permission || can(item.permission)
  );

  return (
    <aside
      className={cn(
        "flex flex-col bg-gray-900 border-r border-gray-800 transition-all duration-200 shrink-0",
        sidebarOpen ? "w-56" : "w-16"
      )}
    >
      {/* Brand */}
      <div className="flex items-center gap-3 px-4 py-5 border-b border-gray-800">
        <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center shrink-0">
          <Server className="w-4 h-4 text-white" />
        </div>
        {sidebarOpen && (
          <span className="font-bold text-white text-sm tracking-wide">VMOrbit</span>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-4 space-y-0.5 px-2 overflow-y-auto sidebar-scroll">
        {visibleItems.map((item) => {
          const active = item.exact ? pathname === item.href : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors",
                active
                  ? "bg-blue-600/20 text-blue-400"
                  : "text-gray-400 hover:text-white hover:bg-gray-800"
              )}
            >
              <item.icon className="w-4 h-4 shrink-0" />
              {sidebarOpen && <span>{item.label}</span>}
            </Link>
          );
        })}
      </nav>

      {/* Version */}
      {sidebarOpen && (
        <div className="px-4 py-3 border-t border-gray-800">
          <p className="text-xs text-gray-600">VMOrbit v1.0</p>
        </div>
      )}
    </aside>
  );
}
