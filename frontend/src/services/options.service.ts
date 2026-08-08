/**
 * Dropdown options service — the single source for {id, name} pairs in every
 * form. Backed by the shared POST /api/v1/options endpoint so forms never ask
 * users to type raw UUIDs.
 */
import { http } from "./api/client";
import { API } from "./api/paths";

/** Entity keys the options endpoint knows. Keep in sync with backend/internal/modules/options. */
export type OptionEntity =
  | "users"
  | "roles"
  | "compliances"
  | "regulations"
  | "violations"
  | "checklists"
  | "deadlines"
  | "corrective_actions"
  | "audits"
  | "reports";

export interface OptionItem {
  id: string;
  name: string;
  /** Optional secondary label (e.g. email for users, code for roles). */
  sub?: string;
}

export interface OptionsQuery {
  entities: OptionEntity[];
  search?: string;
  limit?: number;
  /** Resolve specific ids (edit forms) — map of entity → id list. */
  ids?: Partial<Record<OptionEntity, string[]>>;
}

/**
 * Fetch options for one or more entities in a single request.
 * Returns a partial record: only requested (and allowed) entities appear.
 */
export function fetchOptions(query: OptionsQuery, signal?: AbortSignal): Promise<Partial<Record<OptionEntity, OptionItem[]>>> {
  return http.post<Partial<Record<OptionEntity, OptionItem[]>>>(API.options, query, signal);
}

/** Convenience: fetch one entity's options. */
export function fetchEntityOptions(entity: OptionEntity, search?: string, signal?: AbortSignal, limit = 50): Promise<OptionItem[]> {
  return fetchOptions({ entities: [entity], search: search || undefined, limit }, signal).then((r) => r[entity] ?? []);
}

/** Resolve the display name for a stored id (edit forms). */
export function resolveOption(entity: OptionEntity, id: string, signal?: AbortSignal): Promise<OptionItem | undefined> {
  if (!id) return Promise.resolve(undefined);
  return fetchOptions({ entities: [entity], ids: { [entity]: [id] } }, signal).then((r) => r[entity]?.[0]);
}

/**
 * Maps an entity_type value to the options entity feeding its id picker.
 * Unknown/custom types yield undefined — the id picker is disabled for them.
 */
export const ENTITY_TYPE_TO_OPTION: Partial<Record<string, OptionEntity>> = {
  compliance: "compliances",
  audit: "audits",
  violation: "violations",
  regulation: "regulations",
  checklist: "checklists",
  corrective_action: "corrective_actions",
  report: "reports",
};
