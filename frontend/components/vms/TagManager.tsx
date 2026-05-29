"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Tag as TagIcon, Loader2, Check } from "lucide-react";
import { tagApi } from "@/lib/api/tags";
import { TagBadge } from "./TagBadge";
import type { Tag } from "@/types";

interface TagManagerProps {
  vmId: string;
}

/**
 * Full tag management panel for the VM detail page.
 * Shows current tags with remove buttons, and a picker to add new ones.
 */
export function TagManager({ vmId }: TagManagerProps) {
  const queryClient = useQueryClient();
  const [showPicker, setShowPicker] = useState(false);
  const [newTagName, setNewTagName] = useState("");
  const [newTagColor, setNewTagColor] = useState("#3B82F6");
  const [creatingNew, setCreatingNew] = useState(false);

  // All available tags
  const { data: allTags = [] } = useQuery<Tag[]>({
    queryKey: ["tags"],
    queryFn: () => tagApi.list(),
  });

  // Tags on this VM
  const { data: vmTags = [], isLoading } = useQuery<Tag[]>({
    queryKey: ["vm-tags", vmId],
    queryFn: () => tagApi.listByVM(vmId),
  });

  const vmTagIds = new Set(vmTags.map((t) => t.id));

  const addMut = useMutation({
    mutationFn: (tagId: string) => tagApi.addToVM(vmId, tagId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vm-tags", vmId] });
      queryClient.invalidateQueries({ queryKey: ["vm", vmId] });
    },
  });

  const removeMut = useMutation({
    mutationFn: (tagId: string) => tagApi.removeFromVM(vmId, tagId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vm-tags", vmId] });
      queryClient.invalidateQueries({ queryKey: ["vm", vmId] });
    },
  });

  const createMut = useMutation({
    mutationFn: (payload: { name: string; color: string }) =>
      tagApi.create(payload),
    onSuccess: (tag) => {
      queryClient.invalidateQueries({ queryKey: ["tags"] });
      // Auto-attach the newly created tag to this VM.
      addMut.mutate(tag.id);
      setNewTagName("");
      setCreatingNew(false);
    },
  });

  const handleCreateAndAdd = () => {
    if (!newTagName.trim()) return;
    createMut.mutate({ name: newTagName.trim(), color: newTagColor });
  };

  const PRESET_COLORS = [
    "#3B82F6", // blue
    "#10B981", // green
    "#F59E0B", // amber
    "#EF4444", // red
    "#8B5CF6", // purple
    "#EC4899", // pink
    "#06B6D4", // cyan
    "#F97316", // orange
    "#6B7280", // gray
    "#14B8A6", // teal
  ];

  return (
    <div className="space-y-4">
      {/* Current tags */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-gray-400 uppercase tracking-wide">
            Current Tags
          </span>
          <button
            onClick={() => setShowPicker((v) => !v)}
            className="flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300 transition-colors"
          >
            <Plus className="w-3 h-3" />
            Add tag
          </button>
        </div>

        {isLoading ? (
          <div className="flex items-center gap-2 text-gray-500 text-xs">
            <Loader2 className="w-3 h-3 animate-spin" />
            Loading tags…
          </div>
        ) : vmTags.length === 0 ? (
          <p className="text-xs text-gray-600 italic">No tags assigned</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {vmTags.map((tag) => (
              <TagBadge
                key={tag.id}
                tag={tag}
                size="md"
                onRemove={() => removeMut.mutate(tag.id)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Tag picker */}
      {showPicker && (
        <div className="bg-gray-800/60 border border-gray-700 rounded-xl p-4 space-y-3">
          <p className="text-xs font-medium text-gray-300">Add existing tag</p>
          <div className="flex flex-wrap gap-1.5 max-h-40 overflow-y-auto">
            {allTags
              .filter((t) => !vmTagIds.has(t.id))
              .map((tag) => (
                <button
                  key={tag.id}
                  onClick={() => addMut.mutate(tag.id)}
                  disabled={addMut.isPending}
                  className="transition-opacity hover:opacity-80 disabled:opacity-50"
                >
                  <TagBadge tag={tag} size="md" />
                </button>
              ))}
            {allTags.filter((t) => !vmTagIds.has(t.id)).length === 0 && (
              <p className="text-xs text-gray-500 italic">All tags already assigned</p>
            )}
          </div>

          <div className="border-t border-gray-700 pt-3">
            <button
              onClick={() => setCreatingNew((v) => !v)}
              className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1 transition-colors"
            >
              <TagIcon className="w-3 h-3" />
              {creatingNew ? "Cancel new tag" : "Create new tag"}
            </button>

            {creatingNew && (
              <div className="mt-3 space-y-2">
                <input
                  type="text"
                  placeholder="Tag name (e.g. production)"
                  value={newTagName}
                  onChange={(e) => setNewTagName(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleCreateAndAdd()}
                  className="w-full px-3 py-1.5 bg-gray-900 border border-gray-600 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
                <div className="flex items-center gap-2">
                  <span className="text-xs text-gray-400">Color:</span>
                  <div className="flex gap-1.5 flex-wrap">
                    {PRESET_COLORS.map((c) => (
                      <button
                        key={c}
                        onClick={() => setNewTagColor(c)}
                        className="w-5 h-5 rounded-full border-2 transition-all"
                        style={{
                          backgroundColor: c,
                          borderColor: newTagColor === c ? "white" : "transparent",
                        }}
                        title={c}
                      />
                    ))}
                  </div>
                </div>
                <button
                  onClick={handleCreateAndAdd}
                  disabled={!newTagName.trim() || createMut.isPending}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-xs rounded-lg transition-colors"
                >
                  {createMut.isPending ? (
                    <Loader2 className="w-3 h-3 animate-spin" />
                  ) : (
                    <Check className="w-3 h-3" />
                  )}
                  Create &amp; Add
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
