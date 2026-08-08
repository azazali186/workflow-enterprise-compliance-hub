import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { AuditLog } from "@/types/entities";

export const auditLogsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<AuditLog>> =>
    http.post<Paginated<AuditLog>>(API.auditLogsSearch, query, signal),

  get: (id: string): Promise<AuditLog> => http.post<AuditLog>(API.auditLogsGet, { id }),
};
