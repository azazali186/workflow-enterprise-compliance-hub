import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Regulation } from "@/types/entities";

export interface RegulationCreateBody {
  title: string;
  code: string;
  description?: string;
  jurisdiction?: string;
  status?: string;
  effective_date?: string | null;
  expiry_date?: string | null;
}

export interface RegulationUpdateBody {
  id: string;
  title?: string;
  code?: string;
  description?: string;
  jurisdiction?: string;
  status?: string;
  effective_date?: string | null;
  expiry_date?: string | null;
}

export const regulationsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Regulation>> =>
    http.post<Paginated<Regulation>>(API.regulationsSearch, query, signal),

  get: (id: string): Promise<Regulation> => http.post<Regulation>(API.regulationsGet, { id }),

  create: (body: RegulationCreateBody): Promise<Regulation> => http.post<Regulation>(API.regulations, body),

  update: (body: RegulationUpdateBody): Promise<Regulation> => http.patch<Regulation>(API.regulations, body),

  remove: (id: string): Promise<void> => http.del<void>(API.regulations, { id }),
};
