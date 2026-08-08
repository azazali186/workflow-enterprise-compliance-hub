import { ChevronDown, ChevronUp, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { PaginationBar } from "@/components/ui/PaginationBar";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { DateRangeInputs, type DateRangeValue } from "@/components/ui/Toolbar";
import { useCursorList } from "@/hooks/useCursorList";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { entityLabel } from "@/lib/constants";
import { formatDateTime, shortId } from "@/lib/format";
import { auditLogsApi } from "@/services/auditlogs.service";
import type { AuditLog } from "@/types/entities";
import { cn } from "@/lib/cn";

const ACTIONS = ["login", "logout", "create", "update", "delete", "check", "acknowledge", "resolve"];

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "created_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function AuditLogsPage() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [actionFilter, setActionFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [entityFilter, setEntityFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [expanded, setExpanded] = useState<string | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.actor_id = debouncedSearch;
    if (actionFilter) f.action = actionFilter;
    if (statusFilter) f.status = statusFilter;
    if (entityFilter) f.entity_type = entityFilter;
    return f;
  }, [debouncedSearch, actionFilter, statusFilter, entityFilter]);

  const list = useCursorList<AuditLog>({
    queryKey: ["audit-logs-list"],
    queryFn: auditLogsApi.search,
    limit: 15,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const columns: Column<AuditLog>[] = [
    {
      key: "action",
      header: "Action",
      sortable: true,
      render: (l) => {
        const tone =
          l.action === "login" || l.action === "logout"
            ? l.status === "success"
              ? "info"
              : "danger"
            : l.status === "failure"
              ? "danger"
              : "neutral";
        return <Badge tone={tone}>{l.action}</Badge>;
      },
    },
    {
      key: "entity_type",
      header: "Entity",
      sortable: true,
      render: (l) => (
        <span className="text-slate-700">
          {entityLabel(l.entity_type)}
          {l.entity_id ? ` · ${shortId(l.entity_id)}` : ""}
        </span>
      ),
    },
    { key: "actor_id", header: "Actor", render: (l) => <span className="font-mono text-xs text-slate-600">{shortId(l.actor_id) || "system"}</span> },
    { key: "ip", header: "IP", hideBelow: "lg", render: (l) => <span className="font-mono text-xs text-slate-500">{l.ip || "—"}</span> },
    { key: "created_at", header: "When", sortable: true, render: (l) => <span className="text-slate-600">{formatDateTime(l.created_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || actionFilter || statusFilter || entityFilter || dateRange.from || dateRange.to);
  const rows = list.page?.items;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Audit log"
        description="Who did what, when — logins, CRUD operations, and lifecycle actions with before/after snapshots."
      />

      <DataTable
        columns={columns}
        rows={rows}
        keyExtractor={(l) => l.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No audit entries"
        emptyDescription={hasFilters ? "No entries match the current filters." : "Actions recorded on this system will appear here."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by actor" htmlFor="audit-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="audit-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact actor id" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Action" htmlFor="audit-action">
              <Select
                id="audit-action"
                value={actionFilter}
                onChange={(e) => setActionFilter(e.target.value)}
                placeholder="All actions"
                options={ACTIONS.map((a) => ({ value: a, label: a }))}
                className="w-36"
              />
            </Field>
            <Field label="Result" htmlFor="audit-status">
              <Select
                id="audit-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All results"
                options={[
                  { value: "success", label: "Success" },
                  { value: "failure", label: "Failure" },
                ]}
                className="w-36"
              />
            </Field>
            <Field label="Entity" htmlFor="audit-entity">
              <Select
                id="audit-entity"
                value={entityFilter}
                onChange={(e) => setEntityFilter(e.target.value)}
                placeholder="All entities"
                options={["user", "compliance", "alert", "violation", "auditlog", "role"].map((v) => ({ value: v, label: entityLabel(v) }))}
                className="w-40"
              />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setActionFilter("");
                  setStatusFilter("");
                  setEntityFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
      />

      {/* Expandable change details for entries with snapshots */}
      {rows && rows.length > 0 && (
        <div className="space-y-2">
          {rows.map((l) => {
            const hasDetails = Boolean(l.changes || l.before_data || l.after_data);
            if (!hasDetails) return null;
            const open = expanded === l.id;
            return (
              <div key={`detail-${l.id}`} className="overflow-hidden rounded-lg border border-slate-200/80 bg-white shadow-card">
                <button
                  type="button"
                  onClick={() => setExpanded(open ? null : l.id)}
                  className="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-slate-50"
                >
                  <span className="min-w-0 truncate text-slate-700">
                    <span className="font-medium text-slate-900">{l.action}</span>
                    <span className="text-slate-400"> on </span>
                    {entityLabel(l.entity_type)}
                  </span>
                  {open ? <ChevronUp className="h-4 w-4 shrink-0 text-slate-400" /> : <ChevronDown className="h-4 w-4 shrink-0 text-slate-400" />}
                </button>
                {open && (
                  <div className="space-y-3 border-t border-slate-100 px-4 py-3">
                    <ChangeBlock title="Changes" value={l.changes} />
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <ChangeBlock title="Before" value={l.before_data} />
                      <ChangeBlock title="After" value={l.after_data} />
                    </div>
                    {l.metadata ? <ChangeBlock title="Metadata" value={l.metadata} /> : null}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <PaginationBar
        rangeText={list.rangeText}
        pageNumber={list.pageNumber}
        isFirstPage={list.isFirstPage}
        hasMore={list.hasMore}
        isFetching={list.isFetching}
        isLoading={list.isLoading}
        onPrevious={list.loadPrevious}
        onNext={list.loadNext}
      />
    </div>
  );
}

function ChangeBlock({ title, value }: { title: string; value: unknown }) {
  if (value === null || value === undefined) return null;
  let text = "";
  try {
    text = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  } catch {
    text = String(value);
  }
  return (
    <div className="min-w-0">
      <p className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">{title}</p>
      <pre className={cn("max-h-56 overflow-auto rounded-lg bg-slate-50 p-3 font-mono text-xs leading-relaxed text-slate-700")}>
        {text}
      </pre>
    </div>
  );
}
