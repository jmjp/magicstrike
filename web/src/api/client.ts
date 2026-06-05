import axios from "axios";
import { storage } from "@/lib/storage";

export const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080/api/v1";

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  timeout: 30_000,
});

/* ─── Request interceptor — inject JWT ─── */
api.interceptors.request.use((config) => {
  const token = storage.getToken();
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

/* ─── Response interceptor — 401 refresh then retry ─── */
let refreshPromise: Promise<string | null> | null = null;

async function attemptRefresh(): Promise<string | null> {
  try {
    const { data } = await axios.post(`${BASE_URL}/auth/refresh`, null, {
      headers: { Authorization: `Bearer ${storage.getToken()}` },
    });
    const authData = data.data;
    storage.setAuth(
      authData.access_token,
      authData.session_id,
      authData.user,
    );
    return authData.access_token;
  } catch {
    storage.clear();
    return null;
  }
}

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    const isAuthRoute =
      originalRequest?.url?.includes("/auth/callback") ||
      originalRequest?.url?.includes("/auth/magic-link");

    // Only attempt refresh on 401 and not already retried, and not on auth endpoints
    if (
      error.response?.status === 401 &&
      originalRequest &&
      !originalRequest._retry &&
      !isAuthRoute
    ) {
      originalRequest._retry = true;

      if (!refreshPromise) {
        refreshPromise = attemptRefresh().finally(() => {
          refreshPromise = null;
        });
      }

      const newToken = await refreshPromise;
      if (newToken) {
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return api(originalRequest);
      }

      // Refresh failed — redirect to login
      storage.clear();
      window.location.href = "/login";
    }

    return Promise.reject(error);
  },
);
