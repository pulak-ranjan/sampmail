import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, BarChart3, Globe, ShieldCheck, Key, MailWarning, Network,
  ListOrdered, Webhook, Settings, FileText, Lock, LogOut, Menu, X, ServerCog,
  Wrench, Thermometer, Mail, Users, Tag, PieChart, Send, Zap, List, Filter, Ban, Radio, ShieldAlert, Building, Server, CheckCircle
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

  const adminLinks = [
    { path: '/admin', icon: LayoutDashboard, label: 'Dashboard' },
    { path: '/admin/tenants', icon: Building, label: 'Organizations' },
    { path: '/admin/users', icon: Users, label: 'Users' },
    { path: '/services', icon: Server, label: 'Services' },
    { path: '/network', label: 'Network & IPs', isHeader: true },
    { path: '/sending-ips', icon: Radio, label: 'Sending IPs' },
    { path: '/ips', icon: Network, label: 'IP Inventory' },
    { path: '/proxies', icon: ServerCog, label: 'Proxies' },
    { path: '/warmup', icon: Thermometer, label: 'IP Warmup' },
    { path: '/system', label: 'System', isHeader: true },
    { path: '/queue', icon: ListOrdered, label: 'Queue' },
    { path: '/logs', icon: FileText, label: 'System Logs' },
    { path: '/security', icon: Lock, label: 'Security' },
    { path: '/ssl', icon: ShieldAlert, label: 'SSL / HTTPS' },
    { path: '/tools', icon: Wrench, label: 'System Tools' },
    { path: '/config', icon: ServerCog, label: 'Config Gen' },
    { path: '/settings', icon: Settings, label: 'Settings' },
  ];

  const userLinks = [
    { path: '/', icon: LayoutDashboard, label: 'Dashboard' },
    { path: '/marketing', label: 'Marketing', isHeader: true },
    { path: '/campaigns', icon: Send, label: 'Campaigns' },
    { path: '/automations', icon: Zap, label: 'Automations' },
    { path: '/templates', icon: Mail, label: 'Templates' },
    { path: '/audience', label: 'Audience', isHeader: true },
    { path: '/lists', icon: List, label: 'Lists' },
    { path: '/subscribers', icon: Users, label: 'Subscribers' },
    { path: '/tags', icon: Tag, label: 'Tags' },
    { path: '/segments', icon: Filter, label: 'Segments' },
    { path: '/suppressions', icon: Ban, label: 'Suppressions' },
    { path: '/verify', icon: CheckCircle, label: 'Email Verification' },
    { path: '/reports', label: 'Reports', isHeader: true },
    { path: '/stats', icon: BarChart3, label: 'Statistics' },
    { path: '/analytics', icon: PieChart, label: 'Analytics' },
    { path: '/configuration', label: 'Configuration', isHeader: true },
    { path: '/domains', icon: Globe, label: 'Domains' },
    { path: '/apikeys', icon: Key, label: 'API Keys' },
    { path: '/webhooks', icon: Webhook, label: 'Webhooks' },
    { path: '/dmarc', icon: ShieldCheck, label: 'DMARC' },
    { path: '/dkim', icon: Key, label: 'DKIM' },
    { path: '/bounce', icon: MailWarning, label: 'Bounce' },
  ];

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const NavItem = ({ link, onClick }) => {
    if (link.isHeader) {
      return (
        <div className="px-3 pt-4 pb-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
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
          "flex items-center gap-3 px-3 py-2.5 rounded-md text-sm font-medium transition-all duration-200",
          isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
        )}
      >
        <Icon className="w-4 h-4" />
        {link.label}
      </Link>
    );
  };

  return (
    <div className="min-h-screen bg-background flex flex-col md:flex-row">
      {/* Mobile Header */}
      <div className="md:hidden border-b bg-card flex items-center justify-between p-4 sticky top-0 z-30">
        <div className="font-bold text-lg flex items-center gap-2">
          <img src="/sampmail-logo.png" alt="SampMail" className="w-8 h-8 rounded-lg" />
          SampMail
        </div>
        <button onClick={() => setIsMobileOpen(!isMobileOpen)} className="p-2 -mr-2">
          {isMobileOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
        </button>
      </div>

      {/* Sidebar */}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-40 w-64 bg-card border-r transform transition-transform duration-300 ease-in-out md:translate-x-0 md:static md:h-screen flex flex-col",
        isMobileOpen ? "translate-x-0" : "-translate-x-full"
      )}>
        <div className="p-6 border-b flex items-center gap-3">
          <img src="/sampmail-logo.png" alt="SampMail" className="w-10 h-10 rounded-lg" />
          <div>
            <div className="font-bold text-foreground">SampMail</div>
            <div className="text-xs text-muted-foreground">Admin Panel</div>
          </div>
        </div>

        <div className="px-3 pt-4 mb-2">
          <OrganizationSwitcher />
        </div>

        <nav className="flex-1 overflow-y-auto p-4 space-y-1">
          {user?.is_super_admin && (
            <>
              <div className="px-3 pb-2 text-xs font-semibold text-primary uppercase tracking-wider">
                Admin Station
              </div>
              {adminLinks.map((link, i) => (
                <NavItem key={link.path || i} link={link} onClick={() => setIsMobileOpen(false)} />
              ))}
              <div className="my-4 border-t border-border" />
            </>
          )}

          {/* Show User Links only if Org is selected OR user is not superadmin (regular user) */}
          {(orgId || !user?.is_super_admin) && (
            <>
              {user?.is_super_admin && (
                <div className="px-3 pb-2 text-xs font-semibold text-primary uppercase tracking-wider">
                  Org Workspace
                </div>
              )}
              {userLinks.map((link, i) => (
                <NavItem key={link.path || i} link={link} onClick={() => setIsMobileOpen(false)} />
              ))}
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
