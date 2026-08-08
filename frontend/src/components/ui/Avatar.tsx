import { cn } from "@/lib/cn";

const palette = [
  "bg-primary-100 text-primary-700",
  "bg-info-100 text-info-600",
  "bg-success-100 text-success-700",
  "bg-warning-100 text-warning-700",
  "bg-danger-100 text-danger-600",
  "bg-slate-200 text-slate-700",
];

function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

export function Avatar({ name, className }: { name: string; className?: string }) {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0;
  const tone = palette[hash % palette.length];

  return (
    <span
      className={cn(
        "inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold",
        tone,
        className,
      )}
      aria-hidden="true"
    >
      {initialsOf(name)}
    </span>
  );
}
