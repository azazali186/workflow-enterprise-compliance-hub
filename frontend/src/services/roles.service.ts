import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Role } from "@/types/entities";

export const rolesApi = {
  search: (query: ListQuery = { limit: 100, include_summary: false }, signal?: AbortSignal): Promise<Paginated<Role>> =>
    http.post<Paginated<Role>>(API.rolesSearch, query, signal),

  /** Convenience: every role (few rows), sorted by name. */
  listAll: (signal?: AbortSignal): Promise<Role[]> =>
    rolesApi.search({ limit: 100, sort: { column: "name", direction: "asc" }, include_summary: false }, signal).then(
      (r) => r.items,
    ),
};
