import { forwardRef, type InputHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, invalid, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        "h-9.5 w-full rounded-lg border bg-white px-3 text-sm text-slate-900 shadow-sm transition-colors duration-150",
        "placeholder:text-slate-400",
        "focus:outline-none focus:ring-2 focus:ring-offset-0",
        invalid
          ? "border-danger-300 focus:border-danger-400 focus:ring-danger-500/30"
          : "border-slate-200 focus:border-primary-500 focus:ring-primary-500/30",
        "disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400",
        className,
      )}
      aria-invalid={invalid || undefined}
      {...rest}
    />
  );
});
