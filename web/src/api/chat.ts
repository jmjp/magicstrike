import { api, BASE_URL } from "./client";
import { storage } from "@/lib/storage";
import type {
  ApiResponse,
  ChatInteractData,
  ChatSessionDetail,

  PaginatedChats,
  StreamCallbacks,
} from "./types";

export async function listChats(
  limit = 20,
  offset = 0,
): Promise<PaginatedChats> {
  const { data } = await api.get<ApiResponse<PaginatedChats>>("/chat", {
    params: { limit, offset },
  });
  return data.data;
}

export async function createChat(params: {
  match_ids: string[];
  question: string;
}): Promise<ChatInteractData> {
  const { data } = await api.post<ApiResponse<ChatInteractData>>(
    "/chat",
    params,
  );
  return data.data;
}

export async function getChat(id: string): Promise<ChatSessionDetail> {
  const { data } = await api.get<ApiResponse<ChatSessionDetail>>(
    `/chat/${id}`,
  );
  return data.data;
}

export async function deleteChat(id: string): Promise<void> {
  await api.delete(`/chat/${id}`);
}

async function readSSEStream(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  callbacks: StreamCallbacks,
): Promise<void> {
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      let nl = buffer.indexOf("\n");

      while (nl !== -1) {
        const line = buffer.slice(0, nl).replace(/\r$/, "");
        buffer = buffer.slice(nl + 1);
        nl = buffer.indexOf("\n");

        if (line === "" || line.startsWith(":")) continue;

        if (line.startsWith("data:")) {
          const payload = line.slice(5).trim();
          if (payload === "[DONE]") {
            callbacks.onDone({ source: "qdrant", data_points: [] });
            return;
          }
          if (payload) {
            try {
              const json = JSON.parse(payload);
              if (json.session_id) callbacks.onSessionId?.(json.session_id);
              if (json.type === "error") callbacks.onError(json.message || json.detail || "Stream error");
              else if (json.type === "chunk" || json.content) callbacks.onChunk(json.content);
              else if (json.type === "complete" || json.done) {
                callbacks.onDone({
                  source: json.source || "qdrant",
                  data_points: json.data_points || []
                });
                return;
              }
            } catch {
               // Ignore JSON parse errors
            }
          }
        }
      }
    }
  } catch (err: unknown) {
    if (import.meta.env.DEV) {
      const msg = (err as Error).message ?? "";
      if (!/incomplete.*chunk|network.*error/i.test(msg)) {
        throw err;
      }
    }
  }
}

export function createChatStream(
  params: { match_ids: string[]; question: string },
  callbacks: StreamCallbacks,
): AbortController {
  const controller = new AbortController();
  const token = storage.getToken();

  fetch(`${BASE_URL}/chat/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: token ? `Bearer ${token}` : "",
    },
    body: JSON.stringify(params),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        callbacks.onError(`HTTP ${response.status}${text ? ` — ${text}` : ""}`);
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) {
        callbacks.onError("Browser does not support streaming responses.");
        return;
      }

      try {
        await readSSEStream(reader, callbacks);
      } catch (err: unknown) {
        if ((err as Error).name !== "AbortError") {
          callbacks.onError((err as Error).message ?? "Stream read error");
        }
      }
    })
    .catch((err: Error) => {
      if (err.name !== "AbortError") {
        callbacks.onError(err.message ?? "Network error");
      }
    });

  return controller;
}

export function continueChatStream(
  id: string,
  question: string,
  callbacks: StreamCallbacks,
): AbortController {
  const controller = new AbortController();
  const token = storage.getToken();

  fetch(`${BASE_URL}/chat/stream/${id}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: token ? `Bearer ${token}` : "",
    },
    body: JSON.stringify({ question }),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        callbacks.onError(`HTTP ${response.status}${text ? ` — ${text}` : ""}`);
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) {
        callbacks.onError("Browser does not support streaming responses.");
        return;
      }

      try {
        await readSSEStream(reader, callbacks);
      } catch (err: unknown) {
        if ((err as Error).name !== "AbortError") {
          callbacks.onError((err as Error).message ?? "Stream read error");
        }
      }
    })
    .catch((err: Error) => {
      if (err.name !== "AbortError") {
        callbacks.onError(err.message ?? "Network error");
      }
    });

  return controller;
}
