import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  ArrowLeft,
  AlertTriangle,
  Bot,
  User,
  Database,
  BrainCircuit,
  Copy,
  Check,
  ChevronDown,
  ArrowUp,
  StopCircle,
} from "lucide-react";
import { getChat, continueChatStream } from "@/api/chat";
import type {
  ChatMessage,
  ChatSessionDetail,
  DataPoint,
} from "@/api/types";
import { Button, Badge, Spinner } from "@/components/ui";
import { formatDateTime } from "@/lib/format";


/* ─── Streaming state sentinel ─── */
interface StreamState {
  question: string;
  content: string;
  source: string | null;
  dataPoints: DataPoint[] | null;
}

export function ChatRoom() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const bottomRef = useRef<HTMLDivElement>(null);
  const chatContainerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const streamRef = useRef<StreamState | null>(null);

  // SSE Smooth Typing Refs
  const targetContentRef = useRef("");
  const displayedContentRef = useRef("");
  const streamDoneRef = useRef(false);
  const typingTimerRef = useRef<number | null>(null);
  const streamMetaRef = useRef<any>(null);
  const streamQuestionRef = useRef("");

  const mountedRef = useRef(true);
  const [session, setSession] = useState<ChatSessionDetail | null>(null);
  const [status, setStatus] = useState<
    "loading" | "data" | "error" | "empty" | "new-stream"
  >("loading");
  const [question, setQuestion] = useState("");
  const [streaming, setStreaming] = useState<StreamState | null>(null);
  const [error, setError] = useState("");
  const [showScrollBottom, setShowScrollBottom] = useState(false);
  const [initialMessageCount, setInitialMessageCount] = useState<number | null>(null);

  /* ─── Track mounted state (StrictMode-safe) ─── */
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      // Do NOT abort the stream here — StrictMode double-invoke
      // would kill the first request. The browser cancels the fetch
      // when the tab closes; user cancels via the Stop button.
    };
  }, []);

  // Set the count when first data loads:
  useEffect(() => {
    if (status === "data" && initialMessageCount === null && session) {
      setInitialMessageCount(session.messages.length);
    }
  }, [status, session, initialMessageCount]);

  /* ─── Smooth typing loop ─── */
  const startTypingLoop = () => {
    if (typingTimerRef.current !== null) return;

    const tick = () => {
      const target = targetContentRef.current;
      const current = displayedContentRef.current;

      if (current.length < target.length) {
        const diff = target.length - current.length;
        // Catch-up algorithm: write faster if target is far ahead, otherwise write smoothly
        const step = diff > 40 ? Math.ceil(diff / 4) : diff > 10 ? 3 : 1;
        const nextContent = current + target.slice(current.length, current.length + step);

        displayedContentRef.current = nextContent;
        setStreaming((prev) => {
          if (!prev) return null;
          const next = { ...prev, content: nextContent };
          streamRef.current = next;
          return next;
        });

        typingTimerRef.current = requestAnimationFrame(tick);
      } else if (streamDoneRef.current) {
        finalizeMessage();
      } else {
        // Queue empty but stream not completed, wait for more chunks
        typingTimerRef.current = requestAnimationFrame(tick);
      }
    };

    typingTimerRef.current = requestAnimationFrame(tick);
  };

  const stopTypingLoop = () => {
    if (typingTimerRef.current !== null) {
      cancelAnimationFrame(typingTimerRef.current);
      typingTimerRef.current = null;
    }
  };

  const finalizeMessage = () => {
    stopTypingLoop();
    const meta = streamMetaRef.current;
    const answer = displayedContentRef.current;
    const q = streamQuestionRef.current;

    const newMsg: ChatMessage = {
      question: q,
      answer,
      source: (meta?.source ?? "qdrant") as "clickhouse" | "qdrant",
      data_points: meta?.data_points ?? [],
      created_at: new Date().toISOString(),
    };

    // Append to session messages
    setSession((s) =>
      s ? { ...s, messages: [newMsg, ...s.messages] } : s,
    );
    setStreaming(null);
    streamRef.current = null;

    // Reset loop variables
    targetContentRef.current = "";
    displayedContentRef.current = "";
    streamDoneRef.current = false;
    streamMetaRef.current = null;

    if (status === "empty" || status === "new-stream") setStatus("data");

    // Focus back on textarea
    setTimeout(() => {
      if (textareaRef.current) {
        textareaRef.current.focus();
        textareaRef.current.style.height = "auto";
      }
    }, 50);
  };

  /* ─── Load session (or handle new-stream redirect) ─── */

  const fetchSession = useCallback(async () => {
    if (!id) return;
    if (!mountedRef.current) return;
    setStatus("loading");
    try {
      const data = await getChat(id);
      if (!mountedRef.current) return;
      setSession(data);
      setStatus(data.messages.length === 0 ? "empty" : "data");
    } catch (err: unknown) {
      if (!mountedRef.current) return;
      const e = err as { response?: { status?: number } };
      if (e.response?.status === 404) setStatus("empty");
      else setStatus("error");
    }
  }, [id]);

  useEffect(() => {
    // If redirected from ChatList with a pending question, start streaming
    const pendingQuestion = searchParams.get("question");
    const pendingMatches = searchParams.get("matches");

    if (pendingQuestion && pendingMatches && id) {
      const matchIds = pendingMatches.split(",").filter(Boolean);
      setStatus("new-stream");
      startPendingStream(id, pendingQuestion, matchIds);
      // Clean URL silently (no remount)
      window.history.replaceState(null, "", `/chat/${id}`);
    } else {
      fetchSession();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  /* ─── Scroll helpers ─── */

  const scrollToBottom = (behavior: ScrollBehavior = "smooth") => {
    bottomRef.current?.scrollIntoView({ behavior });
  };

  useEffect(() => {
    if (streaming) {
      // During active streaming, scroll instantly to prevent animation fights and jitter,
      // but only if the user is already looking at/near the bottom of the chat.
      if (chatContainerRef.current) {
        const { scrollTop, scrollHeight, clientHeight } = chatContainerRef.current;
        const isNearBottom = scrollHeight - scrollTop - clientHeight < 150;
        if (isNearBottom) {
          chatContainerRef.current.scrollTo({
            top: chatContainerRef.current.scrollHeight,
            behavior: "auto",
          });
        }
      }
    } else {
      scrollToBottom("smooth");
    }
  }, [session?.messages, streaming?.content]);

  useEffect(() => {
    if (streaming) {
      scrollToBottom("auto");
    }
  }, [streaming]);

  /* ─── Auto-resize input ─── */
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`;
    }
  }, [question]);

  /* ─── Scroll-to-bottom visibility ─── */
  const handleScroll = () => {
    if (!chatContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } =
      chatContainerRef.current;
    setShowScrollBottom(scrollHeight - scrollTop - clientHeight > 300);
  };

  /* ─── Start streaming for a pending question (redirected from ChatList) ─── */
  function startPendingStream(sessionId: string, q: string, _matchIds: string[]) {
    const initialStream = { question: q, content: "", source: null, dataPoints: null };
    setStreaming(initialStream);
    streamRef.current = initialStream;
    streamQuestionRef.current = q;
    targetContentRef.current = "";
    displayedContentRef.current = "";
    streamDoneRef.current = false;
    streamMetaRef.current = null;

    abortRef.current = continueChatStream(sessionId, q, {
      onChunk: (text) => {
        if (!mountedRef.current) return;
        targetContentRef.current += text;
        startTypingLoop();
      },
      onDone: (meta) => {
        if (!mountedRef.current) return;
        streamMetaRef.current = meta;
        streamDoneRef.current = true;
        if (displayedContentRef.current.length >= targetContentRef.current.length) {
          finalizeMessage();
        }
      },
      onError: (msg) => {
        if (!mountedRef.current) return;
        stopTypingLoop();
        setError(msg);
        setStreaming(null);
        streamRef.current = null;
      },
    });
  }

  /* ─── Send / continue stream ─── */

  const handleSend = async (textToSend?: string) => {
    const activeQuestion = textToSend || question;
    if (!id || !activeQuestion.trim() || streaming) return;

    if (activeQuestion.length > 500) {
      setError("Question must be 500 characters or less.");
      return;
    }

    setError("");
    if (!textToSend) setQuestion("");

    // Start streaming
    const initialStream = { question: activeQuestion.trim(), content: "", source: null, dataPoints: null };
    setStreaming(initialStream);
    streamRef.current = initialStream;
    streamQuestionRef.current = activeQuestion.trim();
    targetContentRef.current = "";
    displayedContentRef.current = "";
    streamDoneRef.current = false;
    streamMetaRef.current = null;

    abortRef.current = continueChatStream(id, activeQuestion.trim(), {
      onChunk: (text) => {
        if (!mountedRef.current) return;
        targetContentRef.current += text;
        startTypingLoop();
      },
      onDone: (meta) => {
        if (!mountedRef.current) return;
        streamMetaRef.current = meta;
        streamDoneRef.current = true;
        if (displayedContentRef.current.length >= targetContentRef.current.length) {
          finalizeMessage();
        }
      },
      onError: (msg) => {
        if (!mountedRef.current) return;
        stopTypingLoop();
        setError(msg);
        setStreaming(null);
        streamRef.current = null;
      },
    });
  };

  /* ─── Stop streaming ─── */
  const handleStop = () => {
    abortRef.current?.abort();
    abortRef.current = null;
    stopTypingLoop();

    const answer = displayedContentRef.current;
    const q = streamQuestionRef.current;
    if (answer) {
      const newMsg: ChatMessage = {
        question: q,
        answer: answer + "\n\n*[Response interrupted]*",
        source: "qdrant",
        data_points: [],
        created_at: new Date().toISOString(),
      };
      setSession((s) =>
        s ? { ...s, messages: [newMsg, ...s.messages] } : s,
      );
      if (status === "empty" || status === "new-stream") setStatus("data");
    }
    setStreaming(null);
    streamRef.current = null;

    targetContentRef.current = "";
    displayedContentRef.current = "";
    streamDoneRef.current = false;
    streamMetaRef.current = null;
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  /* ─── Cleanup on unmount ─── */
  useEffect(() => {
    return () => {
      abortRef.current?.abort();
      stopTypingLoop();
    };
  }, []);

  /* ═══════════════════════════════════════════════
     LOADING STATE
     ═══════════════════════════════════════════════ */
  if (status === "loading") {
    return (
      <div className="flex items-center justify-center py-32">
        <Spinner size="lg" />
      </div>
    );
  }

  /* ═══════════════════════════════════════════════
     ERROR STATE
     ═══════════════════════════════════════════════ */
  if (status === "error") {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertTriangle size={48} className="mb-4 text-error" />
        <h3 className="text-lg font-semibold text-text-primary">
          Failed to load chat
        </h3>
        <Button
          variant="secondary"
          className="mt-6"
          onClick={() => navigate("/chat")}
        >
          Back to Chats
        </Button>
      </div>
    );
  }



  const displayMessages = session ? [...session.messages].reverse() : [];
  if (streaming) {
    displayMessages.push({
      question: streaming.question,
      answer: streaming.content,
      source: (streaming.source || "qdrant") as any,
      data_points: streaming.dataPoints || [],
      created_at: new Date().toISOString(),
      isStreaming: true,
    } as ChatMessage & { isStreaming?: boolean });
  }

  /* ═══════════════════════════════════════════════
     RENDER
     ═══════════════════════════════════════════════ */
  return (
    <div className="flex flex-col bg-bg-primary flex-1 min-h-0 -mx-4 -my-6 h-[calc(100dvh-96px)] md:h-[calc(100vh-96px)] overflow-hidden relative">
      {/* ─── Header ─── */}
      <header className="shrink-0 px-4 py-3 flex items-center justify-between border-b border-border/50 bg-bg-primary/90 backdrop-blur-md z-10">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate("/chat")}
            className="rounded-xl p-2 text-text-dim hover:bg-bg-secondary hover:text-accent border border-transparent hover:border-border/40 transition-all duration-200"
            aria-label="Back to chats"
          >
            <ArrowLeft size={16} />
          </button>
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent text-bg-primary">
              <Bot size={14} strokeWidth={2} />
            </div>
            <div>
              <h1 className="text-sm font-semibold text-text-primary">
                Tactical Advisor
              </h1>
              <p className="text-[11px] text-text-dim">
                {session?.match_ids.length ?? 0} match
                {(session?.match_ids.length ?? 0) !== 1 ? "es" : ""} loaded
              </p>
            </div>
          </div>
        </div>
        <Badge variant="neutral" className="text-[10px]">
          {displayMessages.length} message{displayMessages.length !== 1 ? "s" : ""}
        </Badge>
      </header>

      {/* ─── Messages ─── */}
      <div
        ref={chatContainerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto scroll-smooth"
      >
        <div className="max-w-3xl mx-auto px-4 py-8">
          {displayMessages.length === 0 && !streaming && status !== "new-stream" ? (
            /* ═══ WELCOME / EMPTY ═══ */
            <div className="flex flex-col items-center justify-center text-center py-12 animate-fade-in">
              <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-bg-secondary border border-border/50 shadow-sm">
                <Bot size={28} className="text-text-secondary" strokeWidth={1.5} />
              </div>
              <h2 className="text-xl font-semibold text-text-primary mb-2">
                Match Analysis Assistant
              </h2>
              <p className="text-sm text-text-secondary max-w-md mb-8 leading-relaxed">
                Ask anything about the matched rounds, kill details, strategy
                performance, or match highlights.
              </p>

            </div>
          ) : (
            /* ═══ MESSAGE LIST ═══ */
            <div className="space-y-10">
              {displayMessages.map((msg, i) => {
                const isNewMessage = initialMessageCount !== null && i >= initialMessageCount;
                return (
                  <MessageBubble
                    key={`msg-${i}`}
                    message={msg}
                    index={i}
                    isNew={isNewMessage}
                    isStreaming={(msg as any).isStreaming}
                    onStop={handleStop}
                  />
                );
              })}
            </div>
          )}

          <div ref={bottomRef} />
        </div>
      </div>

      {/* ─── Scroll-to-bottom FAB ─── */}
      {showScrollBottom && (
        <button
          onClick={() => scrollToBottom("smooth")}
          className="absolute bottom-[140px] left-1/2 -translate-x-1/2 p-2 rounded-full bg-bg-tertiary text-text-secondary hover:text-accent hover:bg-border border border-border/50 shadow-lg hover:shadow-xl transition-all duration-200 z-10 animate-fade-in"
          aria-label="Scroll to bottom"
        >
          <ChevronDown size={16} strokeWidth={2.5} />
        </button>
      )}

      {/* ─── Input Area ─── */}
      <div className="shrink-0 bg-bg-primary px-4 pb-4 pt-2">
        <div className="max-w-3xl mx-auto">

          {/* Error banner */}
          {error && (
            <div className="mb-3 flex items-center gap-2 rounded-xl bg-red-500/10 border border-red-500/20 px-4 py-2.5 text-[13px] text-error animate-slide-up">
              <AlertTriangle size={14} />
              {error}
            </div>
          )}

          {/* Input Box */}
          <div
            className={`
              relative flex items-end bg-bg-secondary border rounded-[20px]
              transition-all duration-200
              ${question.trim()
                ? "border-border-accent shadow-[0_0_0_1px_rgba(255,255,255,0.05)]"
                : "border-border hover:border-border-accent"
              }
              focus-within:border-accent/30 focus-within:shadow-[0_0_0_2px_rgba(255,255,255,0.08)]
            `}
          >
            <textarea
              ref={textareaRef}
              value={question}
              onChange={(e) => {
                setQuestion(e.target.value);
                if (error) setError("");
              }}
              onKeyDown={handleKeyDown}
              placeholder={
                streaming ? "Waiting for response..." : "Ask about the match..."
              }
              maxLength={500}
              rows={1}
              disabled={!!streaming}
              className="w-full bg-transparent text-[15px] text-text-primary placeholder:text-text-dim resize-none focus:outline-none px-5 py-3.5 leading-relaxed min-h-[48px] max-h-[200px]"
              style={{ height: "auto" }}
            />

            <div className="flex items-center gap-2 pr-2.5 pb-2.5">
              {question.length > 0 && !streaming && (
                <span
                  className={`text-[10px] font-mono tabular-nums ${question.length >= 450 ? "text-error" : "text-text-dim"
                    }`}
                >
                  {question.length}/500
                </span>
              )}
              <button
                onClick={() => handleSend()}
                disabled={(!question.trim() && !streaming) || !!streaming}
                className={`
                  flex items-center justify-center rounded-full transition-all duration-200
                  ${question.trim() && !streaming
                    ? "bg-accent text-bg-primary hover:bg-accent-hover shadow-sm hover:shadow-md"
                    : "bg-bg-tertiary text-text-dim cursor-not-allowed"
                  }
                  p-2
                `}
                aria-label="Send message"
              >
                <ArrowUp size={16} strokeWidth={2.5} />
              </button>
            </div>
          </div>

          {/* Disclaimer */}
          <p className="text-center text-[10px] text-text-dim mt-2.5 px-4">
            Tactical Advisor uses AI to analyze match data. Verify important
            information before use.
          </p>
        </div>
      </div>
    </div>
  );
}

/* ═══════════════════════════════════════════════════════════
   MESSAGE BUBBLE (completed & streaming message inline)
   ═══════════════════════════════════════════════════════════ */

function MessageBubble({
  message,
  index,
  isNew,
  isStreaming,
  onStop,
}: {
  message: ChatMessage;
  index: number;
  isNew?: boolean;
  isStreaming?: boolean;
  onStop?: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(message.answer);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div
      className="flex flex-col gap-6 animate-slide-up"
      style={{
        animationDelay: isNew ? "0ms" : `${index * 60}ms`,
        animationFillMode: "both",
      }}
    >
      {/* ── User Question ── */}
      <div className="flex items-start gap-3 justify-end ml-12">
        <div className="flex flex-col items-end gap-1.5 max-w-[85%]">
          <div className="bg-bg-secondary border border-border/60 px-5 py-3 rounded-2xl rounded-tr-md text-[15px] text-text-primary leading-relaxed shadow-sm">
            <p className="whitespace-pre-wrap">{message.question}</p>
          </div>
        </div>
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-bg-tertiary ring-1 ring-border/60 mt-0.5">
          <User size={13} className="text-text-secondary" />
        </div>
      </div>

      {/* ── AI Answer ── */}
      <div className="flex items-start gap-3 mr-12 group/msg">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent text-bg-primary mt-0.5">
          <Bot size={13} strokeWidth={2} />
        </div>

        <div className="flex-1 min-w-0 space-y-3">
          <div className="chat-prose">
            {isStreaming && !message.answer ? (
              /* Typing dots when nothing received yet */
              <div className="flex items-center gap-1.5 py-1">
                <span
                  className="h-2 w-2 rounded-full bg-text-dim animate-typing-dot"
                  style={{ animationDelay: "0ms" }}
                />
                <span
                  className="h-2 w-2 rounded-full bg-text-dim animate-typing-dot"
                  style={{ animationDelay: "200ms" }}
                />
                <span
                  className="h-2 w-2 rounded-full bg-text-dim animate-typing-dot"
                  style={{ animationDelay: "400ms" }}
                />
              </div>
            ) : (
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  code: ({ className, children, ...props }) => {
                    const codeText = String(children).replace(/\n$/, "");
                    const isInline = !codeText.includes("\n");
                    if (isInline) {
                      return (
                        <code
                          className="bg-bg-tertiary px-1.5 py-0.5 rounded-md font-mono text-[13px] text-info border border-border/40"
                          {...props}
                        >
                          {children}
                        </code>
                      );
                    }
                    const lang = className?.replace("language-", "") ?? "text";
                    return <CodeBlock language={lang} code={codeText} />;
                  },
                  a: ({ ...props }) => (
                    <a
                      className="text-info underline underline-offset-2 hover:text-blue-400 transition-colors"
                      target="_blank"
                      rel="noopener noreferrer"
                      {...props}
                    />
                  ),
                }}
              >
                {preprocessMath(message.answer)}
              </ReactMarkdown>
            )}
          </div>

          {/* Data Points */}
          {!isStreaming && message.data_points && message.data_points.length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-1 animate-fade-in">
              {message.data_points.map((dp, i) => (
                <span
                  key={i}
                  className="inline-flex items-center gap-1 rounded-full bg-bg-tertiary/70 border border-border/40 px-2.5 py-1 text-[11px] text-text-secondary"
                >
                  <span className="text-text-dim font-medium">{dp.label}:</span>
                  <span>{dp.value}</span>
                </span>
              ))}
            </div>
          )}

          {/* Footer or Stop Button */}
          {isStreaming ? (
            onStop && (
              <button
                onClick={onStop}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-bg-tertiary border border-border/50 text-[11px] text-text-secondary hover:text-error hover:border-red-500/20 hover:bg-red-500/5 transition-all duration-200 cursor-pointer animate-fade-in"
              >
                <StopCircle size={12} />
                Stop generating
              </button>
            )
          ) : (
            <div className="flex items-center justify-between text-[11px] text-text-dim pt-2 animate-fade-in">
              <div className="flex items-center gap-1.5">
                {message.source === "clickhouse" ? (
                  <Database size={10} className="text-success" />
                ) : (
                  <BrainCircuit size={10} className="text-info" />
                )}
                <span className="font-medium">
                  {message.source === "clickhouse" ? "Structured" : "Vector"}
                </span>
                <span className="opacity-40">·</span>
                <span className="opacity-60">
                  {formatDateTime(message.created_at)}
                </span>
              </div>

              <button
                onClick={handleCopy}
                className="flex items-center gap-1.5 px-2 py-1 rounded-lg hover:bg-bg-secondary hover:text-accent transition-all duration-150 cursor-pointer"
                title="Copy response"
              >
                {copied ? (
                  <>
                    <Check size={11} className="text-success" />
                    <span className="text-success text-[10px] font-medium">
                      Copied
                    </span>
                  </>
                ) : (
                  <>
                    <Copy size={11} />
                    <span className="text-[10px]">Copy</span>
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/* ═══════════════════════════════════════════════════════════
   CODE BLOCK (with header & copy)
   ═══════════════════════════════════════════════════════════ */

function CodeBlock({ language, code }: { language: string; code: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="my-4 rounded-xl overflow-hidden border border-border/60 bg-bg-secondary">
      <div className="flex items-center justify-between px-4 py-2 border-b border-border/40 bg-bg-tertiary/50">
        <span className="text-[11px] font-medium text-text-dim uppercase tracking-wider">
          {language}
        </span>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 px-2 py-1 rounded-md hover:bg-bg-tertiary text-text-dim hover:text-accent transition-all duration-150 cursor-pointer"
        >
          {copied ? (
            <>
              <Check size={11} className="text-success" />
              <span className="text-[10px] text-success font-medium">
                Copied
              </span>
            </>
          ) : (
            <>
              <Copy size={11} />
              <span className="text-[10px]">Copy code</span>
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto p-4">
        <code className="font-mono text-[13px] leading-relaxed text-text-primary">
          {code}
        </code>
      </pre>
    </div>
  );
}

function preprocessMath(text: string): string {
  if (!text) return text;

  let formatted = text;

  // Helper to format LaTeX math constructs to readable math expressions
  const formatFormula = (formula: string): string => {
    let clean = formula;

    // Remove LaTeX text formatting
    clean = clean.replace(/\\text\s*\{([^{}]+)\}/g, '$1');
    clean = clean.replace(/\\mathrm\s*\{([^{}]+)\}/g, '$1');
    clean = clean.replace(/\\mathbf\s*\{([^{}]+)\}/g, '$1');

    // Handle fractions \frac{num}{den} -> (num / den)
    let prev;
    do {
      prev = clean;
      clean = clean.replace(/\\frac\s*\{([^{}]+)\}\s*\{([^{}]+)\}/g, '($1 / $2)');
    } while (clean !== prev);

    // Common LaTeX symbols
    clean = clean.replace(/\\times/g, '×');
    clean = clean.replace(/\\cdot/g, '·');
    clean = clean.replace(/\\div/g, '÷');
    clean = clean.replace(/\\pm/g, '±');
    clean = clean.replace(/\\ge/g, '≥');
    clean = clean.replace(/\\le/g, '≤');
    clean = clean.replace(/\\neq/g, '≠');
    clean = clean.replace(/\\approx/g, '≈');
    clean = clean.replace(/\\infty/g, '∞');
    clean = clean.replace(/\\pi/g, 'π');
    clean = clean.replace(/\\alpha/g, 'α');
    clean = clean.replace(/\\beta/g, 'β');
    clean = clean.replace(/\\theta/g, 'θ');

    // Clean slashes and spaces
    clean = clean.replace(/\\/g, ' ');
    clean = clean.replace(/\s+/g, ' ').trim();
    return clean;
  };

  // 1. Process display math blocks: \[ ... \] or $$ ... $$
  const displayMathRegex = /(\\\[|\$\$\s*)([\s\S]*?)(\\\]|\s*\$\$)/g;
  formatted = formatted.replace(displayMathRegex, (_, _start, formula) => {
    if (!formula.trim()) return "";
    const cleanFormula = formatFormula(formula);
    if (!cleanFormula) return "";
    return `\n\n> \`${cleanFormula}\`\n\n`;
  });

  // 2. Process bracketed math blocks: [ \text{...} ]
  // Usually starts with [ and ends with ] containing a math structure like \frac, \text, \times
  const bracketMathRegex = /\[\s*(\\text|\\frac|\\mathrm|\\times|\\cdot|\\div)[\s\S]*?\]/g;
  formatted = formatted.replace(bracketMathRegex, (match) => {
    const formula = match.slice(1, -1);
    if (!formula.trim()) return match;
    const cleanFormula = formatFormula(formula);
    if (!cleanFormula) return match;
    return `\n\n> \`${cleanFormula}\`\n\n`;
  });

  // 3. Process inline math blocks: \( ... \)
  const inlineMathRegex = /\\\( ([\s\S]*?) \\\)/g;
  formatted = formatted.replace(inlineMathRegex, (_, formula) => {
    if (!formula.trim()) return "";
    const cleanFormula = formatFormula(formula);
    if (!cleanFormula) return "";
    return ` \`${cleanFormula}\` `;
  });

  return formatted;
}

