/**
 * Entity types mirroring the backend models (all fields snake_case).
 * Optional fields are nullable per the DB schema.
 */

export interface BaseEntity {
  id: string;
  created_at: string;
  updated_at: string;
}

export interface Permission {
  name: string;
  route: string;
}

export interface Role extends BaseEntity {
  name: string;
  code: string;
  description?: string;
  permissions?: Permission[];
}

export interface User extends BaseEntity {
  username: string;
  email?: string;
  role_id?: string;
  role?: Role;
  status: string;
  last_login_at?: string | null;
}

/** Shape returned by POST /auth/login. */
export interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: {
    id: string;
    username: string;
    email?: string;
    role_id?: string;
    role_code: string;
    role_name?: string;
    last_login?: string | null;
  };
}

/** Shape returned by POST /auth/me. */
export interface MeResponse {
  user: User;
  role: Role;
  permissions: Permission[];
}

export interface Compliance extends BaseEntity {
  name: string;
  description?: string;
  status: string;
  risk_level: string;
  owner_id?: string;
  regulation_id?: string;
  due_date?: string | null;
  last_reviewed_at?: string | null;
  metadata?: unknown;
}

export interface Alert extends BaseEntity {
  type: string;
  title: string;
  message?: string;
  severity: string;
  status: string;
  entity_type?: string;
  entity_id?: string;
  acknowledged_by?: string;
  resolved_at?: string | null;
}

export interface AuditLog extends BaseEntity {
  action: string;
  status: string;
  entity_type?: string;
  entity_id?: string;
  actor_id?: string;
  ip?: string;
  user_agent?: string;
  before_data?: unknown;
  after_data?: unknown;
  changes?: unknown;
  metadata?: unknown;
}

export interface Violation extends BaseEntity {
  title: string;
  description?: string;
  severity: string;
  status: string;
  compliance_id?: string;
  regulation_id?: string;
  detected_at?: string | null;
  resolved_at?: string | null;
}

export interface Regulation extends BaseEntity {
  title: string;
  code: string;
  description?: string;
  jurisdiction?: string;
  status: string;
  effective_date?: string | null;
  expiry_date?: string | null;
}

export interface Checklist extends BaseEntity {
  title: string;
  description?: string;
  status: string;
  compliance_id?: string;
  owner_id?: string;
  due_date?: string | null;
  items?: unknown;
}

export interface Deadline extends BaseEntity {
  title: string;
  description?: string;
  status: string;
  entity_type?: string;
  entity_id?: string;
  deadline_at: string;
  completed_at?: string | null;
}

export interface CorrectiveAction extends BaseEntity {
  title: string;
  description?: string;
  status: string;
  violation_id?: string;
  owner_id?: string;
  due_date?: string | null;
  completed_at?: string | null;
}

/** Backend model: models.Audit — a scheduled/executed compliance audit. */
export interface Audit extends BaseEntity {
  title: string;
  description?: string;
  status: string;
  compliance_id?: string;
  auditor_id?: string;
  scheduled_at?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
}

export interface Report extends BaseEntity {
  title: string;
  type: string;
  status: string;
  description?: string;
  compliance_id?: string;
  data?: unknown;
  generated_at?: string | null;
}

/** Event pushed through the bus (POST /notifications/events -> bus.Event). */
export interface BusEvent {
  id: string;
  subject: string;
  type: string;
  payload?: unknown;
  timestamp: string;
}

/** Compact saga state from the observability ring (POST /sagas/search). */
export interface SagaSummary {
  id: string;
  type: string;
  entity_id: string;
  status: string;
  current_step: number;
  error?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
}

export interface CountPair {
  key: string;
  count: number;
}

/** POST /analytics/summary response. */
export interface AnalyticsSummary {
  totals: CountPair[];
  open_violations: number;
  open_alerts: number;
  overdue_deadlines: number;
}

/* --- enum value lists (from models/enums.go + handler defaults) --- */

export const COMPLIANCE_STATUSES = ["draft", "active", "compliant", "non_compliant", "archived"] as const;
export const ALERT_STATUSES = ["open", "acknowledged", "resolved"] as const;
export const SEVERITIES = ["low", "medium", "high", "critical"] as const;
export const USER_STATUSES = ["active", "disabled"] as const;
export const VIOLATION_STATUSES = ["open", "in_review", "resolved", "closed"] as const;
export const AUDIT_STATUSES = ["scheduled", "in_progress", "completed", "cancelled"] as const;
export const DEADLINE_STATUSES = ["upcoming", "due", "overdue", "completed"] as const;
export const REGULATION_STATUSES = ["active", "inactive", "draft", "expired"] as const;
export const CHECKLIST_STATUSES = ["open", "in_progress", "completed"] as const;
export const CORRECTIVE_ACTION_STATUSES = ["open", "in_progress", "completed", "cancelled"] as const;
export const REPORT_TYPES = ["summary", "detailed", "compliance", "audit"] as const;
export const REPORT_STATUSES = ["generated", "failed"] as const;
export const SAGA_TYPES = ["compliance_check", "audit_execution", "violation_processing", "corrective_action_flow"] as const;
export const SAGA_STATUSES = ["pending", "active", "completed", "failed", "compensated"] as const;
export const ROLE_CODES = ["admin", "compliance_officer", "viewer"] as const;
