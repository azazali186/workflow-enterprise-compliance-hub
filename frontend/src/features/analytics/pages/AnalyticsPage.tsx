import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, BarChart3, CalendarX2, Scale } from "lucide-react";
import { useMemo } from "react";
import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import { StatCard } from "@/components/ui/StatCard";
import { useAuth } from "@/hooks/useAuth";
import { severityMeta, statusMeta } from "@/lib/constants";
import { analyticsApi } from "@/services/analytics.service";
import { PERM } from "@/services/api/paths";
import type { CountPair } from "@/types/entities";
import { cn } from "@/lib/cn";

const TOTAL_LABELS: Record<string, { label: string; icon?: string }> = {
  regulations: { label: "Regulations" },
  compliances: { label: "Compliances" },
  audits: { label: "Audits" },
  checklists: { label: "Checklists" },
  alerts: { label: "Alerts" },
  reports: { label: "Reports" },
  violations: { label: "Violations" },
  corrective_actions: { label: "Corrective actions" },
  deadlines: { label: "Deadlines" },
  audit_logs: { label: "Audit log entries" },
};

export function AnalyticsPage() {
  const { can } = useAuth();
  const enabled = can(PERM.analyticsSummary) || can(PERM.analyticsCompliances);

  const summary = useQuery({ queryKey: ["analytics-summary"], queryFn: analyticsApi.summary, enabled });
  const compliances = useQuery({ queryKey: ["analytics-compliances"], queryFn: analyticsApi.compliances, enabled: can(PERM.analyticsCompliances) });
  const audits = useQuery({ queryKey: ["analytics-audits"], queryFn: analyticsApi.audits, enabled: can(PERM.analyticsAudits) });
  const violations = useQuery({ queryKey: ["analytics-violations"], queryFn: analyticsApi.violations, enabled: can(PERM.analyticsViolations) });
  const deadlines = useQuery({ queryKey: ["analytics-deadlines"], queryFn: analyticsApi.deadlines, enabled: can(PERM.analyticsDeadlines) });

  const totals = useMemo(() => {
    const map: Record<string, number> = {};
    for (const t of summary.data?.totals ?? []) map[t.key] = t.count;
    return map;
  }, [summary.data]);

  return (
    <div className="space-y-5">
      <PageHeader title="Analytics" description="Aggregate distributions across the compliance program — computed live from the database." />

      {/* Headline counters */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Open violations" value={summary.data?.open_violations} loading={summary.isLoading} tone="danger" icon={Scale} />
        <StatCard label="Open alerts" value={summary.data?.open_alerts} loading={summary.isLoading} tone="warning" icon={AlertTriangle} />
        <StatCard label="Overdue deadlines" value={summary.data?.overdue_deadlines} loading={summary.isLoading} tone="danger" icon={CalendarX2} />
        <StatCard label="Total records" value={Object.values(totals).reduce((a, b) => a + b, 0)} loading={summary.isLoading} tone="neutral" icon={BarChart3} />
      </div>

      {/* Entity totals */}
      <Card>
        <div className="border-b border-slate-100 px-5 py-4">
          <h2 className="text-sm font-semibold text-slate-900">Records by entity</h2>
        </div>
        {summary.isLoading ? (
          <div className="grid grid-cols-2 gap-4 p-5 sm:grid-cols-3 lg:grid-cols-5">
            {Array.from({ length: 10 }).map((_, i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-px overflow-hidden bg-slate-100 sm:grid-cols-3 lg:grid-cols-5">
            {Object.entries(TOTAL_LABELS).map(([key, meta]) => (
              <div key={key} className="bg-white px-5 py-4">
                <p className="text-2xl font-bold tabular tracking-tight text-slate-900">{totals[key] ?? 0}</p>
                <p className="mt-0.5 truncate text-xs font-medium text-slate-500">{meta.label}</p>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Distributions */}
      <div className="grid grid-cols-1 gap-5 xl:grid-cols-2">
        <DistributionCard title="Compliances by status" data={compliances.data} loading={compliances.isLoading} meta={statusMeta} />
        <DistributionCard title="Audits by status" data={audits.data} loading={audits.isLoading} meta={statusMeta} />
        <DistributionCard title="Violations by severity" data={violations.data} loading={violations.isLoading} meta={severityMeta} />
        <DistributionCard title="Deadlines by status" data={deadlines.data} loading={deadlines.isLoading} meta={statusMeta} />
      </div>
    </div>
  );
}

function DistributionCard({ title, data, loading, meta }: { title: string; data?: CountPair[]; loading: boolean; meta: Record<string, { label: string; tone: "neutral" | "info" | "success" | "warning" | "danger" }> }) {
  const max = Math.max(1, ...(data ?? []).map((d) => d.count));
  return (
    <Card>
      <div className="border-b border-slate-100 px-5 py-4">
        <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
      </div>
      <div className="space-y-3.5 p-5">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-6" />)
        ) : data && data.length > 0 ? (
          data.map((d) => (
            <div key={d.key}>
              <div className="mb-1 flex items-center justify-between text-xs">
                <span className="font-medium text-slate-600">{meta[d.key]?.label ?? d.key}</span>
                <span className="tabular text-slate-500">{d.count}</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-slate-100">
                <div
                  className={cn("h-full rounded-full transition-all duration-500", toneClass(meta[d.key]?.tone ?? "neutral"))}
                  style={{ width: `${(d.count / max) * 100}%` }}
                />
              </div>
            </div>
          ))
        ) : (
          <p className="py-4 text-center text-sm text-slate-400">No records yet.</p>
        )}
      </div>
    </Card>
  );
}

const barTones: Record<string, string> = {
  neutral: "bg-slate-400",
  info: "bg-primary-500",
  success: "bg-success-500",
  warning: "bg-warning-500",
  danger: "bg-danger-500",
};

function toneClass(tone: string): string {
  return barTones[tone] ?? barTones.neutral;
}
