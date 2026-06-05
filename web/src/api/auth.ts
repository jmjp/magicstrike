import { api } from "./client";
import type { ApiResponse, AuthSuccessData } from "./types";

/**
 * POST /auth/magic-link — Request a magic link email.
 * Always returns 202 (even if email doesn't exist).
 */
export async function requestMagicLink(email: string): Promise<void> {
  await api.post("/auth/magic-link", { email });
}

/**
 * POST /auth/callback — Exchange magic link token for JWT session.
 */
export async function authCallback(
  token: string,
): Promise<AuthSuccessData> {
  const { data } = await api.post<ApiResponse<AuthSuccessData>>(
    "/auth/callback",
    { token },
  );
  return data.data;
}

/**
 * POST /auth/refresh — Get a new JWT using current session.
 */
export async function refreshSession(): Promise<AuthSuccessData> {
  const { data } = await api.post<ApiResponse<AuthSuccessData>>(
    "/auth/refresh",
  );
  return data.data;
}

/**
 * DELETE /auth/session — Logout and invalidate current session.
 */
export async function logout(): Promise<void> {
  await api.delete("/auth/session");
}
