/**
 * Permission utilities for the frontend.
 *
 * Permissions are embedded in the JWT as "resource:action" strings and stored
 * in the auth store's user.roles array. The helper functions here provide a
 * clean API for checking permissions in components.
 */

import type { User } from "@/types";

/** All permission strings used in the application. */
export const Permissions = {
  // VM operations
  VM_READ: "vm:read",
  VM_WRITE: "vm:write",
  VM_DELETE: "vm:delete",
  VM_POWER: "vm:power",
  VM_SNAPSHOT: "vm:snapshot",
  VM_BULK: "vm:bulk",
  VM_CONSOLE: "vm:console",

  // Hypervisor / provider management
  HYPERVISOR_READ: "hypervisor:read",
  HYPERVISOR_WRITE: "hypervisor:write",
  HYPERVISOR_DELETE: "hypervisor:delete",

  // User management
  USER_READ: "user:read",
  USER_WRITE: "user:write",
  USER_DELETE: "user:delete",

  // Role management
  ROLE_READ: "role:read",
  ROLE_WRITE: "role:write",
  ROLE_DELETE: "role:delete",

  // Tasks
  TASK_READ: "task:read",
  TASK_WRITE: "task:write",

  // Audit logs
  AUDIT_READ: "audit:read",

  // Tags
  TAG_READ: "tag:read",
  TAG_WRITE: "tag:write",
  TAG_DELETE: "tag:delete",

  // Policy governance
  POLICY_READ: "policy:read",
  POLICY_WRITE: "policy:write",
  POLICY_DELETE: "policy:delete",

  // Approval workflows
  APPROVAL_READ: "approval:read",
  APPROVAL_WRITE: "approval:write",
} as const;

export type Permission = (typeof Permissions)[keyof typeof Permissions];

/**
 * Extracts the flat list of "resource:action" permission strings from a user's
 * roles. This is used when the JWT doesn't carry permissions directly (e.g.
 * after a page refresh where we re-fetch the user from /auth/me).
 */
export function extractPermissions(user: User | null): string[] {
  if (!user?.roles) return [];
  const set = new Set<string>();
  for (const role of user.roles) {
    if (role.permissions) {
      for (const perm of role.permissions) {
        set.add(`${perm.resource}:${perm.action}`);
      }
    }
  }
  return Array.from(set);
}

/**
 * Returns true if the user has the given permission.
 * Super-admins (role name "super_admin") always return true.
 */
export function hasPermission(user: User | null, permission: string): boolean {
  if (!user) return false;

  // Super-admin bypass
  if (user.roles?.some((r) => r.name === "super_admin")) return true;

  const perms = extractPermissions(user);
  return perms.includes(permission);
}

/**
 * Returns true if the user has ALL of the given permissions.
 */
export function hasAllPermissions(user: User | null, permissions: string[]): boolean {
  return permissions.every((p) => hasPermission(user, p));
}

/**
 * Returns true if the user has ANY of the given permissions.
 */
export function hasAnyPermission(user: User | null, permissions: string[]): boolean {
  return permissions.some((p) => hasPermission(user, p));
}

/** Role display names and colors for badges. */
export const RoleMeta: Record<string, { label: string; color: string }> = {
  super_admin: { label: "Super Admin", color: "bg-red-500/20 text-red-400 border-red-500/30" },
  admin: { label: "Admin", color: "bg-orange-500/20 text-orange-400 border-orange-500/30" },
  operator: { label: "Operator", color: "bg-blue-500/20 text-blue-400 border-blue-500/30" },
  readonly: { label: "Read Only", color: "bg-gray-500/20 text-gray-400 border-gray-500/30" },
};

export function getRoleMeta(name: string) {
  return RoleMeta[name] ?? { label: name, color: "bg-gray-500/20 text-gray-400 border-gray-500/30" };
}
