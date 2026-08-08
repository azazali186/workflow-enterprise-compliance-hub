import { forwardRef, type TextareaHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, invalid, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={cn(
        "w-full rounded-lg border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm transition-colors duration-150",
        "placeholder:text-slate-400",
        "focus:outline-none focus:ring-2",
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
