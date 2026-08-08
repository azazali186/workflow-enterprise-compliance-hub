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
import { entityLabel, statusMeta } from "@/lib/constants";
import { formatDateTime, relativeTime } from "@/lib/format";
import { deadlinesApi } from "@/services/deadlines.service";
import { PERM } from "@/services/api/paths";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { DEADLINE_STATUSES, type Deadline } from "@/types/entities";
import { DeadlineFormModal, type DeadlineFormValues } from "../components/DeadlineFormModal";

const ENTITY_TYPES = ["compliance", "audit", "checklist", "correctiveaction"];

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "deadline_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function DeadlinesPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [entityFilter, setEntityFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Deadline | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Deadline | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (entityFilter) f.entity_type = entityFilter;
    return f;
  }, [debouncedSearch, statusFilter, entityFilter]);

  const list = useCursorList<Deadline>({
    queryKey: ["deadlines-list"],
    queryFn: deadlinesApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.deadlinesCreate);
  const canUpdate = can(PERM.deadlinesUpdate);
  const canDelete = can(PERM.deadlinesDelete);
  const canComplete = can(PERM.deadlinesComplete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["deadlines-list"] });

  const createMutation = useMutation({
    mutationFn: deadlinesApi.create,
    onSuccess: (d) => {
      toast.success("Deadline created", d.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not create deadline", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: deadlinesApi.update,
    onSuccess: (d) => {
      toast.success("Deadline updated", d.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not update deadline", ApiError.is(e) ? e.userMessage : undefined),
  });

  const completeMutation = useMutation({
    mutationFn: (id: string) => deadlinesApi.complete(id),
    onSuccess: (d) => {
      toast.success("Deadline completed", d.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not complete deadline", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deadlinesApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["deadlines-list"] });
      const snapshot = qc.getQueriesData<Paginated<Deadline>>({ queryKey: ["deadlines-list"] });
      qc.setQueriesData<Paginated<Deadline>>({ queryKey: ["deadlines-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((d) => d.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete deadline", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Deadline deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Deadline>[] = [
    {
      key: "title",
      header: "Deadline",
      sortable: true,
      className: "min-w-52",
      render: (d) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{d.title}</p>
          {d.description && <p className="truncate text-xs text-slate-500">{d.description}</p>}
        </div>
      ),
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (d) => {
        const meta = statusMeta[d.status] ?? { label: d.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    { key: "deadline_at", header: "Deadline at", sortable: true, render: (d) => <span className="text-slate-700">{formatDateTime(d.deadline_at)}</span> },
    {
      key: "entity_type",
      header: "Entity",
      hideBelow: "md",
      render: (d) => (
        <span className="text-slate-600">
          {entityLabel(d.entity_type)}
          {d.entity_id ? ` · ${d.entity_id.slice(0, 8)}` : ""}
        </span>
      ),
    },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (d) => <span className="text-slate-500">{relativeTime(d.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || entityFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete || canComplete;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Deadlines"
        description="Approaching and overdue compliance deadlines, evaluated by the deadline job."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New deadline
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(d) => d.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No deadlines found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Create a deadline to start tracking it."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="deadlines-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="deadlines-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="deadlines-status">
              <Select
                id="deadlines-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={DEADLINE_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Entity" htmlFor="deadlines-entity">
              <Select
                id="deadlines-entity"
                value={entityFilter}
                onChange={(e) => setEntityFilter(e.target.value)}
                placeholder="All entities"
                options={ENTITY_TYPES.map((v) => ({ value: v, label: entityLabel(v) }))}
                className="w-40"
              />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
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
        rowActions={(d) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${d.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canComplete && d.status !== "completed" ? [{ key: "complete", label: "Mark completed", icon: <CheckCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void completeMutation.mutate(d.id) }] : []),
                ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(d) }] : []),
                ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(d) }] : []),
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

      <DeadlineFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        deadline={editTarget}
        onSubmit={async (values: DeadlineFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete deadline"
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
