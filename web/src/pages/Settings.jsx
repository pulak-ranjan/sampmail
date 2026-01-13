import React, { useEffect, useState } from "react";
import {
  Save, Server, Globe, Network, Bot, Key, Loader2, CheckCircle,
  Mail, Shield, ExternalLink, AlertCircle, Webhook, MessageSquare
} from "lucide-react";
import { getSettings, saveSettings } from "../api";
import { cn } from "../lib/utils";

export default function Settings() {
  const [form, setForm] = useState({
    // Server
    main_hostname: "",
    main_server_ip: "",
    smtp_listen_addr: "",

    // Reacher (Email Verification)
    reacher_url: "",
    reacher_api_key: "",
    reacher_bin_path: "",
    proxy_url: "",

    // AI
    ai_provider: "",
    ai_api_key: "",

    // Webhooks
    webhook_url: "",
    webhook_enabled: false,
    bounce_alert_pct: 5,

    // WhatsApp
    whatsapp_phone_number_id: "",
    whatsapp_access_token: "",
    whatsapp_verify_token: "",

    // CORS
    allowed_origins: ""
  });

  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState({ text: "", type: "" });
  const [activeSection, setActiveSection] = useState("mta");
  const [testingReacher, setTestingReacher] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const s = await getSettings();
        setForm((f) => ({ ...f, ...s }));
      } catch (err) {
        setMsg({ text: "Failed to load settings", type: "error" });
      }
    })();
  }, []);

  const onChange = (e) => {
    const { name, value, type, checked } = e.target;
    setForm((f) => ({ ...f, [name]: type === "checkbox" ? checked : value }));
  };

  const onSubmit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setMsg({ text: "", type: "" });
    try {
      await saveSettings(form);
      setMsg({ text: "Settings saved successfully!", type: "success" });
      // Refetch settings to show current values
      const updated = await getSettings();
      setForm((f) => ({ ...f, ...updated, ai_api_key: "", reacher_api_key: "", whatsapp_access_token: "" }));
    } catch (err) {
      setMsg({ text: err.message || "Failed to save settings", type: "error" });
    } finally {
      setSaving(false);
    }
  };

  const testReacherConnection = async () => {
    if (!form.reacher_url && !form.reacher_bin_path) {
      setMsg({ text: "Configure Reacher URL or binary path first", type: "error" });
      return;
    }
    setTestingReacher(true);
    try {
      const token = localStorage.getItem("sampmail_token");
      const res = await fetch("/api/contacts/verify", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify({ email: "test@gmail.com" })
      });
      const data = await res.json();
      if (data.status) {
        setMsg({ text: `Reacher is working! Test result: ${data.status}`, type: "success" });
      } else {
        setMsg({ text: "Reacher test completed but no status returned", type: "warning" });
      }
    } catch (err) {
      setMsg({ text: `Reacher test failed: ${err.message}`, type: "error" });
    } finally {
      setTestingReacher(false);
    }
  };

  const sections = [
    { id: "mta", label: "MTA / Server", icon: Server },
    { id: "reacher", label: "Email Verification", icon: Shield },
    { id: "ai", label: "AI Integration", icon: Bot },
    { id: "webhooks", label: "Webhooks", icon: Webhook },
    { id: "whatsapp", label: "WhatsApp", icon: MessageSquare },
    { id: "advanced", label: "Advanced", icon: Network },
  ];

  return (
    <div className="max-w-4xl mx-auto space-y-6 py-4">
      <div>
        <h1 className="text-3xl font-bold tracking-tight text-foreground">Configuration</h1>
        <p className="text-muted-foreground">Configure email verification, MTA, and integrations</p>
      </div>

      {msg.text && (
        <div className={cn(
          "p-4 rounded-lg border flex items-center gap-3",
          msg.type === "error" ? "bg-red-500/10 text-red-400 border-red-500/20" :
            msg.type === "success" ? "bg-green-500/10 text-green-400 border-green-500/20" :
              "bg-yellow-500/10 text-yellow-400 border-yellow-500/20"
        )}>
          {msg.type === "error" ? <AlertCircle className="w-5 h-5" /> : <CheckCircle className="w-5 h-5" />}
          {msg.text}
        </div>
      )}

      <div className="flex gap-6">
        {/* Section Navigation */}
        <div className="w-48 shrink-0">
          <nav className="space-y-1 sticky top-6">
            {sections.map((s) => (
              <button
                key={s.id}
                onClick={() => setActiveSection(s.id)}
                className={cn(
                  "w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors text-left",
                  activeSection === s.id
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
              >
                <s.icon className="w-4 h-4" />
                {s.label}
              </button>
            ))}
          </nav>
        </div>

        {/* Settings Form */}
        <form onSubmit={onSubmit} className="flex-1 space-y-6">

          {/* MTA / Server Section */}
          {activeSection === "mta" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <Server className="w-5 h-5 text-blue-500" />
                  MTA / Server Configuration
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Configure your mail server hostname and IP settings for KumoMTA
                </p>
              </div>

              <div className="grid gap-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Main Hostname (FQDN)</label>
                  <div className="relative">
                    <Globe className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <input
                      name="main_hostname"
                      value={form.main_hostname}
                      onChange={onChange}
                      className="w-full pl-9 h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                      placeholder="mta.example.com"
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    The fully qualified domain name for your MTA. Must have valid PTR record.
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Main Server IP</label>
                  <div className="relative">
                    <Network className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <input
                      name="main_server_ip"
                      value={form.main_server_ip}
                      onChange={onChange}
                      className="w-full pl-9 h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                      placeholder="104.234.26.151"
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Primary IP address for outbound email sending
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">SMTP Listen Address</label>
                  <input
                    name="smtp_listen_addr"
                    value={form.smtp_listen_addr}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="0.0.0.0:25"
                  />
                  <p className="text-xs text-muted-foreground">
                    Address and port for SMTP listener (default: 0.0.0.0:25)
                  </p>
                </div>
              </div>

              <div className="bg-muted/30 rounded-lg p-4">
                <h4 className="font-medium text-sm mb-2">📖 KumoMTA Setup Guide</h4>
                <ol className="text-xs text-muted-foreground space-y-1 list-decimal list-inside">
                  <li>Install KumoMTA: <code className="bg-muted px-1 rounded">dnf install kumomta</code></li>
                  <li>Configure DNS: Add A record for your hostname</li>
                  <li>Configure PTR: Contact your VPS provider to set reverse DNS</li>
                  <li>Start service: <code className="bg-muted px-1 rounded">systemctl start kumomta</code></li>
                </ol>
                <a
                  href="https://docs.kumomta.com"
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline mt-2"
                >
                  Full Documentation <ExternalLink className="w-3 h-3" />
                </a>
              </div>
            </div>
          )}

          {/* Reacher / Email Verification Section */}
          {activeSection === "reacher" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <Shield className="w-5 h-5 text-green-500" />
                  Email Verification (Reacher)
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Configure Reacher for verifying emails (especially Gmail, Yahoo, Outlook)
                </p>
              </div>

              <div className="bg-blue-500/10 border border-blue-500/20 rounded-lg p-4">
                <h4 className="font-medium text-sm text-blue-400 mb-2">ℹ️ About Reacher</h4>
                <p className="text-xs text-muted-foreground">
                  Reacher is the most accurate email verification tool. It can verify Gmail, Yahoo, and other
                  providers that block direct SMTP checks. You can either:
                </p>
                <ul className="text-xs text-muted-foreground mt-2 space-y-1 list-disc list-inside">
                  <li><strong>Self-host</strong>: Run Reacher binary on your server (recommended)</li>
                  <li><strong>API</strong>: Use hosted Reacher service (reacher.email)</li>
                </ul>
              </div>

              <div className="grid gap-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Reacher Binary Path (Recommended)</label>
                  <input
                    name="reacher_bin_path"
                    value={form.reacher_bin_path}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm font-mono focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="/usr/local/bin/check_if_email_exists"
                  />
                  <p className="text-xs text-muted-foreground">
                    Path to the <code>check_if_email_exists</code> binary
                  </p>
                </div>

                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-card px-2 text-muted-foreground">Or use API</span>
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Reacher API URL</label>
                  <input
                    name="reacher_url"
                    value={form.reacher_url}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="http://localhost:8080 or https://api.reacher.email"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Reacher API Key</label>
                  <div className="relative">
                    <Key className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <input
                      name="reacher_api_key"
                      type="password"
                      value={form.reacher_api_key}
                      onChange={onChange}
                      className="w-full pl-9 h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                      placeholder="For hosted Reacher only"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Proxy URL (Optional)</label>
                  <input
                    name="proxy_url"
                    value={form.proxy_url}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="socks5://user:pass@proxy.example.com:1080"
                  />
                  <p className="text-xs text-muted-foreground">
                    SOCKS5 or HTTP proxy for verification (helps avoid IP blocks)
                  </p>
                </div>
              </div>

              <button
                type="button"
                onClick={testReacherConnection}
                disabled={testingReacher}
                className="w-full py-2 bg-muted hover:bg-muted/80 rounded-lg text-sm font-medium flex items-center justify-center gap-2"
              >
                {testingReacher ? <Loader2 className="w-4 h-4 animate-spin" /> : <CheckCircle className="w-4 h-4" />}
                Test Reacher Connection
              </button>

              <div className="bg-muted/30 rounded-lg p-4">
                <h4 className="font-medium text-sm mb-2">📖 Reacher Installation</h4>
                <div className="text-xs text-muted-foreground space-y-2">
                  <p><strong>Option 1: Download Binary</strong></p>
                  <pre className="bg-muted p-2 rounded overflow-x-auto">
                    {`# Download latest release
wget https://github.com/reacherhq/check-if-email-exists/releases/latest/download/check_if_email_exists-x86_64-linux.tar.gz
tar -xzf check_if_email_exists-*.tar.gz
mv check_if_email_exists /usr/local/bin/
chmod +x /usr/local/bin/check_if_email_exists`}
                  </pre>
                  <p><strong>Option 2: Run as Docker</strong></p>
                  <pre className="bg-muted p-2 rounded overflow-x-auto">
                    {`docker run -p 8080:8080 reacherhq/backend:latest`}
                  </pre>
                </div>
              </div>
            </div>
          )}

          {/* AI Integration Section */}
          {activeSection === "ai" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <Bot className="w-5 h-5 text-purple-500" />
                  AI Integration
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Configure AI provider for smart assistance and content generation
                </p>
              </div>

              <div className="grid gap-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">AI Provider</label>
                  <select
                    name="ai_provider"
                    value={form.ai_provider}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                  >
                    <option value="">Select Provider</option>
                    <option value="openai">OpenAI (GPT-4o)</option>
                    <option value="deepseek">DeepSeek (V3)</option>
                    <option value="gemini">Google Gemini (Flash)</option>
                  </select>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">API Key</label>
                  <div className="relative">
                    <Key className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <input
                      name="ai_api_key"
                      type="password"
                      value={form.ai_api_key}
                      onChange={onChange}
                      className="w-full pl-9 h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                      placeholder="sk-... or your API key"
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">API key is write-only and never displayed</p>
                </div>
              </div>
            </div>
          )}

          {/* Webhooks Section */}
          {activeSection === "webhooks" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <Webhook className="w-5 h-5 text-orange-500" />
                  Webhooks
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Receive real-time notifications for email events
                </p>
              </div>

              <div className="grid gap-4">
                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="webhook_enabled"
                    name="webhook_enabled"
                    checked={form.webhook_enabled}
                    onChange={onChange}
                    className="w-4 h-4 rounded"
                  />
                  <label htmlFor="webhook_enabled" className="text-sm font-medium">Enable Webhooks</label>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Webhook URL</label>
                  <input
                    name="webhook_url"
                    value={form.webhook_url}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="https://your-app.com/webhooks/email"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Bounce Alert Threshold (%)</label>
                  <input
                    type="number"
                    name="bounce_alert_pct"
                    value={form.bounce_alert_pct}
                    onChange={onChange}
                    min="0"
                    max="100"
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                  />
                  <p className="text-xs text-muted-foreground">
                    Alert when bounce rate exceeds this percentage
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* WhatsApp Section */}
          {activeSection === "whatsapp" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <MessageSquare className="w-5 h-5 text-green-500" />
                  WhatsApp Business API
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  Connect WhatsApp Business for messaging campaigns
                </p>
              </div>

              <div className="grid gap-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Phone Number ID</label>
                  <input
                    name="whatsapp_phone_number_id"
                    value={form.whatsapp_phone_number_id}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="123456789012345"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Access Token</label>
                  <input
                    name="whatsapp_access_token"
                    type="password"
                    value={form.whatsapp_access_token}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="EAA..."
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Verify Token</label>
                  <input
                    name="whatsapp_verify_token"
                    value={form.whatsapp_verify_token}
                    onChange={onChange}
                    className="w-full h-10 rounded-lg border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="Your webhook verify token"
                  />
                </div>
              </div>
            </div>
          )}

          {/* Advanced Section */}
          {activeSection === "advanced" && (
            <div className="bg-card border border-border rounded-xl p-6 space-y-6">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <Network className="w-5 h-5 text-gray-500" />
                  Advanced Settings
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  CORS and other advanced configurations
                </p>
              </div>

              <div className="grid gap-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Allowed Origins (CORS)</label>
                  <textarea
                    name="allowed_origins"
                    value={form.allowed_origins}
                    onChange={onChange}
                    rows={3}
                    className="w-full rounded-lg border bg-background p-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    placeholder="https://app.example.com, https://admin.example.com"
                  />
                  <p className="text-xs text-muted-foreground">
                    Comma-separated list of allowed origins for CORS
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Save Button */}
          <div className="flex justify-end">
            <button
              type="submit"
              disabled={saving}
              className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-8 py-3 rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
            >
              {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
              {saving ? "Saving..." : "Save Settings"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
