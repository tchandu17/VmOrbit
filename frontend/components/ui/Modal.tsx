"use client";
import { useEffect, useRef } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: React.ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
  footer?: React.ReactNode;
}

const sizeStyles = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-lg",
  xl: "max-w-xl",
};

export function Modal({ open, onClose, title, description, children, size = "md", footer }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleEsc(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleEsc);
    return () => document.removeEventListener("keydown", handleEsc);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <>
      {/* Backdrop */}
      <div
        ref={overlayRef}
        className="fixed inset-0 bg-black/60 backdrop-blur-sm z-[150]"
        onClick={(e) => {
          if (e.target === overlayRef.current) onClose();
        }}
      />

      {/* Modal */}
      <div className="fixed inset-0 flex items-center justify-center z-[151] p-4">
        <div className={cn("bg-gray-900 border border-gray-700 rounded-xl shadow-2xl w-full", sizeStyles[size])}>
          {/* Header */}
          <div className="flex items-start justify-between px-5 py-4 border-b border-gray-800">
            <div>
              <h2 className="text-base font-semibold text-white">{title}</h2>
              {description && <p className="text-sm text-gray-500 mt-0.5">{description}</p>}
            </div>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors -mt-1"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* Body */}
          <div className="px-5 py-4 max-h-[60vh] overflow-y-auto">{children}</div>

          {/* Footer */}
          {footer && (
            <div className="flex items-center justify-end gap-2 px-5 py-3 border-t border-gray-800">
              {footer}
            </div>
          )}
        </div>
      </div>
    </>
  );
}
