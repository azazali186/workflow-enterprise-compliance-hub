import { useMutation, useQueryClient } from "@tanstack/react-query";
import { MoreVertical, Plus, Search, Trash2 } from "lucide-react";
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
import { statusMeta } from "@/lib/constants";
import { formatDate, relativeTime } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { regulationsApi } from "@/services/regulations.service";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { REGULATION_STATUSES, type Regulation } from "@/types/entities";
import { RegulationFormModal, type RegulationFormValues } from "../components/RegulationFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "effective_date", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function RegulationsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [jurisdictionFilter, setJurisdictionFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Regulation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Regulation | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (jurisdictionFilter) f.jurisdiction = jurisdictionFilter;
    return f;
  }, [debouncedSearch, statusFilter, jurisdictionFilter]);

  const list = useCursorList<Regulation>({
    queryKey: ["regulations-list"],
    queryFn: regulationsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.regulationsCreate);
  const canUpdate = can(PERM.regulationsUpdate);
  const canDelete = can(PERM.regulationsDelete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["regulations-list"] });

  const createMutation = useMutation({
    mutationFn: regulationsApi.create,
    onSuccess: (r) => {
      toast.success("Regulation created", r.code);
      invalidate();
    },
    onError: (e) => toast.error("Could not create regulation", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: regulationsApi.update,
    onSuccess: (r) => {
      toast.success("Regulation updated", r.code);
      invalidate();
    },
    onError: (e) => toast.error("Could not update regulation", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => regulationsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["regulations-list"] });
      const snapshot = qc.getQueriesData<Paginated<Regulation>>({ queryKey: ["regulations-list"] });
      qc.setQueriesData<Paginated<Regulation>>({ queryKey: ["regulations-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((r) => r.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete regulation", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Regulation deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Regulation>[] = [
    {
      key: "code",
      header: "Code",
      sortable: true,
      render: (r) => <span className="rounded-md bg-slate-100 px-2 py-0.5 font-mono text-xs font-medium text-slate-700">{r.code}</span>,
    },
    {
      key: "title",
      header: "Regulation",
      sortable: true,
      className: "min-w-52",
      render: (r) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{r.title}</p>
          {r.description && <p className="truncate text-xs text-slate-500">{r.description}</p>}
        </div>
      ),
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (r) => {
        const meta = statusMeta[r.status] ?? { label: r.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    { key: "jurisdiction", header: "Jurisdiction", sortable: true, hideBelow: "md", render: (r) => <span className="text-slate-600">{r.jurisdiction || "—"}</span> },
    { key: "effective_date", header: "Effective", sortable: true, hideBelow: "lg", render: (r) => <span className="text-slate-500">{formatDate(r.effective_date)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (r) => <span className="text-slate-500">{relativeTime(r.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || jurisdictionFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Regulations"
        description="The regulatory requirements governing your compliance program."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New regulation
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(r) => r.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No regulations found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Add your first regulation to start mapping requirements."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="regulations-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="regulations-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="regulations-status">
              <Select
                id="regulations-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={REGULATION_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Jurisdiction" htmlFor="regulations-jurisdiction">
              <Input id="regulations-jurisdiction" value={jurisdictionFilter} onChange={(e) => setJurisdictionFilter(e.target.value)} placeholder="Exact jurisdiction" className="w-44" />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setStatusFilter("");
                  setJurisdictionFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
        rowActions={(r) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${r.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(r) }] : []),
                ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(r) }] : []),
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

      <RegulationFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        regulation={editTarget}
        onSubmit={async (values: RegulationFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete regulation"
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
