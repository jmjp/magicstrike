import type { ReactNode } from "react";

type BadgeVariant = "success" | "error" | "warning" | "info" | "neutral";

interface BadgeProps {
  variant?: BadgeVariant;
  children: ReactNode;
  className?: string;
}

const variantStyles: Record<BadgeVariant, string> = {
  success: "bg-green-500/10 text-success border-green-500/20",
  error: "bg-red-500/10 text-error border-red-500/20",
  warning: "bg-amber-500/10 text-warning border-amber-500/20",
  info: "bg-blue-500/10 text-info border-blue-500/20",
  neutral: "bg-bg-tertiary text-text-secondary border-border",
};

export function Badge({ variant = "neutral", children, className = "" }: BadgeProps) {
  return (
    <span
      className={`
        inline-flex items-center rounded-md border px-2 py-0.5
        font-mono text-xs font-medium
        ${variantStyles[variant]}
        ${className}
      `}
    >
      {children}
    </span>
  );
}
