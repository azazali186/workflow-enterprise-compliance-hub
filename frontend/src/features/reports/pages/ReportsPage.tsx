import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Eye, FileText, MoreVertical, Plus, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { Dropdown } from "@/components/ui/Dropdown";
import { Field } from "@/components/ui/Field";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { PaginationBar } from "@/components/ui/PaginationBar";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { DateRangeInputs, type DateRangeValue } from "@/components/ui/Toolbar";
import { useAuth } from "@/hooks/useAuth";
import { useCursorList } from "@/hooks/useCursorList";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { reportTypeLabel, statusMeta } from "@/lib/constants";
import { formatDateTime, relativeTime } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { reportsApi } from "@/services/reports.service";
import { toast } from "@/store/toast.slice";
import { ApiError, type Paginated } from "@/types/api";
import { REPORT_STATUSES, REPORT_TYPES, type Report } from "@/types/entities";
import { ReportFormModal, type ReportFormValues } from "../components/ReportFormModal";

function dateToRange(value: DateRangeValue): { field: string; from?: string; to?: string } | undefined {
  if (!value.from && !value.to) return undefined;
  return { field: "generated_at", from: value.from ? `${value.from}T00:00:00.000Z` : undefined, to: value.to ? `${value.to}T23:59:59.999Z` : undefined };
}

