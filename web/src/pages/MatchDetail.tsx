import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, AlertTriangle, MessageSquare } from "lucide-react";
import { getMatch } from "@/api/matches";
import type { Match } from "@/api/types";
import {
  Card,
  StatusDot,
  Badge,
  Spinner,
  Button,
  EmptyState,
} from "@/components/ui";
import { formatDateTime, formatScore } from "@/lib/format";

export function MatchDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [match, setMatch] = useState<Match | null>(null);
  const [status, setStatus] = useState<
    "loading" | "data" | "empty" | "error"
  >("loading");

  useEffect(() => {
    if (!id) return;

    setStatus("loading");
    getMatch(id)
      .then((data) => {
        setMatch(data);
        setStatus("data");
      })
      .catch((err) => {
        if (err.response?.status === 404) {
          setStatus("empty");
        } else {
          setStatus("error");
        }
      });
  }, [id]);

  if (status === "loading") {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner size="lg" />
      </div>
    );
  }

  if (status === "empty" || !match) {
    return (
      <EmptyState
        title="Match not found"
        description="This match doesn't exist or you don't have access to it."
        action={{ label: "Back to Matches", onClick: () => navigate("/") }}
      />
    );
  }

  if (status === "error") {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertTriangle size={48} className="mb-4 text-error" />
        <h3 className="text-lg font-semibold text-text-primary">
          Failed to load match
        </h3>
        <Button
          variant="secondary"
          className="mt-6"
          onClick={() => navigate("/")}
        >
          Back to Matches
        </Button>
      </div>
    );
  }

  const score = formatScore(match.score_a, match.score_b);

  return (
    <div>
      {/* Back */}
      <button
        onClick={() => navigate("/")}
        className="mb-6 inline-flex items-center gap-2 text-sm text-text-secondary hover:text-accent transition-colors"
      >
        <ArrowLeft size={16} />
        Back to Matches
      </button>

      <Card padding="lg">
        {/* Header */}
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-accent">
              {match.team_a ?? "TBD"} vs {match.team_b ?? "TBD"}
            </h1>
            {score && (
              <p className="mt-2 font-mono text-3xl font-bold text-accent">
                {score}
              </p>
            )}
          </div>
          <div className="flex items-center gap-3">
            <StatusDot status={match.status} label />
            <Button
              variant="secondary"
              size="sm"
              icon={<MessageSquare size={14} />}
              onClick={() =>
                navigate(`/chat?match_id=${match.id}`)
              }
            >
              Ask AI
            </Button>
          </div>
        </div>

        {/* Details grid */}
        <div className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          <DetailItem label="Map" value={match.map_name ?? "—"} />
          <DetailItem
            label="Rounds"
            value={match.total_rounds?.toString() ?? "—"}
          />
          <DetailItem label="Team A" value={match.team_a ?? "—"} />
          <DetailItem label="Team B" value={match.team_b ?? "—"} />
          <DetailItem label="Score A" value={match.score_a?.toString() ?? "—"} />
          <DetailItem label="Score B" value={match.score_b?.toString() ?? "—"} />
          <DetailItem
            label="Status"
            value={
              <Badge
                variant={
                  match.status === "finished"
                    ? "success"
                    : match.status === "failed"
                      ? "error"
                      : match.status === "started"
                        ? "info"
                        : match.status === "aborted"
                          ? "neutral"
                          : "warning"
                }
              >
                {match.status}
              </Badge>
            }
          />
          <DetailItem
            label="Created"
            value={formatDateTime(match.created_at)}
          />
        </div>

        {/* Match ID (technical) */}
        <div className="mt-6 border-t border-border pt-4">
          <p className="font-mono text-xs text-text-dim">
            ID: {match.id}
          </p>
          {match.demo_md5 && (
            <p className="font-mono text-xs text-text-dim mt-1">
              MD5: {match.demo_md5}
            </p>
          )}
        </div>
      </Card>
    </div>
  );
}

function DetailItem({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div>
      <dt className="text-xs font-medium text-text-dim uppercase tracking-wide">
        {label}
      </dt>
      <dd className="mt-1 text-sm text-text-primary">{value}</dd>
    </div>
  );
}
