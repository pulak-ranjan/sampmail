import React, { useEffect, useState } from "react";
import {
  Globe,
  Mail,
  Cpu,
  MemoryStick,
  Server,
  Activity,
  ShieldAlert,
  Sparkles
} from "lucide-react";
import { getDashboardStats, getAIInsights } from "../api";
import { cn } from "../lib/utils";

export default function Dashboard() {
  const [stats, setStats] = useState(null);
  const [insight, setInsight] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const s = await getDashboardStats();
        setStats(s);
      } catch (err) {
        setError("Failed to load stats");
      }
    })();
  }, []);

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

  if (!stats) return <div className="p-8 text-muted-foreground flex justify-center">Loading dashboard...</div>;

  return (
    <div className="space-y-8 animate-in fade-in duration-500">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h2 className="text-3xl font-bold tracking-tight text-foreground">Dashboard</h2>
          <p className="text-muted-foreground">Overview of your email infrastructure.</p>
        </div>
        <button
          onClick={getAI}
          disabled={loading}
          className={cn(
            "relative px-4 py-2 rounded-lg text-sm font-medium flex items-center gap-2",
            "bg-gradient-to-r from-cyan-500 to-purple-600 text-white",
            "hover:from-cyan-400 hover:to-purple-500",
            "shadow-lg shadow-purple-500/25 hover:shadow-purple-500/40",
            "transition-all duration-300 hover:scale-105",
            loading && "opacity-70"
          )}
        >
          <div className="absolute inset-0 rounded-lg bg-white/20 animate-pulse opacity-0" />
          {loading ? <Sparkles className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
          {loading ? "Analyzing..." : "AI Log Analysis"}
        </button>
      </div>

      {insight && (
        <div className="bg-gradient-to-r from-cyan-950/50 via-purple-950/50 to-fuchsia-950/50 border border-cyan-500/20 p-6 rounded-xl relative overflow-hidden backdrop-blur-sm">
          <div className="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-cyan-400 to-purple-500" />
          <div className="absolute inset-0 bg-gradient-to-r from-cyan-500/5 to-purple-500/5" />
          <h3 className="font-semibold text-cyan-400 mb-2 flex items-center gap-2 relative">
            <Sparkles className="w-4 h-4 animate-pulse" /> AI Insight
          </h3>
          <div className="text-sm text-foreground/80 whitespace-pre-wrap leading-relaxed relative">
            {insight}
          </div>
        </div>
      )}

      {/* Main Stats */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Domains" value={stats.domains} icon={Globe} color="text-blue-500" />
        <StatCard label="Active Senders" value={stats.senders} icon={Mail} color="text-emerald-500" />
        <StatCard label="CPU Load" value={stats.cpu_load} icon={Cpu} color="text-orange-500" />
        <StatCard label="RAM Usage" value={stats.ram_usage} icon={MemoryStick} color="text-purple-500" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Infrastructure Health - Glassmorphism */}
        <div className="bg-gray-900/60 backdrop-blur-xl border border-white/10 rounded-xl p-6 shadow-xl shadow-black/20">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
            <Activity className="w-5 h-5 text-cyan-400" />
            Service Status
          </h3>
          <div className="space-y-3">
            <ServiceRow name="MTA Service" status={stats.kumo_status} />
            <ServiceRow name="Dovecot" status={stats.dovecot_status} />
            <ServiceRow name="Fail2Ban" status={stats.f2b_status} />
          </div>
        </div>

        {/* Open Ports - Glassmorphism */}
        <div className="bg-gray-900/60 backdrop-blur-xl border border-white/10 rounded-xl p-6 shadow-xl shadow-black/20">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2 text-foreground">
            <Server className="w-5 h-5 text-purple-400" />
            Open Ports
          </h3>
          <div className="flex flex-wrap gap-2">
            {stats.open_ports ? (
              stats.open_ports.split(", ").map(port => (
                <span
                  key={port}
                  className="bg-white/5 text-foreground px-3 py-1.5 rounded-lg text-sm font-mono border border-white/10 hover:border-cyan-500/30 hover:bg-white/10 transition-all duration-200"
                >
                  {port}
                </span>
              ))
            ) : (
              <span className="text-muted-foreground text-sm">Scanning...</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value, icon: Icon, color }) {
  // Map colors to glow variants
  const colorMap = {
    'text-blue-500': 'from-blue-500/20 to-cyan-500/20 border-blue-500/30',
    'text-emerald-500': 'from-emerald-500/20 to-cyan-500/20 border-emerald-500/30',
    'text-orange-500': 'from-orange-500/20 to-amber-500/20 border-orange-500/30',
    'text-purple-500': 'from-purple-500/20 to-fuchsia-500/20 border-purple-500/30',
  };
  const glowClass = colorMap[color] || colorMap['text-blue-500'];

  return (
    <div className={cn(
      "group relative bg-card/60 backdrop-blur-sm border rounded-xl p-6",
      "hover:bg-card/80 transition-all duration-300 hover:scale-[1.02]",
      "border-white/5 hover:border-white/20",
      "shadow-lg shadow-black/20"
    )}>
      {/* Gradient accent on hover */}
      <div className={cn(
        "absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-300",
        "bg-gradient-to-br", glowClass
      )} />

      <div className="relative flex justify-between items-start">
        <div>
          <p className="text-sm font-medium text-muted-foreground/70">{label}</p>
          <h4 className="text-2xl font-bold mt-2 text-foreground">{value}</h4>
        </div>
        <div className={cn(
          "p-2.5 rounded-lg bg-gradient-to-br transition-transform duration-300 group-hover:scale-110",
          color.replace('text-', 'from-').replace('-500', '-400/20'),
          "to-gray-500/20"
        )}>
          <Icon className={cn("w-5 h-5", color)} />
        </div>
      </div>

      {/* Glow effect */}
      <div className={cn(
        "absolute -inset-px rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-300",
        "bg-gradient-to-r from-cyan-500/20 via-purple-500/20 to-fuchsia-500/20 blur-xl"
      )} />
    </div>
  );
}

function ServiceRow({ name, status }) {
  const isActive = status === "active";
  return (
    <div className="flex items-center justify-between p-3 rounded-lg border bg-white/5 hover:bg-white/10 transition-colors duration-200 border-white/5">
      <span className="font-medium text-foreground">{name}</span>
      <div className="flex items-center gap-2">
        <span className={cn(
          "text-xs font-medium px-2.5 py-1 rounded-full capitalize",
          isActive
            ? "bg-emerald-500/15 text-emerald-400 border border-emerald-500/20"
            : "bg-red-500/15 text-red-400 border border-red-500/20"
        )}>
          {status || "Unknown"}
        </span>
        <div className={cn(
          "w-2 h-2 rounded-full",
          isActive
            ? "bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)] animate-pulse"
            : "bg-red-500"
        )} />
      </div>
    </div>
  );
}
