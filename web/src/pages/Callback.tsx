import { useEffect, useState, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { Spinner } from "@/components/ui/Spinner";
import { Button } from "@/components/ui/Button";
import { AlertTriangle } from "lucide-react";

export function Callback() {
  const [searchParams] = useSearchParams();
  const { handleCallback } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState(false);
  const calledRef = useRef(false);

  useEffect(() => {
    const token = searchParams.get("token");

    console.log(token);

    if (!token) {
      setError(true);
      return;
    }

    if (calledRef.current) return;
    calledRef.current = true;

    handleCallback(token)
      .then(() => navigate("/", { replace: true }))
      .catch(() => setError(true));
  }, [searchParams, handleCallback, navigate]);

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4">
        <AlertTriangle size={48} className="text-warning" />
        <h1 className="text-xl font-semibold text-text-primary">
          Authentication failed
        </h1>
        <p className="text-sm text-text-secondary">
          The magic link is invalid or has expired.
        </p>
        <Button variant="primary" onClick={() => navigate("/login")}>
          Back to Login
        </Button>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4">
      <Spinner size="lg" />
      <p className="text-sm text-text-secondary">Verifying your login...</p>
    </div>
  );
}
