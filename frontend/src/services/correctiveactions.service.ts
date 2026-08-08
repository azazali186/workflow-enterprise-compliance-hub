import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { CorrectiveAction } from "@/types/entities";

export interface CorrectiveActionCreateBody {
  title: string;
  description?: string;
  status?: string;
  violation_id?: string;
  owner_id?: string;
  due_date?: string | null;
}

export interface CorrectiveActionUpdateBody {
  id: string;
  title?: string;
  description?: string;
  status?: string;
  violation_id?: string;
  owner_id?: string;
  due_date?: string | null;
}

export const correctiveActionsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<CorrectiveAction>> =>
    http.post<Paginated<CorrectiveAction>>(API.correctiveActionsSearch, query, signal),

  get: (id: string): Promise<CorrectiveAction> => http.post<CorrectiveAction>(API.correctiveActionsGet, { id }),

  create: (body: CorrectiveActionCreateBody): Promise<CorrectiveAction> => http.post<CorrectiveAction>(API.correctiveActions, body),

  update: (body: CorrectiveActionUpdateBody): Promise<CorrectiveAction> => http.patch<CorrectiveAction>(API.correctiveActions, body),

  remove: (id: string): Promise<void> => http.del<void>(API.correctiveActions, { id }),

  /** Closes the remediation plan and drives the CorrectiveActionFlow saga. */
  complete: (id: string): Promise<CorrectiveAction> => http.post<CorrectiveAction>(API.correctiveActionsComplete, { id }),
};
