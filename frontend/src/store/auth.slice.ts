import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { LoginResponse, MeResponse, Permission, Role, User } from "@/types/entities";

const TOKEN_KEY = "ch.token";

function readStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

function persistToken(token: string | null): void {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* storage unavailable — session simply won't survive a reload */
  }
}

export type AuthStatus = "idle" | "authenticated" | "unauthenticated";

export interface AuthState {
  token: string | null;
  user: User | null;
  role: Role | null;
  permissions: Permission[];
  status: AuthStatus;
}

const initialState: AuthState = {
  token: readStoredToken(),
  user: null,
  role: null,
  permissions: [],
  status: "idle",
};

const authSlice = createSlice({
  name: "auth",
  initialState,
  reducers: {
    /** Boot-time hydration from persisted storage. */
    hydrate(state, action: PayloadAction<string | null>) {
      state.token = action.payload;
      state.status = "idle";
    },
    /** /auth/me succeeded — full user, role, and granted permissions. */
    sessionLoaded(state, action: PayloadAction<MeResponse>) {
      state.user = action.payload.user;
      state.role = action.payload.role;
      state.permissions = action.payload.permissions;
      state.status = "authenticated";
    },
    /** /auth/me failed or the token is invalid — back to login. */
    sessionFailed(state) {
      state.user = null;
      state.role = null;
      state.permissions = [];
      state.token = null;
      state.status = "unauthenticated";
    },
    /** Login succeeded — token recorded; /me fills the rest. */
    signedIn(state, action: PayloadAction<{ token: string; user: LoginResponse["user"] }>) {
      state.token = action.payload.token;
      state.status = "idle";
      persistToken(action.payload.token);
      // Minimal session identity so the shell can render before /me resolves.
      state.user = {
        id: action.payload.user.id,
        username: action.payload.user.username,
        email: action.payload.user.email,
        role_id: action.payload.user.role_id,
        status: "active",
        created_at: "",
        updated_at: "",
      };
    },
    signedOut(state) {
      state.token = null;
      state.user = null;
      state.role = null;
      state.permissions = [];
      state.status = "unauthenticated";
      persistToken(null);
    },
  },
});

export const authActions = authSlice.actions;
export default authSlice.reducer;
