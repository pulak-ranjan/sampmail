import React, { useState, useEffect } from 'react';
import { Shield, Lock, Globe, RefreshCw, CheckCircle, AlertCircle, ExternalLink } from 'lucide-react';

const SSLPage = () => {
    const [sslStatus, setSslStatus] = useState(null);
    const [loading, setLoading] = useState(true);
    const [domain, setDomain] = useState('');
    const [email, setEmail] = useState('');
    const [installing, setInstalling] = useState(false);
    const [message, setMessage] = useState(null);

    useEffect(() => {
        fetchSSLStatus();
    }, []);

    const fetchSSLStatus = async () => {
        try {
            const res = await fetch('/api/system/ssl', {
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            if (res.ok) {
                const data = await res.json();
                setSslStatus(data);
                if (data.domain) setDomain(data.domain);
            }
        } catch (error) {
            console.error('Failed to fetch SSL status:', error);
        } finally {
            setLoading(false);
        }
    };

    const installSSL = async () => {
        if (!domain || !email) {
            setMessage({ type: 'error', text: 'Domain and email are required' });
            return;
        }

        setInstalling(true);
        setMessage(null);

        try {
            const res = await fetch('/api/system/ssl/install', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                },
                body: JSON.stringify({ domain, email }),
            });

            const data = await res.json();

            if (res.ok) {
                setMessage({ type: 'success', text: 'SSL certificate installed successfully!' });
                fetchSSLStatus();
            } else {
                setMessage({ type: 'error', text: data.error || 'Failed to install SSL' });
            }
        } catch (error) {
            setMessage({ type: 'error', text: 'Failed to install SSL certificate' });
        } finally {
            setInstalling(false);
        }
    };

    const renewSSL = async () => {
        setInstalling(true);
        try {
            const res = await fetch('/api/system/ssl/renew', {
                method: 'POST',
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });

            const data = await res.json();

            if (res.ok) {
                setMessage({ type: 'success', text: 'SSL certificate renewed successfully!' });
                fetchSSLStatus();
            } else {
                setMessage({ type: 'error', text: data.error || 'Failed to renew SSL' });
            }
        } catch (error) {
            setMessage({ type: 'error', text: 'Failed to renew SSL certificate' });
        } finally {
            setInstalling(false);
        }
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    const hasSSL = sslStatus?.enabled;
    const daysUntilExpiry = sslStatus?.days_until_expiry;

    return (
        <div className="p-6 max-w-4xl mx-auto">
            <div className="mb-6">
                <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
                    <Shield className="w-7 h-7" />
                    SSL / HTTPS Settings
                </h1>
                <p className="text-gray-500 mt-1">Secure your SampMail panel with Let's Encrypt SSL</p>
            </div>

            {message && (
                <div className={`mb-6 p-4 rounded-lg flex items-center gap-2 ${message.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                    }`}>
                    {message.type === 'success' ? <CheckCircle className="w-5 h-5" /> : <AlertCircle className="w-5 h-5" />}
                    {message.text}
                </div>
            )}

            {/* Current Status */}
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 shadow mb-6">
                <h2 className="text-lg font-semibold mb-4 dark:text-white flex items-center gap-2">
                    <Lock className="w-5 h-5" />
                    Current Status
                </h2>

                <div className="flex items-center gap-4">
                    <div className={`w-16 h-16 rounded-full flex items-center justify-center ${hasSSL ? 'bg-green-100 text-green-600' : 'bg-yellow-100 text-yellow-600'
                        }`}>
                        {hasSSL ? <Lock className="w-8 h-8" /> : <AlertCircle className="w-8 h-8" />}
                    </div>
                    <div>
                        <div className="flex items-center gap-2">
                            <span className={`text-lg font-semibold ${hasSSL ? 'text-green-600' : 'text-yellow-600'}`}>
                                {hasSSL ? 'HTTPS Enabled' : 'HTTP Only'}
                            </span>
                            {hasSSL && (
                                <span className="px-2 py-0.5 bg-green-100 text-green-600 text-xs rounded-full">
                                    Secure
                                </span>
                            )}
                        </div>
                        {hasSSL ? (
                            <div className="text-sm text-gray-500 mt-1">
                                <p>Domain: <strong>{sslStatus.domain}</strong></p>
                                <p>Expires: {sslStatus.expiry_date} ({daysUntilExpiry} days remaining)</p>
                                <p>Issuer: {sslStatus.issuer || "Let's Encrypt"}</p>
                            </div>
                        ) : (
                            <p className="text-sm text-gray-500 mt-1">
                                Your panel is not secured with HTTPS. Install an SSL certificate below.
                            </p>
                        )}
                    </div>
                </div>

                {hasSSL && daysUntilExpiry < 30 && (
                    <div className="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg flex items-start gap-2">
                        <AlertTriangle className="w-5 h-5 text-yellow-600 shrink-0" />
                        <p className="text-yellow-700 dark:text-yellow-400 text-sm">
                            Your certificate expires in {daysUntilExpiry} days. Consider renewing it soon.
                        </p>
                    </div>
                )}

                {hasSSL && (
                    <button
                        onClick={renewSSL}
                        disabled={installing}
                        className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
                    >
                        <RefreshCw className={`w-4 h-4 ${installing ? 'animate-spin' : ''}`} />
                        {installing ? 'Renewing...' : 'Renew Certificate'}
                    </button>
                )}
            </div>

            {/* Install SSL */}
            {!hasSSL && (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-6 shadow">
                    <h2 className="text-lg font-semibold mb-4 dark:text-white flex items-center gap-2">
                        <Globe className="w-5 h-5" />
                        Install SSL Certificate
                    </h2>

                    <p className="text-gray-500 text-sm mb-4">
                        Get a free SSL certificate from Let's Encrypt. Make sure your domain points to this server before installing.
                    </p>

                    <div className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Domain Name
                            </label>
                            <input
                                type="text"
                                value={domain}
                                onChange={(e) => setDomain(e.target.value)}
                                className="w-full border dark:border-gray-600 rounded-lg p-3 dark:bg-gray-700 dark:text-white"
                                placeholder="mail.yourdomain.com"
                            />
                            <p className="text-xs text-gray-400 mt-1">
                                This domain must point to this server's IP address
                            </p>
                        </div>

                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Email Address
                            </label>
                            <input
                                type="email"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                className="w-full border dark:border-gray-600 rounded-lg p-3 dark:bg-gray-700 dark:text-white"
                                placeholder="admin@yourdomain.com"
                            />
                            <p className="text-xs text-gray-400 mt-1">
                                Let's Encrypt will send expiry notifications to this email
                            </p>
                        </div>

                        <button
                            onClick={installSSL}
                            disabled={installing || !domain || !email}
                            className="w-full px-6 py-3 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 font-medium flex items-center justify-center gap-2"
                        >
                            {installing ? (
                                <>
                                    <RefreshCw className="w-5 h-5 animate-spin" />
                                    Installing SSL...
                                </>
                            ) : (
                                <>
                                    <Lock className="w-5 h-5" />
                                    Install SSL Certificate
                                </>
                            )}
                        </button>
                    </div>

                    <div className="mt-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                        <h3 className="font-medium text-blue-700 dark:text-blue-400 mb-2">Prerequisites</h3>
                        <ul className="text-sm text-blue-600 dark:text-blue-300 space-y-2">
                            <li className="flex items-center gap-2"><Check className="w-4 h-4" /> Domain must point to this server (A record)</li>
                            <li className="flex items-center gap-2"><Check className="w-4 h-4" /> Port 80 must be open and accessible</li>
                            <li className="flex items-center gap-2"><Check className="w-4 h-4" /> Certbot must be installed on the server</li>
                        </ul>
                    </div>
                </div>
            )}

            {/* Manual Instructions */}
            <div className="mt-6 bg-gray-50 dark:bg-gray-800/50 rounded-xl p-6">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-2">Manual SSL Setup</h3>
                <p className="text-sm text-gray-500 mb-3">
                    If automatic installation fails, you can run these commands on your server:
                </p>
                <pre className="bg-gray-900 text-green-400 p-4 rounded-lg text-sm overflow-x-auto">
                    {`# Install SSL for your domain
sudo certbot --nginx -d yourdomain.com

# Auto-renew (already configured via cron)
sudo certbot renew --dry-run`}
                </pre>
                <a
                    href="https://certbot.eff.org/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-3 inline-flex items-center gap-1 text-blue-600 hover:underline text-sm"
                >
                    Learn more about Certbot <ExternalLink className="w-3 h-3" />
                </a>
            </div>
        </div>
    );
};

export default SSLPage;
