import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, useEffect, type ReactNode } from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { AppLayout } from "@/components/layout/AppLayout";
import { ShieldCheck } from "lucide-react";
import { authApi } from "@/services/auth.service";
import { setToken } from "@/services/api/client";
import { useAppDispatch, useAppSelector } from "@/store/hooks";
import { authActions } from "@/store/auth.slice";

const LandingPage = lazy(() => import("@/features/landing/pages/LandingPage").then((m) => ({ default: m.LandingPage })));
const LoginPage = lazy(() => import("@/features/auth/pages/LoginPage").then((m) => ({ default: m.LoginPage })));
const DashboardPage = lazy(() => import("@/features/dashboard/pages/DashboardPage").then((m) => ({ default: m.DashboardPage })));
const UsersPage = lazy(() => import("@/features/users/pages/UsersPage").then((m) => ({ default: m.UsersPage })));
const CompliancesPage = lazy(() => import("@/features/compliances/pages/CompliancesPage").then((m) => ({ default: m.CompliancesPage })));
const AlertsPage = lazy(() => import("@/features/alerts/pages/AlertsPage").then((m) => ({ default: m.AlertsPage })));
const AuditLogsPage = lazy(() => import("@/features/auditlogs/pages/AuditLogsPage").then((m) => ({ default: m.AuditLogsPage })));
const ViolationsPage = lazy(() => import("@/features/violations/pages/ViolationsPage").then((m) => ({ default: m.ViolationsPage })));
const RegulationsPage = lazy(() => import("@/features/regulations/pages/RegulationsPage").then((m) => ({ default: m.RegulationsPage })));
const ChecklistsPage = lazy(() => import("@/features/checklists/pages/ChecklistsPage").then((m) => ({ default: m.ChecklistsPage })));
const DeadlinesPage = lazy(() => import("@/features/deadlines/pages/DeadlinesPage").then((m) => ({ default: m.DeadlinesPage })));
const CorrectiveActionsPage = lazy(() => import("@/features/correctiveactions/pages/CorrectiveActionsPage").then((m) => ({ default: m.CorrectiveActionsPage })));
const AuditsPage = lazy(() => import("@/features/audits/pages/AuditsPage").then((m) => ({ default: m.AuditsPage })));
const ReportsPage = lazy(() => import("@/features/reports/pages/ReportsPage").then((m) => ({ default: m.ReportsPage })));
const NotificationsPage = lazy(() => import("@/features/notifications/pages/NotificationsPage").then((m) => ({ default: m.NotificationsPage })));
const SagasPage = lazy(() => import("@/features/sagas/pages/SagasPage").then((m) => ({ default: m.SagasPage })));
const AnalyticsPage = lazy(() => import("@/features/analytics/pages/AnalyticsPage").then((m) => ({ default: m.AnalyticsPage })));

function FullPageLoader() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-4 bg-canvas">
      <span className="brand-mark flex h-10 w-10 animate-pulse items-center justify-center rounded-xl" aria-hidden="true">
        <ShieldCheck className="h-5 w-5 text-white" />
      </span>
      <p className="text-sm text-slate-500">Loading your workspace…</p>
    </div>
  );
}

function Lazy({ children }: { children: ReactNode }) {
  return <Suspense fallback={<FullPageLoader />}>{children}</Suspense>;
}

/** Fetches /auth/me once per session to restore the full user + permissions. */
function SessionBootstrap({ children }: { children: ReactNode }) {
  const dispatch = useAppDispatch();
  const token = useAppSelector((s) => s.auth.token);

  const me = useQuery({
    queryKey: ["me"],
    queryFn: ({ signal }) => authApi.me(signal),
    enabled: Boolean(token),
    staleTime: 5 * 60_000,
    // One retry so a transient network blip can't bounce a valid session to
    // the login screen.
    retry: 1,
  });

  useEffect(() => {
    if (me.data) dispatch(authActions.sessionLoaded(me.data));
    else if (me.isError) {
      setToken(null);
      dispatch(authActions.sessionFailed());
    }
  }, [me.data, me.isError, dispatch]);

  if (me.isPending) return <FullPageLoader />;
  if (me.isError) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAppSelector((s) => s.auth.token);
  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: (
      <Lazy>
        <LandingPage />
      </Lazy>
    ),
  },
  {
    path: "/login",
    element: (
      <Lazy>
        <LoginPage />
      </Lazy>
    ),
  },
  {
    // The authenticated console lives under /app; the marketing landing owns /.
    path: "/app",
    element: (
      <RequireAuth>
        <SessionBootstrap>
          <AppLayout />
        </SessionBootstrap>
      </RequireAuth>
    ),
    children: [
      { index: true, element: (<Lazy><DashboardPage /></Lazy>) },
      { path: "compliances", element: (<Lazy><CompliancesPage /></Lazy>) },
      { path: "regulations", element: (<Lazy><RegulationsPage /></Lazy>) },
      { path: "checklists", element: (<Lazy><ChecklistsPage /></Lazy>) },
      { path: "audits", element: (<Lazy><AuditsPage /></Lazy>) },
      { path: "violations", element: (<Lazy><ViolationsPage /></Lazy>) },
      { path: "alerts", element: (<Lazy><AlertsPage /></Lazy>) },
      { path: "deadlines", element: (<Lazy><DeadlinesPage /></Lazy>) },
      { path: "corrective-actions", element: (<Lazy><CorrectiveActionsPage /></Lazy>) },
      { path: "reports", element: (<Lazy><ReportsPage /></Lazy>) },
      { path: "notifications", element: (<Lazy><NotificationsPage /></Lazy>) },
      { path: "analytics", element: (<Lazy><AnalyticsPage /></Lazy>) },
      { path: "sagas", element: (<Lazy><SagasPage /></Lazy>) },
      { path: "audit-logs", element: (<Lazy><AuditLogsPage /></Lazy>) },
      { path: "users", element: (<Lazy><UsersPage /></Lazy>) },
      { path: "*", element: <Navigate to="/app" replace /> },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
]);
