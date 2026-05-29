"use client";
import { useQuery } from "@tanstack/react-query";
import { Tag as TagIcon, X } from "lucide-react";
import { tagApi } from "@/lib/api/tags";
import type { Tag } from "@/types";

interface TagFilterProps {
  selectedTagIds: string[];
  onChange: (tagIds: string[]) => void;
}

/**
 * Multi-select tag filter for the VM inventory page.
 * Shows color-coded tag pills; clicking toggles selection.
 */
export function TagFilter({ selectedTagIds, onChange }: TagFilterProps) {
  const { data: tags = [] } = useQuery<Tag[]>({
    queryKey: ["tags"],
    queryFn: () => tagApi.list(),
  });

  if (tags.length === 0) return null;

  const toggle = (id: string) => {
    if (selectedTagIds.includes(id)) {
      onChange(selectedTagIds.filter((t) => t !== id));
    } else {
      onChange([...selectedTagIds, id]);
    }
  };

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <div className="flex items-center gap-1 text-xs text-gray-500">
        <TagIcon className="w-3 h-3" />
        <span>Tags:</span>
      </div>
      {tags.map((tag) => {
        const active = selectedTagIds.includes(tag.id);
        return (
          <button
            key={tag.id}
            onClick={() => toggle(tag.id)}
            className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-medium transition-all"
            style={{
              backgroundColor: active ? `${tag.color}33` : `${tag.color}11`,
              color: active ? tag.color : `${tag.color}99`,
              border: `1px solid ${active ? tag.color + "88" : tag.color + "33"}`,
              opacity: active ? 1 : 0.7,
            }}
          >
            <span
              className="w-1.5 h-1.5 rounded-full shrink-0"
              style={{ backgroundColor: tag.color }}
            />
            {tag.name}
            {active && <X className="w-2.5 h-2.5 ml-0.5" />}
          </button>
        );
      })}
      {selectedTagIds.length > 0 && (
        <button
          onClick={() => onChange([])}
          className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
        >
          Clear
        </button>
      )}
    </div>
  );
}
