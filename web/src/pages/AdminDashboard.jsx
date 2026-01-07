import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
    Globe,
    Mail,
    Cpu,
    MemoryStick,
    Server,
    Activity,
    Shield,
    Settings,
    FileText,
    Users,
    Building,
    Lock,
    Sparkles,
    ArrowRight,
    RefreshCw
} from "lucide-react";
import { getDashboardStats, getAIInsights } from "../api";
import { cn } from "../lib/utils";

export default function AdminDashboard() {
    const [stats, setStats] = useState(null);
    const [insight, setInsight] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");

    useEffect(() => {
        loadStats();
    }, []);

    const loadStats = async () => {
        try {
            const s = await getDashboardStats();
            setStats(s);
        } catch (err) {
            setError("Failed to load stats");
        }
    };

    const getAI = async () => {
        setLoading(true);
        setInsight("");
        try {
            const res = await getAIInsights();
            setInsight(res.analysis || res.insight);
        } catch (err) {
            setInsight("Error: " + err.message);
        } finally {
            setLoading(false);
        }
    };

    if (!stats) {
        return (
            <div className="flex items-center justify-center h-64">
                <RefreshCw className="w-8 h-8 animate-spin text-primary" />
            </div>
        );
    }

    const quickLinks = [
        { path: '/services', icon: Server, label: 'Services', desc: 'Install & manage KumoMTA, Dovecot' },
        { path: '/admin/tenants', icon: Building, label: 'Organizations', desc: 'Create & manage tenants' },
        { path: '/logs', icon: FileText, label: 'System Logs', desc: 'View service logs' },
        { path: '/security', icon: Lock, label: 'Security', desc: 'Fail2Ban & firewall' },
        { path: '/ssl', icon: Shield, label: 'SSL/HTTPS', desc: 'Manage certificates' },
        { path: '/settings', icon: Settings, label: 'Settings', desc: 'System configuration' },
    ];

    return (
        <div className="space-y-8 animate-in fade-in duration-500">
            {/* Header */}
            <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight text-foreground">Admin Dashboard</h1>
                    <p className="text-muted-foreground">System-wide overview and management</p>
                </div>
                <button
                    onClick={getAI}
                    disabled={loading}
                    className="bg-primary hover:bg-primary/90 text-primary-foreground px-4 py-2 rounded-md text-sm font-medium flex items-center gap-2 transition-colors shadow-sm"
                >
                    {loading ? <Sparkles className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                    {loading ? "Analyzing..." : "AI Log Analysis"}
                </button>
            </div>

            {/* AI Insight */}
            {insight && (
                <div className="bg-card border border-primary/20 p-6 rounded-xl shadow-sm relative overflow-hidden">
                    <div className="absolute top-0 left-0 w-1 h-full bg-primary" />
                    <h3 className="font-semibold text-primary mb-2 flex items-center gap-2">
                        <Sparkles className="w-4 h-4" /> AI Insight
                    </h3>
                    <div className="text-sm text-foreground/80 whitespace-pre-wrap leading-relaxed">
                        {insight}
                    </div>
                </div>
            )}

            {/* System Stats */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <StatCard label="Total Domains" value={stats.domains} icon={Globe} color="text-blue-500" />
                <StatCard label="Active Senders" value={stats.senders} icon={Mail} color="text-emerald-500" />
                <StatCard label="CPU Load" value={stats.cpu_load} icon={Cpu} color="text-orange-500" />
                <StatCard label="RAM Usage" value={stats.ram_usage} icon={MemoryStick} color="text-purple-500" />
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Service Status */}
                <div className="bg-card border rounded-xl p-6 shadow-sm">
                    <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                        <Activity className="w-5 h-5 text-muted-foreground" />
                        Service Status
                    </h3>
                    <div className="space-y-3">
                        <ServiceRow name="MTA Service (KumoMTA)" status={stats.kumo_status} />
                        <ServiceRow name="Dovecot (IMAP/POP3)" status={stats.dovecot_status} />
                        <ServiceRow name="Fail2Ban" status={stats.f2b_status} />
                    </div>
                    <Link
                        to="/services"
                        className="mt-4 inline-flex items-center gap-2 text-sm text-primary hover:underline"
                    >
                        Manage Services <ArrowRight className="w-4 h-4" />
                    </Link>
                </div>

                {/* Open Ports */}
                <div className="bg-card border rounded-xl p-6 shadow-sm">
                    <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                        <Server className="w-5 h-5 text-muted-foreground" />
                        Open Ports
                    </h3>
                    <div className="flex flex-wrap gap-2">
                        {stats.open_ports ? (
                            stats.open_ports.split(", ").map(port => (
                                <span key={port} className="bg-secondary text-secondary-foreground px-3 py-1.5 rounded-md text-sm font-mono border">
                                    {port}
                                </span>
                            ))
                        ) : (
                            <span className="text-muted-foreground text-sm">Scanning...</span>
                        )}
                    </div>
                </div>
            </div>

            {/* Quick Actions */}
            <div className="bg-card border rounded-xl p-6 shadow-sm">
                <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                    <Settings className="w-5 h-5 text-muted-foreground" />
                    Quick Actions
                </h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                    {quickLinks.map((link) => {
                        const Icon = link.icon;
                        return (
                            <Link
                                key={link.path}
                                to={link.path}
                                className="flex items-start gap-3 p-4 rounded-lg border bg-background hover:bg-muted/50 transition-colors group"
                            >
                                <div className="p-2 rounded-lg bg-primary/10 text-primary">
                                    <Icon className="w-5 h-5" />
                                </div>
                                <div>
                                    <div className="font-medium text-foreground group-hover:text-primary transition-colors">
                                        {link.label}
                                    </div>
                                    <div className="text-xs text-muted-foreground">{link.desc}</div>
                                </div>
                            </Link>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}

function StatCard({ label, value, icon: Icon, color }) {
    return (
        <div className="bg-card border rounded-xl p-6 shadow-sm hover:shadow-md transition-shadow">
            <div className="flex justify-between items-start">
                <div>
                    <p className="text-sm font-medium text-muted-foreground">{label}</p>
                    <h4 className="text-2xl font-bold mt-2 text-foreground">{value}</h4>
                </div>
                <div className={cn("p-2 rounded-lg bg-secondary", color)}>
                    <Icon className="w-5 h-5" />
                </div>
            </div>
        </div>
    );
}

function ServiceRow({ name, status }) {
    const isActive = status === "active";
    return (
        <div className="flex items-center justify-between p-3 rounded-lg border bg-background/50">
            <span className="font-medium text-foreground">{name}</span>
            <div className="flex items-center gap-2">
                <span className={cn(
                    "text-xs font-medium px-2 py-1 rounded-full capitalize",
                    isActive ? "bg-green-500/10 text-green-600" : "bg-red-500/10 text-red-600"
                )}>
                    {status || "Unknown"}
                </span>
                <div className={cn("w-2 h-2 rounded-full", isActive ? "bg-green-500 animate-pulse" : "bg-red-500")} />
            </div>
        </div>
    );
}
