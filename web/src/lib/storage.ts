const KEYS = {
  TOKEN: "magicstrike_token",
  USER: "magicstrike_user",
  SESSION: "magicstrike_session",
} as const;

export interface StoredUser {
  id: string;
  username: string;
  email: string;
  avatar?: string;
  points: number;
  blocked: boolean;
  created_at: string;
  updated_at: string;
}

export const storage = {
  getToken(): string | null {
    return sessionStorage.getItem(KEYS.TOKEN);
  },

  setToken(token: string): void {
    sessionStorage.setItem(KEYS.TOKEN, token);
  },

  getUser(): StoredUser | null {
    const raw = sessionStorage.getItem(KEYS.USER);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as StoredUser;
    } catch {
      return null;
    }
  },

  setUser(user: StoredUser): void {
    sessionStorage.setItem(KEYS.USER, JSON.stringify(user));
  },

  getSession(): string | null {
    return sessionStorage.getItem(KEYS.SESSION);
  },

  setSession(sessionId: string): void {
    sessionStorage.setItem(KEYS.SESSION, sessionId);
  },

  setAuth(token: string, sessionId: string, user: StoredUser): void {
    this.setToken(token);
    this.setSession(sessionId);
    this.setUser(user);
      window.dispatchEvent(new Event('auth-changed'));
  },

  clear(): void {
    sessionStorage.removeItem(KEYS.TOKEN);
    sessionStorage.removeItem(KEYS.USER);
    sessionStorage.removeItem(KEYS.SESSION);
      window.dispatchEvent(new Event('auth-changed'));
  },
};
