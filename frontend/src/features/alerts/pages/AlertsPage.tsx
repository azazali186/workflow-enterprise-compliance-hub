import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, MoreVertical, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { Dropdown } from "@/components/ui/Dropdown";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { PaginationBar } from "@/components/ui/PaginationBar";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { DateRangeInputs, type DateRangeValue } from "@/components/ui/Toolbar";
import { useAuth } from "@/hooks/useAuth";
import { useCursorList } from "@/hooks/useCursorList";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { entityLabel, severityMeta, statusMeta } from "@/lib/constants";
import { relativeTime, shortId } from "@/lib/format";
import { alertsApi } from "@/services/alerts.service";
import { PERM } from "@/services/api/paths";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { ALERT_STATUSES, SEVERITIES, type Alert } from "@/types/entities";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "created_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function AlertsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [deleteTarget, setDeleteTarget] = useState<Alert | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.type = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (severityFilter) f.severity = severityFilter;
    return f;
  }, [debouncedSearch, statusFilter, severityFilter]);

  const list = useCursorList<Alert>({
    queryKey: ["alerts-list"],
    queryFn: alertsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canAck = can(PERM.alertsAcknowledge);
  const canResolve = can(PERM.alertsResolve);
  const canDelete = can(PERM.alertsDelete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["alerts-list"] });

  const acknowledge = useMutation({
    mutationFn: (id: string) => alertsApi.acknowledge(id),
    onSuccess: () => {
      toast.success("Alert acknowledged");
      invalidate();
    },
    onError: (e) => toast.error("Could not acknowledge alert", ApiError.is(e) ? e.userMessage : undefined),
  });

  const resolve = useMutation({
    mutationFn: (id: string) => alertsApi.resolve(id),
    onSuccess: () => {
      toast.success("Alert resolved");
      invalidate();
    },
    onError: (e) => toast.error("Could not resolve alert", ApiError.is(e) ? e.userMessage : undefined),
  });

  const remove = useMutation({
    mutationFn: (id: string) => alertsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["alerts-list"] });
      const snapshot = qc.getQueriesData<Paginated<Alert>>({ queryKey: ["alerts-list"] });
      qc.setQueriesData<Paginated<Alert>>({ queryKey: ["alerts-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((a) => a.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete alert", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Alert deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Alert>[] = [
    {
      key: "title",
      header: "Alert",
      className: "min-w-64",
      render: (a) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{a.title}</p>
          <p className="truncate text-xs text-slate-500">{a.type}</p>
        </div>
      ),
    },
    {
      key: "severity",
      header: "Severity",
      sortable: true,
      render: (a) => {
        const meta = severityMeta[a.severity] ?? { label: a.severity, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (a) => {
        const meta = statusMeta[a.status] ?? { label: a.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    {
      key: "entity_type",
      header: "Entity",
      hideBelow: "md",
      render: (a) => (
        <span className="text-slate-600">
          {entityLabel(a.entity_type)}
          {a.entity_id ? ` · ${shortId(a.entity_id)}` : ""}
        </span>
      ),
    },
    { key: "created_at", header: "Created", sortable: true, hideBelow: "lg", render: (a) => <span className="text-slate-500">{relativeTime(a.created_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || severityFilter || dateRange.from || dateRange.to);
  const anyAction = canAck || canResolve || canDelete;

  return (
    <div className="space-y-5">
      <PageHeader title="Alerts" description="Compliance alerts raised against monitored entities." />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(a) => a.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No alerts"
        emptyDescription={hasFilters ? "No alerts match the current filters." : "Alerts raised by the compliance engine will appear here."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by type" htmlFor="alerts-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="alerts-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact type" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="alerts-status">
              <Select
                id="alerts-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={ALERT_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Severity" htmlFor="alerts-severity">
              <Select
                id="alerts-severity"
                value={severityFilter}
                onChange={(e) => setSeverityFilter(e.target.value)}
                placeholder="All severities"
                options={SEVERITIES.map((s) => ({ value: s, label: severityMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setStatusFilter("");
                  setSeverityFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
        rowActions={(a) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for alert ${a.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canAck && a.status === "open" ? [{ key: "ack", label: "Acknowledge", icon: <CheckCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void acknowledge.mutate(a.id) }] : []),
                ...(canResolve && a.status !== "resolved" ? [{ key: "resolve", label: "Mark resolved", icon: <CheckCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void resolve.mutate(a.id) }] : []),
                ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(a) }] : []),
              ]}
            />
          ) : undefined
        }
      />

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

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete alert"
        description={deleteTarget ? `"${deleteTarget.title}" will be removed from the alert list.` : ""}
        loading={remove.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) void remove.mutate(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}
