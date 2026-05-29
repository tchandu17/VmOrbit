"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Shield, ChevronDown, ChevronRight } from "lucide-react";
import { roleApi } from "@/lib/api/roles";
import { RoleBadge } from "@/components/rbac/RoleBadge";
import { usePermissions } from "@/store/usePermissions";
import { Permissions } from "@/lib/permissions";
import type { Role, Permission } from "@/types";

// ─────────────────────────────────────────────────────────────────────────────
// Permission matrix grouped by resource
// ─────────────────────────────────────────────────────────────────────────────
function PermissionMatrix({
  allPerms,
  selectedIds,
  onChange,
}: {
  allPerms: Permission[];
  selectedIds: Set<string>;
  onChange: (id: string, checked: boolean) => void;
}) {
  // Group by resource
  const grouped = allPerms.reduce<Record<string, Permission[]>>((acc, p) => {
    if (!acc[p.resource]) acc[p.resource] = [];
    acc[p.resource].push(p);
    return acc;
  }, {});

  return (
    <div className="space-y-3">
      {Object.entries(grouped).map(([resource, perms]) => (
        <div key={resource} className="bg-gray-800/50 rounded-lg p-3">
          <p className="text-xs font-semibold text-gray-300 uppercase tracking-wide mb-2">{resource}</p>
          <div className="flex flex-wrap gap-2">
            {perms.map((p) => (
              <label key={p.id} className="flex items-center gap-1.5 cursor-pointer">
                <input
                  type="checkbox"
                  checked={selectedIds.has(p.id)}
                  onChange={(e) => onChange(p.id, e.target.checked)}
                  className="w-3.5 h-3.5 rounded border-gray-600 bg-gray-700 text-blue-500 focus:ring-blue-500 focus:ring-offset-0"
                />
                <span className="text-xs text-gray-300">{p.action}</span>
              </label>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Create Role Modal
// ─────────────────────────────────────────────────────────────────────────────
function CreateRoleModal({ allPerms, onClose }: { allPerms: Permission[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const create = useMutation({
    mutationFn: () =>
      roleApi.create({ name, description, permission_ids: Array.from(selectedIds) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roles"] });
      onClose();
    },
  });

  function toggle(id: string, checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      checked ? next.add(id) : next.delete(id);
      return next;
    });
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-2xl shadow-2xl max-h-[90vh] overflow-y-auto">
        <h2 className="font-semibold text-white mb-5 text-lg">Create Role</h2>

        {create.error && (
          <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
            {(create.error as Error).message}
          </div>
        )}

        <div className="space-y-4 mb-5">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. vm_operator"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What can this role do?"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <div className="mb-5">
          <label className="block text-xs font-medium text-gray-400 mb-2">
            Permissions ({selectedIds.size} selected)
          </label>
          <PermissionMatrix allPerms={allPerms} selectedIds={selectedIds} onChange={toggle} />
        </div>

        <div className="flex gap-3">
          <button
            onClick={() => create.mutate()}
            disabled={create.isPending || !name.trim()}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {create.isPending ? "Creating…" : "Create Role"}
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Edit Role Modal
// ─────────────────────────────────────────────────────────────────────────────
function EditRoleModal({
  role,
  allPerms,
  onClose,
}: {
  role: Role;
  allPerms: Permission[];
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(role.name);
  const [description, setDescription] = useState(role.description);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(
    new Set(role.permissions?.map((p) => p.id) ?? [])
  );

  const update = useMutation({
    mutationFn: () => roleApi.update(role.id, { name, description }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["roles"] }),
  });

  const setPerms = useMutation({
    mutationFn: () => roleApi.setPermissions(role.id, Array.from(selectedIds)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roles"] });
      onClose();
    },
  });

  function toggle(id: string, checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      checked ? next.add(id) : next.delete(id);
      return next;
    });
  }

  async function handleSave() {
    await update.mutateAsync();
    await setPerms.mutateAsync();
  }

  const saving = update.isPending || setPerms.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-2xl shadow-2xl max-h-[90vh] overflow-y-auto">
        <h2 className="font-semibold text-white mb-5 text-lg">Edit Role</h2>

        <div className="space-y-4 mb-5">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <div className="mb-5">
          <label className="block text-xs font-medium text-gray-400 mb-2">
            Permissions ({selectedIds.size} selected)
          </label>
          <PermissionMatrix allPerms={allPerms} selectedIds={selectedIds} onChange={toggle} />
        </div>

        <div className="flex gap-3">
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {saving ? "Saving…" : "Save Changes"}
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition-colors"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Role Row (expandable)
// ─────────────────────────────────────────────────────────────────────────────
function RoleRow({
  role,
  allPerms,
  canWrite,
  canDelete,
  onEdit,
  onDelete,
}: {
  role: Role;
  allPerms: Permission[];
  canWrite: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const permCount = role.permissions?.length ?? 0;

  return (
    <>
      <tr className="border-b border-gray-800/50 hover:bg-gray-800/20 transition-colors">
        <td className="px-4 py-3">
          <button
            onClick={() => setExpanded((v) => !v)}
            className="flex items-center gap-2 text-left"
          >
            {expanded ? (
              <ChevronDown className="w-3.5 h-3.5 text-gray-500" />
            ) : (
              <ChevronRight className="w-3.5 h-3.5 text-gray-500" />
            )}
            <RoleBadge name={role.name} />
          </button>
        </td>
        <td className="px-4 py-3 text-sm text-gray-400">{role.description || "—"}</td>
        <td className="px-4 py-3 text-sm text-gray-400">{permCount} permissions</td>
        <td className="px-4 py-3">
          <div className="flex items-center gap-1">
            {canWrite && (
              <button
                onClick={onEdit}
                className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-700 transition-colors"
                title="Edit role"
              >
                <Pencil className="w-3.5 h-3.5" />
              </button>
            )}
            {canDelete && (
              <button
                onClick={onDelete}
                className="p-1.5 rounded-lg text-gray-400 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                title="Delete role"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-gray-800/50 bg-gray-800/10">
          <td colSpan={4} className="px-6 py-3">
            {permCount === 0 ? (
              <p className="text-xs text-gray-600">No permissions assigned</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {role.permissions?.map((p) => (
                  <span
                    key={p.id}
                    className="px-2 py-0.5 rounded text-xs bg-gray-700/60 text-gray-300 border border-gray-700"
                  >
                    {p.resource}:{p.action}
                  </span>
                ))}
              </div>
            )}
          </td>
        </tr>
      )}
    </>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main Page
// ─────────────────────────────────────────────────────────────────────────────
export default function RolesPage() {
  const queryClient = useQueryClient();
  const { can } = usePermissions();
  const [showCreate, setShowCreate] = useState(false);
  const [editRole, setEditRole] = useState<Role | null>(null);

  const { data: roles = [], isLoading } = useQuery({
    queryKey: ["roles"],
    queryFn: roleApi.list,
  });

  const { data: allPerms = [] } = useQuery({
    queryKey: ["permissions"],
    queryFn: roleApi.listPermissions,
  });

  const deleteRole = useMutation({
    mutationFn: roleApi.delete,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["roles"] }),
  });

  const canWrite = can(Permissions.ROLE_WRITE);
  const canDelete = can(Permissions.ROLE_DELETE);

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Roles & Permissions</h1>
          <p className="text-gray-400 text-sm mt-0.5">{roles.length} roles · {allPerms.length} permissions</p>
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
          >
            <Plus className="w-4 h-4" /> New Role
          </button>
        )}
      </div>

      {/* Roles table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["Role", "Description", "Permissions", "Actions"].map((h) => (
                <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <tr key={i} className="border-b border-gray-800/50">
                  {Array.from({ length: 4 }).map((_, j) => (
                    <td key={j} className="px-4 py-3">
                      <div className="h-4 bg-gray-800 rounded animate-pulse w-24" />
                    </td>
                  ))}
                </tr>
              ))
            ) : roles.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-12 text-center text-gray-500">
                  No roles found. Run <code className="text-blue-400">make seed</code> to create defaults.
                </td>
              </tr>
            ) : (
              roles.map((role) => (
                <RoleRow
                  key={role.id}
                  role={role}
                  allPerms={allPerms}
                  canWrite={canWrite}
                  canDelete={canDelete}
                  onEdit={() => setEditRole(role)}
                  onDelete={() => {
                    if (confirm(`Delete role "${role.name}"? This will remove it from all users.`)) {
                      deleteRole.mutate(role.id);
                    }
                  }}
                />
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Permissions reference */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-4 h-4 text-blue-400" />
          <h2 className="font-semibold text-white text-sm">All Permissions</h2>
          <span className="text-xs text-gray-500">({allPerms.length})</span>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2">
          {allPerms.map((p) => (
            <div
              key={p.id}
              className="flex items-center gap-2 px-2.5 py-1.5 bg-gray-800/50 rounded-lg border border-gray-700/50"
            >
              <span className="text-xs text-blue-400 font-medium">{p.resource}</span>
              <span className="text-xs text-gray-500">:</span>
              <span className="text-xs text-gray-300">{p.action}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Modals */}
      {showCreate && <CreateRoleModal allPerms={allPerms} onClose={() => setShowCreate(false)} />}
      {editRole && (
        <EditRoleModal role={editRole} allPerms={allPerms} onClose={() => setEditRole(null)} />
      )}
    </div>
  );
}
