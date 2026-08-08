import { useQuery } from "@tanstack/react-query";
import { Activity, Bell, ShieldCheck, Users } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { Badge } from "@/components/ui/Badge";
import { Card, CardBody, CardHeader } from "@/components/ui/Card";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { StatCard } from "@/components/ui/StatCard";
import { riskMeta, severityMeta, statusMeta } from "@/lib/constants";
import { relativeTime, shortId } from "@/lib/format";
import { PERM } from "@/services/api/paths";
import { alertsApi } from "@/services/alerts.service";
import { compliancesApi } from "@/services/compliances.service";
import { usersApi } from "@/services/users.service";
import { violationsApi } from "@/services/violations.service";
import type { Alert, Compliance } from "@/types/entities";
import { cn } from "@/lib/cn";

interface SummaryData {
  total?: number;
  groups: Array<{ label: string; count: number; tone: "neutral" | "info" | "success" | "warning" | "danger" }>;
}

function parseSummary(summary: Record<string, unknown> | undefined, groupKey: string, meta: Record<string, { label: string; tone: "neutral" | "info" | "success" | "warning" | "danger" }>): SummaryData {
  if (!summary) return { groups: [] };
  const total = typeof summary.total === "number" ? summary.total : undefined;
  const raw = summary[groupKey];
  const groups: SummaryData["groups"] = [];
  if (raw && typeof raw === "object") {
    for (const [k, v] of Object.entries(raw as Record<string, unknown>)) {
      if (typeof v === "number") groups.push({ label: meta[k]?.label ?? k, count: v, tone: meta[k]?.tone ?? "neutral" });
    }
    groups.sort((a, b) => b.count - a.count);
  }
  return { total, groups };
}

export function DashboardPage() {
  const { can, user } = useAuth();
  const showUsers = can(PERM.usersSearch);

  const compliances = useQuery({
    queryKey: ["dash", "compliances"],
    queryFn: () => compliancesApi.search({ limit: 1, include_summary: true }),
  });
  const openAlerts = useQuery({
    queryKey: ["dash", "open-alerts"],
    queryFn: () => alertsApi.search({ limit: 1, filters: { status: "open" }, include_summary: true }),
  });
  const openViolations = useQuery({
    queryKey: ["dash", "open-violations"],
    queryFn: () => violationsApi.search({ limit: 1, filters: { status: "open" }, include_summary: true }),
  });
  const userSummary = useQuery({
    queryKey: ["dash", "users"],
    queryFn: () => usersApi.search({ limit: 1, include_summary: true }),
    enabled: showUsers,
  });
  const recentAlerts = useQuery({
    queryKey: ["dash", "recent-alerts"],
    queryFn: () => alertsApi.search({ limit: 5, sort: { column: "created_at", direction: "desc" }, include_summary: false }),
  });
  const recentCompliances = useQuery({
    queryKey: ["dash", "recent-compliances"],
    queryFn: () => compliancesApi.search({ limit: 5, sort: { column: "updated_at", direction: "desc" }, include_summary: false }),
  });

  const cStatus = parseSummary(compliances.data?.pagination.summary, "status", statusMeta);
  const aSeverity = parseSummary(openAlerts.data?.pagination.summary, "severity", severityMeta);
  const vSeverity = parseSummary(openViolations.data?.pagination.summary, "severity", severityMeta);
  const uStatus = parseSummary(userSummary.data?.pagination.summary, "status", statusMeta);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight text-slate-900">Good day{user ? `, ${user.username}` : ""}</h1>
        <p className="mt-1 text-sm text-slate-500">A live view of your compliance posture across monitored entities.</p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="Compliances"
          value={cStatus.total ?? null}
          icon={ShieldCheck}
          tone="info"
          loading={compliances.isPending}
          breakdown={cStatus.groups}
        />
        <StatCard
          label="Open alerts"
          value={aSeverity.total ?? null}
          icon={Bell}
          tone="warning"
          loading={openAlerts.isPending}
          breakdown={aSeverity.groups}
        />
        <StatCard
          label="Open violations"
          value={vSeverity.total ?? null}
          icon={Activity}
          tone="danger"
          loading={openViolations.isPending}
          breakdown={vSeverity.groups}
        />
        {showUsers ? (
          <StatCard
            label="Users"
            value={uStatus.total ?? null}
            icon={Users}
            tone="neutral"
            loading={userSummary.isPending}
            breakdown={uStatus.groups}
          />
        ) : (
          <Card className="p-5">
            <p className="text-sm font-medium text-slate-500">Role</p>
            <p className="mt-2 text-sm text-slate-600">
              You have <span className="font-semibold text-slate-900">read-only</span> access. User administration is available to administrators.
            </p>
          </Card>
        )}
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RecentAlertsCard data={recentAlerts.data?.items} loading={recentAlerts.isPending} />
        <RecentCompliancesCard data={recentCompliances.data?.items} loading={recentCompliances.isPending} />
      </div>
    </div>
  );
}

