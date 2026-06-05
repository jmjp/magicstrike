import { api, BASE_URL } from "./client";
import { storage } from "@/lib/storage";
import type {
  ApiResponse,
  ChatInteractData,
  ChatSessionDetail,
  PaginatedChats,
  StreamCallbacks,
} from "./types";

/* ═══════════════════════════════════════════════════════════
   REST (non-streaming)
   ═══════════════════════════════════════════════════════════ */

/**
 * GET /chat — List chat sessions.
 */
export async function listChats(
  limit = 20,
  offset = 0,
): Promise<PaginatedChats> {
  const { data } = await api.get<ApiResponse<PaginatedChats>>("/chat", {
    params: { limit, offset },
  });
  return data.data;
}

/**
 * POST /chat — Create a new chat session (non-streaming fallback).
 */
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

/**
 * GET /chat/:id — Get chat session history (latest 10 messages, newest first).
 */
export async function getChat(id: string): Promise<ChatSessionDetail> {
  const { data } = await api.get<ApiResponse<ChatSessionDetail>>(
    `/chat/${id}`,
  );
  return data.data;
}

/**
 * DELETE /chat/:id — Delete a chat session.
 */
export async function deleteChat(id: string): Promise<void> {
  await api.delete(`/chat/${id}`);
}

/* ═══════════════════════════════════════════════════════════
   SSE Streaming
   ═══════════════════════════════════════════════════════════ */

/**
 * Read the response body stream line by line.  Each complete line that
 * carries a payload is dispatched *immediately* — we do NOT wait for
 * blank-line terminators because many backends omit them (NDJSON style).
 *
 * Supported formats (auto-detected):
 *   data: {"content": "..."}               ← standard SSE
 *   data: {"type":"chunk","content":"..."}  ← typed SSE
 *   {"content": "..."}                      ← bare NDJSON
 *   {"choices":[{"delta":{"content":"..."}}]} ← OpenAI-style
 *   {"delta":{"text":"..."}}                ← Anthropic-style
 *   raw text (non-JSON)                    ← treated as a chunk
 */
async function readSSEStream(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  callbacks: StreamCallbacks,
): Promise<void> {
  const decoder = new TextDecoder();
  let buffer = "";
  let completed = false;

  const markComplete = () => {
    completed = true;
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      const rawChunk = decoder.decode(value, { stream: true });
      console.log("[SSE] ← raw bytes:", JSON.stringify(rawChunk.slice(0, 300)));
      buffer += rawChunk;

      // Process every complete line immediately
      let nl = buffer.indexOf("\n");
      while (nl !== -1) {
        const line = buffer.slice(0, nl).replace(/\r$/, "");
        buffer = buffer.slice(nl + 1);
        processLine(line, callbacks, markComplete);
        nl = buffer.indexOf("\n");
      }
    }

    // Flush whatever is left (final line without trailing newline)
    if (buffer.trim()) {
      console.log("[SSE] ← final flush:", JSON.stringify(buffer.slice(0, 200)));
      processLine(buffer, callbacks, markComplete);
    }
  } catch (err: unknown) {
    const msg = (err as Error).message ?? "";
    // INCOMPLETE_CHUNKED_ENCODING / network errors after we already
    // have content are harmless — the server just closed the stream.
    if (/incomplete.*chunk|network.*error/i.test(msg)) {
      console.log("[SSE] Stream ended:", msg);
    } else {
      throw err;
    }
  }

  // Ensure we always finalize (raw-text streams have no explicit "done" event)
  if (!completed) {
    console.log("[SSE] stream ended without explicit done — finalizing");
    callbacks.onDone({ source: "qdrant", data_points: [] });
  }
}

