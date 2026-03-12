import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  RefreshCw,
  Trash2,
  Zap,
  Inbox,
  Clock,
  AlertCircle,
  CheckCircle2,
  Mail,
  Server,
  Play,
  Square,
  RotateCcw,
  Search,
  X
} from 'lucide-react';
import { cn } from '../lib/utils';

export default function QueuePage() {
  const [messages, setMessages] = useState([]);
  const [stats, setStats] = useState(null);
  const [kumoStatus, setKumoStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [limit, setLimit] = useState(100);
  const [domainFilter, setDomainFilter] = useState('');
  const [actionLoading, setActionLoading] = useState(null);

  const token = localStorage.getItem('sampmail_token');
  const headers = useMemo(() => ({ Authorization: `Bearer ${token}` }), [token]);

  const fetchQueue = useCallback(async () => {
    setLoading(true);
    try {
      const url = domainFilter
        ? `/api/queue?limit=${limit}&domain=${encodeURIComponent(domainFilter)}`
        : `/api/queue?limit=${limit}`;
      const res = await fetch(url, { headers });
      if (res.status === 401) { window.location.href = '/login'; return; }
      const data = await res.json();
      setMessages(Array.isArray(data) ? data : []);
    } catch (e) { console.error(e); setMessages([]); }
    setLoading(false);
  }, [limit, domainFilter, headers]);

  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch('/api/queue/stats', { headers });
      if (res.ok) setStats(await res.json());
    } catch (e) { console.error(e); }
  }, [headers]);

  const fetchKumoStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/kumo/status', { headers });
      if (res.ok) setKumoStatus(await res.json());
    } catch (e) { console.error(e); }
  }, [headers]);

  useEffect(() => {
    fetchQueue();
    fetchStats();
    fetchKumoStatus();
  }, [limit, fetchQueue, fetchStats, fetchKumoStatus]);

  const deleteMessage = async (id) => {
    if (!confirm('Delete this message from queue?')) return;
    setActionLoading('delete-' + id);
    try {
      await fetch(`/api/queue/${id}`, { method: 'DELETE', headers });
      fetchQueue(); fetchStats();
    } catch (e) { console.error(e); }
    setActionLoading(null);
  };

  const flushQueue = async () => {
    if (!confirm('Retry all deferred messages now?')) return;
    setActionLoading('flush');
    try {
      await fetch('/api/queue/flush', { method: 'POST', headers });
      fetchQueue(); fetchStats();
    } catch (e) { console.error(e); }
    setActionLoading(null);
  };

  const retryAll = async () => {
    if (!confirm('Retry all deferred messages?')) return;
    setActionLoading('retry-all');
    try {
      await fetch('/api/queue/retry/all', { method: 'POST', headers });
      fetchQueue(); fetchStats();
    } catch (e) { console.error(e); }
    setActionLoading(null);
  };

  const deleteBounced = async () => {
    if (!confirm('Delete all bounced messages from queue? This cannot be undone.')) return;
    setActionLoading('delete-bounced');
    try {
      await fetch('/api/queue/bounced', { method: 'DELETE', headers });
      fetchQueue(); fetchStats();
    } catch (e) { console.error(e); }
    setActionLoading(null);
  };

  const restartKumo = async () => {
    if (!confirm('Restart KumoMTA service?')) return;
    setActionLoading('restart');
    try {
      await fetch('/api/kumo/restart', { method: 'POST', headers });
      setTimeout(fetchKumoStatus, 2000);
    } catch (e) { console.error(e); }
    setActionLoading(null);
  };

  const formatDate = (d) => d ? new Date(d).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '-';
  const formatSize = (b) => b > 1024 ? `${(b / 1024).toFixed(1)} KB` : `${b} B`;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Mail Queue</h1>
          <p className="text-muted-foreground">Monitor and manage outbound messages via KumoMTA.</p>
        </div>
        <div className="flex gap-2 flex-wrap">
          <button onClick={() => { fetchQueue(); fetchStats(); fetchKumoStatus(); }} className="flex items-center gap-2 h-10 px-4 rounded-md bg-secondary text-secondary-foreground hover:bg-secondary/80 text-sm font-medium transition-colors">
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
          <button onClick={retryAll} disabled={actionLoading === 'retry-all'} className="flex items-center gap-2 h-10 px-4 rounded-md bg-blue-600 text-white hover:bg-blue-700 text-sm font-medium transition-colors disabled:opacity-50">
            <RotateCcw className="w-4 h-4" /> {actionLoading === 'retry-all' ? 'Retrying...' : 'Retry All'}
          </button>
          <button onClick={flushQueue} disabled={actionLoading === 'flush'} className="flex items-center gap-2 h-10 px-4 rounded-md bg-amber-600 text-white hover:bg-amber-700 text-sm font-medium transition-colors disabled:opacity-50">
            <Zap className="w-4 h-4" /> {actionLoading === 'flush' ? 'Flushing...' : 'Flush'}
          </button>
          <button onClick={deleteBounced} disabled={actionLoading === 'delete-bounced'} className="flex items-center gap-2 h-10 px-4 rounded-md bg-red-600 text-white hover:bg-red-700 text-sm font-medium transition-colors disabled:opacity-50">
            <Trash2 className="w-4 h-4" /> {actionLoading === 'delete-bounced' ? 'Deleting...' : 'Delete Bounced'}
          </button>
        </div>
      </div>

      {/* KumoMTA Status */}
      <div className="bg-card border rounded-xl p-4 shadow-sm">
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div className="flex items-center gap-3">
            <div className={cn("w-3 h-3 rounded-full", kumoStatus?.running ? "bg-green-500" : "bg-red-500")} />
            <span className="font-medium">KumoMTA</span>
            <span className={cn("text-sm px-2 py-0.5 rounded-full",
              kumoStatus?.health === 'healthy' ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800")}>
              {kumoStatus?.health || 'unknown'}
            </span>
            {kumoStatus?.version && <span className="text-sm text-muted-foreground">v{kumoStatus.version}</span>}
          </div>
          <button onClick={restartKumo} disabled={actionLoading === 'restart'} className="flex items-center gap-2 h-8 px-3 rounded-md bg-secondary text-secondary-foreground hover:bg-secondary/80 text-sm font-medium transition-colors disabled:opacity-50">
            <RotateCcw className="w-3 h-3" /> {actionLoading === 'restart' ? 'Restarting...' : 'Restart'}
          </button>
        </div>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <QueueStat label="Total Messages" value={stats.total} icon={Inbox} />
          <QueueStat label="Queued (Active)" value={stats.queued} icon={Mail} color="text-blue-500" />
          <QueueStat label="Deferred (Retry)" value={stats.deferred} icon={Clock} color="text-amber-500" />
          <QueueStat label="Total Size" value={formatSize(stats.total_size)} icon={AlertCircle} />
        </div>
      )}

      {/* Domain Filter */}
      <div className="flex gap-2 items-center">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Filter by domain (e.g., gmail.com)"
            value={domainFilter}
            onChange={(e) => setDomainFilter(e.target.value)}
            className="w-full h-10 pl-10 pr-4 rounded-md border bg-background text-sm focus:ring-2 focus:ring-ring"
          />
          {domainFilter && (
            <button onClick={() => setDomainFilter('')} className="absolute right-3 top-1/2 -translate-y-1/2">
              <X className="w-4 h-4 text-muted-foreground hover:text-foreground" />
            </button>
          )}
        </div>
        <select value={limit} onChange={e => setLimit(+e.target.value)} className="h-10 rounded-md border bg-background px-3 py-2 text-sm">
          <option value={50}>50</option>
          <option value={100}>100</option>
          <option value={500}>500</option>
        </select>
      </div>

      {/* Queue Table */}
      <div className="bg-card border rounded-xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-muted-foreground">Loading queue data...</div>
        ) : messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center p-16 text-center">
            <div className="p-4 bg-green-100 dark:bg-green-900/20 rounded-full mb-4">
              <CheckCircle2 className="w-12 h-12 text-green-600 dark:text-green-400" />
            </div>
            <h3 className="text-xl font-semibold mb-1">Queue is Empty</h3>
            <p className="text-muted-foreground">All messages have been delivered or processed.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="bg-muted/50 text-muted-foreground uppercase text-xs">
                <tr>
                  <th className="px-4 py-3 font-medium">ID</th>
                  <th className="px-4 py-3 font-medium">Domain</th>
                  <th className="px-4 py-3 font-medium">From</th>
                  <th className="px-4 py-3 font-medium">To</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium text-center">Retries</th>
                  <th className="px-4 py-3 font-medium text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {messages.map(msg => (
                  <tr key={msg.id} className="hover:bg-muted/50 transition-colors group">
                    <td className="px-4 py-3 font-mono text-xs text-muted-foreground" title={msg.id}>
                      {msg.id?.substring(0, 12)}...
                    </td>
                    <td className="px-4 py-3">{msg.domain || '-'}</td>
                    <td className="px-4 py-3 truncate max-w-[150px]" title={msg.from}>{msg.from || '-'}</td>
                    <td className="px-4 py-3 truncate max-w-[150px]" title={msg.to}>{msg.to || '-'}</td>
                    <td className="px-4 py-3">
                      <span className={cn(
                        "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize",
                        msg.status === 'deferred'
                          ? "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400"
                          : msg.status === 'bounced'
                          ? "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400"
                          : "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400"
                      )}>
                        {msg.status || 'queued'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center">{msg.retry_count || 0}</td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => deleteMessage(msg.id)}
                        disabled={actionLoading === 'delete-' + msg.id}
                        className="p-1.5 hover:bg-destructive/10 text-muted-foreground hover:text-destructive rounded-md transition-colors disabled:opacity-50"
                        title="Delete Message"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function QueueStat({ label, value, icon: Icon, color }) {
  return (
    <div className="bg-card border rounded-xl p-4 shadow-sm flex items-center justify-between">
      <div>
        <p className="text-sm font-medium text-muted-foreground">{label}</p>
        <p className="text-2xl font-bold mt-1">{value || 0}</p>
      </div>
      <div className={cn("p-2 rounded-lg bg-secondary", color)}>
        <Icon className="w-5 h-5" />
      </div>
    </div>
  );
}
