import React, { useState, useEffect, useCallback } from 'react';
import { 
  RefreshCw, Download, AlertCircle, CheckCircle, 
  ArrowUpCircle, Clock, Shield, Settings, RotateCcw,
  ExternalLink, Package, GitBranch
} from 'lucide-react';

const UpdateManager = () => {
  const [status, setStatus] = useState(null);
  const [checking, setChecking] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState(null);
  const [showSettings, setShowSettings] = useState(false);
  const [settings, setSettings] = useState({
    auto_update: false,
    channel: 'lts'
  });

  // Fetch update status
  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/system/updates');
      if (res.ok) {
        const data = await res.json();
        setStatus(data);
        if (data.update_in_progress) {
          setInstalling(true);
          setProgress(data.update_progress);
        }
      }
    } catch (err) {
      console.error('Failed to fetch update status:', err);
    }
  }, []);

  // Check for updates
  const checkForUpdates = async () => {
    setChecking(true);
    setError(null);
    try {
      const res = await fetch('/api/system/updates/check', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        setStatus(data);
      } else {
        const err = await res.json();
        setError(err.error || 'Failed to check for updates');
      }
    } catch (err) {
      setError('Network error while checking for updates');
    } finally {
      setChecking(false);
    }
  };

  // Install update
  const installUpdate = async () => {
    if (!window.confirm('Install update now? The application will restart automatically.')) {
      return;
    }

    setInstalling(true);
    setProgress(0);
    setError(null);

    try {
      const res = await fetch('/api/system/updates/install', { method: 'POST' });
      if (res.ok) {
        // Poll for progress
        const pollProgress = setInterval(async () => {
          try {
            const progressRes = await fetch('/api/system/updates/progress');
            if (progressRes.ok) {
              const data = await progressRes.json();
              setProgress(data.progress);
              if (data.error) {
                setError(data.error);
                setInstalling(false);
                clearInterval(pollProgress);
              }
              if (data.progress >= 100) {
                clearInterval(pollProgress);
                // Application will restart, show message
                setProgress(100);
              }
            }
          } catch {
            // Connection lost - probably restarting
            clearInterval(pollProgress);
            setProgress(100);
          }
        }, 1000);
      } else {
        const err = await res.json();
        setError(err.error || 'Failed to start update');
        setInstalling(false);
      }
    } catch (err) {
      setError('Network error while installing update');
      setInstalling(false);
    }
  };

  // Save settings
  const saveSettings = async () => {
    try {
      const res = await fetch('/api/system/updates/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
      });
      if (res.ok) {
        setShowSettings(false);
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to save settings');
    }
  };

  // Initial fetch and polling
  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 60000); // Check every minute
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Format date
  const formatDate = (dateStr) => {
    if (!dateStr) return 'N/A';
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  };

  // Format size
  const formatSize = (bytes) => {
    if (!bytes) return 'N/A';
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(1)} MB`;
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Package className="w-6 h-6 text-blue-500" />
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            System Updates
          </h2>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowSettings(!showSettings)}
            className="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700"
            title="Settings"
          >
            <Settings className="w-5 h-5" />
          </button>
          <button
            onClick={checkForUpdates}
            disabled={checking || installing}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw className={`w-4 h-4 ${checking ? 'animate-spin' : ''}`} />
            {checking ? 'Checking...' : 'Check for Updates'}
          </button>
        </div>
      </div>

      {/* Error Alert */}
      {error && (
        <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg flex items-center gap-3">
          <AlertCircle className="w-5 h-5 text-red-500" />
          <span className="text-red-700 dark:text-red-300">{error}</span>
          <button 
            onClick={() => setError(null)}
            className="ml-auto text-red-500 hover:text-red-700"
          >
            ×
          </button>
        </div>
      )}

      {/* Settings Panel */}
      {showSettings && (
        <div className="mb-6 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
          <h3 className="font-medium text-gray-900 dark:text-white mb-4">Update Settings</h3>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <label className="text-gray-700 dark:text-gray-300">Auto-update</label>
              <input
                type="checkbox"
                checked={settings.auto_update}
                onChange={(e) => setSettings({...settings, auto_update: e.target.checked})}
                className="w-5 h-5 text-blue-500 rounded"
              />
            </div>
            <div className="flex items-center justify-between">
              <label className="text-gray-700 dark:text-gray-300">Update Channel</label>
              <select
                value={settings.channel}
                onChange={(e) => setSettings({...settings, channel: e.target.value})}
                className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
              >
                <option value="lts">LTS (Recommended)</option>
                <option value="stable">Stable</option>
                <option value="beta">Beta</option>
              </select>
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowSettings(false)}
                className="px-4 py-2 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-600 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={saveSettings}
                className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Version Info */}
      {status && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
          {/* Current Version */}
          <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div className="flex items-center gap-2 mb-2">
              <GitBranch className="w-5 h-5 text-gray-500" />
              <span className="text-sm text-gray-500 dark:text-gray-400">Current Version</span>
            </div>
            <div className="text-2xl font-bold text-gray-900 dark:text-white">
              v{status.current_version}
            </div>
            <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Last checked: {formatDate(status.last_checked)}
            </div>
          </div>

          {/* Latest Version */}
          <div className={`p-4 rounded-lg ${
            status.update_available 
              ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' 
              : 'bg-gray-50 dark:bg-gray-700'
          }`}>
            <div className="flex items-center gap-2 mb-2">
              {status.update_available ? (
                <ArrowUpCircle className="w-5 h-5 text-green-500" />
              ) : (
                <CheckCircle className="w-5 h-5 text-gray-500" />
              )}
              <span className="text-sm text-gray-500 dark:text-gray-400">
                {status.update_available ? 'Update Available' : 'Latest Version'}
              </span>
            </div>
            <div className={`text-2xl font-bold ${
              status.update_available 
                ? 'text-green-600 dark:text-green-400' 
                : 'text-gray-900 dark:text-white'
            }`}>
              v{status.latest_version || status.current_version}
            </div>
            {status.latest_lts_version && (
              <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                <Shield className="w-4 h-4 inline mr-1" />
                LTS: v{status.latest_lts_version}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Update Available Banner */}
      {status?.update_available && !installing && (
        <div className="mb-6 p-6 bg-gradient-to-r from-blue-500 to-indigo-600 rounded-lg text-white">
          <div className="flex items-start justify-between">
            <div>
              <h3 className="text-lg font-semibold mb-2">
                SampMail v{status.latest_version} is available!
              </h3>
              <div className="text-sm opacity-90 mb-4">
                <Clock className="w-4 h-4 inline mr-1" />
                Released: {formatDate(status.release_date)}
                {status.download_size > 0 && (
                  <span className="ml-4">
                    <Download className="w-4 h-4 inline mr-1" />
                    Size: {formatSize(status.download_size)}
                  </span>
                )}
              </div>
              {status.release_notes && (
                <div className="text-sm opacity-90 bg-white/10 rounded p-3 max-h-32 overflow-y-auto">
                  <pre className="whitespace-pre-wrap font-sans">{status.release_notes}</pre>
                </div>
              )}
            </div>
            <button
              onClick={installUpdate}
              className="flex items-center gap-2 px-6 py-3 bg-white text-blue-600 rounded-lg font-semibold hover:bg-blue-50 transition-colors"
            >
              <Download className="w-5 h-5" />
              Update Now
            </button>
          </div>
        </div>
      )}

      {/* Installation Progress */}
      {installing && (
        <div className="mb-6 p-6 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
          <div className="flex items-center gap-3 mb-4">
            <RefreshCw className="w-6 h-6 text-blue-500 animate-spin" />
            <h3 className="text-lg font-semibold text-blue-700 dark:text-blue-300">
              {progress < 100 ? 'Installing Update...' : 'Update Complete!'}
            </h3>
          </div>
          
          {/* Progress Bar */}
          <div className="w-full bg-gray-200 dark:bg-gray-600 rounded-full h-3 mb-2">
            <div 
              className="bg-blue-500 h-3 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          
          <div className="flex justify-between text-sm text-gray-600 dark:text-gray-400">
            <span>
              {progress < 10 && 'Creating backup...'}
              {progress >= 10 && progress < 70 && 'Downloading update...'}
              {progress >= 70 && progress < 95 && 'Installing...'}
              {progress >= 95 && progress < 100 && 'Finalizing...'}
              {progress >= 100 && 'Restarting application...'}
            </span>
            <span>{progress}%</span>
          </div>

          {progress >= 100 && (
            <div className="mt-4 p-3 bg-green-100 dark:bg-green-800/30 rounded text-green-700 dark:text-green-300 text-sm">
              Update installed successfully! The application will restart momentarily.
              <br />
              If the page doesn't refresh automatically, <a href="/" className="underline">click here</a>.
            </div>
          )}
        </div>
      )}

      {/* Up to Date Message */}
      {status && !status.update_available && !installing && (
        <div className="p-6 bg-gray-50 dark:bg-gray-700 rounded-lg text-center">
          <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-3" />
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-1">
            You're up to date!
          </h3>
          <p className="text-gray-600 dark:text-gray-400">
            SampMail v{status.current_version} is the latest version.
          </p>
        </div>
      )}

      {/* Footer Links */}
      <div className="mt-6 pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between text-sm">
        <a 
          href="https://github.com/AuroraOSS/SampMail/releases" 
          target="_blank" 
          rel="noopener noreferrer"
          className="flex items-center gap-1 text-blue-500 hover:text-blue-600"
        >
          <ExternalLink className="w-4 h-4" />
          View All Releases
        </a>
        <span className="text-gray-500 dark:text-gray-400">
          Licensed under AGPL v3
        </span>
      </div>
    </div>
  );
};

export default UpdateManager;
