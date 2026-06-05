type Status = "waiting" | "started" | "finished" | "aborted" | "failed";

interface StatusDotProps {
  status: Status;
  label?: boolean;
}

const statusConfig: Record<
  Status,
  { color: string; pulse: boolean; text: string }
> = {
  waiting: {
    color: "bg-amber-500",
    pulse: true,
    text: "Waiting",
  },
  started: {
    color: "bg-blue-500",
    pulse: true,
    text: "Processing",
  },
  finished: {
    color: "bg-success",
    pulse: false,
    text: "Finished",
  },
  aborted: {
    color: "bg-warning",
    pulse: false,
    text: "Aborted",
  },
  failed: {
    color: "bg-error",
    pulse: false,
    text: "Failed",
  },
};

export function StatusDot({ status, label = false }: StatusDotProps) {
  const cfg = (statusConfig[status] as typeof statusConfig[keyof typeof statusConfig] | undefined) || {
    color: "bg-gray-500",
    pulse: false,
    text: String(status || "Unknown"),
  };

  return (
    <span className="inline-flex items-center gap-2">
      <span className="relative flex h-2.5 w-2.5">
        {cfg.pulse && (
          <span
            className={`absolute inset-0 animate-ping rounded-full ${cfg.color} opacity-60`}
          />
        )}
        <span
          className={`relative inline-flex h-2.5 w-2.5 rounded-full ${cfg.color}`}
        />
      </span>
      {label && (
        <span className="text-xs font-medium text-text-secondary">
          {cfg.text}
        </span>
      )}
    </span>
  );
}
