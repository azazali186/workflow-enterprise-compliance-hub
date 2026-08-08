import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Violation } from "@/types/entities";

export interface ViolationCreateBody {
  title: string;
  description?: string;
  severity?: string;
  status?: string;
  compliance_id?: string;
  regulation_id?: string;
}

export interface ViolationUpdateBody {
  id: string;
  title?: string;
  description?: string;
  severity?: string;
  status?: string;
  compliance_id?: string;
  regulation_id?: string;
}

export const violationsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Violation>> =>
    http.post<Paginated<Violation>>(API.violationsSearch, query, signal),

  get: (id: string): Promise<Violation> => http.post<Violation>(API.violationsGet, { id }),

  create: (body: ViolationCreateBody): Promise<Violation> => http.post<Violation>(API.violations, body),

  update: (body: ViolationUpdateBody): Promise<Violation> => http.patch<Violation>(API.violations, body),

  remove: (id: string): Promise<void> => http.del<void>(API.violations, { id }),

  /** Marks the violation resolved and drives the ViolationProcessing saga. */
  resolve: (id: string): Promise<Violation> => http.post<Violation>(API.violationsResolve, { id }),
};
