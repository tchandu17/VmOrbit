export interface HelpArticle {
  id: string;
  title: string;
  category: string;
  summary: string;
  content: string;
  keywords: string[];
  relatedPages?: string[];
}

export interface HelpCategory {
  id: string;
  label: string;
  description: string;
  articles: HelpArticle[];
}

const articles: HelpArticle[] = [
  // Getting Started
  {
    id: "getting-started",
    title: "Getting Started with VMOrbit",
    category: "getting-started",
    summary: "Learn the basics of VMOrbit and set up your first provider.",
    content: `## Welcome to VMOrbit

VMOrbit is a unified hypervisor management platform that lets you manage virtual machines across multiple providers from a single control plane.

### Quick Start Steps

1. **Add a Provider** — Navigate to Infrastructure → Providers and click "Add Provider". Enter your vCenter or Proxmox credentials.
2. **Test Connection** — After adding, click "Test Connection" to verify connectivity.
3. **Sync Inventory** — Click "Sync" to discover all VMs, hosts, and datastores from your provider.
4. **Manage VMs** — Go to Infrastructure → Virtual Machines to see all discovered VMs across providers.

### Key Concepts

- **Provider**: A hypervisor platform (VMware vCenter, Proxmox VE) that VMOrbit connects to.
- **Inventory Sync**: The process of discovering and importing VMs, hosts, networks, and datastores from a provider.
- **Tasks**: All long-running operations (power actions, syncs, snapshots) run as async tasks you can monitor.`,
    keywords: ["start", "begin", "setup", "first", "new", "introduction", "basics"],
    relatedPages: ["/dashboard/hypervisors", "/dashboard/vms"],
  },
  {
    id: "adding-providers",
    title: "Adding Hypervisor Providers",
    category: "providers",
    summary: "Connect VMware vCenter or Proxmox VE to VMOrbit.",
    content: `## Adding a Provider

### VMware vCenter

1. Go to **Infrastructure → Providers**
2. Click **Add Provider**
3. Select **VMware vCenter** as the type
4. Enter the connection details:
   - **Name**: A friendly name (e.g., "Production vCenter")
   - **Host**: vCenter FQDN or IP (e.g., vcenter.example.com)
   - **Port**: Usually 443
   - **Username**: A vCenter user with read access (e.g., administrator@vsphere.local)
   - **Password**: The user's password
   - **TLS Verify**: Enable for production, disable for self-signed certs
5. Click **Test Connection** to verify
6. Click **Save**

### Proxmox VE

1. Same steps, but select **Proxmox VE** as the type
2. Use an API token instead of username/password for better security
3. Enter the Proxmox host IP and port (default 8006)

### After Adding

- Click **Sync** on the provider card to trigger an inventory discovery
- The sync runs as a background task — monitor progress in the Tasks panel
- VMs will appear in the Virtual Machines page once sync completes`,
    keywords: ["provider", "hypervisor", "vcenter", "proxmox", "add", "connect", "register"],
    relatedPages: ["/dashboard/hypervisors"],
  },
  {
    id: "inventory-sync",
    title: "Inventory Synchronization",
    category: "providers",
    summary: "How VMOrbit discovers and imports your infrastructure.",
    content: `## Inventory Sync

When you trigger a sync, VMOrbit connects to your provider and discovers:

- **Virtual Machines** — All VMs with their configuration, power state, and metadata
- **Hosts** — ESXi hosts or Proxmox nodes
- **Datastores** — Storage volumes and capacity
- **Networks** — Virtual switches and port groups
- **Clusters** — Compute clusters and resource pools

### How It Works

1. VMOrbit opens a session to the provider API
2. Fetches all managed objects in parallel (5 concurrent workers for VMware)
3. Maps provider-specific data to VMOrbit's unified model
4. Stores results in the database with full metadata

### Sync Frequency

- **Manual**: Click "Sync" on any provider card
- **Scheduled**: Set up recurring syncs in Automation → Schedules
- **Recommended**: Every 15-30 minutes for active environments

### Monitoring Sync Progress

- Watch the real-time progress bar on the provider card
- Check the Tasks page for detailed status
- WebSocket updates push progress to your browser automatically`,
    keywords: ["sync", "inventory", "discover", "import", "refresh"],
    relatedPages: ["/dashboard/hypervisors", "/dashboard/tasks"],
  },
  {
    id: "vm-operations",
    title: "VM Power Operations",
    category: "vm-operations",
    summary: "Power on, off, reboot, and suspend virtual machines.",
    content: `## VM Power Operations

### Available Actions

| Action | Description | Graceful? |
|--------|-------------|-----------|
| **Power On** | Start a stopped VM | N/A |
| **Power Off** | Hard power off (like pulling the plug) | No |
| **Shutdown** | Guest OS graceful shutdown (requires VMware Tools) | Yes |
| **Reboot** | Guest OS reboot | Yes |
| **Suspend** | Pause VM state to disk | N/A |

### How to Perform Power Actions

1. Navigate to **Infrastructure → Virtual Machines**
2. Find your VM in the table
3. Click the **Actions** menu (⋮) on the right
4. Select the desired power action
5. Confirm the action in the dialog

### Bulk Operations

Select multiple VMs using checkboxes, then use the bulk action toolbar to perform power operations on all selected VMs simultaneously.

### Task Tracking

All power operations run as async tasks. You'll see:
- A toast notification when the task is created
- Real-time progress in the Tasks panel (top-right Activity icon)
- Final status (success/failure) when complete`,
    keywords: ["power", "on", "off", "reboot", "suspend", "shutdown", "start", "stop"],
    relatedPages: ["/dashboard/vms"],
  },
  {
    id: "snapshots",
    title: "Snapshot Management",
    category: "snapshots",
    summary: "Create, revert, and delete VM snapshots.",
    content: `## Snapshots

Snapshots capture the state of a VM at a point in time, allowing you to revert if something goes wrong.

### Creating a Snapshot

1. Go to the VM detail page (click a VM name)
2. Switch to the **Snapshots** tab
3. Click **Create Snapshot**
4. Enter a name and optional description
5. Choose whether to include memory state
6. Click **Create**

### Reverting to a Snapshot

1. In the Snapshots tab, find the snapshot
2. Click **Revert**
3. Confirm — this will replace the current VM state

### Deleting Snapshots

- Delete old snapshots to reclaim disk space
- Snapshots grow over time as the VM writes new data
- Best practice: Don't keep snapshots longer than 72 hours

### Important Notes

- Snapshots are NOT backups — they depend on the base disk
- Too many snapshots degrade VM performance
- Always delete snapshots after completing maintenance`,
    keywords: ["snapshot", "revert", "point-in-time", "rollback", "backup"],
    relatedPages: ["/dashboard/vms"],
  },
  {
    id: "automation-schedules",
    title: "Scheduled Operations",
    category: "automation",
    summary: "Set up recurring tasks and automated workflows.",
    content: `## Scheduled Operations

Automate routine tasks by creating schedules that run on a cron-like pattern.

### Creating a Schedule

1. Go to **Automation → Schedules**
2. Click **Create Schedule**
3. Configure:
   - **Name**: Descriptive name
   - **Type**: What to run (inventory sync, power action, etc.)
   - **Target**: Which provider or VM(s)
   - **Frequency**: Cron expression or preset (hourly, daily, weekly)
   - **Enabled**: Toggle on/off without deleting

### Common Use Cases

- **Inventory sync every 15 minutes** — Keep VM data fresh
- **Power off dev VMs at 7 PM** — Save resources overnight
- **Power on dev VMs at 7 AM** — Ready for the workday
- **Weekly snapshot of critical VMs** — Automated protection

### Monitoring

- View schedule execution history in the Tasks page
- Failed executions are logged and can trigger notifications`,
    keywords: ["schedule", "cron", "recurring", "automated", "timer"],
    relatedPages: ["/dashboard/schedules"],
  },
  {
    id: "rbac",
    title: "Roles & Permissions (RBAC)",
    category: "rbac",
    summary: "Manage user access with role-based access control.",
    content: `## Role-Based Access Control

VMOrbit uses RBAC to control what users can see and do.

### Built-in Roles

| Role | Access Level |
|------|-------------|
| **Super Admin** | Full access to everything |
| **Admin** | Manage providers, VMs, users |
| **Operator** | VM operations, view infrastructure |
| **Read Only** | View-only access |

### Permission Format

Permissions use a \`resource:action\` format:
- \`vm:read\` — View VMs
- \`vm:power\` — Power operations
- \`vm:snapshot\` — Snapshot management
- \`hypervisor:write\` — Add/edit providers
- \`audit:read\` — View audit logs

### Managing Roles

1. Go to **Governance → RBAC**
2. View existing roles and their permissions
3. Create custom roles with specific permission sets
4. Assign roles to users in **Administration → Users**

### Best Practices

- Follow least-privilege principle
- Create role per team function (e.g., "DB Team Operator")
- Audit role assignments regularly`,
    keywords: ["rbac", "role", "permission", "access", "security", "user"],
    relatedPages: ["/dashboard/roles", "/dashboard/users"],
  },
  {
    id: "analytics-overview",
    title: "Analytics & Capacity Planning",
    category: "analytics",
    summary: "Monitor resource utilization and plan capacity.",
    content: `## Analytics

VMOrbit provides analytics to help you understand resource utilization and plan for growth.

### Available Views

- **Dashboard → Analytics** — Overview of CPU, memory, and storage utilization
- **Intelligence → Capacity Planning** — Projected resource exhaustion dates
- **Intelligence → Optimization** — Right-sizing recommendations
- **Intelligence → Forecasting** — Trend-based demand predictions

### Key Metrics

- **CPU Utilization** — Average and peak across hosts
- **Memory Usage** — Allocated vs. consumed
- **Storage Capacity** — Used, provisioned, and available
- **VM Density** — VMs per host ratio

### Optimization Recommendations

VMOrbit analyzes VM resource usage and suggests:
- **Oversized VMs** — VMs using less than 20% of allocated resources
- **Undersized VMs** — VMs consistently hitting resource limits
- **Idle VMs** — VMs with no activity for extended periods`,
    keywords: ["analytics", "capacity", "utilization", "forecast", "optimize", "metrics"],
    relatedPages: ["/dashboard/analytics", "/dashboard/analytics/capacity"],
  },
  {
    id: "troubleshooting",
    title: "Troubleshooting Common Issues",
    category: "troubleshooting",
    summary: "Solutions for common problems and error messages.",
    content: `## Troubleshooting

### Provider Connection Failed

**Symptoms**: "Connection failed" or timeout errors when testing a provider.

**Solutions**:
1. Verify the hostname/IP is reachable from the VMOrbit server
2. Check the port is correct (443 for vCenter, 8006 for Proxmox)
3. Verify credentials are correct
4. If using TLS verify, ensure the certificate is valid
5. Check firewall rules between VMOrbit and the provider

### Inventory Sync Stuck

**Symptoms**: Sync task stays at 0% or doesn't complete.

**Solutions**:
1. Check the Tasks page for error messages
2. Verify the provider is still accessible (test connection)
3. For large environments, syncs can take several minutes — be patient
4. If stuck for >10 minutes, cancel and retry

### VMs Not Appearing After Sync

**Symptoms**: Sync completes but some VMs are missing.

**Solutions**:
1. Check if the user account has permission to see those VMs in the provider
2. Verify the VMs are in the datacenter/cluster being synced
3. Check the sync task logs for "skipped" entries

### WebSocket Disconnections

**Symptoms**: Real-time updates stop, progress bars freeze.

**Solutions**:
1. Check your network connection
2. The client auto-reconnects — wait 5-10 seconds
3. Refresh the page if updates don't resume
4. Check browser console for WebSocket errors`,
    keywords: ["troubleshoot", "error", "problem", "fix", "issue", "debug", "help"],
    relatedPages: ["/dashboard/tasks", "/dashboard/hypervisors"],
  },
  {
    id: "keyboard-shortcuts",
    title: "Keyboard Shortcuts",
    category: "getting-started",
    summary: "Navigate VMOrbit faster with keyboard shortcuts.",
    content: `## Keyboard Shortcuts

### Global

| Shortcut | Action |
|----------|--------|
| \`Ctrl/⌘ + K\` | Open command palette |
| \`Ctrl/⌘ + /\` | Toggle help panel |
| \`Ctrl/⌘ + B\` | Toggle sidebar |

### Navigation

Use the command palette (⌘K) to quickly jump to any page by typing its name.

### Tables

| Shortcut | Action |
|----------|--------|
| \`/\` | Focus search/filter input |
| \`Escape\` | Clear selection |

### Tips

- The command palette searches pages, VMs, providers, and tasks
- Type a few characters to filter — no need for exact matches
- Use arrow keys + Enter to navigate results without a mouse`,
    keywords: ["keyboard", "shortcut", "hotkey", "key", "fast", "quick"],
    relatedPages: [],
  },
];

