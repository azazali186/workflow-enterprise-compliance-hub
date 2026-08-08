/** Shared display metadata: labels + badge tones for backend enum values. */

export type Tone = "neutral" | "info" | "success" | "warning" | "danger";

export interface Meta {
  label: string;
  tone: Tone;
}

export const statusMeta: Record<string, Meta> = {
  draft: { label: "Draft", tone: "neutral" },
  active: { label: "Active", tone: "info" },
  disabled: { label: "Disabled", tone: "neutral" },
  compliant: { label: "Compliant", tone: "success" },
  non_compliant: { label: "Non-compliant", tone: "danger" },
  archived: { label: "Archived", tone: "neutral" },
  open: { label: "Open", tone: "warning" },
  acknowledged: { label: "Acknowledged", tone: "info" },
  resolved: { label: "Resolved", tone: "success" },
  in_review: { label: "In review", tone: "warning" },
  closed: { label: "Closed", tone: "neutral" },
  scheduled: { label: "Scheduled", tone: "neutral" },
  in_progress: { label: "In progress", tone: "info" },
  completed: { label: "Completed", tone: "success" },
  cancelled: { label: "Cancelled", tone: "neutral" },
  upcoming: { label: "Upcoming", tone: "neutral" },
  due: { label: "Due", tone: "warning" },
  overdue: { label: "Overdue", tone: "danger" },
  success: { label: "Success", tone: "success" },
  failure: { label: "Failure", tone: "danger" },
  failed: { label: "Failed", tone: "danger" },
  generated: { label: "Generated", tone: "success" },
  pending: { label: "Pending", tone: "neutral" },
  compensated: { label: "Compensated", tone: "warning" },
  expired: { label: "Expired", tone: "neutral" },
  inactive: { label: "Inactive", tone: "neutral" },
};

export const sagaTypeLabels: Record<string, string> = {
  compliance_check: "Compliance check",
  audit_execution: "Audit execution",
  violation_processing: "Violation processing",
  corrective_action_flow: "Corrective action flow",
};

export function sagaTypeLabel(type?: string | null): string {
  return (type && sagaTypeLabels[type]) || type || "—";
}

export const reportTypeLabels: Record<string, string> = {
  summary: "Summary",
  detailed: "Detailed",
  compliance: "Compliance",
  audit: "Audit",
};

export function reportTypeLabel(type?: string | null): string {
  return (type && reportTypeLabels[type]) || type || "—";
}

export const severityMeta: Record<string, Meta> = {
  low: { label: "Low", tone: "neutral" },
  medium: { label: "Medium", tone: "warning" },
  high: { label: "High", tone: "danger" },
  critical: { label: "Critical", tone: "danger" },
};

export const riskMeta: Record<string, Meta> = {
  low: { label: "Low risk", tone: "neutral" },
  medium: { label: "Medium risk", tone: "warning" },
  high: { label: "High risk", tone: "danger" },
  critical: { label: "Critical risk", tone: "danger" },
};

export const roleLabels: Record<string, string> = {
  admin: "Administrator",
  compliance_officer: "Compliance Officer",
  viewer: "Viewer",
};

export function roleLabel(code?: string | null): string {
  return (code && roleLabels[code]) || code || "—";
}

export const entityLabels: Record<string, string> = {
  user: "User",
  role: "Role",
  compliance: "Compliance",
  alert: "Alert",
  auditlog: "Audit log",
  violation: "Violation",
  regulation: "Regulation",
  checklist: "Checklist",
  audit: "Audit",
  report: "Report",
  correctiveaction: "Corrective action",
  deadline: "Deadline",
  saga: "Saga",
  notification: "Notification",
};

export function entityLabel(key?: string | null): string {
  return (key && entityLabels[key]) || key || "—";
}

/** Entity_type options for polymorphic refs (deadlines, notifications). The
 * backend stores the type as free-form text; this curated list drives the picker. */
export const ENTITY_TYPES: Array<{ value: string; label: string }> = [
  { value: "compliance", label: "Compliance" },
  { value: "audit", label: "Audit" },
  { value: "violation", label: "Violation" },
  { value: "regulation", label: "Regulation" },
  { value: "checklist", label: "Checklist" },
  { value: "corrective_action", label: "Corrective action" },
  { value: "report", label: "Report" },
];
