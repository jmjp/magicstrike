import { Outlet } from "react-router-dom";
import { Navbar } from "./Navbar";

export function AppShell() {
  return (
    <div className="flex min-h-screen flex-col">
      <Navbar />
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6">
        <Outlet />
      </main>
      <footer className="border-t border-border py-3 text-center">
        <p className="font-mono text-xs text-text-dim">
          Connected to API — MagicStrike v1
        </p>
      </footer>
    </div>
  );
}
