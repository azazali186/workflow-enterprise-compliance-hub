import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, MoreVertical, Plus, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
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
import { severityMeta, statusMeta } from "@/lib/constants";
import { formatDate, relativeTime } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { violationsApi } from "@/services/violations.service";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { SEVERITIES, VIOLATION_STATUSES, type Violation } from "@/types/entities";
import { ViolationFormModal, type ViolationFormValues } from "../components/ViolationFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "detected_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function ViolationsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Violation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Violation | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (severityFilter) f.severity = severityFilter;
    return f;
  }, [debouncedSearch, statusFilter, severityFilter]);

  const list = useCursorList<Violation>({
    queryKey: ["violations-list"],
    queryFn: violationsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.violationsCreate);
  const canUpdate = can(PERM.violationsUpdate);
  const canDelete = can(PERM.violationsDelete);
  const canResolve = can(PERM.violationsResolve);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["violations-list"] });

  const createMutation = useMutation({
    mutationFn: violationsApi.create,
    onSuccess: (v) => {
      toast.success("Violation created", v.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not create violation", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: violationsApi.update,
    onSuccess: (v) => {
      toast.success("Violation updated", v.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not update violation", ApiError.is(e) ? e.userMessage : undefined),
  });

  const resolveMutation = useMutation({
    mutationFn: (id: string) => violationsApi.resolve(id),
    onSuccess: (v) => {
      toast.success("Violation resolved", v.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not resolve violation", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => violationsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["violations-list"] });
      const snapshot = qc.getQueriesData<Paginated<Violation>>({ queryKey: ["violations-list"] });
      qc.setQueriesData<Paginated<Violation>>({ queryKey: ["violations-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((v) => v.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete violation", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Violation deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Violation>[] = [
    {
      key: "title",
      header: "Violation",
      sortable: true,
      className: "min-w-56",
      render: (v) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{v.title}</p>
          {v.description && <p className="truncate text-xs text-slate-500">{v.description}</p>}
        </div>
      ),
    },
    {
      key: "severity",
      header: "Severity",
      sortable: true,
      render: (v) => {
        const meta = severityMeta[v.severity] ?? { label: v.severity, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (v) => {
        const meta = statusMeta[v.status] ?? { label: v.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    { key: "detected_at", header: "Detected", sortable: true, hideBelow: "md", render: (v) => <span className="text-slate-600">{formatDate(v.detected_at)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (v) => <span className="text-slate-500">{relativeTime(v.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || severityFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete || canResolve;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Violations"
        description="Detected compliance violations with severity and resolution status."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New violation
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(v) => v.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No violations found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Violations raised by the compliance engine will appear here."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="violations-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="violations-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="violations-status">
              <Select
                id="violations-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={VIOLATION_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Severity" htmlFor="violations-severity">
              <Select
                id="violations-severity"
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
        rowActions={(v) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${v.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canResolve && v.status !== "resolved" && v.status !== "closed"
                  ? [{ key: "resolve", label: "Mark resolved", icon: <CheckCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void resolveMutation.mutate(v.id) }]
                  : []),
                ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(v) }] : []),
                ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(v) }] : []),
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

      <ViolationFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        violation={editTarget}
        onSubmit={async (values: ViolationFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete violation"
        description={deleteTarget ? `"${deleteTarget.title}" will be soft-deleted and hidden from lists. Audit history is preserved.` : ""}
        loading={deleteMutation.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) void deleteMutation.mutate(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}
