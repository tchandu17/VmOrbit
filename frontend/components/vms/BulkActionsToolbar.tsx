"use client";
import { useState } from "react";
import { Power, RotateCcw, Camera, X, Loader2, AlertTriangle } from "lucide-react";
import * as Dialog from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

export type BulkAction = "power-on" | "power-off" | "reboot" | "snapshot";

interface BulkActionsToolbarProps {
  selectedCount: number;
  onClearSelection: () => void;
  onAction: (action: BulkAction, snapshotName?: string) => void;
  isPending: boolean;
}

/**
 * Floating toolbar that appears when VMs are selected.
 * Shows bulk action buttons and a count of selected VMs.
 */
export function BulkActionsToolbar({
  selectedCount,
  onClearSelection,
  onAction,
  isPending,
}: BulkActionsToolbarProps) {
  const [confirmAction, setConfirmAction] = useState<BulkAction | null>(null);
  const [snapshotName, setSnapshotName] = useState("");

  if (selectedCount === 0) return null;

  const handleConfirm = () => {
    if (confirmAction === "snapshot") {
      if (!snapshotName.trim()) return;
      onAction("snapshot", snapshotName.trim());
    } else if (confirmAction) {
      onAction(confirmAction);
    }
    setConfirmAction(null);
    setSnapshotName("");
  };

  const ACTIONS: { id: BulkAction; label: string; icon: React.ElementType; color: string }[] = [
    { id: "power-on",  label: "Power On",  icon: Power,    color: "bg-green-600/20 hover:bg-green-600/30 text-green-400 border-green-600/40" },
    { id: "power-off", label: "Power Off", icon: Power,    color: "bg-red-600/20 hover:bg-red-600/30 text-red-400 border-red-600/40" },
    { id: "reboot",    label: "Reboot",    icon: RotateCcw, color: "bg-yellow-600/20 hover:bg-yellow-600/30 text-yellow-400 border-yellow-600/40" },
    { id: "snapshot",  label: "Snapshot",  icon: Camera,   color: "bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border-blue-600/40" },
  ];

  const confirmLabels: Record<BulkAction, string> = {
    "power-on":  "Power On",
    "power-off": "Power Off",
    "reboot":    "Reboot",
    "snapshot":  "Create Snapshots",
  };

  return (
    <>
      {/* Confirm dialog */}
      <Dialog.Root open={!!confirmAction} onOpenChange={(v) => !v && setConfirmAction(null)}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 bg-black/60 z-40 backdrop-blur-sm" />
          <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-full max-w-md bg-gray-900 border border-gray-700 rounded-2xl p-6 shadow-2xl">
            <div className="flex items-start gap-4 mb-4">
              <div className="p-2 bg-yellow-900/40 rounded-lg shrink-0">
                <AlertTriangle className="w-5 h-5 text-yellow-400" />
              </div>
              <div className="flex-1">
                <Dialog.Title className="text-white font-semibold text-base mb-1">
                  {confirmAction ? confirmLabels[confirmAction] : ""} — {selectedCount} VM{selectedCount !== 1 ? "s" : ""}
                </Dialog.Title>
                <Dialog.Description className="text-gray-400 text-sm">
                  {confirmAction === "power-off" &&
                    `This will hard-stop ${selectedCount} VM${selectedCount !== 1 ? "s" : ""}. Any unsaved work in the guest OS will be lost.`}
                  {confirmAction === "reboot" &&
                    `This will reboot ${selectedCount} VM${selectedCount !== 1 ? "s" : ""}. Each guest OS will receive a graceful reboot signal.`}
                  {confirmAction === "power-on" &&
                    `This will power on ${selectedCount} VM${selectedCount !== 1 ? "s" : ""}.`}
                  {confirmAction === "snapshot" &&
                    `This will create a snapshot on ${selectedCount} VM${selectedCount !== 1 ? "s" : ""}. Enter a name for the snapshots.`}
                </Dialog.Description>
              </div>
            </div>

            {confirmAction === "snapshot" && (
              <input
                type="text"
                placeholder="Snapshot name (e.g. pre-maintenance)"
                value={snapshotName}
                onChange={(e) => setSnapshotName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleConfirm()}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 mb-4"
                autoFocus
              />
            )}

            <div className="flex justify-end gap-3">
              <button
                onClick={() => setConfirmAction(null)}
                className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleConfirm}
                disabled={confirmAction === "snapshot" && !snapshotName.trim()}
                className={cn(
                  "px-4 py-2 text-sm text-white rounded-lg transition-colors disabled:opacity-50",
                  confirmAction === "power-off" ? "bg-red-600 hover:bg-red-700"
                    : confirmAction === "reboot" ? "bg-yellow-600 hover:bg-yellow-700"
                    : "bg-blue-600 hover:bg-blue-700"
                )}
              >
                {confirmAction ? confirmLabels[confirmAction] : "Confirm"}
              </button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>

      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-3 bg-blue-950/60 border border-blue-800/60 rounded-xl">
        <div className="flex items-center gap-2 text-sm text-blue-300 font-medium">
          <span className="bg-blue-600 text-white text-xs font-bold px-2 py-0.5 rounded-full">
            {selectedCount}
          </span>
          VM{selectedCount !== 1 ? "s" : ""} selected
        </div>

        <div className="h-4 w-px bg-blue-800/60" />

        <div className="flex items-center gap-1.5 flex-wrap">
          {ACTIONS.map(({ id, label, icon: Icon, color }) => (
            <button
              key={id}
              onClick={() => setConfirmAction(id)}
              disabled={isPending}
              className={cn(
                "flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs border transition-colors disabled:opacity-50 disabled:cursor-not-allowed",
                color
              )}
            >
              {isPending ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <Icon className="w-3.5 h-3.5" />
              )}
              {label}
            </button>
          ))}
        </div>

        <div className="ml-auto">
          <button
            onClick={onClearSelection}
            className="p-1.5 text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded-lg transition-colors"
            title="Clear selection"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>
    </>
  );
}
