"use client";
import { cn } from "@/lib/utils";

interface CardProps {
  children: React.ReactNode;
  className?: string;
  padding?: "sm" | "md" | "lg";
  hover?: boolean;
}

const paddingStyles = {
  sm: "p-3",
  md: "p-5",
  lg: "p-6",
};

export function Card({ children, className, padding = "md", hover }: CardProps) {
  return (
    <div
      className={cn(
        "bg-gray-900 border border-gray-800 rounded-xl",
        paddingStyles[padding],
        hover && "hover:border-gray-700 transition-colors cursor-pointer",
        className
      )}
    >
      {children}
    </div>
  );
}

interface CardHeaderProps {
  title: string;
  description?: string;
  icon?: React.ElementType;
  actions?: React.ReactNode;
}

export function CardHeader({ title, description, icon: Icon, actions }: CardHeaderProps) {
  return (
    <div className="flex items-start justify-between mb-4">
      <div className="flex items-center gap-2.5">
        {Icon && (
          <div className="w-8 h-8 rounded-lg bg-blue-600/15 border border-blue-500/20 flex items-center justify-center">
            <Icon className="w-4 h-4 text-blue-400" />
          </div>
        )}
        <div>
          <h3 className="text-sm font-semibold text-white">{title}</h3>
          {description && <p className="text-xs text-gray-500 mt-0.5">{description}</p>}
        </div>
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
