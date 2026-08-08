import { ArrowDown, ArrowUp, ChevronsUpDown } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import type { SortSpec } from "@/types/api";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { SkeletonRows } from "./Skeleton";

export interface Column<T> {
  key: string;
  header: ReactNode;
  /** Enables server-side sorting; sortKey defaults to key. */
  sortable?: boolean;
  sortKey?: string;
  align?: "left" | "right" | "center";
  className?: string;
  /** Responsive hiding: column hidden below the given breakpoint. */
  hideBelow?: "sm" | "md" | "lg";
  render?: (row: T) => ReactNode;
}

const hideClass = { sm: "hidden sm:table-cell", md: "hidden md:table-cell", lg: "hidden lg:table-cell" } as const;

export interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[] | undefined;
  keyExtractor: (row: T) => string;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  sort?: SortSpec;
  onSortChange?: (sort: SortSpec | undefined) => void;
  /** Optional trailing actions cell per row (e.g. a Dropdown menu). */
  rowActions?: (row: T) => ReactNode;
  onRowClick?: (row: T) => void;
  emptyTitle?: string;
  emptyDescription?: string;
  toolbar?: ReactNode;
  skeletonRows?: number;
}

export function DataTable<T>({
  columns,
  rows,
  keyExtractor,
  isLoading = false,
  isError = false,
  error,
  onRetry,
  sort,
  onSortChange,
  rowActions,
  onRowClick,
  emptyTitle = "Nothing here yet",
  emptyDescription = "Try adjusting your filters, or add your first record.",
  toolbar,
  skeletonRows = 6,
}: DataTableProps<T>) {
  const toggleSort = (col: Column<T>) => {
    if (!onSortChange || !col.sortable) return;
    const key = col.sortKey ?? col.key;
    if (sort?.column === key) {
      onSortChange(sort.direction === "asc" ? { column: key, direction: "desc" } : { column: key, direction: "asc" });
    } else {
      onSortChange({ column: key, direction: "desc" });
    }
  };

  const sortIcon = (col: Column<T>) => {
    if (!col.sortable) return null;
    const active = sort?.column === (col.sortKey ?? col.key);
    if (!active) return <ChevronsUpDown className="h-3.5 w-3.5 text-slate-300" aria-hidden="true" />;
    return sort.direction === "asc" ? (
      <ArrowUp className="h-3.5 w-3.5 text-primary-600" aria-hidden="true" />
    ) : (
      <ArrowDown className="h-3.5 w-3.5 text-primary-600" aria-hidden="true" />
    );
  };

  if (isError) return <ErrorState error={error} onRetry={onRetry} />;

  const showEmpty = !isLoading && rows && rows.length === 0;

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200/80 bg-white shadow-card">
      {toolbar && <div className="border-b border-slate-100 px-4 py-3">{toolbar}</div>}

      {showEmpty ? (
        <EmptyState title={emptyTitle} description={emptyDescription} />
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-100 text-sm">
            <thead>
              <tr className="text-left">
                {columns.map((col) => (
                  <th
                    key={col.key}
                    scope="col"
                    aria-sort={
                      sort?.column === (col.sortKey ?? col.key)
                        ? sort.direction === "asc"
                          ? "ascending"
                          : "descending"
                        : undefined
                    }
                    className={cn(
                      "whitespace-nowrap px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500",
                      col.align === "right" && "text-right",
                      col.align === "center" && "text-center",
                      col.hideBelow && hideClass[col.hideBelow],
                    )}
                  >
                    {col.sortable ? (
                      <button
                        type="button"
                        onClick={() => toggleSort(col)}
                        className={cn(
                          "inline-flex items-center gap-1.5 transition-colors hover:text-slate-800",
                          sort?.column === (col.sortKey ?? col.key) && "text-primary-700",
                        )}
                      >
                        {col.header}
                        {sortIcon(col)}
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                ))}
                {rowActions && <th className="w-10 px-2" />}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {isLoading || !rows ? (
                <tr>
                  <td colSpan={columns.length + (rowActions ? 1 : 0)} className="p-0">
                    <SkeletonRows rows={skeletonRows} cols={columns.length} />
                  </td>
                </tr>
              ) : (
                rows.map((row) => {
                  const id = keyExtractor(row);
                  return (
                    <tr
                      key={id}
                      onClick={onRowClick ? () => onRowClick(row) : undefined}
                      className={cn("transition-colors hover:bg-slate-50/70", onRowClick && "cursor-pointer")}
                    >
                      {columns.map((col) => (
                        <td
                          key={col.key}
                          className={cn(
                            "px-4 py-3 align-middle text-slate-700",
                            col.align === "right" && "text-right tabular",
                            col.align === "center" && "text-center",
                            col.hideBelow && hideClass[col.hideBelow],
                            col.className,
                          )}
                        >
                          {col.render ? col.render(row) : String((row as Record<string, unknown>)[col.key] ?? "—")}
                        </td>
                      ))}
                      {rowActions && <td className="px-2 text-right">{rowActions(row)}</td>}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
