import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export interface FieldProps {
  label: string;
  htmlFor?: string;
  required?: boolean;
  error?: string | null;
  hint?: string;
  className?: string;
  children: ReactNode;
}

export function Field({ label, htmlFor, required, error, hint, className, children }: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <label htmlFor={htmlFor} className="text-sm font-medium text-slate-700">
        {label}
        {required && (
          <span className="ml-0.5 text-danger-500" aria-hidden="true">
            *
          </span>
        )}
      </label>
      {children}
      {error ? (
        <p className="text-xs text-danger-600" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p className="text-xs text-slate-500">{hint}</p>
      ) : null}
    </div>
  );
}
