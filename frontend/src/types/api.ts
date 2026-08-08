/**
 * Shared API contract types. These mirror the backend response envelope
 * (respond.Body / respond.ErrorBody) and the cursor-pagination engine
 * (internal/pagination) — never invent shapes the backend does not send.
 */

/** Success envelope: { success: true, data?, meta?, message? } */
export interface ApiEnvelope<T> {
  success: boolean;
  data?: T;
  meta?: unknown;
  message?: string;
}

/** Error envelope: { success: false, error, code?, message? } */
export interface ApiErrorBody {
  success: false;
  error: string;
  code?: string;
  message?: string;
}

/** Normalized error thrown by the API client for every failed request. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: ApiErrorBody | null;

  constructor(status: number, code: string, message: string, body: ApiErrorBody | null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.body = body;
  }

  static is(err: unknown): err is ApiError {
    return err instanceof ApiError;
  }

  /** Human-friendly message preferring the backend's explicit message field. */
  get userMessage(): string {
    return this.body?.message || this.message;
  }
}

/** Pagination block returned by every list/search endpoint. */
export interface PaginationSummary {
  count: number;
  limit: number;
  has_more: boolean;
  cursor?: string;
  next_cursor?: string;
  /** Present when include_summary: true — always contains "total". */
  summary?: Record<string, unknown>;
}

/** Standard paginated result: { items, pagination }. */
export interface Paginated<T> {
  items: T[];
  pagination: PaginationSummary;
}

export type SortDirection = "asc" | "desc";

export interface SortSpec {
  column: string;
  direction: SortDirection;
}

export interface DateRangeSpec {
  /** Logical column, defaults to created_at on the backend. */
  field?: string;
  from?: string | null;
  to?: string | null;
}

/** Body of every list/report request (backend pagination.Query). */
export interface ListQuery {
  cursor?: string;
  limit?: number;
  sort?: SortSpec;
  /** Equality / IN filters on allowlisted columns. */
  filters?: Record<string, unknown>;
  date_range?: DateRangeSpec;
  include_summary?: boolean;
}