/** Dispatch a single text line (SSE, NDJSON, or raw). */
function processLine(
  line: string,
  callbacks: StreamCallbacks,
  finish: () => void,
): void {
  // Skip keep-alive comments and empty lines
  if (line === "" || line.startsWith(":")) return;
  // event: / id: / retry: lines are SSE metadata — ignore
  if (/^(event|id|retry):/i.test(line)) return;

  // ── Extract payload ──
  let payload = line;
  if (payload.startsWith("data: ")) {
    payload = payload.slice(6).trim();
  } else if (payload.startsWith("data:")) {
    payload = payload.slice(5).trim();
  }

  if (payload === "" || payload === "[DONE]") return;

  console.log("[SSE] payload:", payload.slice(0, 200));

  // ── Try JSON ──
  try {
    const json = JSON.parse(payload);
    console.log("[SSE] → JSON:", json);
    dispatchSSE(json, callbacks, finish);
    return;
  } catch {
    // not JSON — try concatenated JSON objects
  }

  // ── Concatenated JSON?  e.g. {"a":1}{"b":2} ──
  const parts = payload.split(/\}\s*\{/);
  if (parts.length > 1) {
    let dispatched = false;
    for (let i = 0; i < parts.length; i++) {
      let chunk = parts[i];
      if (i > 0) chunk = "{" + chunk;
      if (i < parts.length - 1) chunk = chunk + "}";
      try {
        const json = JSON.parse(chunk);
        dispatchSSE(json, callbacks, finish);
        dispatched = true;
      } catch {
        // skip
      }
    }
    if (dispatched) return;
  }

  // ── Fallback: raw text → treat as content chunk (preserve newline) ──
  console.log("[SSE] → raw text chunk");
  callbacks.onChunk(payload + "\n");
}

/**
 * Route a parsed SSE JSON object to the appropriate callback.
 * `markComplete` is called when an explicit "complete" event is received
 * (prevents the stream-end fallback from calling onDone twice).
 */
function dispatchSSE(
  json: Record<string, unknown> | string,
  callbacks: StreamCallbacks,
  markComplete: () => void,
): void {
  if (typeof json === "string") {
    callbacks.onChunk(json);
    return;
  }

  const type = json.type as string | undefined;

  // ── Meta / session_id ──
  if (json.session_id) {
    callbacks.onSessionId?.(json.session_id as string);
  }

  // ── Error ──
  if (type === "error") {
    callbacks.onError(
      (json.message ?? json.detail ?? "Stream error") as string,
    );
    return;
  }

  // ── Content chunk (direct) ──
  const content = json.content as string | undefined;
  if (content !== undefined) {
    callbacks.onChunk(content);
    return;
  }

  // ── OpenAI-style: choices[0].delta.content ──
  const choices = json.choices as
    | Array<{ delta?: { content?: string } }>
    | undefined;
  if (choices?.[0]?.delta?.content) {
    callbacks.onChunk(choices[0].delta.content);
    return;
  }

  // ── Anthropic-style: delta.text ──
  const delta = json.delta as { text?: string } | undefined;
  if (delta?.text) {
    callbacks.onChunk(delta.text);
    return;
  }

  // ── Complete / Done ──
  if (type === "complete" || type === "done" || json.done === true) {
    markComplete();
    callbacks.onDone({
      source: (json.source ?? "qdrant") as string,
      data_points: (json.data_points ?? []) as Array<{
        label: string;
        value: string;
      }>,
    });
  }

  // OpenAI-style finish
  if (
    choices?.[0] &&
    !choices[0].delta &&
    (json as Record<string, unknown>).finish_reason
  ) {
    markComplete();
    callbacks.onDone({ source: "qdrant", data_points: [] });
  }
}

/**
 * POST /chat/stream — Create a NEW chat session with SSE streaming.
 *
 * The stream emits:
 *   1. Optional meta event with `session_id`
 *   2. Content chunks (`content` or `choices[0].delta.content`)
 *   3. Completion event (`type: "complete"`) with `source` & `data_points`
 *
 * @returns AbortController to cancel the stream.
 */
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

/**
 * POST /chat/stream/{id} — Continue an existing chat session with SSE streaming.
 *
 * Same event format as `createChatStream`.
 *
 * @returns AbortController to cancel the stream.
 */
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
