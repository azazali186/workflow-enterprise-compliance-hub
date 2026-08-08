import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import type { Tone } from "@/lib/constants";
import { Card } from "./Card";
import { SkeletonText } from "./Skeleton";

const iconTones: Record<Tone, string> = {
  neutral: "bg-slate-100 text-slate-600",
  info: "bg-info-100 text-info-600",
  success: "bg-success-100 text-success-700",
  warning: "bg-warning-100 text-warning-700",
  danger: "bg-danger-100 text-danger-700",
};

export interface StatCardProps {
  label: string;
  value: number | string | null | undefined;
  icon: LucideIcon;
  tone?: Tone;
  loading?: boolean;
  /** Optional colored count chips under the value. */
  breakdown?: Array<{ label: string; count: number; tone?: Tone }>;
  footer?: string;
}

export function StatCard({ label, value, icon: Icon, tone = "neutral", loading, breakdown, footer }: StatCardProps) {
  return (
    <Card className="p-5">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-slate-500">{label}</p>
        <span className={cn("flex h-8 w-8 items-center justify-center rounded-lg", iconTones[tone])}>
          <Icon className="h-4 w-4" aria-hidden="true" />
        </span>
      </div>
      {loading ? (
        <SkeletonText className="mt-3 h-8 w-20" />
      ) : (
        <p className="mt-2 text-3xl font-bold tabular tracking-tight text-slate-900">
          {value ?? "—"}
        </p>
      )}
      {breakdown && breakdown.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {breakdown.map((b) => (
            <span
              key={b.label}
              className="inline-flex items-center gap-1.5 rounded-md bg-slate-50 px-2 py-0.5 text-xs text-slate-600 ring-1 ring-inset ring-slate-200"
            >
              {b.label}
              <span className="font-semibold tabular">{b.count}</span>
            </span>
          ))}
        </div>
      )}
      {footer && <p className="mt-3 text-xs text-slate-400">{footer}</p>}
    </Card>
  );
}
