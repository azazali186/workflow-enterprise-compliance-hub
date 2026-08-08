import { X } from "lucide-react";
import type { ReactNode } from "react";
import { Field } from "./Field";
import { Input } from "./Input";
import { cn } from "@/lib/cn";

/** Consistent flex-wrap row for filter controls. */
export function Toolbar({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex flex-wrap items-end gap-2.5", className)}>{children}</div>;
}

export interface DateRangeValue {
  from?: string | null;
  to?: string | null;
}

export interface DateRangeInputsProps {
  value: DateRangeValue;
  onChange: (value: DateRangeValue) => void;
  className?: string;
}

/** From/To date filter — emits "yyyy-mm-dd" strings the page converts to RFC3339. */
export function DateRangeInputs({ value, onChange, className }: DateRangeInputsProps) {
  return (
    <div className={cn("flex flex-wrap items-end gap-2.5", className)}>
      <Field label="From" htmlFor="range-from">
        <Input
          id="range-from"
          type="date"
          value={value.from ?? ""}
          onChange={(e) => onChange({ ...value, from: e.target.value || null })}
          className="w-40"
        />
      </Field>
      <Field label="To" htmlFor="range-to">
        <Input
          id="range-to"
          type="date"
          value={value.to ?? ""}
          onChange={(e) => onChange({ ...value, to: e.target.value || null })}
          className="w-40"
        />
      </Field>
      {(value.from || value.to) && (
        <button
          type="button"
          onClick={() => onChange({ from: null, to: null })}
          className="inline-flex h-9.5 items-center gap-1.5 rounded-lg px-2 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
        >
          <X className="h-3.5 w-3.5" aria-hidden="true" />
          Clear
        </button>
      )}
    </div>
  );
}
