import { ChevronLeft, ChevronRight } from "lucide-react";
import { Spinner } from "./Spinner";
import { cn } from "@/lib/cn";

export interface PaginationBarProps {
  /** "Showing 11–20 of 245" — null hides the summary. */
  rangeText: string | null;
  pageNumber: number;
  isFirstPage: boolean;
  hasMore: boolean;
  isFetching?: boolean;
  /** True while the current page is still loading (suppresses the summary). */
  isLoading?: boolean;
  onPrevious: () => void;
  onNext: () => void;
  className?: string;
}

export function PaginationBar({
  rangeText,
  pageNumber,
  isFirstPage,
  hasMore,
  isFetching = false,
  isLoading = false,
  onPrevious,
  onNext,
  className,
}: PaginationBarProps) {
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-3 border-t border-slate-100 px-4 py-3",
        className,
      )}
    >
      <p className="min-w-0 truncate text-sm text-slate-500">
        {isLoading ? "Loading…" : rangeText ?? (isFirstPage ? "No results" : "")}
      </p>
      <div className="flex shrink-0 items-center gap-1.5">
        <span className="mr-1 hidden text-xs text-slate-400 sm:inline">Page {pageNumber}</span>
        <button
          type="button"
          onClick={onPrevious}
          disabled={isFirstPage}
          className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 text-sm text-slate-600 shadow-sm transition-colors hover:bg-slate-50 disabled:pointer-events-none disabled:opacity-40"
          aria-label="Previous page"
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          <span className="hidden sm:inline">Previous</span>
        </button>
        <button
          type="button"
          onClick={onNext}
          disabled={!hasMore || isFetching}
          className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 text-sm text-slate-600 shadow-sm transition-colors hover:bg-slate-50 disabled:pointer-events-none disabled:opacity-40"
          aria-label="Next page"
        >
          <span className="hidden sm:inline">Next</span>
          {isFetching ? (
            <Spinner size={14} />
          ) : (
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          )}
        </button>
      </div>
    </div>
  );
}
