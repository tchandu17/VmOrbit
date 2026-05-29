import { X } from "lucide-react";
import type { Tag } from "@/types";

interface TagBadgeProps {
  tag: Tag;
  onRemove?: () => void;
  size?: "sm" | "md";
}

/**
 * Renders a color-coded tag pill.
 * Pass `onRemove` to show a remove button (used in tag management UI).
 */
export function TagBadge({ tag, onRemove, size = "sm" }: TagBadgeProps) {
  const padding = size === "md" ? "px-2.5 py-1" : "px-2 py-0.5";
  const text = size === "md" ? "text-xs" : "text-[10px]";

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full font-medium ${padding} ${text}`}
      style={{
        backgroundColor: `${tag.color}22`, // 13% opacity background
        color: tag.color,
        border: `1px solid ${tag.color}55`, // 33% opacity border
      }}
    >
      <span
        className="w-1.5 h-1.5 rounded-full shrink-0"
        style={{ backgroundColor: tag.color }}
      />
      {tag.name}
      {onRemove && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="ml-0.5 hover:opacity-70 transition-opacity"
          title={`Remove tag "${tag.name}"`}
        >
          <X className="w-2.5 h-2.5" />
        </button>
      )}
    </span>
  );
}
