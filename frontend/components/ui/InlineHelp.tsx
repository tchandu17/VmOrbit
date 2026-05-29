"use client";
import { useState } from "react";
import { HelpCircle } from "lucide-react";
import { cn } from "@/lib/utils";

interface InlineHelpProps {
  text: string;
  className?: string;
}

/**
 * Small help icon that shows a tooltip on hover.
 * Use for contextual explanations next to labels or form fields.
 */
export function InlineHelp({ text, className }: InlineHelpProps) {
  const [show, setShow] = useState(false);

  return (
    <span className={cn("relative inline-flex", className)}>
      <button
        type="button"
        onMouseEnter={() => setShow(true)}
        onMouseLeave={() => setShow(false)}
        onFocus={() => setShow(true)}
        onBlur={() => setShow(false)}
        className="p-0.5 text-gray-600 hover:text-gray-400 transition-colors"
        aria-label="Help"
      >
        <HelpCircle className="w-3.5 h-3.5" />
      </button>
      {show && (
        <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg shadow-lg text-xs text-gray-300 whitespace-nowrap max-w-xs z-50">
          {text}
          <div className="absolute top-full left-1/2 -translate-x-1/2 w-2 h-2 bg-gray-800 border-r border-b border-gray-700 rotate-45 -mt-1" />
        </div>
      )}
    </span>
  );
}
