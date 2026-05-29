import {
  LayoutDashboard,
  HeartPulse,
  BarChart3,
  Server,
  Cpu,
  Network,
  HardDrive,
  Database,
  Globe,
  Layers,
  Activity,
  Camera,
  Copy,
  PackagePlus,
  FileBox,
  Clock,
  GitBranch,
  Shield,
  MessageSquare,
  ShieldCheck,
  FileText,
  Bell,
  MonitorDot,
  TrendingUp,
  Lightbulb,
  Settings,
  Link2,
  Archive,
  Code2,
  Gauge,
  Users,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  id: string;
  label: string;
  href: string;
  icon: LucideIcon;
  exact?: boolean;
  permission?: string;
  description?: string;
  keywords?: string[];
}

export interface NavGroup {
  id: string;
  label: string;
  icon: LucideIcon;
  items: NavItem[];
}

export const navigationGroups: NavGroup[] = [
  {
    id: "dashboard",
    label: "Dashboard",
    icon: LayoutDashboard,
    items: [
      {
        id: "nav-overview",
        label: "Overview",
        href: "/dashboard",
        icon: LayoutDashboard,
        exact: true,
        description: "Platform overview and key metrics",
        keywords: ["home", "summary", "dashboard"],
      },
      {
        id: "nav-infra-health",
        label: "Infrastructure Health",
        href: "/dashboard/health",
        icon: HeartPulse,
        permission: "hypervisor:read",
        description: "Monitor provider connectivity and health",
        keywords: ["health", "status", "connectivity"],
      },
      {
        id: "nav-analytics",
        label: "Analytics",
        href: "/dashboard/analytics",
        icon: BarChart3,
        permission: "hypervisor:read",
        description: "Resource utilization and performance analytics",
        keywords: ["analytics", "charts", "metrics", "performance"],
      },
    ],
  },
  {
    id: "infrastructure",
    label: "Infrastructure",
    icon: Server,
    items: [
      {
        id: "nav-providers",
        label: "Providers",
        href: "/dashboard/hypervisors",
        icon: Cpu,
        permission: "hypervisor:read",
        description: "Manage hypervisor providers (vCenter, Proxmox)",
        keywords: ["hypervisor", "vcenter", "proxmox", "provider", "esxi"],
      },
      {
        id: "nav-hosts",
        label: "Hosts",
        href: "/dashboard/infrastructure",
        icon: HardDrive,
        permission: "hypervisor:read",
        description: "ESXi hosts and compute nodes",
        keywords: ["host", "esxi", "node", "compute"],
      },
      {
        id: "nav-vms",
        label: "Virtual Machines",
        href: "/dashboard/vms",
        icon: Server,
        permission: "vm:read",
        description: "VM inventory across all providers",
        keywords: ["vm", "virtual machine", "instance", "guest"],
      },
      {
        id: "nav-networks",
        label: "Networks",
        href: "/dashboard/infrastructure?tab=networks",
        icon: Network,
        permission: "hypervisor:read",
        description: "Virtual networks and port groups",
        keywords: ["network", "vlan", "portgroup", "switch"],
      },
      {
        id: "nav-datastores",
        label: "Datastores",
        href: "/dashboard/infrastructure?tab=datastores",
        icon: Database,
        permission: "hypervisor:read",
        description: "Storage datastores and capacity",
        keywords: ["datastore", "storage", "disk", "vmfs", "nfs"],
      },
      {
        id: "nav-environments",
        label: "Environments",
        href: "/dashboard/environments",
        icon: Layers,
        permission: "vm:read",
        description: "Logical environment groupings",
        keywords: ["environment", "dev", "staging", "production"],
      },
    ],
  },
  {
    id: "operations",
    label: "Operations",
    icon: Activity,
    items: [
      {
        id: "nav-tasks",
        label: "Tasks",
        href: "/dashboard/tasks",
        icon: Activity,
        permission: "task:read",
        description: "Async task queue and history",
        keywords: ["task", "job", "queue", "running"],
      },
      {
        id: "nav-snapshots",
        label: "Snapshots",
        href: "/dashboard/vms?tab=snapshots",
        icon: Camera,
        permission: "vm:snapshot",
        description: "VM snapshot management",
        keywords: ["snapshot", "backup", "restore", "point-in-time"],
      },
      {
        id: "nav-bulk-ops",
        label: "Bulk Operations",
        href: "/dashboard/vms?bulk=true",
        icon: Copy,
        permission: "vm:bulk",
        description: "Multi-VM batch operations",
        keywords: ["bulk", "batch", "multi", "mass"],
      },
      {
        id: "nav-templates",
        label: "Templates",
        href: "/dashboard/templates",
        icon: FileBox,
        permission: "vm:read",
        description: "VM templates for provisioning",
        keywords: ["template", "golden image", "base"],
      },
      {
        id: "nav-provisioning",
        label: "Provisioning",
        href: "/dashboard/provisioning",
        icon: PackagePlus,
        permission: "vm:write",
        description: "Deploy new VMs from templates",
        keywords: ["provision", "deploy", "create", "clone"],
      },
    ],
  },
  {
    id: "automation",
    label: "Automation",
    icon: GitBranch,
    items: [
      {
        id: "nav-schedules",
        label: "Schedules",
        href: "/dashboard/schedules",
        icon: Clock,
        permission: "task:read",
        description: "Scheduled tasks and recurring jobs",
        keywords: ["schedule", "cron", "recurring", "timer"],
      },
      {
        id: "nav-workflows",
        label: "Workflows",
        href: "/dashboard/automation",
        icon: GitBranch,
        permission: "task:read",
        description: "Automation workflows and pipelines",
        keywords: ["workflow", "automation", "pipeline", "orchestration"],
      },
      {
        id: "nav-policies",
        label: "Policies",
        href: "/dashboard/governance/policies",
        icon: Shield,
        permission: "policy:read",
        description: "Governance policies and rules",
        keywords: ["policy", "rule", "compliance", "governance"],
      },
      {
        id: "nav-approvals",
        label: "Approvals",
        href: "/dashboard/governance/approvals",
        icon: MessageSquare,
        permission: "approval:read",
        description: "Pending approval workflows",
        keywords: ["approval", "review", "pending", "authorize"],
      },
    ],
  },
  {
    id: "governance",
    label: "Governance",
    icon: ShieldCheck,
    items: [
      {
        id: "nav-rbac",
        label: "RBAC",
        href: "/dashboard/roles",
        icon: Users,
        permission: "role:read",
        description: "Roles and permissions management",
        keywords: ["rbac", "role", "permission", "access control"],
      },
      {
        id: "nav-audit",
        label: "Audit Logs",
        href: "/dashboard/audit",
        icon: FileText,
        permission: "audit:read",
        description: "Platform audit trail",
        keywords: ["audit", "log", "trail", "history", "who"],
      },
      {
        id: "nav-notifications",
        label: "Notifications",
        href: "/dashboard/notifications",
        icon: Bell,
        permission: "audit:read",
        description: "Alert rules and notification channels",
        keywords: ["notification", "alert", "email", "webhook"],
      },
      {
        id: "nav-provider-health",
        label: "Provider Health",
        href: "/dashboard/system",
        icon: MonitorDot,
        permission: "hypervisor:read",
        description: "Provider connectivity monitoring",
        keywords: ["health", "uptime", "connectivity", "monitor"],
      },
    ],
  },
  {
    id: "intelligence",
    label: "Intelligence",
    icon: TrendingUp,
    items: [
      {
        id: "nav-capacity",
        label: "Capacity Planning",
        href: "/dashboard/analytics/capacity",
        icon: Gauge,
        permission: "hypervisor:read",
        description: "Resource capacity analysis and planning",
        keywords: ["capacity", "planning", "growth", "utilization"],
      },
      {
        id: "nav-optimization",
        label: "Optimization",
        href: "/dashboard/analytics/recommendations",
        icon: Lightbulb,
        permission: "hypervisor:read",
        description: "Right-sizing and optimization recommendations",
        keywords: ["optimize", "recommendation", "right-size", "savings"],
      },
      {
        id: "nav-forecasting",
        label: "Forecasting",
        href: "/dashboard/analytics/forecasting",
        icon: TrendingUp,
        permission: "hypervisor:read",
        description: "Resource demand forecasting",
        keywords: ["forecast", "predict", "trend", "future"],
      },
    ],
  },
  {
    id: "administration",
    label: "Administration",
    icon: Settings,
    items: [
      {
        id: "nav-settings",
        label: "System Settings",
        href: "/dashboard/settings",
        icon: Settings,
        description: "Platform configuration",
        keywords: ["settings", "config", "preferences"],
      },
      {
        id: "nav-users",
        label: "Users",
        href: "/dashboard/users",
        icon: Users,
        permission: "user:read",
        description: "User account management",
        keywords: ["user", "account", "member"],
      },
      {
        id: "nav-integrations",
        label: "Integrations",
        href: "/dashboard/admin",
        icon: Link2,
        permission: "hypervisor:read",
        description: "Third-party integrations and API keys",
        keywords: ["integration", "api", "webhook", "connect"],
      },
      {
        id: "nav-backup",
        label: "Backup & Recovery",
        href: "/dashboard/backups",
        icon: Archive,
        permission: "hypervisor:read",
        description: "Database backup status and recovery",
        keywords: ["backup", "recovery", "restore", "disaster"],
      },
      {
        id: "nav-api-mgmt",
        label: "API Management",
        href: "/dashboard/admin?tab=api",
        icon: Code2,
        permission: "hypervisor:read",
        description: "API tokens and rate limits",
        keywords: ["api", "token", "key", "rate limit"],
      },
      {
        id: "nav-platform-status",
        label: "Platform Status",
        href: "/dashboard/status",
        icon: Globe,
        permission: "hypervisor:read",
        description: "System health and service status",
        keywords: ["status", "uptime", "service", "health"],
      },
    ],
  },
];

/** Flat list of all nav items for search */
export function getAllNavItems(): (NavItem & { group: string })[] {
  return navigationGroups.flatMap((group) =>
    group.items.map((item) => ({ ...item, group: group.label }))
  );
}

/** Search nav items by query */
export function searchNavItems(query: string): (NavItem & { group: string })[] {
  const q = query.toLowerCase().trim();
  if (!q) return [];
  return getAllNavItems().filter(
    (item) =>
      item.label.toLowerCase().includes(q) ||
      item.description?.toLowerCase().includes(q) ||
      item.keywords?.some((k) => k.includes(q))
  );
}
