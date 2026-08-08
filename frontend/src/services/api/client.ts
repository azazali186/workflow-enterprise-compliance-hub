/**
 * Central API client. All requests flow through here so that:
 *  - the bearer token is attached automatically,
 *  - the backend envelope is normalized into plain data,
 *  - every failure becomes a typed ApiError (network, HTTP, or API-level),
 *  - a 401 on an authenticated request clears the session once.
 */
import { ApiError, type ApiEnvelope, type ApiErrorBody } from "@/types/api";

const API_BASE = import.meta.env.VITE_API_URL ?? "";

let token: string | null = null;
let unauthorizedHandler: (() => void) | null = null;

export function setToken(next: string | null): void {
  token = next;
}

export function getToken(): string | null {
  return token;
}

/** Called exactly once when an authenticated request receives 401. */
export function onUnauthorized(handler: () => void): void {
  unauthorizedHandler = handler;
}

interface RequestOptions {
  method: string;
  body?: unknown;
  signal?: AbortSignal;
}

/**
 * Removes top-level empty-string fields from request bodies.
 * The backend binds optional UUID fields (compliance_id, regulation_id,
 * owner_id, …) with strict UUID parsing: sending "" yields
 * `bind body failed, err=invalid UUID length: 0` → 400. Omitted fields
 * bind as zero values instead. `null` is preserved (e.g. clearing a date).
 */
function stripEmptyStrings(body: unknown): unknown {
  if (!body || typeof body !== "object" || Array.isArray(body)) return body;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(body as Record<string, unknown>)) {
    if (v !== "") out[k] = v;
  }
  return out;
}

async function request<T>(path: string, opts: RequestOptions): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (token) headers.Authorization = `Bearer ${token}`;

  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      method: opts.method,
      headers,
      body: opts.body !== undefined ? JSON.stringify(stripEmptyStrings(opts.body)) : undefined,
      signal: opts.signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err;
    throw new ApiError(0, "network_error", "Could not reach the server. Check your connection and try again.", null);
  }

  // 204 No Content — nothing to parse.
  if (res.status === 204) return undefined as T;

  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    /* non-JSON body (proxy error pages etc.) */
  }

  if (!res.ok) {
    const body = (payload ?? null) as ApiErrorBody | null;
    if (res.status === 401 && token) unauthorizedHandler?.();
    throw new ApiError(
      res.status,
      body?.code ?? "http_error",
      body?.error || body?.message || `Request failed (${res.status})`,
      body,
    );
  }

  // Normalize the standard envelope; tolerate bare payloads defensively.
  const env = payload as ApiEnvelope<T> | null;
  if (env && typeof env === "object" && "success" in env) {
    if (env.success === false) {
      const body = payload as ApiErrorBody;
      if (res.status === 401 && token) unauthorizedHandler?.();
      throw new ApiError(res.status, body?.code ?? "api_error", body?.error || body?.message || "Request failed", body);
    }
    return env.data as T;
  }
  return payload as T;
}

export const http = {
  get<T>(path: string, signal?: AbortSignal): Promise<T> {
    return request<T>(path, { method: "GET", signal });
  },
  post<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
    return request<T>(path, { method: "POST", body, signal });
  },
  patch<T>(path: string, body: unknown, signal?: AbortSignal): Promise<T> {
    return request<T>(path, { method: "PATCH", body, signal });
  },
  del<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
    return request<T>(path, { method: "DELETE", body, signal });
  },
};
