import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Checklist } from "@/types/entities";

export interface ChecklistCreateBody {
  title: string;
  description?: string;
  status?: string;
  compliance_id?: string;
  owner_id?: string;
  due_date?: string | null;
  items?: unknown;
}

export interface ChecklistUpdateBody {
  id: string;
  title?: string;
  description?: string;
  status?: string;
  compliance_id?: string;
  owner_id?: string;
  due_date?: string | null;
  items?: unknown;
}

export const checklistsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Checklist>> =>
    http.post<Paginated<Checklist>>(API.checklistsSearch, query, signal),

  get: (id: string): Promise<Checklist> => http.post<Checklist>(API.checklistsGet, { id }),

  create: (body: ChecklistCreateBody): Promise<Checklist> => http.post<Checklist>(API.checklists, body),

  update: (body: ChecklistUpdateBody): Promise<Checklist> => http.patch<Checklist>(API.checklists, body),

  remove: (id: string): Promise<void> => http.del<void>(API.checklists, { id }),
};
