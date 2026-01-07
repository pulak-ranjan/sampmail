import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
    Send,
    Users,
    Mail,
    FileText,
    BarChart3,
    TrendingUp,
    Clock,
    Zap,
    List,
    ArrowRight,
    RefreshCw,
    Eye,
    MousePointer
} from "lucide-react";
import { cn } from "../lib/utils";

export default function UserDashboard() {
    const [stats, setStats] = useState(null);
    const [recentActivity, setRecentActivity] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        loadDashboardData();
    }, []);

    const loadDashboardData = async () => {
        try {
            const token = localStorage.getItem('sampmail_token');
            const orgId = localStorage.getItem('sampmail_org_id');

            if (!orgId) {
                setLoading(false);
                return;
            }

            // Fetch campaign stats
            const statsRes = await fetch('/api/v2/campaigns/stats', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'X-Organization-ID': orgId
                }
            });

            if (statsRes.ok) {
                const data = await statsRes.json();
                setStats(data);
            } else {
                // Default stats if API fails
                setStats({
                    total_campaigns: 0,
                    total_sent: 0,
                    total_opens: 0,
                    total_clicks: 0,
                    open_rate: 0,
                    click_rate: 0
                });
            }

            // Fetch recent activity
            const activityRes = await fetch('/api/v2/activity?limit=5', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'X-Organization-ID': orgId
                }
            });

            if (activityRes.ok) {
                const data = await activityRes.json();
                setRecentActivity(Array.isArray(data) ? data : []);
            }
        } catch (err) {
            console.error('Failed to load dashboard:', err);
            setStats({
                total_campaigns: 0,
                total_sent: 0,
                total_opens: 0,
                total_clicks: 0,
                open_rate: 0,
                click_rate: 0
            });
        } finally {
            setLoading(false);
        }
    };

    const orgId = localStorage.getItem('sampmail_org_id');

    if (!orgId) {
        return (
            <div className="flex items-center justify-center h-[60vh]">
                <div className="text-center max-w-md">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-primary/10 flex items-center justify-center">
                        <Users className="w-8 h-8 text-primary" />
                    </div>
                    <h2 className="text-2xl font-bold text-foreground mb-2">Select an Organization</h2>
                    <p className="text-muted-foreground mb-6">
                        Please select an organization from the dropdown in the sidebar to view your dashboard.
                    </p>
                    <div className="text-sm text-muted-foreground">
                        Don't have an organization? Contact your administrator.
                    </div>
                </div>
            </div>
        );
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <RefreshCw className="w-8 h-8 animate-spin text-primary" />
            </div>
        );
    }

    const quickLinks = [
        { path: '/campaigns', icon: Send, label: 'Campaigns', desc: 'Create & send email campaigns', color: 'text-blue-500' },
        { path: '/automations', icon: Zap, label: 'Automations', desc: 'Build automated workflows', color: 'text-yellow-500' },
        { path: '/templates', icon: FileText, label: 'Templates', desc: 'Design email templates', color: 'text-purple-500' },
        { path: '/lists', icon: List, label: 'Lists', desc: 'Manage subscriber lists', color: 'text-green-500' },
        { path: '/subscribers', icon: Users, label: 'Subscribers', desc: 'View all contacts', color: 'text-cyan-500' },
        { path: '/analytics', icon: BarChart3, label: 'Analytics', desc: 'Detailed performance stats', color: 'text-orange-500' },
    ];

    return (
        <div className="space-y-8 animate-in fade-in duration-500">
            {/* Header */}
            <div>
                <h1 className="text-3xl font-bold tracking-tight text-foreground">Dashboard</h1>
                <p className="text-muted-foreground">Welcome back! Here's your email marketing overview.</p>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <StatCard
                    label="Campaigns Sent"
                    value={stats?.total_campaigns || 0}
                    icon={Send}
                    color="text-blue-500"
                    trend="+12%"
                />
                <StatCard
                    label="Total Emails"
                    value={formatNumber(stats?.total_sent || 0)}
                    icon={Mail}
                    color="text-emerald-500"
                    trend="+8%"
                />
                <StatCard
                    label="Open Rate"
                    value={`${(stats?.open_rate || 0).toFixed(1)}%`}
                    icon={Eye}
                    color="text-purple-500"
                    trend="+2.3%"
                />
                <StatCard
                    label="Click Rate"
                    value={`${(stats?.click_rate || 0).toFixed(1)}%`}
                    icon={MousePointer}
                    color="text-orange-500"
                    trend="+1.5%"
                />
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Performance Overview */}
                <div className="lg:col-span-2 bg-card border rounded-xl p-6 shadow-sm">
                    <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                        <TrendingUp className="w-5 h-5 text-muted-foreground" />
                        Performance Overview
                    </h3>
                    <div className="grid grid-cols-3 gap-4 text-center">
                        <div className="p-4 bg-muted/50 rounded-lg">
                            <div className="text-3xl font-bold text-foreground">{formatNumber(stats?.total_opens || 0)}</div>
                            <div className="text-sm text-muted-foreground">Total Opens</div>
                        </div>
                        <div className="p-4 bg-muted/50 rounded-lg">
                            <div className="text-3xl font-bold text-foreground">{formatNumber(stats?.total_clicks || 0)}</div>
                            <div className="text-sm text-muted-foreground">Total Clicks</div>
                        </div>
                        <div className="p-4 bg-muted/50 rounded-lg">
                            <div className="text-3xl font-bold text-foreground">{stats?.total_campaigns || 0}</div>
                            <div className="text-sm text-muted-foreground">Campaigns</div>
                        </div>
                    </div>
                    <Link
                        to="/analytics"
                        className="mt-4 inline-flex items-center gap-2 text-sm text-primary hover:underline"
                    >
                        View Detailed Analytics <ArrowRight className="w-4 h-4" />
                    </Link>
                </div>

                {/* Recent Activity */}
                <div className="bg-card border rounded-xl p-6 shadow-sm">
                    <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                        <Clock className="w-5 h-5 text-muted-foreground" />
                        Recent Activity
                    </h3>
                    {recentActivity.length > 0 ? (
                        <div className="space-y-3">
                            {recentActivity.map((activity, idx) => (
                                <div key={idx} className="flex items-start gap-3 text-sm">
                                    <div className="w-2 h-2 mt-2 rounded-full bg-primary" />
                                    <div>
                                        <div className="text-foreground">{activity.description}</div>
                                        <div className="text-xs text-muted-foreground">{activity.time}</div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="text-center py-8 text-muted-foreground">
                            <Clock className="w-8 h-8 mx-auto mb-2 opacity-50" />
                            <p className="text-sm">No recent activity</p>
                        </div>
                    )}
                </div>
            </div>

            {/* Quick Actions */}
            <div className="bg-card border rounded-xl p-6 shadow-sm">
                <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
                    <Zap className="w-5 h-5 text-muted-foreground" />
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
                                <div className={cn("p-2 rounded-lg bg-muted", link.color)}>
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

function StatCard({ label, value, icon: Icon, color, trend }) {
    return (
        <div className="bg-card border rounded-xl p-6 shadow-sm hover:shadow-md transition-shadow">
            <div className="flex justify-between items-start">
                <div>
                    <p className="text-sm font-medium text-muted-foreground">{label}</p>
                    <h4 className="text-2xl font-bold mt-2 text-foreground">{value}</h4>
                    {trend && (
                        <p className="text-xs text-green-600 mt-1 flex items-center gap-1">
                            <TrendingUp className="w-3 h-3" /> {trend}
                        </p>
                    )}
                </div>
                <div className={cn("p-2 rounded-lg bg-secondary", color)}>
                    <Icon className="w-5 h-5" />
                </div>
            </div>
        </div>
    );
}

function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
}
