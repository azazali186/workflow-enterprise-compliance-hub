import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/services/auth.service";
import { setToken } from "@/services/api/client";
import { useAppDispatch, useAppSelector } from "@/store/hooks";
import { authActions } from "@/store/auth.slice";

/**
 * Session state + RBAC helpers. `can` mirrors the backend Permission
 * middleware: the admin role bypasses every check; everyone else must hold
 * the exact "METHOD path" permission (from /auth/me).
 */
export function useAuth() {
  const auth = useAppSelector((s) => s.auth);
  const dispatch = useAppDispatch();
  const queryClient = useQueryClient();

  const can = useCallback(
    (route: string): boolean => {
      if (auth.role?.code === "admin") return true;
      return auth.permissions.some((p) => p.route === route);
    },
    [auth.permissions, auth.role],
  );

  const logout = useCallback(() => {
    // Invalidate the server-side single-session token first (the client token
    // is still attached), then clear local state regardless of the result.
    authApi.logout().catch(() => undefined);
    // Purge the cached /me session so a different user signing in on this
    // browser never inherits the previous identity or permissions.
    queryClient.removeQueries({ queryKey: ["me"] });
    setToken(null);
    dispatch(authActions.signedOut());
  }, [dispatch, queryClient]);

  return {
    token: auth.token,
    user: auth.user,
    role: auth.role,
    permissions: auth.permissions,
    status: auth.status,
    isAuthenticated: auth.status === "authenticated",
    can,
    logout,
  };
}
