"use client";
import { useState } from "react";
import { Bell } from "lucide-react";
import { cn } from "@/lib/utils";
import { ChannelsTab } from "./ChannelsTab";
import { RulesTab } from "./RulesTab";
import { HistoryTab } from "./HistoryTab";

type Tab = "channels" | "rules" | "history";

export default function NotificationsPage() {
  const [tab, setTab] = useState<Tab>("channels");

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Bell className="w-6 h-6 text-blue-400" />
          Notifications
        </h1>
        <p className="text-gray-400 text-sm mt-0.5">
          Configure channels, rules, and view delivery history
        </p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-900 border border-gray-800 rounded-xl p-1 w-fit">
        {(["channels", "rules", "history"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={cn(
              "px-4 py-2 rounded-lg text-sm font-medium transition-colors capitalize",
              tab === t
                ? "bg-blue-600 text-white"
                : "text-gray-400 hover:text-white hover:bg-gray-800"
            )}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "channels" && <ChannelsTab />}
      {tab === "rules"    && <RulesTab />}
      {tab === "history"  && <HistoryTab />}
    </div>
  );
}
