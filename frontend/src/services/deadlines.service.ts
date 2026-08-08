import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Deadline } from "@/types/entities";

export interface DeadlineCreateBody {
  title: string;
  description?: string;
  status?: string;
  entity_type?: string;
  entity_id?: string;
  deadline_at: string;
}

export interface DeadlineUpdateBody {
  id: string;
  title?: string;
  description?: string;
  status?: string;
  entity_type?: string;
  entity_id?: string;
  deadline_at?: string;
}

export const deadlinesApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Deadline>> =>
    http.post<Paginated<Deadline>>(API.deadlinesSearch, query, signal),

  get: (id: string): Promise<Deadline> => http.post<Deadline>(API.deadlinesGet, { id }),

  create: (body: DeadlineCreateBody): Promise<Deadline> => http.post<Deadline>(API.deadlines, body),

  update: (body: DeadlineUpdateBody): Promise<Deadline> => http.patch<Deadline>(API.deadlines, body),

  remove: (id: string): Promise<void> => http.del<void>(API.deadlines, { id }),

  /** Marks the deadline completed. */
  complete: (id: string): Promise<Deadline> => http.post<Deadline>(API.deadlinesComplete, { id }),
};
