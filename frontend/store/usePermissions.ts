"use client";
import { useAuthStore } from "./useAuthStore";
import { hasPermission, hasAllPermissions, hasAnyPermission } from "@/lib/permissions";

/**
 * Hook for checking permissions in components.
 *
 * Usage:
 *   const { can, canAny, canAll } = usePermissions();
 *   if (can("vm:power")) { ... }
 */
export function usePermissions() {
  const user = useAuthStore((s) => s.user);

  return {
    /** Returns true if the user has the given permission. */
    can: (permission: string) => hasPermission(user, permission),
    /** Returns true if the user has ALL of the given permissions. */
    canAll: (permissions: string[]) => hasAllPermissions(user, permissions),
    /** Returns true if the user has ANY of the given permissions. */
    canAny: (permissions: string[]) => hasAnyPermission(user, permissions),
    /** The current user object. */
    user,
  };
}
