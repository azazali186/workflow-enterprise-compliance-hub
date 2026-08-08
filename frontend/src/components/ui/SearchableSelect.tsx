/**
 * SearchableSelect — a premium combobox for foreign-key fields.
 *
 * It replaces the manual-UUID inputs that used to litter the forms: options
 * are loaded from the shared POST /api/v1/options endpoint, filtered
 * server-side with a debounced search, and the stored value's label is
 * resolved via the ids filter so edit forms show a friendly name instead of a
 * raw UUID. Keyboard navigation, loading skeletons, empty/error states and a
 * clear affordance are all built in.
 */
import { AnimatePresence, motion } from "framer-motion";
import { Check, ChevronDown, Loader2, Search, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { useDebouncedValue } from "@/hooks/useDebounce";
import { cn } from "@/lib/cn";
import {
  fetchEntityOptions,
  resolveOption,
  type OptionEntity,
  type OptionItem,
} from "@/services/options.service";

export interface SearchableSelectProps {
  /** Options entity to load (e.g. "compliances"). */
  entity: OptionEntity;
  /** Selected entity id — "" or a real uuid. */
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
  /** Show a clear (×) affordance when a value is set. */
  clearable?: boolean;
  id?: string;
  autoFocus?: boolean;
  className?: string;
  /** Server-search debounce in ms. */
  searchDelay?: number;
  /** Max options requested per search. */
  limit?: number;
}

export function SearchableSelect({
  entity,
  value,
  onChange,
  placeholder = "Search and select…",
  disabled,
  invalid,
  clearable = true,
  id,
  autoFocus,
  className,
  searchDelay = 250,
  limit = 50,
}: SearchableSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounced = useDebouncedValue(query.trim(), searchDelay);

  // Resolve the stored value's label (edit forms carry only the id).
  const valueQuery = useQuery({
    queryKey: ["options-value", entity, value],
    queryFn: ({ signal }) => resolveOption(entity, value, signal),
    enabled: Boolean(value),
    staleTime: 5 * 60_000,
  });

  // Server-side searchable list, refetched when the debounced query settles.
  const listQuery = useQuery({
    queryKey: ["options", entity, debounced],
    queryFn: ({ signal }) => fetchEntityOptions(entity, debounced || undefined, signal, limit),
    staleTime: 60_000,
    enabled: open || debounced.length > 0,
  });

  const selected = valueQuery.data;

  // Merged + instant client-side filter: the typed text filters the loaded
  // page immediately while the debounced server search refines it.
  const items = useMemo<OptionItem[]>(() => {
    const all = listQuery.data ?? [];
    const seen = new Set(all.map((i) => i.id));
    const merged = selected && !seen.has(selected.id) ? [selected, ...all] : all;
    const q = query.trim().toLowerCase();
    if (!q) return merged;
    return merged.filter((i) => i.name.toLowerCase().includes(q) || (i.sub ?? "").toLowerCase().includes(q));
  }, [listQuery.data, selected, query]);

  // Keep the highlight index in range as results change.
  useEffect(() => {
    setHighlight((h) => (h >= items.length ? Math.max(0, items.length - 1) : h));
  }, [items.length]);

  // Close on outside interaction and Escape.
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [open]);

  // Combobox contract: focus the search input the moment the panel opens so
  // arrow-key navigation and typing work immediately after clicking the trigger.
  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  const openPanel = () => {
    if (disabled) return;
    setQuery("");
    setHighlight(0);
    setOpen(true);
  };

  const closePanel = () => {
    setOpen(false);
    setQuery("");
  };

  const pick = (item: OptionItem) => {
    onChange(item.id);
    closePanel();
  };

  const clear = () => {
    onChange("");
    setQuery("");
    setHighlight(0);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(h + 1, Math.max(0, items.length - 1)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const item = items[highlight];
      if (item) pick(item);
    } else if (e.key === "Escape") {
      e.preventDefault();
      closePanel();
    }
  };

  const isLoading = open && listQuery.isPending && listQuery.data === undefined;
  const isError = open && listQuery.isError;
  const isEmpty = open && !isLoading && !isError && items.length === 0;
  // Resolved label; falls back to the raw value while the ids lookup runs
  // (or if the stored record no longer exists).
  const label = selected?.name ?? value;

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        type="button"
        id={id}
        disabled={disabled}
        onClick={() => (open ? closePanel() : openPanel())}
        className={cn(
          "flex h-9.5 w-full items-center justify-between gap-2 rounded-lg border bg-white px-3 text-left text-sm shadow-sm transition-colors duration-150",
          "focus:outline-none focus:ring-2",
          invalid
            ? "border-danger-300 focus:border-danger-400 focus:ring-danger-500/30"
            : "border-slate-200 focus:border-primary-500 focus:ring-primary-500/30",
          open && "border-primary-500 ring-2 ring-primary-500/30",
          disabled && "cursor-not-allowed bg-slate-50 text-slate-400",
          clearable && value && "pr-13",
          className,
        )}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-invalid={invalid || undefined}
      >
        <span className={cn("truncate", !label && "text-slate-400")} title={label}>
          {label || placeholder}
        </span>
        <ChevronDown
          className={cn("h-4 w-4 shrink-0 text-slate-400 transition-transform duration-150", open && "rotate-180")}
          aria-hidden="true"
        />
      </button>

      {/* Clear affordance — a real button outside the trigger so no
          interactive element nests inside another (HTML validity + a11y). */}
      {clearable && value && !disabled && (
        <button
          type="button"
          aria-label="Clear selection"
          onClick={clear}
          className="absolute right-8 top-1/2 z-10 -translate-y-1/2 rounded p-0.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      )}

      <AnimatePresence>
        {open && (
          <motion.div
            role="listbox"
            className="absolute z-30 mt-1.5 w-full min-w-56 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-pop"
            initial={{ opacity: 0, y: -4, scale: 0.99 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.99 }}
            transition={{ duration: 0.12 }}
          >
            <div className="relative border-b border-slate-100">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
              <input
                ref={inputRef}
                autoFocus={autoFocus}
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setHighlight(0);
                }}
                onKeyDown={onKeyDown}
                placeholder="Type to search…"
                className="h-10 w-full rounded-t-lg bg-transparent pl-9 pr-8 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none"
                aria-label="Search options"
              />
              {listQuery.isFetching && (
                <Loader2 className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-slate-400" aria-hidden="true" />
              )}
            </div>

            <ul className="max-h-64 overflow-y-auto overscroll-contain py-1">
              {isLoading &&
                Array.from({ length: 4 }).map((_, i) => (
                  <li key={i} className="flex items-center gap-3 px-3 py-2">
                    <div className="skeleton h-3.5 w-full rounded" aria-hidden="true" />
                  </li>
                ))}

              {isError && (
                <li className="px-3 py-3 text-center text-xs text-danger-600">
                  Failed to load options. Try again.
                </li>
              )}

              {isEmpty && (
                <li className="px-3 py-4 text-center text-sm text-slate-500">
                  No matches for “{query.trim() || "…"}”.
                </li>
              )}

              {!isLoading &&
                !isError &&
                items.map((item, i) => {
                  const active = i === highlight;
                  const isSelected = item.id === value;
                  return (
                    <li key={item.id}>
                      <button
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onMouseEnter={() => setHighlight(i)}
                        onClick={() => pick(item)}
                        className={cn(
                          "flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm transition-colors",
                          active ? "bg-slate-50 text-slate-900" : "text-slate-700",
                          isSelected && "bg-primary-50 text-primary-800",
                        )}
                      >
                        <span className="min-w-0">
                          <span className="block truncate font-medium">{item.name}</span>
                          {item.sub && <span className="block truncate text-xs text-slate-400">{item.sub}</span>}
                        </span>
                        {isSelected && <Check className="h-4 w-4 shrink-0 text-primary-600" aria-hidden="true" />}
                      </button>
                    </li>
                  );
                })}
            </ul>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
