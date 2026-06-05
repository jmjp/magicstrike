import { BrowserRouter, Routes, Route } from "react-router-dom";
import { AuthProvider } from "@/contexts/AuthContext";
import { AppShell } from "@/components/layout/AppShell";
import { ProtectedRoute } from "@/components/layout/ProtectedRoute";
import { Login } from "@/pages/Login";
import { Callback } from "@/pages/Callback";
import { Dashboard } from "@/pages/Dashboard";
import { MatchDetail } from "@/pages/MatchDetail";
import { UploadPage } from "@/pages/Upload";
import { ChatList } from "@/pages/ChatList";
import { ChatRoom } from "@/pages/ChatRoom";

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          {/* Public */}
          <Route path="/login" element={<Login />} />
          <Route path="/auth/callback" element={<Callback />} />

          {/* Protected */}
          <Route element={<ProtectedRoute />}>
            <Route element={<AppShell />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/matches/:id" element={<MatchDetail />} />
              <Route path="/upload" element={<UploadPage />} />
              <Route path="/chat" element={<ChatList />} />
              <Route path="/chat/:id" element={<ChatRoom />} />
            </Route>
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
