import { NavLink, useNavigate } from "react-router-dom";
import {
  LayoutDashboard,
  Upload,
  MessageSquare,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";

const navItems = [
  { to: "/", label: "Matches", icon: LayoutDashboard },
  { to: "/upload", label: "Upload", icon: Upload },
  { to: "/chat", label: "Chat", icon: MessageSquare },
];

export function Navbar() {
  const { state, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  return (
    <nav className="sticky top-0 z-40 border-b border-border bg-bg-primary/80 backdrop-blur-lg">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        {/* Logo */}
        <NavLink
          to="/"
          className="flex items-center gap-2.5 font-semibold text-accent"
        >
          <span className="hidden text-base sm:inline">Magic Strike</span>
        </NavLink>

        {/* Nav links */}
        <div className="flex items-center gap-1">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                `inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${isActive
                  ? "bg-bg-tertiary text-accent"
                  : "text-text-secondary hover:text-accent hover:bg-bg-tertiary"
                }`
              }
            >
              <Icon size={16} />
              <span className="hidden sm:inline">{label}</span>
            </NavLink>
          ))}
        </div>

        {/* User */}
        <div className="flex items-center gap-3">
          <div className="hidden items-center gap-2 sm:flex">
            <div className="flex h-7 w-7 items-center justify-center rounded-full bg-bg-tertiary text-xs font-semibold text-accent">
              {state.user?.username?.charAt(0).toUpperCase() ?? "?"}
            </div>
            <span className="text-sm text-text-secondary">
              {state.user?.username ?? "User"}
            </span>
          </div>
          <button
            onClick={handleLogout}
            className="rounded-lg p-2 text-text-dim hover:bg-bg-tertiary hover:text-error transition-colors"
            aria-label="Logout"
            title="Logout"
          >
            <LogOut size={16} />
          </button>
        </div>
      </div>
    </nav>
  );
}
