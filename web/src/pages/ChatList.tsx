import { useCallback, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Plus,
  MessageSquare,
  AlertTriangle,
  Trash2,
  Send,
  ChevronRight,
  Clock,
  Hash,
} from "lucide-react";
import { listChats, createChat, deleteChat } from "@/api/chat";
import { listMatches } from "@/api/matches";
import type { ChatSessionSummary, Match } from "@/api/types";
import {
  Button,
  Modal,
  Input,
  EmptyState,
  Spinner,
  Badge,
} from "@/components/ui";
import { formatDate } from "@/lib/format";

export function ChatList() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const preselectedMatchId = searchParams.get("match_id");

  /* ─── Chat List ─── */
  const [chats, setChats] = useState<ChatSessionSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<
    "loading" | "data" | "empty" | "error"
  >("loading");

  const fetchChats = useCallback(async () => {
    setStatus("loading");
    try {
      const result = await listChats(20, 0);
      setChats(result.sessions);
      setTotal(result.total);
      setStatus(result.sessions.length === 0 ? "empty" : "data");
    } catch {
      setStatus("error");
    }
  }, []);

  useEffect(() => {
    fetchChats();
  }, [fetchChats]);

  /* ─── New Chat Modal ─── */
  const [modalOpen, setModalOpen] = useState(!!preselectedMatchId);
  const [matches, setMatches] = useState<Match[]>([]);
  const [selectedMatches, setSelectedMatches] = useState<string[]>(
    preselectedMatchId ? [preselectedMatchId] : [],
  );
  const [question, setQuestion] = useState("");
  const [creating, setCreating] = useState(false);
  const [modalError, setModalError] = useState("");
  const [matchesLoading, setMatchesLoading] = useState(false);
  const [matchesError, setMatchesError] = useState("");

  const openModal = useCallback(async () => {
    setModalOpen(true);
    setModalError("");
    setMatchesError("");
    setMatchesLoading(true);
    try {
      const r = await listMatches(50, 0);
      console.log("[ChatList] Matches response:", r);
      const processed = r.matches.filter((m) => m.status === "finished");
      console.log("[ChatList] Processed matches:", processed.length, processed);
      setMatches(processed);
      if (processed.length === 0 && r.matches.length > 0) {
        setMatchesError(
          `${r.matches.length} match(es) found but none are processed yet. Upload and process a demo first.`,
        );
      } else if (processed.length === 0) {
        setMatchesError(
          "No processed matches available. Upload and process a demo first.",
        );
      }
    } catch (err: unknown) {
      console.error("[ChatList] Failed to load matches:", err);
      const msg =
        err instanceof Error ? err.message : "Unknown error loading matches";
      setMatchesError(`Failed to load matches: ${msg}`);
    } finally {
      setMatchesLoading(false);
    }
  }, []);

  const toggleMatch = (id: string) => {
    setSelectedMatches((prev) =>
      prev.includes(id) ? prev.filter((m) => m !== id) : [...prev, id],
    );
  };

  const handleCreateChat = async () => {
    if (selectedMatches.length === 0) {
      setModalError("Select at least one match.");
      return;
    }
    if (!question.trim()) {
      setModalError("Enter a question.");
      return;
    }
    if (question.length > 500) {
      setModalError("Question must be 500 characters or less.");
      return;
    }

    setCreating(true);
    setModalError("");

    try {
      // Use non-streaming POST to create the session quickly.
      // ChatRoom will stream the first answer via continueChatStream.
      const result = await createChat({
        match_ids: selectedMatches,
        question: question.trim(),
      });
      setModalOpen(false);
      // Pass question so ChatRoom can start streaming immediately
      navigate(
        `/chat/${result.session_id}?question=${encodeURIComponent(question.trim())}&matches=${encodeURIComponent(selectedMatches.join(","))}`,
      );
    } catch {
      setModalError("Failed to create chat. Please try again.");
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteChat = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await deleteChat(id);
      setChats((prev) => prev.filter((c) => c.id !== id));
      setTotal((prev) => prev - 1);
    } catch {
      // ignore
    }
  };

  /* ─── Render ─── */

  return (
    <div className="max-w-3xl mx-auto">
      {/* Header */}
      <div className="mb-8 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-accent tracking-tight">
            Chat Sessions
          </h1>
          <p className="mt-1.5 text-sm text-text-secondary">
            {status === "data"
              ? `${total} session${total !== 1 ? "s" : ""} — AI-powered match analysis`
              : "AI-powered match analysis"}
          </p>
        </div>
        <Button
          variant="primary"
          size="sm"
          icon={<Plus size={16} />}
          onClick={openModal}
        >
          New Chat
        </Button>
      </div>

      {/* Content */}
      {renderContent()}

      {/* New Chat Modal */}
      <Modal
        open={modalOpen}
        onClose={() => {
          setModalOpen(false);
          setModalError("");
          setQuestion("");
          setSelectedMatches(preselectedMatchId ? [preselectedMatchId] : []);
        }}
        title="New Analysis Session"
        size="lg"
      >
        <div className="flex flex-col gap-5">
          {/* Match selection */}
          <div>
            <label className="text-sm font-semibold text-text-primary">
              Select Matches
            </label>
            <p className="text-xs text-text-dim mt-0.5 mb-3">
              Choose up to 20 processed matches to analyze together.
            </p>
            {matchesLoading ? (
              <div className="rounded-xl border border-border/50 bg-bg-secondary px-4 py-6 text-center">
                <Spinner size="sm" />
                <p className="text-xs text-text-dim mt-2">Loading matches...</p>
              </div>
            ) : matchesError && matches.length === 0 ? (
              <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 px-4 py-4 text-center">
                <p className="text-xs text-text-secondary">{matchesError}</p>
              </div>
            ) : matches.length > 0 ? (
              <div className="flex max-h-48 flex-col gap-1 overflow-y-auto rounded-xl border border-border/50 bg-bg-secondary p-1">
                {matches.map((m) => {
                  const isSelected = selectedMatches.includes(m.id);
                  return (
                    <button
                      key={m.id}
                      onClick={() => toggleMatch(m.id)}
                      className={`
                        flex items-center gap-3 rounded-lg px-3 py-2.5 text-left
                        transition-all duration-150 cursor-pointer
                        ${isSelected
                          ? "bg-bg-tertiary ring-1 ring-accent/20"
                          : "hover:bg-bg-tertiary/50"
                        }
                      `}
                    >
                      <div
                        className={`
                          flex h-5 w-5 shrink-0 items-center justify-center rounded border-2
                          transition-colors duration-150
                          ${isSelected
                            ? "border-accent bg-accent text-bg-primary"
                            : "border-border hover:border-text-dim"
                          }
                        `}
                      >
                        {isSelected && (
                          <svg
                            width="10"
                            height="10"
                            viewBox="0 0 10 10"
                            fill="none"
                          >
                            <path
                              d="M2 5l2 2 4-4"
                              stroke="currentColor"
                              strokeWidth="2"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                            />
                          </svg>
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <span className="text-sm text-text-primary truncate block">
                          {m.team_a ?? "TBD"} vs {m.team_b ?? "TBD"}
                        </span>
                      </div>
                      {m.map_name && (
                        <Badge variant="neutral">{m.map_name}</Badge>
                      )}
                    </button>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-xl border border-border/50 bg-bg-secondary px-4 py-6 text-center">
                <p className="text-xs text-text-dim">
                  No processed matches available. Upload and process a demo
                  first.
                </p>
              </div>
            )}
            {matchesError && matches.length > 0 && (
              <p className="mt-1.5 text-[10px] text-amber-400">{matchesError}</p>
            )}
            <p className="mt-1.5 text-[10px] text-text-dim text-right">
              {selectedMatches.length}/20 selected
            </p>
          </div>

          {/* Question */}
          <div>
            <label className="text-sm font-semibold text-text-primary">
              Your Question
            </label>
            <p className="text-xs text-text-dim mt-0.5 mb-3">
              What would you like to know about the selected matches?
            </p>
            <Input
              placeholder="e.g. Which team had the best eco round performance?"
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
            <p className="mt-1.5 text-[10px] text-text-dim text-right">
              {question.length}/500 characters
            </p>
          </div>

          {modalError && (
            <div className="flex items-center gap-2 rounded-xl bg-red-500/10 border border-red-500/20 px-4 py-3 text-sm text-error animate-slide-up">
              <AlertTriangle size={16} />
              {modalError}
            </div>
          )}

          <div className="flex justify-end gap-3 pt-1">
            <Button
              variant="ghost"
              onClick={() => setModalOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              icon={<Send size={14} />}
              loading={creating}
              onClick={handleCreateChat}
            >
              Start Analysis
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );

  function renderContent() {
    if (status === "loading") {
      return (
        <div className="flex justify-center py-24">
          <Spinner size="lg" />
        </div>
      );
    }

    if (status === "empty") {
      return (
        <div className="py-16 animate-fade-in">
          <EmptyState
            icon={<MessageSquare size={48} strokeWidth={1.5} />}
            title="No chat sessions yet"
            description="Start a new chat to analyze your matches with AI-powered tactical insights."
            action={{ label: "New Chat", onClick: openModal }}
          />
        </div>
      );
    }

    if (status === "error") {
      return (
        <div className="flex flex-col items-center py-24 text-center animate-fade-in">
          <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-red-500/10 border border-red-500/20">
            <AlertTriangle size={24} className="text-error" />
          </div>
          <h3 className="text-lg font-semibold text-text-primary">
            Failed to load chats
          </h3>
          <p className="mt-1 text-sm text-text-secondary">
            There was a problem connecting to the server.
          </p>
          <Button variant="secondary" className="mt-6" onClick={fetchChats}>
            Retry
          </Button>
        </div>
      );
    }

    return (
      <div className="flex flex-col gap-2 animate-fade-in">
        {chats.map((chat, idx) => (
          <button
            key={chat.id}
            onClick={() => navigate(`/chat/${chat.id}`)}
            className="group flex items-center justify-between p-4 rounded-xl bg-bg-secondary border border-border/40 hover:border-border hover:bg-bg-tertiary/40 transition-all duration-200 text-left cursor-pointer"
            style={{
              animationDelay: `${idx * 50}ms`,
              animationFillMode: "both",
            }}
          >
            <div className="flex items-center gap-4 min-w-0 flex-1">
              {/* Icon */}
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-bg-tertiary border border-border/40 group-hover:border-border/60 transition-colors">
                <MessageSquare
                  size={18}
                  className="text-text-dim group-hover:text-accent transition-colors"
                  strokeWidth={1.5}
                />
              </div>

              {/* Info */}
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-text-primary truncate group-hover:text-accent transition-colors">
                  {chat.last_question}
                </p>
                <div className="flex items-center gap-3 mt-1.5">
                  <span className="inline-flex items-center gap-1 text-[11px] text-text-dim">
                    <Hash size={10} />
                    {chat.match_ids.length} match
                    {chat.match_ids.length > 1 ? "es" : ""}
                  </span>
                  <span className="inline-flex items-center gap-1 text-[11px] text-text-dim">
                    <MessageSquare size={10} />
                    {chat.message_count} msg
                    {chat.message_count !== 1 ? "s" : ""}
                  </span>
                  <span className="inline-flex items-center gap-1 text-[11px] text-text-dim">
                    <Clock size={10} />
                    {formatDate(chat.updated_at)}
                  </span>
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-1 ml-3">
              <button
                onClick={(e) => handleDeleteChat(chat.id, e)}
                className="p-2 rounded-lg text-text-dim hover:bg-bg-tertiary hover:text-error transition-all duration-150 opacity-0 group-hover:opacity-100 cursor-pointer"
                aria-label="Delete chat"
              >
                <Trash2 size={14} />
              </button>
              <ChevronRight
                size={16}
                className="text-text-dim group-hover:text-accent transition-colors"
              />
            </div>
          </button>
        ))}
      </div>
    );
  }
}
