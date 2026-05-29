"use client";
import { type LucideIcon, HelpCircle } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { Breadcrumbs } from "./Breadcrumbs";

interface PageHeaderProps {
  title: string;
  description?: string;
  icon?: LucideIcon;
  actions?: React.ReactNode;
  helpArticleId?: string;
}

export function PageHeader({ title, description, icon: Icon, actions, helpArticleId }: PageHeaderProps) {
  const openHelp = useUIStore((s) => s.openHelpPanel);

  return (
    <div className="mb-6">
      <Breadcrumbs />
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          {Icon && (
            <div className="w-9 h-9 rounded-lg bg-blue-600/15 border border-blue-500/20 flex items-center justify-center shrink-0">
              <Icon className="w-4.5 h-4.5 text-blue-400" />
            </div>
          )}
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold text-white">{title}</h1>
              {helpArticleId && (
                <button
                  onClick={openHelp}
                  className="p-1 rounded text-gray-600 hover:text-blue-400 transition-colors"
                  title="View help for this page"
                >
                  <HelpCircle className="w-4 h-4" />
                </button>
              )}
            </div>
            {description && (
              <p className="text-sm text-gray-500 mt-0.5">{description}</p>
            )}
          </div>
        </div>
        {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
      </div>
    </div>
  );
}