function RecentAlertsCard({ data, loading }: { data: Alert[] | undefined; loading: boolean }) {
  return (
    <Card>
      <CardHeader title="Recent alerts" description="Latest raised by the compliance engine" />
      <CardBody className="p-0">
        {loading ? (
          <SkeletonRows rows={5} cols={3} />
        ) : data && data.length > 0 ? (
          <ul className="divide-y divide-slate-100">
            {data.map((a) => {
              const sev = severityMeta[a.severity] ?? { label: a.severity, tone: "neutral" as const };
              const st = statusMeta[a.status] ?? { label: a.status, tone: "neutral" as const };
              return (
                <li key={a.id} className="flex items-center gap-3 px-5 py-3">
                  <span className={cn("h-2 w-2 shrink-0 rounded-full", dotFor(sev.tone))} aria-hidden="true" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-slate-900">{a.title}</p>
                    <p className="truncate text-xs text-slate-500">
                      {a.type} · {relativeTime(a.created_at)}
                    </p>
                  </div>
                  <Badge tone={sev.tone}>{sev.label}</Badge>
                  <Badge tone={st.tone}>{st.label}</Badge>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="px-5 py-10 text-center text-sm text-slate-500">No alerts yet.</p>
        )}
      </CardBody>
    </Card>
  );
}

function RecentCompliancesCard({ data, loading }: { data: Compliance[] | undefined; loading: boolean }) {
  return (
    <Card>
      <CardHeader title="Recently updated compliances" description="Most recently touched records" />
      <CardBody className="p-0">
        {loading ? (
          <SkeletonRows rows={5} cols={3} />
        ) : data && data.length > 0 ? (
          <ul className="divide-y divide-slate-100">
            {data.map((c) => {
              const st = statusMeta[c.status] ?? { label: c.status, tone: "neutral" as const };
              const rk = riskMeta[c.risk_level] ?? { label: c.risk_level, tone: "neutral" as const };
              return (
                <li key={c.id} className="flex items-center gap-3 px-5 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-slate-900">{c.name}</p>
                    <p className="truncate text-xs text-slate-500">
                      {c.owner_id ? `Owner ${shortId(c.owner_id)} · ` : ""}updated {relativeTime(c.updated_at)}
                    </p>
                  </div>
                  <Badge tone={st.tone}>{st.label}</Badge>
                  <Badge tone={rk.tone}>{rk.label}</Badge>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="px-5 py-10 text-center text-sm text-slate-500">No compliances yet.</p>
        )}
      </CardBody>
    </Card>
  );
}

function dotFor(tone: "neutral" | "info" | "success" | "warning" | "danger"): string {
  switch (tone) {
    case "info":
      return "bg-info-500";
    case "success":
      return "bg-success-500";
    case "warning":
      return "bg-warning-500";
    case "danger":
      return "bg-danger-500";
    default:
      return "bg-slate-400";
  }
}
