import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import type { Tone } from "@/lib/constants";

const toneClasses: Record<Tone, string> = {
  neutral: "bg-slate-100 text-slate-700 ring-slate-200",
  info: "bg-info-100 text-info-600 ring-info-200",
  success: "bg-success-100 text-success-700 ring-success-200",
  warning: "bg-warning-100 text-warning-700 ring-warning-200",
  danger: "bg-danger-100 text-danger-700 ring-danger-200",
};

const dotClasses: Record<Tone, string> = {
  neutral: "bg-slate-400",
  info: "bg-info-500",
  success: "bg-success-500",
  warning: "bg-warning-500",
  danger: "bg-danger-500",
};

export interface BadgeProps {
  tone?: Tone;
  dot?: boolean;
  className?: string;
  children: ReactNode;
}

export function Badge({ tone = "neutral", dot = true, className, children }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
        toneClasses[tone],
        className,
      )}
    >
      {dot && <span className={cn("h-1.5 w-1.5 rounded-full", dotClasses[tone])} aria-hidden="true" />}
      {children}
    </span>
  );
}
