import { useState, useRef, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  Upload as UploadIcon,
  FileUp,
  CheckCircle2,
  AlertTriangle,
} from "lucide-react";
import { requestUpload, uploadToS3, confirmUpload } from "@/api/demos";
import { Button, Card } from "@/components/ui";

type Step = "select" | "uploading" | "confirming" | "done";

const STEPS = [
  { key: "select", label: "Select Demo" },
  { key: "uploading", label: "Upload" },
  { key: "confirming", label: "Confirm" },
  { key: "done", label: "Done" },
] as const;

export function UploadPage() {
  const navigate = useNavigate();
  const fileRef = useRef<HTMLInputElement>(null);

  const [step, setStep] = useState<Step>("select");
  const [file, setFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [progress, setProgress] = useState(0);
  const [resultMatchId, setResultMatchId] = useState<string | null>(null);
  const [globalError, setGlobalError] = useState("");

  const currentStepIdx = STEPS.findIndex((s) => s.key === step);

  const handleFile = (selectedFile: File) => {
    if (!selectedFile.name.endsWith(".dem")) {
      setGlobalError("Only .dem replay files are supported.");
      setFile(null);
      return;
    }
    setFile(selectedFile);
    setGlobalError("");
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (f) {
      handleFile(f);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const f = e.dataTransfer.files?.[0];
    if (f) {
      handleFile(f);
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError("");

    if (!file) {
      setGlobalError("Please select a .dem file to upload.");
      return;
    }

    setStep("uploading");
    try {
      // 1) Request pre-signed URL (omit optional parameters, backend will auto-extract from file)
      const uploadData = await requestUpload({
        filename: file.name,
        team_a: "",
        team_b: "",
        map_name: "",
      });

      // 2) Upload to S3
      await uploadToS3(uploadData.upload_url, file, (pct) =>
        setProgress(pct),
      );

      // 3) Confirm upload
      setStep("confirming");
      const confirmData = await confirmUpload(
        uploadData.match_id,
        uploadData.bucket_key,
      );

      setResultMatchId(confirmData.match_id);
      setStep("done");
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Upload failed. Please try again.";
      setGlobalError(message);
      setStep("select");
    }
  };

  /* ─── Step Indicator ─── */

  const stepIndicator = (
    <div className="mb-8 flex items-center justify-center gap-3">
      {STEPS.map((s, i) => (
        <div key={s.key} className="flex items-center gap-3">
          <div
            className={`
              flex h-8 w-8 items-center justify-center rounded-full text-xs font-bold
              transition-colors duration-300
              ${i < currentStepIdx ? "bg-success text-bg-primary" : ""}
              ${i === currentStepIdx ? "bg-accent text-bg-primary" : ""}
              ${i > currentStepIdx ? "bg-bg-tertiary text-text-dim" : ""}
            `}
          >
            {i < currentStepIdx ? (
              <CheckCircle2 size={14} />
            ) : (
              i + 1
            )}
          </div>
          <span
            className={`text-sm font-medium hidden sm:inline ${
              i <= currentStepIdx ? "text-text-primary" : "text-text-dim"
            }`}
          >
            {s.label}
          </span>
          {i < STEPS.length - 1 && (
            <div
              className={`h-px w-8 transition-colors duration-300 ${
                i < currentStepIdx ? "bg-success" : "bg-border"
              }`}
            />
          )}
        </div>
      ))}
    </div>
  );

  /* ─── Done State ─── */

  if (step === "done") {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-bold text-accent">Upload Demo</h1>
        {stepIndicator}
        <Card padding="lg" className="text-center">
          <CheckCircle2 size={56} className="mx-auto mb-4 text-success" />
          <h2 className="text-xl font-bold text-accent">Upload Complete!</h2>
          <p className="mt-2 text-sm text-text-secondary">
            Your demo has been queued for processing. The tactical analysis will
            be ready shortly.
          </p>
          {resultMatchId && (
            <p className="mt-3 font-mono text-xs text-text-dim">
              Match ID: {resultMatchId}
            </p>
          )}
          <div className="mt-6 flex justify-center gap-3">
            {resultMatchId && (
              <Button
                variant="primary"
                onClick={() => navigate(`/matches/${resultMatchId}`)}
              >
                View Match
              </Button>
            )}
            <Button
              variant="secondary"
              onClick={() => {
                setStep("select");
                setFile(null);
                setProgress(0);
                setResultMatchId(null);
                if (fileRef.current) fileRef.current.value = "";
              }}
            >
              Upload Another
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  /* ─── Uploading State ─── */

  if (step === "uploading" || step === "confirming") {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-bold text-accent">Upload Demo</h1>
        {stepIndicator}
        <Card padding="lg" className="text-center">
          <div className="mb-4">
            {step === "uploading" ? (
              <UploadIcon size={48} className="mx-auto animate-bounce text-accent" />
            ) : (
              <CheckCircle2 size={48} className="mx-auto text-info" />
            )}
          </div>
          <h2 className="text-lg font-semibold text-text-primary">
            {step === "uploading" ? "Uploading demo..." : "Confirming upload..."}
          </h2>
          {step === "uploading" && (
            <>
              <div className="mt-4 h-2 w-full overflow-hidden rounded-full bg-bg-tertiary">
                <div
                  className="h-full rounded-full bg-accent transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
              <p className="mt-2 font-mono text-sm text-text-secondary">
                {progress}%
              </p>
            </>
          )}
          {step === "confirming" && (
            <p className="mt-2 text-sm text-text-secondary">
              Verifying file integrity and enqueuing for processing...
            </p>
          )}
        </Card>
      </div>
    );
  }

  /* ─── File Selection ─── */

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-accent">Upload Demo</h1>
      {stepIndicator}

      <Card padding="lg" className="max-w-xl mx-auto">
        <form onSubmit={handleSubmit} className="flex flex-col gap-6">
          <div className="text-center">
            <h2 className="text-lg font-semibold text-text-primary">Upload CS2 Demo</h2>
            <p className="mt-1.5 text-sm text-text-secondary">
              Select a <strong>.dem</strong> file. The map name, team names, round count, and match scores will be automatically parsed and extracted by the worker.
            </p>
          </div>

          {/* File Picker with Drag and Drop */}
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-text-secondary">
              Demo File
            </label>
            <div
              onClick={() => fileRef.current?.click()}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              className={`
                flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed
                px-6 py-10 text-center transition-all duration-200 cursor-pointer
                ${isDragging ? "border-accent bg-accent/5 scale-[1.01]" : ""}
                ${file ? "border-success/50 bg-success/5" : "border-border hover:border-border-accent hover:bg-bg-tertiary"}
              `}
            >
              <input
                ref={fileRef}
                type="file"
                accept=".dem"
                className="hidden"
                onChange={handleFileChange}
              />
              <div className={`p-3 rounded-full ${file ? "bg-success/15" : "bg-bg-tertiary"}`}>
                <FileUp
                  size={32}
                  className={file ? "text-success animate-pulse" : "text-text-dim"}
                />
              </div>
              <div>
                <p className="text-sm font-semibold text-text-primary">
                  {file ? file.name : "Drag & drop your demo here, or click to browse"}
                </p>
                <p className="mt-1 text-xs text-text-dim">
                  {file ? `${(file.size / (1024 * 1024)).toFixed(1)} MB` : "Supports only CS2 .dem replay files"}
                </p>
              </div>
            </div>
          </div>

          {globalError && (
            <div className="flex items-center gap-2 rounded-lg bg-red-500/10 border border-red-500/20 px-4 py-2.5 text-sm text-error">
              <AlertTriangle size={16} />
              {globalError}
            </div>
          )}

          <Button type="submit" variant="primary" size="lg" className="w-full" disabled={!file}>
            Start Upload
          </Button>
        </form>
      </Card>
    </div>
  );
}
