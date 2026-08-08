import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Alert } from "@/types/entities";

export interface AlertCreateBody {
  type: string;
  title: string;
  message?: string;
  severity?: string;
  status?: string;
  entity_type?: string;
  entity_id?: string;
}

export interface AlertUpdateBody {
  id: string;
  title?: string;
  message?: string;
  severity?: string;
  status?: string;
}

export const alertsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Alert>> =>
    http.post<Paginated<Alert>>(API.alertsSearch, query, signal),

  get: (id: string): Promise<Alert> => http.post<Alert>(API.alertsGet, { id }),

  create: (body: AlertCreateBody): Promise<Alert> => http.post<Alert>(API.alerts, body),

  update: (body: AlertUpdateBody): Promise<Alert> => http.patch<Alert>(API.alerts, body),

  remove: (id: string): Promise<void> => http.del<void>(API.alerts, { id }),

  acknowledge: (id: string): Promise<Alert> => http.post<Alert>(API.alertsAcknowledge, { id }),

  resolve: (id: string): Promise<Alert> => http.post<Alert>(API.alertsResolve, { id }),
};
