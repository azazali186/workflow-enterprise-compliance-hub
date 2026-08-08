import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { Report } from "@/types/entities";

export interface ReportGenerateBody {
  title: string;
  type?: string;
  description?: string;
  compliance_id?: string;
}

export interface ReportUpdateBody {
  id: string;
  title?: string;
  type?: string;
  status?: string;
  description?: string;
  compliance_id?: string;
}

export const reportsApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<Report>> =>
    http.post<Paginated<Report>>(API.reportsSearch, query, signal),

  get: (id: string): Promise<Report> => http.post<Report>(API.reportsGet, { id }),

  /** Assembles a summary report for a compliance entity and persists it. */
  generate: (body: ReportGenerateBody): Promise<Report> => http.post<Report>(API.reportsGenerate, body),

  update: (body: ReportUpdateBody): Promise<Report> => http.patch<Report>(API.reports, body),

  remove: (id: string): Promise<void> => http.del<void>(API.reports, { id }),
};
