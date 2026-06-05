import { type InputHTMLAttributes, type ReactNode, useId } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  icon?: ReactNode;
}

export function Input({
  label,
  error,
  icon,
  className = "",
  id,
  ...props
}: InputProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label
          htmlFor={inputId}
          className="text-sm font-medium text-text-secondary"
        >
          {label}
        </label>
      )}
      <div className="relative">
        {icon && (
          <div className="absolute left-3 top-1/2 -translate-y-1/2 text-text-dim">
            {icon}
          </div>
        )}
        <input
          id={inputId}
          className={`
            w-full rounded-lg border bg-bg-secondary px-4 py-2.5 text-sm
            text-text-primary placeholder:text-text-dim
            transition-colors duration-150
            focus:outline-none focus:ring-2 focus:ring-accent/30 focus:border-border-accent
            ${icon ? "pl-10" : ""}
            ${error ? "border-error focus:ring-error/30" : "border-border"}
            ${className}
          `}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
          {...props}
        />
      </div>
      {error && (
        <p id={`${inputId}-error`} className="text-xs text-error" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
