import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, BarChart3, Globe, ShieldCheck, Key, MailWarning, Network,
  ListOrdered, Webhook, Settings, FileText, Lock, LogOut, Menu, X, ServerCog,
  Wrench, Thermometer, Mail, Users, Tag, PieChart, Send, Zap, List, Filter, Ban, Radio, ShieldAlert, Building, Server, CheckCircle, Brain, Wallet, Plus
} from 'lucide-react';
import { ThemeToggle } from './ThemeProvider';
import { useAuth } from '../AuthContext';
import { cn } from '../lib/utils';
import AIAssistant from './AIAssistant';

import OrganizationSwitcher from './OrganizationSwitcher';

export default function Layout({ children }) {
  const [isMobileOpen, setIsMobileOpen] = useState(false);
  const { logout, user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  const orgId = localStorage.getItem('sampmail_org_id');
  const isAdmin = user?.is_super_admin;

  // ADMIN SECTION - Only visible to super admins (no org selected)
  const adminLinks = [
    { path: '/admin', icon: LayoutDashboard, label: 'Admin Dashboard' },
    { path: '/network', label: 'Network', isHeader: true },
    { path: '/sending-ips', icon: Radio, label: 'Sending IPs' },
    { path: '/ips', icon: Network, label: 'IP Inventory' },
    { path: '/warmup', icon: Thermometer, label: 'IP Warmup' },
    { path: '/system', label: 'System', isHeader: true },
    { path: '/queue', icon: ListOrdered, label: 'Queue' },
    { path: '/logs', icon: FileText, label: 'Logs' },
    { path: '/services', icon: Server, label: 'Services' },
    { path: '/security', icon: Lock, label: 'Security' },
    { path: '/config', icon: ServerCog, label: 'Config Gen' },
    { path: '/admin/tenants', icon: Building, label: 'Organizations' },
    { path: '/admin/users', icon: Users, label: 'Users' },
  ];

  // USER SECTION - Main navigation for regular users and org admins
  const userLinks = [
    { path: '/', icon: LayoutDashboard, label: 'Dashboard' },
    { path: '/main', label: 'Main', isHeader: true },
    { path: '/campaigns', icon: Send, label: 'Campaigns', action: 'new', actionPath: '/campaigns?new=true' },
    { path: '/lists', icon: List, label: 'Lists' },
    { path: '/subscribers', icon: Users, label: 'Subscribers' },
    { path: '/templates', icon: Mail, label: 'Templates' },
    { path: '/tools', label: 'Tools', isHeader: true },
    { path: '/verify', icon: CheckCircle, label: 'Verify Emails' },
    { path: '/automations', icon: Zap, label: 'Automations' },
    { path: '/reports', label: 'Reports', isHeader: true },
    { path: '/stats', icon: BarChart3, label: 'Statistics' },
    { path: '/analytics', icon: PieChart, label: 'Analytics' },
    { path: '/settings', label: 'Settings', isHeader: true },
    { path: '/settings', icon: Settings, label: 'Personal' },
  ];

  // ORG CONFIG - Only for org admins (when org is selected)
  const orgConfigLinks = [
    { path: '/org-config', label: 'Organization', isHeader: true },
    { path: '/domains', icon: Globe, label: 'Domains' },
    { path: '/senders', icon: Send, label: 'Senders' },
    { path: '/apikeys', icon: Key, label: 'API Keys' },
    { path: '/webhooks', icon: Webhook, label: 'Webhooks' },
  ];

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const NavItem = ({ link, onClick }) => {
    if (link.isHeader) {
      return (
        <div className="px-3 pt-4 pb-2 text-xs font-semibold text-muted-foreground/60 uppercase tracking-wider">
          {link.label}
        </div>
      );
    }

    const isActive = location.pathname === link.path;
    const Icon = link.icon;
    return (
      <Link
        to={link.path}
        onClick={onClick}
        className={cn(
          "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200",
          isActive
            ? "bg-gradient-to-r from-cyan-500/20 to-purple-500/20 text-cyan-400 border border-cyan-500/30 shadow-lg shadow-cyan-500/10"
            : "text-muted-foreground/80 hover:bg-white/5 hover:text-foreground border border-transparent hover:border-white/10"
        )}
      >
        <Icon className={cn("w-4 h-4", isActive && "text-cyan-400")} />
        {link.label}
        {isActive && (
          <div className="ml-auto w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-[0_0_8px_rgba(34,211,238,0.8)]" />
        )}
      </Link>
    );
  };

  return (
    <div className="min-h-screen bg-background flex flex-col md:flex-row">
      {/* Mobile Header - Glass Effect */}
      <div className="md:hidden border-b bg-card/80 backdrop-blur-xl flex items-center justify-between p-4 sticky top-0 z-30">
        <div className="font-bold text-lg flex items-center gap-2">
          <div className="relative">
            <img src="/sampmail-logo.png" alt="SampMail" className="w-8 h-8 rounded-lg" />
            <div className="absolute inset-0 rounded-lg bg-cyan-400/20 blur-lg" />
          </div>
          <span className="bg-gradient-to-r from-cyan-400 to-purple-500 bg-clip-text text-transparent font-bold">SampMail</span>
        </div>
        <button onClick={() => setIsMobileOpen(!isMobileOpen)} className="p-2 -mr-2">
          {isMobileOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
        </button>
      </div>

      {/* Sidebar - Glassmorphism */}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-40 w-64 transform transition-all duration-300 ease-in-out md:translate-x-0 md:static md:h-screen flex flex-col",
        "bg-gray-900/80 dark:bg-gray-900/80 backdrop-blur-xl border-r border-white/10",
        isMobileOpen ? "translate-x-0" : "-translate-x-full"
      )}>
        {/* Gradient top accent */}
        <div className="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-cyan-400 via-purple-500 to-fuchsia-500" />

        <div className="p-6 pt-7 border-b border-white/5 flex items-center gap-3">
          <div className="relative">
            <img src="/sampmail-logo.png" alt="SampMail" className="w-10 h-10 rounded-lg" />
            <div className="absolute inset-0 rounded-lg bg-cyan-400/30 blur-md" />
          </div>
          <div>
            <div className="font-bold text-foreground bg-gradient-to-r from-cyan-400 to-purple-400 bg-clip-text text-transparent">SampMail</div>
            <div className="text-xs text-muted-foreground/70">
              {isAdmin && !orgId ? 'Admin Panel' : orgId ? 'Workspace' : 'Select Organization'}
            </div>
          </div>
        </div>

        <div className="px-3 pt-4 mb-2">
          <OrganizationSwitcher />
        </div>

        <nav className="flex-1 overflow-y-auto p-4 space-y-1">
          {/* ADMIN SECTION - Show when no org selected and user is admin */}
          {isAdmin && !orgId && (
            <>
              <div className="px-3 pb-2 text-xs font-semibold text-destructive uppercase tracking-wider flex items-center gap-2">
                <Server className="w-3 h-3" /> System Control
              </div>
              {adminLinks.map((link, i) => (
                <NavItem key={link.path || i} link={link} onClick={() => setIsMobileOpen(false)} />
              ))}
              <div className="my-4 border-t border-border" />
            </>
          )}

          {/* USER SECTION - Always show when org is selected */}
          {(orgId || !isAdmin) && (
            <>
              <div className="px-3 pb-2 text-xs font-semibold text-primary uppercase tracking-wider">
                Main
              </div>
              {userLinks.map((link, i) => (
                <NavItem key={link.path || i} link={link} onClick={() => setIsMobileOpen(false)} />
              ))}

              {/* ORG CONFIG - Show for org admins */}
              {orgId && (
                <>
                  <div className="my-4 border-t border-border" />
                  <div className="px-3 pb-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    Organization
                  </div>
                  {orgConfigLinks.map((link, i) => (
                    <NavItem key={link.path || i} link={link} onClick={() => setIsMobileOpen(false)} />
                  ))}
                </>
              )}
            </>
          )}

          {/* Show admin link to switch to admin mode */}
          {isAdmin && orgId && (
            <>
              <div className="my-4 border-t border-border" />
              <Link
                to="/admin"
                onClick={() => setIsMobileOpen(false)}
                className="flex items-center gap-3 px-3 py-2.5 rounded-md text-sm font-medium text-muted-foreground hover:bg-accent hover:text-accent-foreground border border-dashed"
              >
                <ServerCog className="w-4 h-4 text-destructive" />
                Switch to Admin
              </Link>
            </>
          )}
        </nav>

        <div className="p-4 border-t space-y-4">
          <div className="flex items-center justify-between px-2">
            <span className="text-xs font-medium text-muted-foreground">Theme</span>
            <ThemeToggle />
          </div>
          <button onClick={handleLogout} className="w-full flex items-center gap-3 px-3 py-2.5 rounded-md text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors">
            <LogOut className="w-4 h-4" /> Logout
          </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto h-[calc(100vh-65px)] md:h-screen bg-muted/20 relative">
        <div className="p-4 md:p-8 max-w-7xl mx-auto">{children}</div>

        {/* The Agent is mounted here */}
        <AIAssistant />
      </main>

      {/* Mobile Overlay */}
      {isMobileOpen && <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-30 md:hidden" onClick={() => setIsMobileOpen(false)} />}
    </div>
  );
}
