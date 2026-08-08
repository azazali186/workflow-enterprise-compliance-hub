import { forwardRef, type SelectHTMLAttributes } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cn";

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
  options: SelectOption[];
  /** Placeholder shown when no value is selected (rendered as an empty option). */
  placeholder?: string;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className, invalid, options, placeholder, ...rest },
  ref,
) {
  return (
    <div className="relative">
      <select
        ref={ref}
        className={cn(
          "h-9.5 w-full appearance-none rounded-lg border bg-white px-3 pr-8 text-sm text-slate-900 shadow-sm transition-colors duration-150",
          "focus:outline-none focus:ring-2",
          invalid
            ? "border-danger-300 focus:border-danger-400 focus:ring-danger-500/30"
            : "border-slate-200 focus:border-primary-500 focus:ring-primary-500/30",
          "disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400",
          className,
        )}
        aria-invalid={invalid || undefined}
        {...rest}
      >
        {placeholder !== undefined && (
          <option value="" disabled>
            {placeholder}
          </option>
        )}
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      <ChevronDown
        className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
        aria-hidden="true"
      />
    </div>
  );
});
