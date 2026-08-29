import { lazy, Suspense } from "react";
import { Navigate, Routes, Route, useLocation } from "react-router";
import SidebarLayout from "./layout/SidebarLayout";
import Login from "./pages/Login";
import { useAuth } from "./auth";

// Lazy-loaded, unlike Login above: found in review, the production bundle
// was a single 578kB chunk (162kB gzipped) - every authenticated page's
// code shipped on first load regardless of which one (if any) a session
// actually visits. Login stays eager since it's the one page every
// unauthenticated visit needs immediately; splitting it too would just
// trade one round-trip for another on the most universal path. The
// Suspense boundary below only wraps the authenticated <Routes> for the
// same reason - the unauthenticated branch never touches these imports at
// all.
const Overview = lazy(() => import("./pages/Overview"));
const Unbound = lazy(() => import("./pages/Unbound"));
const AdGuard = lazy(() => import("./pages/AdGuard"));
const Setup = lazy(() => import("./pages/Setup"));
const Stack = lazy(() => import("./pages/Stack"));
const Backups = lazy(() => import("./pages/Backups"));
const Logs = lazy(() => import("./pages/Logs"));

function RouteLoading() {
  return (
    <div className="route-loading" aria-label="Loading">
      <span />
    </div>
  );
}

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
      <Suspense fallback={<RouteLoading />}>
        <Routes>
          <Route path="/" element={<Navigate replace to="/dashboard" />} />
          <Route path="/dashboard" element={<Overview />} />
          <Route path="/unbound" element={<Unbound />} />
          <Route path="/unbound/:section" element={<Unbound />} />
          <Route path="/adguard" element={<AdGuard />} />
          <Route path="/setup" element={<Setup />} />
          <Route path="/stack" element={<Stack />} />
          <Route path="/backups" element={<Backups />} />
          <Route path="/logs" element={<Logs />} />
        </Routes>
      </Suspense>
    </SidebarLayout>
  );
}

export default App;
