import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Plus, AlertTriangle, RefreshCw } from "lucide-react";
import { listMatches } from "@/api/matches";
import type { Match } from "@/api/types";
import { Card, StatusDot, Badge, EmptyState, Button } from "@/components/ui";
import { formatDate, formatScore } from "@/lib/format";

const PAGE_SIZE = 20;

export function Dashboard() {
  const navigate = useNavigate();
  const [offset, setOffset] = useState(0);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['matches', offset],
    queryFn: () => listMatches(PAGE_SIZE, offset),
  });

  const matches = data?.matches ?? [];
  const total = data?.count ?? 0;
  const status = isLoading ? "loading" : isError ? "error" : matches.length === 0 ? "empty" : "data";


  const mapStatusVariant = (
    status: Match["status"],
  ): "success" | "error" | "warning" | "info" | "neutral" => {
    switch (status) {
      case "finished":
        return "success";
      case "failed":
        return "error";
      case "started":
        return "info";
      case "waiting":
        return "warning";
      case "aborted":
        return "neutral";
    }
  };

  /* ─── States ─── */

  if (status === "loading") {
    return (
      <div>
        <div className="mb-6 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-accent">Matches</h1>
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Card key={i} className="animate-pulse">
              <div className="h-4 w-3/4 rounded bg-bg-tertiary" />
              <div className="mt-3 h-3 w-1/2 rounded bg-bg-tertiary" />
              <div className="mt-4 h-3 w-1/3 rounded bg-bg-tertiary" />
            </Card>
          ))}
        </div>
      </div>
    );
  }

  if (status === "empty") {
    return (
      <div>
        <div className="mb-6 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-accent">Matches</h1>
        </div>
        <EmptyState
          title="No matches yet"
          description="Upload a CS2 demo replay to get started with tactical analysis."
          action={{
            label: "Upload Demo",
            onClick: () => navigate("/upload"),
          }}
        />
      </div>
    );
  }

  if (status === "error") {
    return (
      <div>
        <div className="mb-6 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-accent">Matches</h1>
        </div>
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <AlertTriangle size={48} className="mb-4 text-error" />
          <h3 className="text-lg font-semibold text-text-primary">
            Failed to load matches
          </h3>
          <p className="mt-1 text-sm text-text-secondary">
            There was an error fetching your matches.
          </p>
          <Button
            variant="secondary"
            icon={<RefreshCw size={16} />}
            className="mt-6"
            onClick={() => refetch()}
          >
            Retry
          </Button>
        </div>
      </div>
    );
  }

  /* ─── Data ─── */

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-accent">Matches</h1>
        <Button
          variant="primary"
          size="sm"
          icon={<Plus size={16} />}
          onClick={() => navigate("/upload")}
        >
          Upload Demo
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {matches.map((match) => {
          const score = formatScore(match.score_a, match.score_b);

          return (
            <Card
              key={match.id}
              hover
              onClick={() => navigate(`/matches/${match.id}`)}
            >
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <h3 className="font-semibold text-text-primary truncate">
                    {match.team_a ?? "TBD"} vs {match.team_b ?? "TBD"}
                  </h3>
                  {score && (
                    <p className="mt-1 font-mono text-lg font-bold text-accent">
                      {score}
                    </p>
                  )}
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    {match.map_name && (
                      <Badge variant="neutral">{match.map_name}</Badge>
                    )}
                    {match.total_rounds != null && (
                      <Badge variant="neutral">
                        {match.total_rounds} rounds
                      </Badge>
                    )}
                  </div>
                </div>
                <StatusDot status={match.status} />
              </div>

              <div className="mt-3 flex items-center justify-between">
                <Badge variant={mapStatusVariant(match.status)}>
                  {match.status}
                </Badge>
                <span className="font-mono text-xs text-text-dim">
                  {formatDate(match.created_at)}
                </span>
              </div>
            </Card>
          );
        })}
      </div>

      {/* Pagination */}
      {total > PAGE_SIZE && (
        <div className="mt-6 flex items-center justify-center gap-4">
          <Button
            variant="secondary"
            size="sm"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            Previous
          </Button>
          <span className="text-sm text-text-secondary">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}
          </span>
          <Button
            variant="secondary"
            size="sm"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  );
}
