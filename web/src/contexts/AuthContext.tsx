import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useReducer,
  type ReactNode,
} from "react";
import * as authApi from "@/api/auth";
import { storage, type StoredUser } from "@/lib/storage";

/* ─── State ─── */

interface AuthState {
  user: StoredUser | null;
  token: string | null;
  sessionId: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

type AuthAction =
  | { type: "RESTORE"; user: StoredUser; token: string; sessionId: string }
  | { type: "LOGIN"; user: StoredUser; token: string; sessionId: string }
  | { type: "LOGOUT" }
  | { type: "LOADED" };

function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case "RESTORE":
      return {
        ...state,
        user: action.user,
        token: action.token,
        sessionId: action.sessionId,
        isAuthenticated: true,
        isLoading: false,
      };
    case "LOGIN":
      return {
        ...state,
        user: action.user,
        token: action.token,
        sessionId: action.sessionId,
        isAuthenticated: true,
        isLoading: false,
      };
    case "LOGOUT":
      return {
        ...state,
        user: null,
        token: null,
        sessionId: null,
        isAuthenticated: false,
        isLoading: false,
      };
    case "LOADED":
      return { ...state, isLoading: false };
    default:
      return state;
  }
}

const initialState: AuthState = {
  user: null,
  token: null,
  sessionId: null,
  isAuthenticated: false,
  isLoading: true,
};

/* ─── Context ─── */

interface AuthContextValue {
  state: AuthState;
  requestMagicLink: (email: string) => Promise<void>;
  handleCallback: (token: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, initialState);

  // Restore session from localStorage on mount
  useEffect(() => {
    const token = storage.getToken();
    const user = storage.getUser();
    const sessionId = storage.getSession();

    if (token && user && sessionId) {
      dispatch({ type: "RESTORE", user, token, sessionId });
    } else {
      dispatch({ type: "LOADED" });
    }
  }, []);

  const requestMagicLink = useCallback(async (email: string) => {
    await authApi.requestMagicLink(email);
  }, []);

  const handleCallback = useCallback(async (token: string) => {
    const authData = await authApi.authCallback(token);
    storage.setAuth(
      authData.access_token,
      authData.session_id,
      authData.user,
    );
    dispatch({
      type: "LOGIN",
      user: authData.user,
      token: authData.access_token,
      sessionId: authData.session_id,
    });
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch {
      // Even if the API call fails, clear local state
    }
    storage.clear();
    dispatch({ type: "LOGOUT" });
  }, []);

  return (
    <AuthContext.Provider
      value={{ state, requestMagicLink, handleCallback, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
