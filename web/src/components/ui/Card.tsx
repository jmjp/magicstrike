import type { ReactNode, HTMLAttributes } from "react";

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: "sm" | "md" | "lg";
  hover?: boolean;
  children: ReactNode;
}

const paddingStyles = {
  sm: "p-3",
  md: "p-4",
  lg: "p-6",
};

export function Card({
  padding = "md",
  hover = false,
  children,
  className = "",
  ...props
}: CardProps) {
  return (
    <div
      className={`
        rounded-xl border border-border bg-bg-secondary
        ${paddingStyles[padding]}
        ${hover ? "transition-all duration-200 hover:border-border-accent hover:bg-bg-tertiary cursor-pointer" : ""}
        ${className}
      `}
      {...props}
    >
      {children}
    </div>
  );
}
