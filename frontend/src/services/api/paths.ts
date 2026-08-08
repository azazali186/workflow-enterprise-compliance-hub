/** Route constants — keep in sync with backend/internal/server/router.go. */

export const API = {
  login: "/api/v1/auth/login",
  me: "/api/v1/auth/me",
  logout: "/api/v1/auth/logout",

  rolesSearch: "/api/v1/roles/search",

  users: "/api/v1/users",
  usersSearch: "/api/v1/users/search",
  usersGet: "/api/v1/users/get",

  compliances: "/api/v1/compliances",
  compliancesSearch: "/api/v1/compliances/search",
  compliancesGet: "/api/v1/compliances/get",
  compliancesCheck: "/api/v1/compliances/check",

  alerts: "/api/v1/alerts",
  alertsSearch: "/api/v1/alerts/search",
  alertsGet: "/api/v1/alerts/get",
  alertsAcknowledge: "/api/v1/alerts/acknowledge",
  alertsResolve: "/api/v1/alerts/resolve",

  violations: "/api/v1/violations",
  violationsSearch: "/api/v1/violations/search",
  violationsGet: "/api/v1/violations/get",
  violationsResolve: "/api/v1/violations/resolve",

  regulations: "/api/v1/regulations",
  regulationsSearch: "/api/v1/regulations/search",
  regulationsGet: "/api/v1/regulations/get",

  checklists: "/api/v1/checklists",
  checklistsSearch: "/api/v1/checklists/search",
  checklistsGet: "/api/v1/checklists/get",

  deadlines: "/api/v1/deadlines",
  deadlinesSearch: "/api/v1/deadlines/search",
  deadlinesGet: "/api/v1/deadlines/get",
  deadlinesComplete: "/api/v1/deadlines/complete",

  correctiveActions: "/api/v1/corrective-actions",
  correctiveActionsSearch: "/api/v1/corrective-actions/search",
  correctiveActionsGet: "/api/v1/corrective-actions/get",
  correctiveActionsComplete: "/api/v1/corrective-actions/complete",

  audits: "/api/v1/audits",
  auditsSearch: "/api/v1/audits/search",
  auditsGet: "/api/v1/audits/get",
  auditsStart: "/api/v1/audits/start",
  auditsComplete: "/api/v1/audits/complete",

  reports: "/api/v1/reports",
  reportsSearch: "/api/v1/reports/search",
  reportsGet: "/api/v1/reports/get",
  reportsGenerate: "/api/v1/reports/generate",

  notificationsSend: "/api/v1/notifications/send",
  notificationsEvents: "/api/v1/notifications/events",

  sagasSearch: "/api/v1/sagas/search",
  sagasGet: "/api/v1/sagas/get",

  analyticsSummary: "/api/v1/analytics/summary",
  analyticsCompliances: "/api/v1/analytics/compliances",
  analyticsAudits: "/api/v1/analytics/audits",
  analyticsViolations: "/api/v1/analytics/violations",
  analyticsDeadlines: "/api/v1/analytics/deadlines",

  auditLogsSearch: "/api/v1/audit-logs/search",
  auditLogsGet: "/api/v1/audit-logs/get",

  options: "/api/v1/options",
} as const;

/** Permission route strings — exact "METHOD path" matches the backend seeds. */
export const PERM = {
  usersCreate: "POST /api/v1/users",
  usersSearch: "POST /api/v1/users/search",
  usersGet: "POST /api/v1/users/get",
  usersUpdate: "PATCH /api/v1/users",
  usersDelete: "DELETE /api/v1/users",

  compliancesCreate: "POST /api/v1/compliances",
  compliancesSearch: "POST /api/v1/compliances/search",
  compliancesUpdate: "PATCH /api/v1/compliances",
  compliancesDelete: "DELETE /api/v1/compliances",
  compliancesCheck: "POST /api/v1/compliances/check",

  alertsSearch: "POST /api/v1/alerts/search",
  alertsGet: "POST /api/v1/alerts/get",
  alertsCreate: "POST /api/v1/alerts",
  alertsUpdate: "PATCH /api/v1/alerts",
  alertsDelete: "DELETE /api/v1/alerts",
  alertsAcknowledge: "POST /api/v1/alerts/acknowledge",
  alertsResolve: "POST /api/v1/alerts/resolve",

  violationsSearch: "POST /api/v1/violations/search",
  violationsGet: "POST /api/v1/violations/get",
  violationsCreate: "POST /api/v1/violations",
  violationsUpdate: "PATCH /api/v1/violations",
  violationsDelete: "DELETE /api/v1/violations",
  violationsResolve: "POST /api/v1/violations/resolve",

  regulationsSearch: "POST /api/v1/regulations/search",
  regulationsGet: "POST /api/v1/regulations/get",
  regulationsCreate: "POST /api/v1/regulations",
  regulationsUpdate: "PATCH /api/v1/regulations",
  regulationsDelete: "DELETE /api/v1/regulations",

  checklistsSearch: "POST /api/v1/checklists/search",
  checklistsGet: "POST /api/v1/checklists/get",
  checklistsCreate: "POST /api/v1/checklists",
  checklistsUpdate: "PATCH /api/v1/checklists",
  checklistsDelete: "DELETE /api/v1/checklists",

  deadlinesSearch: "POST /api/v1/deadlines/search",
  deadlinesGet: "POST /api/v1/deadlines/get",
  deadlinesCreate: "POST /api/v1/deadlines",
  deadlinesUpdate: "PATCH /api/v1/deadlines",
  deadlinesDelete: "DELETE /api/v1/deadlines",
  deadlinesComplete: "POST /api/v1/deadlines/complete",

  correctiveActionsSearch: "POST /api/v1/corrective-actions/search",
  correctiveActionsGet: "POST /api/v1/corrective-actions/get",
  correctiveActionsCreate: "POST /api/v1/corrective-actions",
  correctiveActionsUpdate: "PATCH /api/v1/corrective-actions",
  correctiveActionsDelete: "DELETE /api/v1/corrective-actions",
  correctiveActionsComplete: "POST /api/v1/corrective-actions/complete",

  auditsSearch: "POST /api/v1/audits/search",
  auditsGet: "POST /api/v1/audits/get",
  auditsCreate: "POST /api/v1/audits",
  auditsUpdate: "PATCH /api/v1/audits",
  auditsDelete: "DELETE /api/v1/audits",
  auditsStart: "POST /api/v1/audits/start",
  auditsComplete: "POST /api/v1/audits/complete",

  reportsSearch: "POST /api/v1/reports/search",
  reportsGet: "POST /api/v1/reports/get",
  reportsCreate: "POST /api/v1/reports",
  reportsUpdate: "PATCH /api/v1/reports",
  reportsDelete: "DELETE /api/v1/reports",
  reportsGenerate: "POST /api/v1/reports/generate",

  notificationsSend: "POST /api/v1/notifications/send",
  notificationsEvents: "POST /api/v1/notifications/events",

  sagasSearch: "POST /api/v1/sagas/search",
  sagasGet: "POST /api/v1/sagas/get",

  analyticsSummary: "POST /api/v1/analytics/summary",
  analyticsCompliances: "POST /api/v1/analytics/compliances",
  analyticsAudits: "POST /api/v1/analytics/audits",
  analyticsViolations: "POST /api/v1/analytics/violations",
  analyticsDeadlines: "POST /api/v1/analytics/deadlines",

  auditLogsSearch: "POST /api/v1/audit-logs/search",
  auditLogsGet: "POST /api/v1/audit-logs/get",

  options: "POST /api/v1/options",
} as const;
