import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Audit } from "@/types/entities";

export interface AuditCreateBody {
  title: string;
  description?: string;
  status?: string;
  compliance_id?: string;
  auditor_id?: string;
  scheduled_at?: string | null;
}

export interface AuditUpdateBody {
  id: string;
  title?: string;
  description?: string;
  status?: string;
  compliance_id?: string;
  auditor_id?: string;
  scheduled_at?: string | null;
}

export const auditsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Audit>> =>
    http.post<Paginated<Audit>>(API.auditsSearch, query, signal),

  get: (id: string): Promise<Audit> => http.post<Audit>(API.auditsGet, { id }),

  create: (body: AuditCreateBody): Promise<Audit> => http.post<Audit>(API.audits, body),

  update: (body: AuditUpdateBody): Promise<Audit> => http.patch<Audit>(API.audits, body),

  remove: (id: string): Promise<void> => http.del<void>(API.audits, { id }),

  /** Marks the audit in progress (drives the AuditExecution saga). */
  start: (id: string): Promise<Audit> => http.post<Audit>(API.auditsStart, { id }),

  /** Marks the audit completed. */
  complete: (id: string): Promise<Audit> => http.post<Audit>(API.auditsComplete, { id }),
};
