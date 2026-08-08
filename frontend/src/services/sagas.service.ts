import { http } from "./api/client";
import { API } from "./api/paths";
import type { SagaSummary } from "@/types/entities";

export interface SagaSearchResult {
  items: SagaSummary[];
  count: number;
}

export const sagasApi = {
  /** Recent saga summaries from the observability ring (type/status filters). */
  search: (type?: string, status?: string, limit?: number): Promise<SagaSearchResult> =>
    http.post<SagaSearchResult>(API.sagasSearch, { type, status, limit: limit ?? 50 }),

  /** Full live state of one saga from Redis, keyed by type + entity_id. */
  get: (type: string, entity_id: string): Promise<SagaSummary> => http.post<SagaSummary>(API.sagasGet, { type, entity_id }),
};
