import { useState, type FormEvent } from "react";
import { Mail, ArrowRight, Sparkles, CheckCircle2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { Button, Input, Card } from "@/components/ui";
import * as v from "valibot";

const emailSchema = v.pipe(
  v.string("Email is required"),
  v.email("Invalid email address"),
  v.maxLength(254, "Email must be 254 characters or less"),
);

type Step = "idle" | "loading" | "success";

export function Login() {
  const { requestMagicLink } = useAuth();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [step, setStep] = useState<Step>("idle");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");

    const result = v.safeParse(emailSchema, email);
    if (!result.success) {
      setError(result.issues[0].message);
      return;
    }

    setStep("loading");
    try {
      await requestMagicLink(email);
      setStep("success");
    } catch {
      setError("Network error. Please check your connection and try again.");
      setStep("idle");
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="mb-10 text-center">
          <div className="mb-4 inline-flex items-center justify-center rounded-2xl bg-bg-secondary p-4 ring-1 ring-border">
            <Sparkles size={32} className="text-accent" />
          </div>
          <h1 className="text-2xl font-bold text-accent">MagicStrike</h1>
          <p className="mt-2 text-sm text-text-secondary">
            CS2 Tactical Analysis Platform
          </p>
        </div>

        <Card padding="lg">
          {step === "success" ? (
            /* ─── Success State ─── */
            <div className="flex flex-col items-center py-6 text-center">
              <CheckCircle2 size={48} className="mb-4 text-success" />
              <h2 className="text-lg font-semibold text-accent">
                Check your email
              </h2>
              <p className="mt-2 text-sm text-text-secondary">
                If an account exists for{" "}
                <span className="font-medium text-text-primary">{email}</span>,
                we&apos;ve sent a magic link.
              </p>
              <p className="mt-4 text-xs text-text-dim">
                In development, check the API logs for the token.
              </p>
            </div>
          ) : (
            /* ─── Form ─── */
            <>
              <h2 className="mb-1 text-lg font-semibold text-text-primary">
                Sign in
              </h2>
              <p className="mb-6 text-sm text-text-secondary">
                Enter your email to receive a magic link.
              </p>

              <form onSubmit={handleSubmit} className="flex flex-col gap-4">
                <Input
                  label="Email"
                  type="email"
                  placeholder="player@example.com"
                  icon={<Mail size={16} />}
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value);
                    if (error) setError("");
                  }}
                  error={error}
                  autoFocus
                  autoComplete="email"
                />

                <Button
                  type="submit"
                  variant="primary"
                  size="lg"
                  loading={step === "loading"}
                  icon={step !== "loading" ? <ArrowRight size={18} /> : undefined}
                  className="w-full"
                >
                  Send Magic Link
                </Button>
              </form>

              <p className="mt-4 text-center text-xs text-text-dim">
                No password needed — we&apos;ll email you a secure login link.
              </p>
            </>
          )}
        </Card>
      </div>
    </div>
  );
}
