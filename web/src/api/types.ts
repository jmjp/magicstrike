/* ─── Shared API Types ─── */

export interface User {
  id: string;
  username: string;
  email: string;
  avatar?: string;
  points: number;
  blocked: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthSuccessData {
  access_token: string;
  session_id: string;
  user: User;
  expires_at: string;
}


export interface Match {
  id: string;
  user_id: string;
  status: "waiting" | "started" | "finished" | "aborted" | "failed";
  team_a?: string;
  team_b?: string;
  demo_md5?: string;
  score_a?: number;
  score_b?: number;
  total_rounds?: number;
  map_name?: string;
  created_at: string;
  updated_at: string;
}

export interface DataPoint {
  label: string;
  value: string;
}

export interface ChatMessage {
  question: string;
  answer: string;
  source: "clickhouse" | "qdrant";
  data_points?: DataPoint[];
  created_at: string;
}

export interface ChatSessionSummary {
  id: string;
  match_ids: string[];
  message_count: number;
  last_question: string;
  created_at: string;
  updated_at: string;
}

export interface ChatSessionDetail {
  id: string;
  match_ids: string[];
  messages: ChatMessage[];
  created_at: string;
  updated_at: string;
  expires_at: string;
}

export interface ChatInteractData {
  session_id: string;
  answer: string;
  source: "clickhouse" | "qdrant";
  matches_used: string[];
  data_points: DataPoint[];
}

export interface UploadRequestData {
  upload_url: string;
  bucket_key: string;
  expires_at: string;
  match_id: string;
}

export interface ApiError {
  type: string;
  title: string;
  status: number;
  detail: string;
  instance?: string;
}

export interface PaginatedMatches {
  matches: Match[];
  limit: number;
  offset: number;
  count: number;
}

export interface PaginatedChats {
  sessions: ChatSessionSummary[];
  total: number;
  limit: number;
  offset: number;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T;
  message?: string;
}

/* ─── SSE Streaming Types ─── */

/** A single SSE event from the chat stream. */
export interface StreamEvent {
  type?: "meta" | "chunk" | "complete" | "error";
  session_id?: string;
  content?: string;
  source?: "clickhouse" | "qdrant";
  data_points?: DataPoint[];
  message?: string;
}

/** Non-streaming fallback for create-chat (used to get session_id quickly). */
export interface StreamCreateMeta {
  session_id: string;
  matches_used: string[];
}

/** Callbacks for the streaming chat consumer. */
export interface StreamCallbacks {
  onSessionId?: (sessionId: string) => void;
  onChunk: (content: string) => void;
  onDone: (meta: { source: string; data_points: DataPoint[] }) => void;
  onError: (error: string) => void;
}
