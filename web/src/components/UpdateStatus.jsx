import React, { useState, useEffect, useCallback } from 'react';
import { 
  Download, 
  RefreshCw, 
  CheckCircle, 
  AlertTriangle, 
  Info,
  ArrowUp,
  Clock,
  Shield,
  RotateCcw,
  ExternalLink
} from 'lucide-react';

// Update Status Component for Dashboard
export function UpdateStatus() {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState(null);

  const fetchStatus = useCallback(async () => {
    try {
      const resp = await fetch('/api/updates/status');
      if (resp.ok) {
        const data = await resp.json();
        setStatus(data);
        setError(null);
      }
    } catch (err) {
      setError('Failed to fetch update status');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
    // Refresh every 5 minutes
    const interval = setInterval(fetchStatus, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const checkForUpdates = async () => {
    setChecking(true);
    try {
      const resp = await fetch('/api/updates/check', { method: 'POST' });
      const data = await resp.json();
      if (data.success) {
        setStatus(data.status);
      } else {
        setError(data.error);
      }
    } catch (err) {
      setError('Failed to check for updates');
    } finally {
      setChecking(false);
    }
  };

  const downloadUpdate = async () => {
    setDownloading(true);
    try {
      const resp = await fetch('/api/updates/download', { method: 'POST' });
      if (resp.ok) {
        // Poll for download completion
        const pollInterval = setInterval(async () => {
          const statusResp = await fetch('/api/updates/status');
          const statusData = await statusResp.json();
          setStatus(statusData);
          if (statusData.download_ready || statusData.error) {
            clearInterval(pollInterval);
            setDownloading(false);
          }
        }, 2000);
      }
    } catch (err) {
      setError('Failed to start download');
      setDownloading(false);
    }
  };

  const applyUpdate = async () => {
    if (!confirm('This will restart the server. Continue?')) return;
    
    setApplying(true);
    try {
      const resp = await fetch('/api/updates/apply', { 
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ restart: true })
      });
      const data = await resp.json();
      if (data.success) {
        // Server will restart, show message
        alert('Update applied! Server is restarting...');
        setTimeout(() => window.location.reload(), 5000);
      } else {
        setError(data.error);
      }
    } catch (err) {
      setError('Failed to apply update');
    } finally {
      setApplying(false);
    }
  };

  const rollback = async () => {
    if (!confirm('Rollback to previous version? This will restart the server.')) return;
    
    try {
      const resp = await fetch('/api/updates/rollback', { method: 'POST' });
      const data = await resp.json();
      if (data.success) {
        alert('Rolled back! Server is restarting...');
        setTimeout(() => window.location.reload(), 5000);
      } else {
        setError(data.error);
      }
    } catch (err) {
      setError('Failed to rollback');
    }
  };

  if (loading) {
    return (
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
        <div className="animate-pulse flex items-center gap-2">
          <div className="w-8 h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
          <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
        </div>
      </div>
    );
  }

  if (!status) return null;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ArrowUp className="w-5 h-5 text-blue-500" />
          <h3 className="font-medium text-gray-900 dark:text-white">System Updates</h3>
        </div>
        <button
          onClick={checkForUpdates}
          disabled={checking}
          className="p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50"
          title="Check for updates"
        >
          <RefreshCw className={`w-4 h-4 ${checking ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Content */}
      <div className="p-4">
        {/* Current Version */}
        <div className="flex items-center justify-between mb-4">
          <div>
            <p className="text-sm text-gray-500 dark:text-gray-400">Current Version</p>
            <p className="text-lg font-semibold text-gray-900 dark:text-white">
              v{status.current_version}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span className="px-2 py-1 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 rounded">
              stable
            </span>
          </div>
        </div>

        {/* Last Checked */}
        {status.last_checked && (
          <div className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 mb-4">
            <Clock className="w-3 h-3" />
            Last checked: {new Date(status.last_checked).toLocaleString()}
          </div>
        )}

        {/* Error */}
        {error && (
          <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
            <div className="flex items-center gap-2 text-red-700 dark:text-red-400">
              <AlertTriangle className="w-4 h-4" />
              <span className="text-sm">{error}</span>
            </div>
          </div>
        )}

        {/* Update Available */}
        {status.available && status.release_info && (
          <div className="mb-4 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0">
                <Info className="w-5 h-5 text-blue-500" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-blue-900 dark:text-blue-100">
                  Update Available: v{status.latest_version}
                </h4>
                {status.release_info.is_lts && (
                  <span className="inline-flex items-center gap-1 mt-1 px-2 py-0.5 text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200 rounded">
                    <Shield className="w-3 h-3" />
                    LTS Release
                  </span>
                )}
                {status.release_info.release_notes && (
                  <p className="mt-2 text-sm text-blue-800 dark:text-blue-200 line-clamp-3">
                    {status.release_info.release_notes.substring(0, 200)}...
                  </p>
                )}
                <div className="mt-3 flex flex-wrap gap-2">
                  {!status.download_ready ? (
                    <button
                      onClick={downloadUpdate}
                      disabled={downloading}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50"
                    >
                      {downloading ? (
                        <>
                          <RefreshCw className="w-4 h-4 animate-spin" />
                          Downloading {status.download_progress}%
                        </>
                      ) : (
                        <>
                          <Download className="w-4 h-4" />
                          Download Update
                        </>
                      )}
                    </button>
                  ) : (
                    <button
                      onClick={applyUpdate}
                      disabled={applying}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-green-600 hover:bg-green-700 rounded-lg disabled:opacity-50"
                    >
                      {applying ? (
                        <>
                          <RefreshCw className="w-4 h-4 animate-spin" />
                          Installing...
                        </>
                      ) : (
                        <>
                          <CheckCircle className="w-4 h-4" />
                          Install & Restart
                        </>
                      )}
                    </button>
                  )}
                  <a
                    href={`https://github.com/pulak-ranjan/sampmail/releases/tag/v${status.latest_version}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg"
                  >
                    <ExternalLink className="w-4 h-4" />
                    View Release Notes
                  </a>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Up to Date */}
        {!status.available && !error && (
          <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
            <div className="flex items-center gap-2 text-green-700 dark:text-green-400">
              <CheckCircle className="w-5 h-5" />
              <span className="font-medium">Your system is up to date</span>
            </div>
          </div>
        )}

        {/* LTS Info */}
        {status.latest_lts && status.latest_lts !== status.current_version && (
          <div className="mt-4 p-3 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-lg">
            <div className="flex items-center gap-2 text-purple-700 dark:text-purple-400">
              <Shield className="w-4 h-4" />
              <span className="text-sm">
                Latest LTS version: <strong>v{status.latest_lts}</strong>
              </span>
            </div>
          </div>
        )}

        {/* Rollback Option */}
        <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={rollback}
            className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            <RotateCcw className="w-4 h-4" />
            Rollback to previous version
          </button>
        </div>
      </div>
    </div>
  );
}

// Update Banner for top of dashboard
export function UpdateBanner() {
  const [status, setStatus] = useState(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const resp = await fetch('/api/updates/status');
        if (resp.ok) {
          const data = await resp.json();
          setStatus(data);
        }
      } catch (err) {
        // Ignore errors for banner
      }
    };

    fetchStatus();
    const interval = setInterval(fetchStatus, 30 * 60 * 1000); // 30 min
    return () => clearInterval(interval);
  }, []);

  if (!status?.available || dismissed) return null;

  const isCritical = status.release_info?.release_notes?.toLowerCase().includes('security');

  return (
    <div className={`${isCritical ? 'bg-red-600' : 'bg-blue-600'} text-white px-4 py-2`}>
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        <div className="flex items-center gap-2">
          {isCritical ? (
            <AlertTriangle className="w-5 h-5" />
          ) : (
            <Info className="w-5 h-5" />
          )}
          <span className="text-sm font-medium">
            {isCritical ? 'Security update available' : 'New version available'}: v{status.latest_version}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <a
            href="/settings#updates"
            className="text-sm font-medium underline hover:no-underline"
          >
            View Details
          </a>
          <button
            onClick={() => setDismissed(true)}
            className="text-white/80 hover:text-white"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  );
}

// Version Info Footer
export function VersionFooter() {
  const [version, setVersion] = useState(null);

  useEffect(() => {
    fetch('/api/version')
      .then(r => r.json())
      .then(setVersion)
      .catch(() => {});
  }, []);

  if (!version) return null;

  return (
    <div className="text-xs text-gray-500 dark:text-gray-400 text-center py-4">
      SampMail v{version.version} • {version.go_version} • {version.os}/{version.arch}
      {version.uptime && <span className="ml-2">• Uptime: {version.uptime}</span>}
    </div>
  );
}

export default UpdateStatus;
