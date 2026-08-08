import { http } from "./api/client";
import { API } from "./api/paths";
import type { LoginResponse, MeResponse } from "@/types/entities";

export const authApi = {
  /** POST /auth/login — public. Returns a bearer token + session user. */
  login: (username: string, password: string): Promise<LoginResponse> =>
    http.post<LoginResponse>(API.login, { username, password }),

  /** POST /auth/me — requires a valid bearer token. */
  me: (signal?: AbortSignal): Promise<MeResponse> => http.post<MeResponse>(API.me, undefined, signal),

  /** POST /auth/logout — invalidates the single-session token. */
  logout: (): Promise<{ logged_out: boolean }> => http.post<{ logged_out: boolean }>(API.logout),
};
