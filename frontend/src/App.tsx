import { lazy, useEffect } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AppShell, BootScreen, ErrorScreen } from "./components/layout/AppShell";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { useApp } from "./app/AppDataProvider";
import { PAGE_META } from "./components/layout/nav";
import { useI18n } from "./i18n";
import { LoginPage } from "./pages/Login";
import { NotFoundPage } from "./pages/NotFound";

// Route-level splitting keeps xterm, noVNC and the heavyweight pages out of
// the initial bundle. The shell and login stay eager for a fast first paint.
const DashboardPage = lazy(() => import("./pages/Dashboard").then((module) => ({ default: module.DashboardPage })));
const StoragePage = lazy(() => import("./pages/Storage").then((module) => ({ default: module.StoragePage })));
const DockerPage = lazy(() => import("./pages/Docker").then((module) => ({ default: module.DockerPage })));
const AppsPage = lazy(() => import("./pages/Apps").then((module) => ({ default: module.AppsPage })));
const SchedulerPage = lazy(() => import("./pages/Scheduler").then((module) => ({ default: module.SchedulerPage })));
const VMsPage = lazy(() => import("./pages/VMs").then((module) => ({ default: module.VMsPage })));
const VMConsolePage = lazy(() => import("./pages/VMConsole").then((module) => ({ default: module.VMConsolePage })));
const NetworkPage = lazy(() => import("./pages/Network").then((module) => ({ default: module.NetworkPage })));
const GPUsPage = lazy(() => import("./pages/GPUs").then((module) => ({ default: module.GPUsPage })));
const UsersPage = lazy(() => import("./pages/Users").then((module) => ({ default: module.UsersPage })));
const NotificationsPage = lazy(() => import("./pages/Notifications").then((module) => ({ default: module.NotificationsPage })));
const PluginsPage = lazy(() => import("./pages/Plugins").then((module) => ({ default: module.PluginsPage })));
const LogsHubPage = lazy(() => import("./pages/LogsHub").then((module) => ({ default: module.LogsHubPage })));
const TerminalPage = lazy(() => import("./pages/Terminal").then((module) => ({ default: module.TerminalPage })));
const ToolsPage = lazy(() => import("./pages/Tools").then((module) => ({ default: module.ToolsPage })));
const TasksPage = lazy(() => import("./pages/Tasks").then((module) => ({ default: module.TasksPage })));
const UserScriptsPage = lazy(() => import("./pages/UserScripts").then((module) => ({ default: module.UserScriptsPage })));
const BalancePage = lazy(() => import("./pages/Balance").then((module) => ({ default: module.BalancePage })));
const SettingsPage = lazy(() => import("./pages/Settings").then((module) => ({ default: module.SettingsPage })));
const AboutPage = lazy(() => import("./pages/About").then((module) => ({ default: module.AboutPage })));

function ProtectedShell() {
  const { session, error, refresh } = useApp();
  if (!session) {
    if (error) return <ErrorScreen error={error} onRetry={refresh} />;
    return <BootScreen message="Restoring your session" />;
  }
  if (!session.authenticated) return <Navigate to="/login" replace />;
  return <AppShell />;
}

function TitleSync() {
  const location = useLocation();
  const { t } = useI18n();
  useEffect(() => {
    const meta = PAGE_META[location.pathname];
    document.title = meta ? `${t(meta.title)} · Aurora` : "Aurora · Unraid";
  }, [location.pathname, t]);
  return null;
}

export default function App() {
  return (
    <>
      <TitleSync />
      <ErrorBoundary>
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        <Route element={<ProtectedShell />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/storage" element={<StoragePage />} />
          <Route path="/zfs" element={<Navigate to="/storage?tab=zfs" replace />} />
          <Route path="/docker" element={<DockerPage />} />
          <Route path="/apps" element={<AppsPage />} />
          <Route path="/scheduler" element={<SchedulerPage />} />
          <Route path="/docker/terminal/:id" element={<TerminalPage />} />
          <Route path="/vms" element={<VMsPage />} />
          <Route path="/vms/console/:id" element={<VMConsolePage />} />
          <Route path="/network" element={<NetworkPage />} />
          <Route path="/gpus" element={<GPUsPage />} />
          <Route path="/users" element={<UsersPage />} />
          <Route path="/notifications" element={<NotificationsPage />} />
          <Route path="/plugins" element={<PluginsPage />} />
          <Route path="/logs" element={<LogsHubPage />} />
          <Route path="/terminal" element={<TerminalPage />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/tasks" element={<TasksPage />} />
          <Route path="/history" element={<Navigate to="/logs?tab=history" replace />} />
          <Route path="/audit" element={<Navigate to="/logs?tab=audit" replace />} />
          <Route path="/scripts" element={<UserScriptsPage />} />
          <Route path="/balance" element={<BalancePage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/ups" element={<Navigate to="/settings" replace />} />
          <Route path="/about" element={<AboutPage />} />
        </Route>

        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      </ErrorBoundary>
    </>
  );
}

