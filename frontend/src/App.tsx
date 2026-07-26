import { Navigate, Routes, Route, useLocation } from "react-router";
import SidebarLayout from "./layout/SidebarLayout";
import Overview from "./pages/Overview";
import Unbound from "./pages/Unbound";
import AdGuard from "./pages/AdGuard";
import Setup from "./pages/Setup";
import Stack from "./pages/Stack";
import Login from "./pages/Login";
import { useAuth } from "./auth";

function App() {
  const auth = useAuth();
  const location = useLocation();

  if (auth.loading) {
    return (
      <div className="auth-loading" aria-label="RootGuard">
        <span />
        <strong>RootGuard</strong>
      </div>
    );
  }

  if (!auth.authenticated) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="*" element={<Navigate replace to="/login" state={{ from: location.pathname }} />} />
      </Routes>
    );
  }

  if (location.pathname === "/login") {
    return <Navigate replace to="/dashboard" />;
  }

  return (
    <SidebarLayout>
      <Routes>
        <Route path="/" element={<Navigate replace to="/dashboard" />} />
        <Route path="/dashboard" element={<Overview />} />
        <Route path="/unbound" element={<Unbound />} />
        <Route path="/adguard" element={<AdGuard />} />
        <Route path="/setup" element={<Setup />} />
        <Route path="/stack" element={<Stack />} />
      </Routes>
    </SidebarLayout>
  );
}

export default App;
