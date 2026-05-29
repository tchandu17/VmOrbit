"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, UserCheck, UserX, Pencil, Trash2, Key, Shield } from "lucide-react";
import { userApi } from "@/lib/api/users";
import { roleApi } from "@/lib/api/roles";
import { formatDate } from "@/lib/utils";
import { RoleBadge } from "@/components/rbac/RoleBadge";
import { usePermissions } from "@/store/usePermissions";
import { Permissions } from "@/lib/permissions";
import type { User, Role } from "@/types";

// ─────────────────────────────────────────────────────────────────────────────
// Create User Modal
// ─────────────────────────────────────────────────────────────────────────────
type CreateFormFields = { email: string; username: string; password: string; first_name: string; last_name: string };

function CreateUserModal({ roles, onClose }: { roles: Role[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<CreateFormFields>({
    email: "", username: "", password: "", first_name: "", last_name: "",
  });
  const [roleIds, setRoleIds] = useState<string[]>([]);
  const form = { ...fields, role_ids: roleIds };

  const create = useMutation({
    mutationFn: userApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      onClose();
    },
  });

  function toggleRole(id: string) {
    setRoleIds((prev) =>
      prev.includes(id) ? prev.filter((r) => r !== id) : [...prev, id]
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-lg shadow-2xl">
        <h2 className="font-semibold text-white mb-5 text-lg">Create User</h2>

        {create.error && (
          <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
            {(create.error as Error).message}
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          {[
            { key: "email", label: "Email", type: "email", placeholder: "user@example.com", span: 2 },
            { key: "username", label: "Username", type: "text", placeholder: "johndoe", span: 1 },
            { key: "password", label: "Password", type: "password", placeholder: "••••••••", span: 1 },
            { key: "first_name", label: "First Name", type: "text", placeholder: "John", span: 1 },
            { key: "last_name", label: "Last Name", type: "text", placeholder: "Doe", span: 1 },
          ].map((f) => (
            <div key={f.key} className={f.span === 2 ? "col-span-2" : ""}>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">{f.label}</label>
              <input
                type={f.type}
                value={fields[f.key as keyof CreateFormFields]}
                placeholder={f.placeholder}
                onChange={(e) => setFields((prev) => ({ ...prev, [f.key]: e.target.value }))}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          ))}
        </div>

        {/* Role selection */}
        <div className="mt-4">
          <label className="block text-xs font-medium text-gray-400 mb-2">Roles</label>
          <div className="flex flex-wrap gap-2">
            {roles.map((role) => (
              <button
                key={role.id}
                type="button"
                onClick={() => toggleRole(role.id)}
                className={`px-3 py-1 rounded-lg text-xs font-medium border transition-colors ${
                  form.role_ids.includes(role.id)
                    ? "bg-blue-600/30 border-blue-500/50 text-blue-300"
                    : "bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600"
                }`}
              >
                {role.name}
              </button>
            ))}
          </div>
        </div>

        <div className="flex gap-3 mt-6">
          <button
            onClick={() => create.mutate(form)}
            disabled={create.isPending}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {create.isPending ? "Creating…" : "Create User"}
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
// Edit User Modal
// ─────────────────────────────────────────────────────────────────────────────
function EditUserModal({ user, roles, onClose }: { user: User; roles: Role[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState({
    first_name: user.first_name,
    last_name: user.last_name,
    is_active: user.is_active,
  });

  const update = useMutation({
    mutationFn: (data: typeof form) => userApi.update(user.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      onClose();
    },
  });

  const assignRole = useMutation({
    mutationFn: ({ roleId }: { roleId: string }) => userApi.assignRole(user.id, roleId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const revokeRole = useMutation({
    mutationFn: ({ roleId }: { roleId: string }) => userApi.revokeRole(user.id, roleId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const currentRoleIds = new Set(user.roles?.map((r) => r.id) ?? []);

  function toggleRole(roleId: string) {
    if (currentRoleIds.has(roleId)) {
      revokeRole.mutate({ roleId });
    } else {
      assignRole.mutate({ roleId });
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-lg shadow-2xl">
        <h2 className="font-semibold text-white mb-5 text-lg">Edit User — @{user.username}</h2>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">First Name</label>
            <input
              type="text"
              value={form.first_name}
              placeholder="John"
              onChange={(e) => setForm((prev) => ({ ...prev, first_name: e.target.value }))}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Last Name</label>
            <input
              type="text"
              value={form.last_name}
              placeholder="Doe"
              onChange={(e) => setForm((prev) => ({ ...prev, last_name: e.target.value }))}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <div className="mt-4 flex items-center gap-3">
          <label className="text-xs font-medium text-gray-400">Active</label>
          <button
            type="button"
            onClick={() => setForm((prev) => ({ ...prev, is_active: !prev.is_active }))}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
              form.is_active ? "bg-blue-600" : "bg-gray-700"
            }`}
          >
            <span
              className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
                form.is_active ? "translate-x-4" : "translate-x-1"
              }`}
            />
          </button>
        </div>

        {/* Role management */}
        <div className="mt-5">
          <label className="block text-xs font-medium text-gray-400 mb-2">Roles</label>
          <div className="flex flex-wrap gap-2">
            {roles.map((role) => (
              <button
                key={role.id}
                type="button"
                onClick={() => toggleRole(role.id)}
                disabled={assignRole.isPending || revokeRole.isPending}
                className={`px-3 py-1 rounded-lg text-xs font-medium border transition-colors disabled:opacity-50 ${
                  currentRoleIds.has(role.id)
                    ? "bg-blue-600/30 border-blue-500/50 text-blue-300"
                    : "bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600"
                }`}
              >
                {role.name}
              </button>
            ))}
          </div>
        </div>

        <div className="flex gap-3 mt-6">
          <button
            onClick={() => update.mutate(form)}
            disabled={update.isPending}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {update.isPending ? "Saving…" : "Save Changes"}
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
// Change Password Modal
// ─────────────────────────────────────────────────────────────────────────────
function ChangePasswordModal({ user, onClose }: { user: User; onClose: () => void }) {
  const [form, setForm] = useState({ current: "", next: "", confirm: "" });
  const [error, setError] = useState("");

  const change = useMutation({
    mutationFn: () => userApi.changePassword(user.id, form.current, form.next),
    onSuccess: onClose,
    onError: (err: Error) => setError(err.message),
  });

  function handleSubmit() {
    setError("");
    if (form.next !== form.confirm) {
      setError("New passwords do not match");
      return;
    }
    if (form.next.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    change.mutate();
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-md shadow-2xl">
        <h2 className="font-semibold text-white mb-5 text-lg">Change Password — @{user.username}</h2>

        {error && (
          <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">{error}</div>
        )}

        <div className="space-y-4">
          {[
            { key: "current", label: "Current Password" },
            { key: "next", label: "New Password" },
            { key: "confirm", label: "Confirm New Password" },
          ].map((f) => (
            <div key={f.key}>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">{f.label}</label>
              <input
                type="password"
                value={(form as Record<string, string>)[f.key]}
                onChange={(e) => setForm((prev) => ({ ...prev, [f.key]: e.target.value }))}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          ))}
        </div>

        <div className="flex gap-3 mt-6">
          <button
            onClick={handleSubmit}
            disabled={change.isPending}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {change.isPending ? "Changing…" : "Change Password"}
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
// Main Page
// ─────────────────────────────────────────────────────────────────────────────
export default function UsersPage() {
  const queryClient = useQueryClient();
  const { can } = usePermissions();
  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [pwUser, setPwUser] = useState<User | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: () => userApi.list({ page_size: 50 }),
  });

  const { data: roles = [] } = useQuery({
    queryKey: ["roles"],
    queryFn: roleApi.list,
    enabled: can(Permissions.ROLE_READ),
  });

  const deleteUser = useMutation({
    mutationFn: userApi.delete,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const users = data?.data ?? [];

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Users</h1>
          <p className="text-gray-400 text-sm mt-0.5">{data?.total ?? 0} total</p>
        </div>
        {can(Permissions.USER_WRITE) && (
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
          >
            <Plus className="w-4 h-4" /> Add User
          </button>
        )}
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800">
              {["User", "Email", "Status", "Last Login", "Roles", "Actions"].map((h) => (
                <th key={h} className="text-left px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wide">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="border-b border-gray-800/50">
                  {Array.from({ length: 6 }).map((_, j) => (
                    <td key={j} className="px-4 py-3">
                      <div className="h-4 bg-gray-800 rounded animate-pulse w-24" />
                    </td>
                  ))}
                </tr>
              ))
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                  No users found
                </td>
              </tr>
            ) : (
              users.map((user) => (
                <tr key={user.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-xs font-medium text-white shrink-0">
                        {user.username[0].toUpperCase()}
                      </div>
                      <div>
                        <p className="font-medium text-white">
                          {user.first_name} {user.last_name}
                        </p>
                        <p className="text-xs text-gray-500">@{user.username}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-gray-300">{user.email}</td>
                  <td className="px-4 py-3">
                    {user.is_active ? (
                      <span className="flex items-center gap-1 text-green-400 text-xs">
                        <UserCheck className="w-3.5 h-3.5" /> Active
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-gray-500 text-xs">
                        <UserX className="w-3.5 h-3.5" /> Inactive
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-400">
                    {user.last_login_at ? formatDate(user.last_login_at) : "Never"}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {user.roles?.length ? (
                        user.roles.map((r) => <RoleBadge key={r.id} name={r.name} />)
                      ) : (
                        <span className="text-xs text-gray-600">—</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      {can(Permissions.USER_WRITE) && (
                        <>
                          <button
                            onClick={() => setEditUser(user)}
                            title="Edit user"
                            className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-700 transition-colors"
                          >
                            <Pencil className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => setPwUser(user)}
                            title="Change password"
                            className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-700 transition-colors"
                          >
                            <Key className="w-3.5 h-3.5" />
                          </button>
                        </>
                      )}
                      {can(Permissions.USER_DELETE) && (
                        <button
                          onClick={() => {
                            if (confirm(`Delete user @${user.username}?`)) {
                              deleteUser.mutate(user.id);
                            }
                          }}
                          title="Delete user"
                          className="p-1.5 rounded-lg text-gray-400 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modals */}
      {showCreate && <CreateUserModal roles={roles} onClose={() => setShowCreate(false)} />}
      {editUser && <EditUserModal user={editUser} roles={roles} onClose={() => setEditUser(null)} />}
      {pwUser && <ChangePasswordModal user={pwUser} onClose={() => setPwUser(null)} />}
    </div>
  );
}
