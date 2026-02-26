import React, { useState, useEffect } from 'react';
import { CheckCircle2, AlertTriangle, Copy, Globe, RefreshCw } from 'lucide-react';
import { cn } from '../lib/utils';
import {
  getOrgDomainConfig,
  updateOrgDomainConfig,
  verifyTrackingDomain,
  verifyUnsubscribeDomain
} from '../api';

export default function OrgDomainConfig({ orgId }) {
  const [config, setConfig] = useState({
    tracking_domain: '',
    tracking_domain_verified: false,
    unsubscribe_domain: '',
    unsubscribe_domain_verified: false,
    tracking_cname_target: '',
    unsub_cname_target: '',
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [verifyingTracking, setVerifyingTracking] = useState(false);
  const [verifyingUnsub, setVerifyingUnsub] = useState(false);
  const [message, setMessage] = useState('');
  const [messageType, setMessageType] = useState('success');

  useEffect(() => {
    if (!orgId) return;
    getOrgDomainConfig(orgId)
      .then(data => setConfig(data || config))
      .catch(err => console.error('Failed to load domain config:', err))
      .finally(() => setLoading(false));
  }, [orgId]);

  const showMessage = (msg, type = 'success') => {
    setMessage(msg);
    setMessageType(type);
    setTimeout(() => setMessage(''), 4000);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await updateOrgDomainConfig(orgId, {
        tracking_domain: config.tracking_domain,
        unsubscribe_domain: config.unsubscribe_domain,
      });
      setConfig(updated);
      showMessage('Domain configuration saved. Verification flags reset — please re-verify.');
    } catch (err) {
      showMessage('Failed to save: ' + err.message, 'error');
    }
    setSaving(false);
  };

  const handleVerifyTracking = async () => {
    setVerifyingTracking(true);
    try {
      const updated = await verifyTrackingDomain(orgId);
      setConfig(prev => ({ ...prev, tracking_domain_verified: updated.tracking_domain_verified }));
      if (updated.tracking_domain_verified) {
        showMessage('Tracking domain verified successfully!');
      } else {
        showMessage('CNAME not found. Ensure the DNS record has propagated.', 'error');
      }
    } catch (err) {
      showMessage('Verification failed: ' + err.message, 'error');
    }
    setVerifyingTracking(false);
  };

  const handleVerifyUnsub = async () => {
    setVerifyingUnsub(true);
    try {
      const updated = await verifyUnsubscribeDomain(orgId);
      setConfig(prev => ({ ...prev, unsubscribe_domain_verified: updated.unsubscribe_domain_verified }));
      if (updated.unsubscribe_domain_verified) {
        showMessage('Unsubscribe domain verified successfully!');
      } else {
        showMessage('CNAME not found. Ensure the DNS record has propagated.', 'error');
      }
    } catch (err) {
      showMessage('Verification failed: ' + err.message, 'error');
    }
    setVerifyingUnsub(false);
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text).then(() => showMessage('Copied to clipboard!'));
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <RefreshCw className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {message && (
        <div className={cn(
          "p-3 rounded-md text-sm flex items-center gap-2",
          messageType === 'error'
            ? "bg-destructive/10 text-destructive"
            : "bg-green-500/10 text-green-600"
        )}>
          {messageType === 'error'
            ? <AlertTriangle className="w-4 h-4 shrink-0" />
            : <CheckCircle2 className="w-4 h-4 shrink-0" />}
          {message}
        </div>
      )}

      <DomainSection
        title="Tracking Domain"
        description="Custom domain for tracking opens and clicks (e.g. track.yourdomain.com)."
        domain={config.tracking_domain}
        onDomainChange={val => setConfig(prev => ({ ...prev, tracking_domain: val }))}
        verified={config.tracking_domain_verified}
        cnameTarget={config.tracking_cname_target}
        onVerify={handleVerifyTracking}
        verifying={verifyingTracking}
        onCopy={copyToClipboard}
      />

      <DomainSection
        title="Unsubscribe Domain"
        description="Custom domain for unsubscribe pages (e.g. unsub.yourdomain.com)."
        domain={config.unsubscribe_domain}
        onDomainChange={val => setConfig(prev => ({ ...prev, unsubscribe_domain: val }))}
        verified={config.unsubscribe_domain_verified}
        cnameTarget={config.unsub_cname_target}
        onVerify={handleVerifyUnsub}
        verifying={verifyingUnsub}
        onCopy={copyToClipboard}
      />

      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50"
        >
          {saving ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Globe className="w-4 h-4" />}
          Save Domain Configuration
        </button>
      </div>
    </div>
  );
}

function DomainSection({ title, description, domain, onDomainChange, verified, cnameTarget, onVerify, verifying, onCopy }) {
  return (
    <div className="bg-card border rounded-xl p-6 shadow-sm space-y-4">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold flex items-center gap-2">
            <Globe className="w-4 h-4 text-muted-foreground" />
            {title}
          </h3>
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        </div>
        {domain && (
          <VerifiedBadge verified={verified} />
        )}
      </div>

      <div className="flex gap-2">
        <input
          type="text"
          value={domain}
          onChange={e => onDomainChange(e.target.value)}
          placeholder="e.g. track.yourdomain.com"
          className="flex-1 h-9 px-3 rounded-md border bg-background text-sm focus:ring-2 focus:ring-ring"
        />
        <button
          type="button"
          onClick={onVerify}
          disabled={verifying || !domain}
          className="px-3 py-1.5 bg-secondary hover:bg-secondary/80 text-secondary-foreground rounded-md text-xs font-medium transition-colors disabled:opacity-50 flex items-center gap-1.5"
        >
          {verifying ? <RefreshCw className="w-3 h-3 animate-spin" /> : <CheckCircle2 className="w-3 h-3" />}
          Verify
        </button>
      </div>

      {cnameTarget && (
        <div className="bg-muted/40 rounded-lg p-4 space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">DNS Configuration Required</p>
          <p className="text-xs text-muted-foreground">Add the following CNAME record to your DNS:</p>
          <div className="flex items-center gap-2 bg-background border rounded-md px-3 py-2">
            <code className="text-xs flex-1 font-mono">
              <span className="text-foreground">{domain || '<your-domain>'}</span>
              <span className="text-muted-foreground mx-2">CNAME</span>
              <span className="text-primary">{cnameTarget}</span>
            </code>
            <button
              type="button"
              onClick={() => onCopy(`${domain} CNAME ${cnameTarget}`)}
              className="text-muted-foreground hover:text-foreground transition-colors"
              title="Copy"
            >
              <Copy className="w-3.5 h-3.5" />
            </button>
          </div>
          <p className="text-[10px] text-muted-foreground">DNS propagation may take up to 48 hours.</p>
        </div>
      )}
    </div>
  );
}

function VerifiedBadge({ verified }) {
  if (verified) {
    return (
      <span className="flex items-center gap-1 text-xs font-medium text-green-600 bg-green-500/10 px-2 py-1 rounded-full">
        <CheckCircle2 className="w-3 h-3" /> Verified
      </span>
    );
  }
  return (
    <span className="flex items-center gap-1 text-xs font-medium text-amber-600 bg-amber-500/10 px-2 py-1 rounded-full">
      <AlertTriangle className="w-3 h-3" /> Unverified
    </span>
  );
}
