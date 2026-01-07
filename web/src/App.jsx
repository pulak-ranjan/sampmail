import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider } from './components/ThemeProvider';
import { AuthProvider, useAuth } from './AuthContext';
import Layout from './components/Layout';

// Pages
import LoginRegister from './pages/LoginRegister';
import Dashboard from './pages/Dashboard';
import StatsPage from './pages/StatsPage';
import Domains from './pages/Domains';
import DMARCPage from './pages/DMARCPage';
import DKIMPage from './pages/DKIMPage';
import BouncePage from './pages/BouncePage';
import IPsPage from './pages/IPsPage';
import QueuePage from './pages/QueuePage';
import WebhooksPage from './pages/WebhooksPage';
import ConfigPage from './pages/ConfigPage';
import LogsPage from './pages/LogsPage';
import SecurityPage from './pages/SecurityPage';
import Settings from './pages/Settings';
import ToolsPage from './pages/ToolsPage';
import WarmupPage from './pages/WarmupPage';
import APIKeysPage from './pages/APIKeysPage';
import TemplatesPage from './pages/TemplatesPage';
import SubscribersPage from './pages/SubscribersPage';
import ListsPage from './pages/ListsPage';
import CampaignsPage from './pages/CampaignsPage';
import AutomationsPage from './pages/AutomationsPage';
import AnalyticsDashboard from './pages/AnalyticsDashboard';
import TagsPage from './pages/TagsPage';
import SegmentsPage from './pages/SegmentsPage';
import SuppressionsPage from './pages/SuppressionsPage';
import ProxiesPage from './pages/ProxiesPage';
import SendingIPsPage from './pages/SendingIPsPage';
import SSLPage from './pages/SSLPage';
import ServiceManagerPage from './pages/ServiceManagerPage';
import AdminDashboard from './pages/AdminDashboard';
import UserDashboard from './pages/UserDashboard';

import TenantsPage from './pages/admin/TenantsPage';

function ProtectedRoute({ children }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="flex items-center justify-center min-h-screen bg-background text-muted-foreground">Loading...</div>;
  if (!user) return <Navigate to="/login" replace />;
  return <Layout>{children}</Layout>;
}

function SuperAdminRoute({ children }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="flex items-center justify-center min-h-screen bg-background text-muted-foreground">Loading...</div>;
  if (!user) return <Navigate to="/login" replace />;
  if (!user.is_super_admin) return <Navigate to="/" replace />;
  return <Layout>{children}</Layout>;
}

// Smart Dashboard: Shows AdminDashboard for super_admin, UserDashboard for regular users
function SmartDashboard() {
  const { user } = useAuth();
  const orgId = localStorage.getItem('sampmail_org_id');

  // Super admin without org selected -> Admin Dashboard
  if (user?.is_super_admin && !orgId) {
    return <AdminDashboard />;
  }

  // User with org selected -> User Dashboard
  return <UserDashboard />;
}

export default function App() {
  return (
    <AuthProvider>
      <ThemeProvider defaultTheme="dark" storageKey="sampmail-theme">
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginRegister />} />

            {/* Superadmin Routes */}
            <Route path="/admin/tenants" element={<SuperAdminRoute><TenantsPage /></SuperAdminRoute>} />
            <Route path="/admin" element={<SuperAdminRoute><AdminDashboard /></SuperAdminRoute>} />

            {/* App Routes */}
            <Route path="/" element={<ProtectedRoute><SmartDashboard /></ProtectedRoute>} />
            <Route path="/campaigns" element={<ProtectedRoute><CampaignsPage /></ProtectedRoute>} />
            <Route path="/automations" element={<ProtectedRoute><AutomationsPage /></ProtectedRoute>} />
            <Route path="/templates" element={<ProtectedRoute><TemplatesPage /></ProtectedRoute>} />
            <Route path="/lists" element={<ProtectedRoute><ListsPage /></ProtectedRoute>} />
            <Route path="/subscribers" element={<ProtectedRoute><SubscribersPage /></ProtectedRoute>} />
            <Route path="/tags" element={<ProtectedRoute><TagsPage /></ProtectedRoute>} />
            <Route path="/segments" element={<ProtectedRoute><SegmentsPage /></ProtectedRoute>} />
            <Route path="/suppressions" element={<ProtectedRoute><SuppressionsPage /></ProtectedRoute>} />
            <Route path="/tools" element={<ProtectedRoute><ToolsPage /></ProtectedRoute>} />
            <Route path="/stats" element={<ProtectedRoute><StatsPage /></ProtectedRoute>} />
            <Route path="/analytics" element={<ProtectedRoute><AnalyticsDashboard /></ProtectedRoute>} />
            <Route path="/domains" element={<ProtectedRoute><Domains /></ProtectedRoute>} />
            <Route path="/dmarc" element={<ProtectedRoute><DMARCPage /></ProtectedRoute>} />
            <Route path="/dkim" element={<ProtectedRoute><DKIMPage /></ProtectedRoute>} />
            <Route path="/bounce" element={<ProtectedRoute><BouncePage /></ProtectedRoute>} />
            <Route path="/ips" element={<ProtectedRoute><IPsPage /></ProtectedRoute>} />
            <Route path="/sending-ips" element={<ProtectedRoute><SendingIPsPage /></ProtectedRoute>} />
            <Route path="/proxies" element={<ProtectedRoute><ProxiesPage /></ProtectedRoute>} />
            <Route path="/queue" element={<ProtectedRoute><QueuePage /></ProtectedRoute>} />
            <Route path="/webhooks" element={<ProtectedRoute><WebhooksPage /></ProtectedRoute>} />
            <Route path="/warmup" element={<ProtectedRoute><WarmupPage /></ProtectedRoute>} />
            <Route path="/apikeys" element={<ProtectedRoute><APIKeysPage /></ProtectedRoute>} />
            <Route path="/config" element={<ProtectedRoute><ConfigPage /></ProtectedRoute>} />
            <Route path="/logs" element={<ProtectedRoute><LogsPage /></ProtectedRoute>} />
            <Route path="/security" element={<ProtectedRoute><SecurityPage /></ProtectedRoute>} />
            <Route path="/ssl" element={<ProtectedRoute><SSLPage /></ProtectedRoute>} />
            <Route path="/services" element={<ProtectedRoute><ServiceManagerPage /></ProtectedRoute>} />
            <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </ThemeProvider>
    </AuthProvider>
  );
}
