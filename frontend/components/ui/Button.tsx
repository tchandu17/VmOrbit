"use client";
import { forwardRef } from "react";
import { cn } from "@/lib/utils";
import { type LucideIcon, Loader2 } from "lucide-react";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "outline";
export type ButtonSize = "sm" | "md" | "lg";

const variantStyles: Record<ButtonVariant, string> = {
  primary: "bg-blue-600 text-white hover:bg-blue-700 border-blue-600",
  secondary: "bg-gray-800 text-gray-200 hover:bg-gray-700 border-gray-700",
  ghost: "bg-transparent text-gray-400 hover:text-white hover:bg-gray-800 border-transparent",
  danger: "bg-red-600/15 text-red-400 hover:bg-red-600/25 border-red-500/30",
  outline: "bg-transparent text-gray-300 hover:bg-gray-800 border-gray-700",
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: "px-2.5 py-1.5 text-xs gap-1.5",
  md: "px-3.5 py-2 text-sm gap-2",
  lg: "px-5 py-2.5 text-sm gap-2",
};

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: LucideIcon;
  iconRight?: LucideIcon;
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", size = "md", icon: Icon, iconRight: IconRight, loading, children, className, disabled, ...props }, ref) => {
    return (
      <button
        ref={ref}
        disabled={disabled || loading}
        className={cn(
          "inline-flex items-center justify-center font-medium rounded-lg border transition-colors",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        {...props}
      >
        {loading ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : Icon ? (
          <Icon className="w-4 h-4" />
        ) : null}
        {children}
        {IconRight && !loading && <IconRight className="w-4 h-4" />}
      </button>
    );
  }
);

Button.displayName = "Button";