export function ReportsPage() {
  const { can } = useAuth();
  const qc = useQueryClient();

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 350);
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [dateRange, setDateRange] = useState<DateRangeValue>({});
  const [sort, setSort] = useState<{ column: string; direction: "asc" | "desc" } | undefined>(undefined);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Report | null>(null);
  const [viewTarget, setViewTarget] = useState<Report | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Report | null>(null);

  const filters = useMemo(() => {
    const f: Record<string, unknown> = {};
    if (debouncedSearch) f.title = debouncedSearch;
    if (typeFilter) f.type = typeFilter;
    if (statusFilter) f.status = statusFilter;
    return f;
  }, [debouncedSearch, typeFilter, statusFilter]);

  const list = useCursorList<Report>({
    queryKey: ["reports-list"],
    queryFn: reportsApi.search,
    limit: 10,
    sort,
    filters,
    dateRange: dateToRange(dateRange),
  });

  const canGenerate = can(PERM.reportsGenerate);
  const canUpdate = can(PERM.reportsUpdate);
  const canDelete = can(PERM.reportsDelete);

  const invalidate = () => void qc.invalidateQueries({ queryKey: ["reports-list"] });

  const generateMutation = useMutation({
    mutationFn: reportsApi.generate,
    onSuccess: (r) => {
      toast.success("Report generated", r.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not generate report", ApiError.is(e) ? e.userMessage : undefined),
  });

  const updateMutation = useMutation({
    mutationFn: reportsApi.update,
    onSuccess: (r) => {
      toast.success("Report updated", r.title);
      invalidate();
    },
    onError: (e) => toast.error("Could not update report", ApiError.is(e) ? e.userMessage : undefined),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => reportsApi.remove(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ["reports-list"] });
      const snapshot = qc.getQueriesData<Paginated<Report>>({ queryKey: ["reports-list"] });
      qc.setQueriesData<Paginated<Report>>({ queryKey: ["reports-list"] }, (old) =>
        old ? { ...old, items: old.items.filter((r) => r.id !== id), pagination: { ...old.pagination, count: old.pagination.count - 1 } } : old,
      );
      return snapshot;
    },
    onError: (e, _id, snapshot) => {
      if (snapshot) for (const [key, data] of snapshot) if (data) qc.setQueryData(key, data);
      toast.error("Could not delete report", ApiError.is(e) ? e.userMessage : undefined);
    },
    onSuccess: () => toast.success("Report deleted"),
    onSettled: invalidate,
  });

  const columns: Column<Report>[] = [
    {
      key: "title",
      header: "Report",
      sortable: true,
      className: "min-w-56",
      render: (r) => (
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary-50 text-primary-600" aria-hidden="true">
            <FileText className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0">
            <p className="truncate font-medium text-slate-900">{r.title}</p>
            {r.description && <p className="truncate text-xs text-slate-500">{r.description}</p>}
          </div>
        </div>
      ),
    },
    {
      key: "type",
      header: "Type",
      sortable: true,
      render: (r) => <span className="text-slate-700">{reportTypeLabel(r.type)}</span>,
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
    { key: "compliance_id", header: "Compliance", hideBelow: "md", render: (r) => <span className="font-mono text-xs text-slate-600">{r.compliance_id?.slice(0, 8) || "—"}</span> },
    { key: "generated_at", header: "Generated", sortable: true, hideBelow: "lg", render: (r) => <span className="text-slate-500">{formatDateTime(r.generated_at)}</span> },
    { key: "updated_at", header: "Updated", hideBelow: "lg", render: (r) => <span className="text-slate-500">{relativeTime(r.updated_at)}</span> },
  ];

  const hasFilters = Boolean(debouncedSearch || typeFilter || statusFilter || dateRange.from || dateRange.to);

  return (
    <div className="space-y-5">
      <PageHeader
        title="Reports"
        description="Generated compliance reports with aggregate summaries."
        actions={
          canGenerate ? (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setGenerateOpen(true)}>
              Generate report
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
        emptyTitle="No reports found"
        emptyDescription={hasFilters ? "Try adjusting or clearing the filters below." : "Generate a report to see compliance aggregates."}
        toolbar={
          <div className="flex flex-wrap items-end gap-2.5">
            <Field label="Filter by title" htmlFor="reports-search">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <Input id="reports-search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Exact title" className="w-44 pl-9" />
              </div>
            </Field>
            <Field label="Type" htmlFor="reports-type">
              <Select
                id="reports-type"
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
                placeholder="All types"
                options={REPORT_TYPES.map((t) => ({ value: t, label: reportTypeLabel(t) }))}
                className="w-36"
              />
            </Field>
            <Field label="Status" htmlFor="reports-status">
              <Select
                id="reports-status"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                placeholder="All statuses"
                options={REPORT_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
                className="w-36"
              />
            </Field>
            <DateRangeInputs value={dateRange} onChange={setDateRange} />
            {hasFilters && (
              <button
                type="button"
                onClick={() => {
                  setSearch("");
                  setTypeFilter("");
                  setStatusFilter("");
                  setDateRange({});
                }}
                className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
              >
                Clear filters
              </button>
            )}
          </div>
        }
        rowActions={(r) => (
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
              { key: "view", label: "View data", icon: <Eye className="h-4 w-4" aria-hidden="true" />, onSelect: () => setViewTarget(r) },
              ...(canUpdate ? [{ key: "edit", label: "Edit", icon: undefined, onSelect: () => setEditTarget(r) }] : []),
              ...(canDelete ? [{ key: "delete", label: "Delete", danger: true, icon: <Trash2 className="h-4 w-4" aria-hidden="true" />, onSelect: () => setDeleteTarget(r) }] : []),
            ]}
          />
        )}
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

      <ReportFormModal
        open={generateOpen || Boolean(editTarget)}
        onClose={() => {
          setGenerateOpen(false);
          setEditTarget(null);
        }}
        report={editTarget}
        mode={editTarget ? "edit" : "generate"}
        onSubmit={async (values: ReportFormValues) => {
          if (editTarget) await updateMutation.mutateAsync({ id: editTarget.id, ...values });
          else {
            const { status: _status, ...generateBody } = values;
            await generateMutation.mutateAsync(generateBody);
          }
          setGenerateOpen(false);
          setEditTarget(null);
        }}
        submitting={generateMutation.isPending || updateMutation.isPending}
      />

      <ReportDataModal report={viewTarget} onClose={() => setViewTarget(null)} />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete report"
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

function ReportDataModal({ report, onClose }: { report: Report | null; onClose: () => void }) {
  let text = "No data payload.";
  if (report?.data !== undefined && report.data !== null) {
    try {
      text = JSON.stringify(report.data, null, 2);
    } catch {
      text = String(report.data);
    }
  }
  return (
    <Modal open={Boolean(report)} onClose={onClose} title={report ? `Report data — ${report.title}` : ""} description="Aggregate summary stored on this report.">
      <pre className="max-h-96 overflow-auto rounded-lg bg-slate-50 p-4 font-mono text-xs leading-relaxed text-slate-700">{text}</pre>
    </Modal>
  );
}
