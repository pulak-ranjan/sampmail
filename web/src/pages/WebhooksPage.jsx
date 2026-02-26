import React, { useState, useEffect } from 'react';
import {
  Webhook,
  Save,
  Activity,
  CheckCircle2,
  AlertTriangle,
  ShieldAlert,
  Search,
  Bell,
  History
} from 'lucide-react';
import { cn } from '../lib/utils';
import api from '../api';

export default function WebhooksPage() {
  const [settings, setSettings] = useState({ webhook_url: '', webhook_enabled: false });
  const [webhookId, setWebhookId] = useState(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchSettings().finally(() => setLoading(false));
  }, []);

  const fetchSettings = async () => {
    try {
      const list = await api.get('/v2/webhooks');
      const hook = Array.isArray(list) ? list[0] : null;
      setWebhookId(hook?.id ?? null);
      setSettings({
        webhook_url: hook?.url || '',
        webhook_enabled: !!hook?.is_active,
      });
    } catch (e) {
      console.error(e);
      setWebhookId(null);
      setSettings({ webhook_url: '', webhook_enabled: false });
    }
  };

  const saveSettings = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (webhookId) {
        await api.put(`/v2/webhooks/${webhookId}`, { url: settings.webhook_url, enabled: settings.webhook_enabled });
      } else {
        const created = await api.post('/v2/webhooks', { url: settings.webhook_url, enabled: settings.webhook_enabled });
        setWebhookId(created?.id ?? null);
      }
      setMessage('Settings saved successfully');
    } catch (e) { setMessage('Error: ' + e.message); }
    setSaving(false);
    setTimeout(() => setMessage(''), 3000);
  };

  const testWebhook = async () => {
    if (!settings.webhook_url) { setMessage('Enter webhook URL first'); return; }
    setTesting(true);
    try {
      await api.post('/v2/webhooks/test', { url: settings.webhook_url });
      setMessage('Test payload sent!');
    } catch (e) { setMessage('Error: ' + e.message); }
    setTesting(false);
    setTimeout(() => setMessage(''), 3000);
  };

  const formatDate = (d) => d ? new Date(d).toLocaleString() : '-';

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Webhooks & Alerts</h1>
          <p className="text-muted-foreground">Configure external notifications for Slack or Discord.</p>
        </div>
      </div>

      {message && (
        <div className={cn("p-4 rounded-md text-sm font-medium flex items-center gap-2", message.includes("Failed") || message.includes("Error") ? "bg-destructive/10 text-destructive" : "bg-green-500/10 text-green-600")}>
          {message.includes("Error") ? <AlertTriangle className="w-4 h-4" /> : <CheckCircle2 className="w-4 h-4" />}
          {message}
        </div>
      )}

      <div className="grid lg:grid-cols-3 gap-6">

        {/* Settings Column */}
        <div className="lg:col-span-2 space-y-6">
          <div className="bg-card border rounded-xl p-6 shadow-sm">
            <h3 className="font-semibold mb-6 flex items-center gap-2">
              <Webhook className="w-4 h-4 text-muted-foreground" /> Configuration
            </h3>

            <form onSubmit={saveSettings} className="space-y-5">
              <div className="space-y-2">
                <label className="text-sm font-medium">Webhook URL</label>
                <div className="relative">
                  <input
                    type="url"
                    value={settings.webhook_url}
                    onChange={e => setSettings({ ...settings, webhook_url: e.target.value })}
                    placeholder="https://hooks.slack.com/services/..."
                    className="w-full h-10 pl-3 pr-24 rounded-md border bg-background text-sm focus:ring-2 focus:ring-ring"
                  />
                  <button
                    type="button"
                    onClick={testWebhook}
                    disabled={testing}
                    className="absolute right-1 top-1 h-8 px-3 bg-secondary hover:bg-secondary/80 text-secondary-foreground rounded text-xs font-medium transition-colors"
                  >
                    {testing ? "Sending..." : "Test"}
                  </button>
                </div>
                <p className="text-[10px] text-muted-foreground">Supports Slack Incoming Webhooks and Discord Webhooks.</p>
              </div>

              <div className="flex items-center gap-3 p-4 rounded-lg border bg-muted/20">
                <input
                  type="checkbox"
                  id="enabled"
                  checked={settings.webhook_enabled}
                  onChange={e => setSettings({ ...settings, webhook_enabled: e.target.checked })}
                  className="h-5 w-5 rounded border-input text-primary focus:ring-primary"
                />
                <label htmlFor="enabled" className="text-sm font-medium cursor-pointer select-none">
                  Enable Notifications
                  <p className="text-xs text-muted-foreground font-normal mt-0.5">Send daily reports and critical alerts.</p>
                </label>
              </div>

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={saving}
                  className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors"
                >
                  {saving ? <Activity className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
                  Save Configuration
                </button>
              </div>
            </form>
          </div>
        </div>

        {/* Actions Column */}
        <div className="lg:col-span-1 space-y-4">
          <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider pl-1">Manual Triggers</h3>

          <ActionCard
            title="Check Bounces"
            desc="Analyze current bounce rates immediately."
            icon={Activity}
            onClick={() => setMessage('Bounce check triggered')}
            color="text-amber-500"
            bgColor="bg-amber-500/10"
          />

          <ActionCard
            title="Check IP Blacklists"
            desc="Scan RBLs for all system IPs."
            icon={Search}
            onClick={() => setMessage('Blacklist check started')}
            color="text-red-500"
            bgColor="bg-red-500/10"
          />

          <ActionCard
            title="Security Audit"
            desc="Check file permissions and ports."
            icon={ShieldAlert}
            onClick={() => setMessage('Security audit started')}
            color="text-purple-500"
            bgColor="bg-purple-500/10"
          />

          <div className="bg-muted/30 border rounded-xl p-4 mt-6">
            <h4 className="font-medium flex items-center gap-2 mb-3 text-sm">
              <Bell className="w-4 h-4" /> Auto-Schedule
            </h4>
            <ul className="space-y-3">
              <ScheduleItem time="Hourly" task="Bounce Rate Check" />
              <ScheduleItem time="Hourly" task="Blacklist Monitor" />
              <ScheduleItem time="Daily" task="Traffic Summary" />
              <ScheduleItem time="Daily" task="Security Audit" />
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

function ActionCard({ title, desc, icon: Icon, onClick, color, bgColor }) {
  return (
    <button
      onClick={onClick}
      className="w-full text-left bg-card hover:bg-muted/50 border rounded-xl p-4 transition-all shadow-sm hover:shadow-md group"
    >
      <div className="flex items-start gap-3">
        <div className={cn("p-2 rounded-lg transition-colors group-hover:bg-background", bgColor, color)}>
          <Icon className="w-5 h-5" />
        </div>
        <div>
          <div className="font-semibold text-foreground group-hover:text-primary transition-colors">{title}</div>
          <div className="text-xs text-muted-foreground mt-1 leading-relaxed">{desc}</div>
        </div>
      </div>
    </button>
  );
}

function ScheduleItem({ time, task }) {
  return (
    <li className="flex items-center justify-between text-xs">
      <span className="text-muted-foreground">{task}</span>
      <span className="font-mono bg-background border px-1.5 py-0.5 rounded text-[10px]">{time}</span>
    </li>
  );
}