export const helpCategories: HelpCategory[] = [
  {
    id: "getting-started",
    label: "Getting Started",
    description: "Learn the basics and set up VMOrbit",
    articles: articles.filter((a) => a.category === "getting-started"),
  },
  {
    id: "providers",
    label: "Providers & Inventory",
    description: "Connect and sync hypervisor providers",
    articles: articles.filter((a) => a.category === "providers"),
  },
  {
    id: "vm-operations",
    label: "VM Operations",
    description: "Power actions, console, and management",
    articles: articles.filter((a) => a.category === "vm-operations"),
  },
  {
    id: "snapshots",
    label: "Snapshots",
    description: "Point-in-time VM state management",
    articles: articles.filter((a) => a.category === "snapshots"),
  },
  {
    id: "automation",
    label: "Automation & Scheduling",
    description: "Recurring tasks and workflows",
    articles: articles.filter((a) => a.category === "automation"),
  },
  {
    id: "rbac",
    label: "RBAC & Security",
    description: "Roles, permissions, and access control",
    articles: articles.filter((a) => a.category === "rbac"),
  },
  {
    id: "analytics",
    label: "Analytics & Intelligence",
    description: "Capacity planning and optimization",
    articles: articles.filter((a) => a.category === "analytics"),
  },
  {
    id: "troubleshooting",
    label: "Troubleshooting",
    description: "Common issues and solutions",
    articles: articles.filter((a) => a.category === "troubleshooting"),
  },
];

export function searchHelpArticles(query: string): HelpArticle[] {
  const q = query.toLowerCase().trim();
  if (!q) return [];
  return articles.filter(
    (a) =>
      a.title.toLowerCase().includes(q) ||
      a.summary.toLowerCase().includes(q) ||
      a.keywords.some((k) => k.includes(q))
  );
}

export function getArticleById(id: string): HelpArticle | undefined {
  return articles.find((a) => a.id === id);
}

export function getContextualHelp(pathname: string): HelpArticle[] {
  return articles.filter((a) => a.relatedPages?.some((p) => pathname.startsWith(p)));
}
