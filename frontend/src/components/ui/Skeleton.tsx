import { cn } from "@/lib/cn";

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("skeleton rounded-md", className)} aria-hidden="true" />;
}

export function SkeletonText({ className }: { className?: string }) {
  return <Skeleton className={cn("h-3.5", className)} />;
}

/** Skeleton table — rows × cols shimmer grid rendered during fetches. */
export function SkeletonRows({ rows = 6, cols = 5, className }: { rows?: number; cols?: number; className?: string }) {
  return (
    <div className={cn("space-y-3 p-5", className)} aria-hidden="true">
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="flex items-center gap-4">
          {Array.from({ length: cols }).map((_, c) => (
            <Skeleton key={c} className="h-4 flex-1" />
          ))}
        </div>
      ))}
    </div>
  );
}
