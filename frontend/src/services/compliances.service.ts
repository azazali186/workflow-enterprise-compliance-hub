import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Compliance } from "@/types/entities";

export interface ComplianceCreateBody {
  name: string;
  description?: string;
  status?: string;
  risk_level?: string;
  owner_id?: string;
  regulation_id?: string;
  due_date?: string | null;
  metadata?: unknown;
}

export interface ComplianceUpdateBody {
  id: string;
  name?: string;
  description?: string;
  status?: string;
  risk_level?: string;
  owner_id?: string;
  regulation_id?: string;
  due_date?: string | null;
}

export interface CheckResult {
  compliance: Compliance;
  changed: boolean;
  reason: string;
}

export const compliancesApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Compliance>> =>
    http.post<Paginated<Compliance>>(API.compliancesSearch, query, signal),

  get: (id: string): Promise<Compliance> => http.post<Compliance>(API.compliancesGet, { id }),

  create: (body: ComplianceCreateBody): Promise<Compliance> => http.post<Compliance>(API.compliances, body),

  update: (body: ComplianceUpdateBody): Promise<Compliance> => http.patch<Compliance>(API.compliances, body),

  remove: (id: string): Promise<void> => http.del<void>(API.compliances, { id }),

  /** Runs the ComplianceCheck saga evaluation for one record. */
  check: (id: string): Promise<CheckResult> => http.post<CheckResult>(API.compliancesCheck, { id }),
};
