import { useMutation, useQueryClient } from "@tanstack/react-query";
import { MoreVertical, Plus, Search, ShieldCheck, Trash2 } from "lucide-react";
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
import { formatDate, relativeTime } from "@/lib/format";
import { riskMeta, statusMeta } from "@/lib/constants";
import { PERM } from "@/services/api/paths";
import { compliancesApi } from "@/services/compliances.service";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { COMPLIANCE_STATUSES, SEVERITIES, type Compliance } from "@/types/entities";
import { ComplianceFormModal, type ComplianceFormValues } from "../components/ComplianceFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return {
    field: "created_at",
    from: value.from ? `${value.from}T00:00:00.000Z` : undefined,
    to: value.to ? `${value.to}T23:59:59.999Z` : undefined,
  };
}

export function CompliancesPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [statusFilter, setStatusFilter] = useState("");
  const [riskFilter, setRiskFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Compliance | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Compliance | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.name = debouncedSearch;
    if (statusFilter) f.status = statusFilter;
    if (riskFilter) f.risk_level = riskFilter;
    return f;
  }, [debouncedSearch, statusFilter, riskFilter]);

  const list = useCursorList<Compliance>({
    queryKey: ["compliances-list"],
    queryFn: compliancesApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canCreate = can(PERM.compliancesCreate);
  const canUpdate = can(PERM.compliancesUpdate);
  const canDelete = can(PERM.compliancesDelete);
  const canCheck = can(PERM.compliancesCheck);

  const createMutation = useMutation({
    mutationFn: compliancesApi.create,
    onSuccess: (c) => {
      toast.success("Compliance created", c.name);
      void qc.invalidateQueries({ queryKey: ["compliances-list"] });
    },
    onError: (e) => toast.error("Could not create compliance", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: compliancesApi.update,
    onSuccess: (c) => {
      toast.success("Compliance updated", c.name);
      void qc.invalidateQueries({ queryKey: ["compliances-list"] });
    },
    onError: (e) => toast.error("Could not update compliance", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => compliancesApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["compliances-list"] });
      const snapshot = qc.getQueriesData<Paginated<Compliance>>({ queryKey: ["compliances-list"] });
      qc.setQueriesData<Paginated<Compliance>>({ queryKey: ["compliances-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((c) => c.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete compliance", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Compliance deleted"),
    onSettled: () => void qc.invalidateQueries({ queryKey: ["compliances-list"] }),
  });

  const checkMutation = useMutation({
    mutationFn: (id: string) => compliancesApi.check(id),
    onSuccess: (res) => {
      if (res.changed) toast.success("Status updated", res.reason);
      else toast.info("No status change", res.reason);
      void qc.invalidateQueries({ queryKey: ["compliances-list"] });
    },
    onError: (e) => toast.error("Check failed", ApiError.is(e) ? e.userMessage : undefined),
  });

  const columns: Column<Compliance>[] = [
    {
      key: "name",
      header: "Compliance",
      sortable: true,
      className: "min-w-56",
      render: (c) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-slate-900">{c.name}</p>
          {c.description && <p className="truncate text-xs text-slate-500">{c.description}</p>}
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
    {
      key: "risk_level",
      header: "Risk",
      sortable: true,
      render: (c) => {
        const meta = riskMeta[c.risk_level] ?? { label: c.risk_level, tone: "neutral" as const };
        return <Badge tone={meta.tone}>{meta.label}</Badge>;
      },
    },
    { key: "due_date", header: "Due", sortable: true, hideBelow: "md", render: (c) => <span className="text-slate-600">{formatDate(c.due_date)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (c) => <span className="text-slate-500">{relativeTime(c.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || statusFilter || riskFilter || dateRange.from || dateRange.to);
  const anyAction = canUpdate || canDelete || canCheck;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Compliances"
        description="Monitored requirements with risk levels and due dates."
        actions={
          canCreate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New compliance
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
        emptyTitle="No compliances found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Create your first compliance record to begin tracking."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by name" htmlFor="compliances-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="compliances-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact name" className="w-52 pl-9" />
              </div>
            </Field>
            <Field label="Status" htmlFor="compliances-status">
              <Select
                id="compliances-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={COMPLIANCE_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <Field label="Risk" htmlFor="compliances-risk">
              <Select
                id="compliances-risk"
                value={riskFilter}
                onChange={(e) => setRiskFilter(e.target.value)}
                placeholder="All risks"
                options={SEVERITIES.map((s) => ({ value: s, label: riskMeta[s]?.label ?? s }))}
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
                  setRiskFilter("");
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
                  aria-label={`Actions for ${c.name}`}
                >
                  <MoreVertical className="h-4 w-4" />
                </button>
              )}
              items={[
                ...(canCheck ? [{ key: "check", label: "Run status check", icon: <ShieldCheck className="h-4 w-4" aria-hidden="true" />, onSelect: () => void checkMutation.mutate(c.id) }] : []),
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

      <ComplianceFormModal
        open={createOpen || Boolean(editTarget)}
        onClose={() => {
          setCreateOpen(false);
          setEditTarget(null);
        }}
        compliance={editTarget}
        onSubmit={async (values: ComplianceFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else await createMutation.mutateAsync(values);
          setCreateOpen(false);
          setEditTarget(null);
        }}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete compliance"
        description={deleteTarget ? `"${deleteTarget.name}" will be soft-deleted and hidden from lists. Audit history is preserved.` : ""}
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
