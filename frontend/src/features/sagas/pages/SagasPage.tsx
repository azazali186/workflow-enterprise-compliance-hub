import { useQuery } from "@tanstack/react-query";
import { Eye, RefreshCw, Workflow } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field } from "@/components/ui/Field";
import { Modal } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { Toolbar } from "@/components/ui/Toolbar";
import { useAuth } from "@/hooks/useAuth";
import { sagaTypeLabel, statusMeta } from "@/lib/constants";
import { formatDateTime, shortId } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { sagasApi } from "@/services/sagas.service";
import { SAGA_STATUSES, SAGA_TYPES, type SagaSummary } from "@/types/entities";

const LIMITS = [10, 25, 50, 100];

export function SagasPage() {
  const { can } = useAuth();
  const canGet = can(PERM.sagasGet);

  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [limit, setLimit] = useState(50);
  const [viewTarget, setViewTarget] = useState<SagaSummary | null>(null);

  const query = useQuery({
    queryKey: ["sagas-search", typeFilter, statusFilter, limit],
    queryFn: () => sagasApi.search(typeFilter || undefined, statusFilter || undefined, limit),
  });

  const hasFilters = Boolean(typeFilter || statusFilter);

  return (
    <div className="space-y-5">
      <PageHeader
        title="Sagas"
        description="Live orchestration state for the long-running business workflows."
        actions={
          <Button variant="secondary" icon={<RefreshCw className="h-4 w-4" />} onClick={() => void query.refetch()} loading={query.isFetching}>
            Refresh
          </Button>
        }
      />

      <Toolbar>
        <Field label="Type" htmlFor="sagas-type">
          <Select
            id="sagas-type"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            placeholder="All types"
            options={SAGA_TYPES.map((t) => ({ value: t, label: sagaTypeLabel(t) }))}
            className="w-52"
          />
        </Field>
        <Field label="Status" htmlFor="sagas-status">
          <Select
            id="sagas-status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            placeholder="All statuses"
            options={SAGA_STATUSES.map((s) => ({ value: s, label: statusMeta[s]?.label ?? s }))}
            className="w-40"
          />
        </Field>
        <Field label="Limit" htmlFor="sagas-limit">
          <Select
            id="sagas-limit"
            value={String(limit)}
            onChange={(e) => setLimit(Number(e.target.value))}
            options={LIMITS.map((n) => ({ value: String(n), label: `${n} rows` }))}
            className="w-32"
          />
        </Field>
        {hasFilters && (
          <button
            type="button"
            onClick={() => {
              setTypeFilter("");
              setStatusFilter("");
            }}
            className="inline-flex h-9.5 items-center rounded-lg px-3 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
          >
            Clear filters
          </button>
        )}
      </Toolbar>

      <Card>
        {query.isError ? (
          <div className="px-5 py-10 text-center text-sm text-slate-500">Could not load sagas. Is the saga engine running?</div>
        ) : query.isLoading ? (
          <div className="space-y-2.5 p-5">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="h-12 animate-pulse rounded-lg bg-slate-100" />
            ))}
          </div>
        ) : query.data && query.data.items.length > 0 ? (
          <div className="divide-y divide-slate-100">
            {query.data.items.map((s) => (
              <div key={s.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5 transition-colors hover:bg-slate-50/70">
                <div className="flex min-w-0 items-center gap-3">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-ink-900 text-white" aria-hidden="true">
                    <Workflow className="h-4 w-4" />
                  </span>
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-slate-900">{sagaTypeLabel(s.type)}</p>
                    <p className="truncate font-mono text-xs text-slate-400">
                      {shortId(s.id)} · entity {shortId(s.entity_id)} · step {s.current_step}
                    </p>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  {s.error ? <span className="max-w-56 truncate font-mono text-xs text-danger-600" title={s.error}>{s.error}</span> : null}
                  <span className="hidden text-xs text-slate-400 sm:inline">{formatDateTime(s.updated_at)}</span>
                  <Badge tone={statusMeta[s.status]?.tone ?? "neutral"}>{statusMeta[s.status]?.label ?? s.status}</Badge>
                  {canGet && (
                    <button
                      type="button"
                      onClick={() => setViewTarget(s)}
                      className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                      aria-label={`View saga ${s.type}`}
                    >
                      <Eye className="h-4 w-4" />
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="px-5 py-10 text-center">
            <Workflow className="mx-auto h-8 w-8 text-slate-300" aria-hidden="true" />
            <p className="mt-3 text-sm font-medium text-slate-700">No sagas observed</p>
            <p className="mt-1 text-xs text-slate-500">Sagas start when their driving events fire — e.g. creating a compliance or violation.</p>
          </div>
        )}
      </Card>

      <SagaDetailModal saga={viewTarget} onClose={() => setViewTarget(null)} />
    </div>
  );
}

function SagaDetailModal({ saga, onClose }: { saga: SagaSummary | null; onClose: () => void }) {
  const detail = useQuery({
    queryKey: ["sagas-get", saga?.type, saga?.entity_id],
    queryFn: () => sagasApi.get(saga!.type, saga!.entity_id),
    enabled: Boolean(saga),
  });

  const text = detail.data ? JSON.stringify(detail.data, null, 2) : detail.isLoading ? "Loading live state…" : "No live state in Redis (saga finished).";

  return (
    <Modal open={Boolean(saga)} onClose={onClose} title={saga ? `Saga — ${sagaTypeLabel(saga.type)}` : ""} description="Full orchestration state from Redis.">
      <pre className="max-h-96 overflow-auto rounded-lg bg-slate-50 p-4 font-mono text-xs leading-relaxed text-slate-700">{text}</pre>
    </Modal>
  );
}
