import { http } from "./api/client";
import { API } from "./api/paths";
import type { AnalyticsSummary, CountPair } from "@/types/entities";

export const analyticsApi = {
  /** Global totals across every entity + key open-item counters. */
  summary: (): Promise<AnalyticsSummary> => http.post<AnalyticsSummary>(API.analyticsSummary, {}),

  compliances: (): Promise<CountPair[]> => http.post<CountPair[]>(API.analyticsCompliances, {}),

  audits: (): Promise<CountPair[]> => http.post<CountPair[]>(API.analyticsAudits, {}),

  violations: (): Promise<CountPair[]> => http.post<CountPair[]>(API.analyticsViolations, {}),

  deadlines: (): Promise<CountPair[]> => http.post<CountPair[]>(API.analyticsDeadlines, {}),
};
