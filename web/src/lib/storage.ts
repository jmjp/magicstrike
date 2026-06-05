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
    return localStorage.getItem(KEYS.TOKEN);
  },

  setToken(token: string): void {
    localStorage.setItem(KEYS.TOKEN, token);
  },

  getUser(): StoredUser | null {
    const raw = localStorage.getItem(KEYS.USER);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as StoredUser;
    } catch {
      return null;
    }
  },

  setUser(user: StoredUser): void {
    localStorage.setItem(KEYS.USER, JSON.stringify(user));
  },

  getSession(): string | null {
    return localStorage.getItem(KEYS.SESSION);
  },

  setSession(sessionId: string): void {
    localStorage.setItem(KEYS.SESSION, sessionId);
  },

  setAuth(token: string, sessionId: string, user: StoredUser): void {
    this.setToken(token);
    this.setSession(sessionId);
    this.setUser(user);
  },

  clear(): void {
    localStorage.removeItem(KEYS.TOKEN);
    localStorage.removeItem(KEYS.USER);
    localStorage.removeItem(KEYS.SESSION);
  },
};
