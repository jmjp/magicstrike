import { Loader2 } from "lucide-react";

interface SpinnerProps {
  size?: "sm" | "md" | "lg";
  className?: string;
}

const sizeMap = { sm: 16, md: 24, lg: 36 };

export function Spinner({ size = "md", className = "" }: SpinnerProps) {
  return (
    <Loader2
      size={sizeMap[size]}
      className={`animate-spin text-text-secondary ${className}`}
      role="status"
      aria-label="Loading"
    />
  );
}
