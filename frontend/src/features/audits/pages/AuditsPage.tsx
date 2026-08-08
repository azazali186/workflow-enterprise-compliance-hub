import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, MoreVertical, Play, Plus, Search, Trash2 } from "lucide-react";
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
import { formatDateTime, relativeTime } from "@/lib/format";
import { auditsApi } from "@/services/audits.service";
import { PERM } from "@/services/api/paths";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { AUDIT_STATUSES, type Audit } from "@/types/entities";
import { AuditFormModal, type AuditFormValues } from "../components/AuditFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "scheduled_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function AuditsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [complianceFilter, setComplianceFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Audit | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Audit | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (complianceFilter) f.compliance_id = complianceFilter;
    return f;
  }, [debouncedSearch, statusFilter, complianceFilter]);

  const list = useCursorList<Audit>({
    queryKey: ["audits-list"],
    queryFn: auditsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.auditsCreate);
  const canUpdate = can(PERM.auditsUpdate);
  const canDelete = can(PERM.auditsDelete);
  const canStart = can(PERM.auditsStart);
  const canComplete = can(PERM.auditsComplete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["audits-list"] });

  const createMutation = useMutation({
    mutationFn: auditsApi.create,
    onSuccess: (a) => {
      toast.success("Audit created", a.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not create audit", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: auditsApi.update,
    onSuccess: (a) => {
      toast.success("Audit updated", a.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not update audit", ApiError.is(e) ? e.userMessage : undefined),
  });

  const startMutation = useMutation({
    mutationFn: (id: string) => auditsApi.start(id),
    onSuccess: (a) => {
      toast.success("Audit started", a.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not start audit", ApiError.is(e) ? e.userMessage : undefined),
  });

  const completeMutation = useMutation({
    mutationFn: (id: string) => auditsApi.complete(id),
    onSuccess: (a) => {
      toast.success("Audit completed", a.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not complete audit", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => auditsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["audits-list"] });
      const snapshot = qc.getQueriesData<Paginated<Audit>>({ queryKey: ["audits-list"] });
      qc.setQueriesData<Paginated<Audit>>({ queryKey: ["audits-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((a) => a.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete audit", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Audit deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Audit>[] = [
    {
      key: "title",
      header: "Audit",
      sortable: true,
      className: "min-w-52",
      render: (a) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{a.title}</p>
          {a.description && <p className="truncate text-xs text-slate-500">{a.description}</p>}
        </div>
      ),
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
    { key: "compliance_id", header: "Compliance", hideBelow: "md", render: (a) => <span className="font-mono text-xs text-slate-600">{a.compliance_id?.slice(0, 8) || "—"}</span> },
    { key: "auditor_id", header: "Auditor", hideBelow: "lg", render: (a) => <span className="text-slate-600">{a.auditor_id || "—"}</span> },
    { key: "scheduled_at", header: "Scheduled", sortable: true, hideBelow: "lg", render: (a) => <span className="text-slate-500">{formatDateTime(a.scheduled_at)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (a) => <span className="text-slate-500">{relativeTime(a.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || complianceFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete || canStart || canComplete;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Audits"
        description="Scheduled and executed compliance audits with lifecycle actions."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New audit
            </Button>
          ) : undefined
        }
      />

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
        emptyTitle="No audits found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Schedule your first audit to start tracking."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="audits-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="audits-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="audits-status">
              <Select
                id="audits-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={AUDIT_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Compliance id" htmlFor="audits-compliance">
              <Input id="audits-compliance" value={complianceFilter} onChange={(e) => setComplianceFilter(e.target.value)} placeholder="Exact UUID" className="w-56" />
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
        rowActions={(a) =>
          anyAction ? (
            <Dropdown
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                  aria-label={`Actions for ${a.title}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canStart && a.status === "scheduled" ? [{ key: "start", label: "Start audit", icon: <Play className="h-4 w-4" aria-hidden="true" />, onSelect: () => void startMutation.mutate(a.id) }] : []),
                ...(canComplete && (a.status === "scheduled" || a.status === "in_progress")
                  ? [{ key: "complete", label: "Mark completed", icon: <CheckCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void completeMutation.mutate(a.id) }]
                  : []),
                ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(a) }] : []),
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

      <AuditFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        audit={editTarget}
        onSubmit={async (values: AuditFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete audit"
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
