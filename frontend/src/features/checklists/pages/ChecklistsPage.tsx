import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ListChecks, MoreVertical, Plus, Search, Trash2 } from "lucide-react";
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
import { checklistsApi } from "@/services/checklists.service";
import { PERM } from "@/services/api/paths";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { CHECKLIST_STATUSES, type Checklist } from "@/types/entities";
import { ChecklistFormModal, type ChecklistFormValues } from "../components/ChecklistFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "created_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function ChecklistsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [complianceFilter, setComplianceFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Checklist | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Checklist | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (complianceFilter) f.compliance_id = complianceFilter;
    return f;
  }, [debouncedSearch, statusFilter, complianceFilter]);

  const list = useCursorList<Checklist>({
    queryKey: ["checklists-list"],
    queryFn: checklistsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.checklistsCreate);
  const canUpdate = can(PERM.checklistsUpdate);
  const canDelete = can(PERM.checklistsDelete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["checklists-list"] });

  const createMutation = useMutation({
    mutationFn: checklistsApi.create,
    onSuccess: (c) => {
      toast.success("Checklist created", c.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not create checklist", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: checklistsApi.update,
    onSuccess: (c) => {
      toast.success("Checklist updated", c.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not update checklist", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => checklistsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["checklists-list"] });
      const snapshot = qc.getQueriesData<Paginated<Checklist>>({ queryKey: ["checklists-list"] });
      qc.setQueriesData<Paginated<Checklist>>({ queryKey: ["checklists-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((c) => c.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete checklist", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Checklist deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Checklist>[] = [
    {
      key: "title",
      header: "Checklist",
      sortable: true,
      className: "min-w-56",
      render: (c) => (
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary-50 text-primary-600" aria-hidden="true">
            <ListChecks className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0">
            <p className="truncate font-medium text-slate-900">{c.title}</p>
            {c.description && <p className="truncate text-xs text-slate-500">{c.description}</p>}
          </div>
        </div>
      ),
    },
    {
      key: "status",
      header: "Status",
      sortable: true,
      render: (c) => {
        const meta = statusMeta[c.status] ?? { label: c.status, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    { key: "compliance_id", header: "Compliance", hideBelow: "md", render: (c) => <span className="font-mono text-xs text-slate-600">{c.compliance_id?.slice(0, 8) || "—"}</span> },
    { key: "owner_id", header: "Owner", hideBelow: "lg", render: (c) => <span className="text-slate-600">{c.owner_id || "—"}</span> },
    { key: "due_date", header: "Due", sortable: true, hideBelow: "lg", render: (c) => <span className="text-slate-500">{formatDate(c.due_date)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (c) => <span className="text-slate-500">{relativeTime(c.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || complianceFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Checklists"
        description="Verification steps attached to compliance records."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New checklist
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        rows={list.page?.items}
        keyExtractor={(c) => c.id}
        isLoading={list.isLoading}
        isError={list.isError}
        error={list.error}
        onRetry={list.refetch}
        sort={sort}
        onSortChange={setSort}
        emptyTitle="No checklists found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Create a checklist to track verification steps."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="checklists-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="checklists-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="checklists-status">
              <Select
                id="checklists-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={CHECKLIST_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Compliance id" htmlFor="checklists-compliance">
              <Input id="checklists-compliance" value={complianceFilter} onChange={(e) => setComplianceFilter(e.target.value)} placeholder="Exact UUID" className="w-56" />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setStatusFilter("");
                  setComplianceFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
        rowActions={(c) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${c.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(c) }] : []),
                ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(c) }] : []),
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

      <ChecklistFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        checklist={editTarget}
        onSubmit={async (values: ChecklistFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete checklist"
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
