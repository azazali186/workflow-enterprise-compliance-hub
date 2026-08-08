import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import type { DateRangeSpec, ListQuery, Paginated, SortSpec } from "@/types/api";

export interface PageEntry<T> {
  data: Paginated<T>;
  /** 1-based index of the first row rendered on this page. */
  start: number;
}

export interface CursorListOptions<T> {
  queryKey: unknown[];
  queryFn: (query: ListQuery, signal: AbortSignal) => Promise<Paginated<T>>;
  limit?: number;
  sort?: SortSpec;
  filters?: Record<string, unknown>;
  dateRange?: DateRangeSpec;
  includeSummary?: boolean;
  enabled?: boolean;
}

function extractTotal<T>(page: Paginated<T> | undefined): number | undefined {
  const summary = page?.pagination.summary;
  return summary && typeof summary.total === "number" ? summary.total : undefined;
}

function rangeText<T>(start: number, page: Paginated<T> | undefined, total: number | undefined): string | null {
  if (!page) return null;
  const count = page.pagination.count;
  if (count === 0) return null;
  const end = start + count - 1;
  return total === undefined ? `Showing ${start}–${end}` : `Showing ${start}–${end} of ${total}`;
}

/**
 * Server-side cursor pagination against the backend pagination engine.
 * Pages are fetched one at a time (cursor = previous page's next_cursor),
 * committed to an in-memory stack, and paged forward/back without refetching.
 * Resets automatically whenever sort/filters/date-range change.
 */
export function useCursorList<T>(opts: CursorListOptions<T>) {
  const limit = opts.limit ?? 10;
  const includeSummary = opts.includeSummary ?? true;

  const depsKey = useMemo(
    () => JSON.stringify([opts.sort, opts.filters, opts.dateRange, limit]),
    [opts.sort, opts.filters, opts.dateRange, limit],
  );

  const [pages, setPages] = useState<PageEntry<T>[]>([]);
  const [activeIdx, setActiveIdx] = useState(0);

  // Reset the page stack whenever the query inputs change.
  useEffect(() => {
    setPages([]);
    setActiveIdx(0);
  }, [depsKey]);

  const needFetch = activeIdx >= pages.length;
  const cursor = needFetch
    ? activeIdx === 0
      ? undefined
      : pages[activeIdx - 1]?.data.pagination.next_cursor
    : undefined;

  const query = useQuery({
    queryKey: [...opts.queryKey, depsKey, activeIdx, cursor ?? ""],
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      opts.queryFn(
        {
          cursor,
          limit,
          sort: opts.sort,
          filters: opts.filters,
          date_range: opts.dateRange,
          include_summary: includeSummary,
        },
        signal,
      ),
    // Always enabled (when opts allows): the key embeds activeIdx, so a
    // cached page renders instantly while a background refetch revalidates.
    // This is what makes invalidateQueries (after mutations) refresh the
    // currently displayed page.
    enabled: opts.enabled !== false,
  });

  // Commit the fetched page into the stack (append a new slot, or replace
  // the current slot with fresher data, preserving its start). The query is
  // always keyed to activeIdx, so an in-flight fetch for a previous page can
  // never overwrite a different slot.
  useEffect(() => {
    if (!query.data) return;
    setPages((prev) => {
      const next = [...prev];
      if (activeIdx < next.length) {
        next[activeIdx] = { data: query.data, start: next[activeIdx].start };
      } else {
        const prevStart = next[activeIdx - 1]?.start ?? 1;
        const prevCount = next[activeIdx - 1]?.data.pagination.count ?? 0;
        next[activeIdx] = { data: query.data, start: prevStart + prevCount };
      }
      return next;
    });
  }, [query.data, activeIdx]);

  const entry = pages[activeIdx];
  const page = needFetch ? query.data : entry?.data;
  const start = entry?.start ?? 1;
  const total = extractTotal(page);
  const hasMore = page?.pagination.has_more ?? false;

  return {
    page,
    start,
    total,
    hasMore,
    isFirstPage: activeIdx === 0,
    isLoading: needFetch && query.isPending,
    isFetching: needFetch && query.isFetching,
    isError: needFetch && query.isError,
    error: needFetch ? query.error : null,
    pageNumber: activeIdx + 1,
    rangeText: rangeText(start, page, total),
    refetch: () => {
      if (needFetch) void query.refetch();
    },
    // Guard: never advance past a page whose cursor isn't committed yet, so
    // rapid clicks can't re-fetch the same rows under a bogus cursor.
    loadNext: () => {
      if (needFetch) return;
      setActiveIdx((i) => i + 1);
    },
    loadPrevious: () => setActiveIdx((i) => Math.max(0, i - 1)),
  };
}
