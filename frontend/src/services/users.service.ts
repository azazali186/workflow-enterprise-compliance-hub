import { http } from "./api/client";
import { API } from "./api/paths";
import type { ListQuery, Paginated } from "@/types/api";
import type { User } from "@/types/entities";

export interface UserCreateBody {
  username: string;
  email?: string;
  password: string;
  role_id: string;
  status?: string;
}

export interface UserUpdateBody {
  id: string;
  username?: string;
  email?: string;
  password?: string;
  role_id?: string;
  status?: string;
}

export const usersApi = {
  search: (query: ListQuery, signal?: AbortSignal): Promise<Paginated<User>> =>
    http.post<Paginated<User>>(API.usersSearch, query, signal),

  get: (id: string): Promise<User> => http.post<User>(API.usersGet, { id }),

  create: (body: UserCreateBody): Promise<User> => http.post<User>(API.users, body),

  update: (body: UserUpdateBody): Promise<User> => http.patch<User>(API.users, body),

  /** Soft delete — returns 204. */
  remove: (id: string): Promise<void> => http.del<void>(API.users, { id }),
};
