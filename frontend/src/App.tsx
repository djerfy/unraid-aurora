import { useEffect } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AppShell, BootScreen, ErrorScreen } from "./components/layout/AppShell";
import { useApp } from "./app/AppDataProvider";
import { PAGE_META } from "./components/layout/nav";
import { useI18n } from "./i18n";
import { DashboardPage } from "./pages/Dashboard";
import { StoragePage } from "./pages/Storage";
import { DockerPage } from "./pages/Docker";
import { VMsPage } from "./pages/VMs";
import { VMConsolePage } from "./pages/VMConsole";
import { NetworkPage } from "./pages/Network";
import { GPUsPage } from "./pages/GPUs";
import { UsersPage } from "./pages/Users";
import { NotificationsPage } from "./pages/Notifications";
import { PluginsPage } from "./pages/Plugins";
import { LogsPage } from "./pages/Logs";
import { TerminalPage } from "./pages/Terminal";
import { ToolsPage } from "./pages/Tools";
import { HistoryPage } from "./pages/History";
import { AuditPage } from "./pages/Audit";
import { UserScriptsPage } from "./pages/UserScripts";
import { ZFSPage } from "./pages/ZFS";
import { UPSPage } from "./pages/UPS";
import { BalancePage } from "./pages/Balance";
import { SettingsPage } from "./pages/Settings";
import { AboutPage } from "./pages/About";
import { LoginPage } from "./pages/Login";
import { NotFoundPage } from "./pages/NotFound";

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
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        <Route element={<ProtectedShell />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/storage" element={<StoragePage />} />
          <Route path="/docker" element={<DockerPage />} />
          <Route path="/docker/terminal/:id" element={<TerminalPage />} />
          <Route path="/vms" element={<VMsPage />} />
          <Route path="/vms/console/:id" element={<VMConsolePage />} />
          <Route path="/network" element={<NetworkPage />} />
          <Route path="/gpus" element={<GPUsPage />} />
          <Route path="/users" element={<UsersPage />} />
          <Route path="/notifications" element={<NotificationsPage />} />
          <Route path="/plugins" element={<PluginsPage />} />
          <Route path="/logs" element={<LogsPage />} />
          <Route path="/terminal" element={<TerminalPage />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/scripts" element={<UserScriptsPage />} />
          <Route path="/zfs" element={<ZFSPage />} />
          <Route path="/ups" element={<UPSPage />} />
          <Route path="/balance" element={<BalancePage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/about" element={<AboutPage />} />
        </Route>

        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  );
}

