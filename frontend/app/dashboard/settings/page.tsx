"use client";
import { useState } from "react";
import { useAuthStore } from "@/store/useAuthStore";

export default function SettingsPage() {
  const { user } = useAuthStore();
  const [saved, setSaved] = useState(false);

  function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <p className="text-gray-400 text-sm mt-0.5">Manage your account and platform preferences</p>
      </div>

      {/* Profile */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
        <h2 className="font-semibold text-white mb-4">Profile</h2>
        <form onSubmit={handleSave} className="space-y-4">
          <div className="flex items-center gap-4 mb-6">
            <div className="w-16 h-16 rounded-2xl bg-blue-600 flex items-center justify-center text-2xl font-bold text-white">
              {user?.username?.[0]?.toUpperCase() ?? "A"}
            </div>
            <div>
              <p className="font-medium text-white">{user?.username ?? "admin"}</p>
              <p className="text-sm text-gray-400">{user?.email ?? "admin@example.com"}</p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">First Name</label>
              <input
                defaultValue={user?.first_name ?? ""}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">Last Name</label>
              <input
                defaultValue={user?.last_name ?? ""}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-gray-400 mb-1.5">Email</label>
            <input
              defaultValue={user?.email ?? ""}
              type="email"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <button
            type="submit"
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {saved ? "✓ Saved" : "Save Changes"}
          </button>
        </form>
      </div>

      {/* Platform Info */}
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
        <h2 className="font-semibold text-white mb-4">Platform</h2>
        <div className="space-y-3 text-sm">
          {[
            { label: "Version", value: "VMOrbit v1.0.0" },
            { label: "Backend", value: "http://localhost:8080" },
            { label: "WebSocket", value: "ws://localhost:8080/ws" },
            { label: "Database", value: "PostgreSQL 16 (Docker)" },
            { label: "Cache", value: "Redis 7 (Docker · port 6380)" },
          ].map((item) => (
            <div key={item.label} className="flex items-center justify-between py-2 border-b border-gray-800 last:border-0">
              <span className="text-gray-400">{item.label}</span>
              <span className="text-gray-200 font-mono text-xs">{item.value}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Danger Zone */}
      <div className="bg-gray-900 border border-red-900/40 rounded-2xl p-6">
        <h2 className="font-semibold text-red-400 mb-4">Danger Zone</h2>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-white font-medium">Sign out of all sessions</p>
            <p className="text-xs text-gray-500 mt-0.5">Revokes all refresh tokens</p>
          </div>
          <button className="px-4 py-2 bg-red-600/20 hover:bg-red-600/30 border border-red-600/40 text-red-400 rounded-lg text-sm transition-colors">
            Sign Out All
          </button>
        </div>
      </div>
    </div>
  );
}
