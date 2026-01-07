import React, { useState, useEffect } from 'react';
import { Server, Play, Square, RotateCcw, Download, CheckCircle, XCircle, Loader2, AlertTriangle, Mail, Inbox, ShieldCheck } from 'lucide-react';
import { getAuthHeaders } from '../api';

const ServiceManagerPage = () => {
    const [services, setServices] = useState([]);
    const [loading, setLoading] = useState(true);
    const [actionInProgress, setActionInProgress] = useState(null);
    const [message, setMessage] = useState(null);

    useEffect(() => {
        fetchServices();
    }, []);

    const fetchServices = async () => {
        try {
            const res = await fetch('/api/services/status', {
                headers: getAuthHeaders(),
            });
            const data = await res.json();
            setServices(Array.isArray(data) ? data : []);
        } catch (error) {
            console.error('Failed to fetch services:', error);
            setMessage({ type: 'error', text: 'Failed to fetch service status' });
        } finally {
            setLoading(false);
        }
    };

    const performAction = async (serviceName, action) => {
        setActionInProgress(`${serviceName}-${action}`);
        setMessage(null);

        try {
            const res = await fetch(`/api/services/${serviceName}/${action}`, {
                method: 'POST',
                headers: getAuthHeaders(),
            });
            const data = await res.json();

            if (res.ok) {
                setMessage({ type: 'success', text: data.message || `${action} completed` });
                // Refresh status after action
                setTimeout(fetchServices, 1000);
            } else {
                setMessage({ type: 'error', text: data.error || 'Action failed' });
            }
        } catch (error) {
            setMessage({ type: 'error', text: error.message });
        } finally {
            setActionInProgress(null);
        }
    };

    const getStatusBadge = (status) => {
        switch (status) {
            case 'running':
                return (
                    <span className="flex items-center gap-1.5 px-2.5 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded-full text-xs font-medium">
                        <CheckCircle className="w-3 h-3" />
                        Running
                    </span>
                );
            case 'stopped':
                return (
                    <span className="flex items-center gap-1.5 px-2.5 py-1 bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 rounded-full text-xs font-medium">
                        <Square className="w-3 h-3" />
                        Stopped
                    </span>
                );
            case 'not_installed':
            default:
                return (
                    <span className="flex items-center gap-1.5 px-2.5 py-1 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded-full text-xs font-medium">
                        <XCircle className="w-3 h-3" />
                        Not Installed
                    </span>
                );
        }
    };

    const getServiceIcon = (name) => {
        switch (name) {
            case 'kumomta':
                return <Mail className="w-8 h-8 text-blue-600" />;
            case 'dovecot':
                return <Inbox className="w-8 h-8 text-purple-600" />;
            case 'reacher':
                return <ShieldCheck className="w-8 h-8 text-green-600" />;
            default:
                return <Server className="w-8 h-8 text-gray-600" />;
        }
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <Loader2 className="w-8 h-8 animate-spin text-blue-600" />
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-3xl font-bold tracking-tight">Service Manager</h1>
                <p className="text-muted-foreground">Install and manage backend services with one click</p>
            </div>

            {message && (
                <div className={`flex items-center gap-2 p-4 rounded-lg ${message.type === 'success'
                    ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 border border-green-200 dark:border-green-800'
                    : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 border border-red-200 dark:border-red-800'
                    }`}>
                    {message.type === 'success' ? <CheckCircle className="w-5 h-5" /> : <AlertTriangle className="w-5 h-5" />}
                    {message.text}
                </div>
            )}

            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                {services.map((service) => (
                    <div
                        key={service.name}
                        className="bg-card border rounded-xl p-6 shadow-sm hover:shadow-md transition-shadow"
                    >
                        <div className="flex items-start justify-between mb-4">
                            <div className="flex items-center gap-3">
                                <div className="p-2 bg-muted rounded-lg">{getServiceIcon(service.name)}</div>
                                <div>
                                    <h3 className="font-semibold text-lg">{service.display_name}</h3>
                                    <p className="text-xs text-muted-foreground">{service.name}</p>
                                </div>
                            </div>
                            {getStatusBadge(service.status)}
                        </div>

                        <p className="text-sm text-muted-foreground mb-6">
                            {service.description}
                        </p>

                        <div className="flex flex-wrap gap-2">
                            {service.status === 'not_installed' ? (
                                <button
                                    onClick={() => performAction(service.name, 'install')}
                                    disabled={actionInProgress !== null}
                                    className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                >
                                    {actionInProgress === `${service.name}-install` ? (
                                        <Loader2 className="w-4 h-4 animate-spin" />
                                    ) : (
                                        <Download className="w-4 h-4" />
                                    )}
                                    Install
                                </button>
                            ) : (
                                <>
                                    {service.status === 'stopped' && (
                                        <button
                                            onClick={() => performAction(service.name, 'start')}
                                            disabled={actionInProgress !== null}
                                            className="flex items-center gap-2 px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                        >
                                            {actionInProgress === `${service.name}-start` ? (
                                                <Loader2 className="w-4 h-4 animate-spin" />
                                            ) : (
                                                <Play className="w-4 h-4" />
                                            )}
                                            Start
                                        </button>
                                    )}
                                    {service.status === 'running' && (
                                        <button
                                            onClick={() => performAction(service.name, 'stop')}
                                            disabled={actionInProgress !== null}
                                            className="flex items-center gap-2 px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                        >
                                            {actionInProgress === `${service.name}-stop` ? (
                                                <Loader2 className="w-4 h-4 animate-spin" />
                                            ) : (
                                                <Square className="w-4 h-4" />
                                            )}
                                            Stop
                                        </button>
                                    )}
                                    <button
                                        onClick={() => performAction(service.name, 'restart')}
                                        disabled={actionInProgress !== null}
                                        className="flex items-center gap-2 px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                    >
                                        {actionInProgress === `${service.name}-restart` ? (
                                            <Loader2 className="w-4 h-4 animate-spin" />
                                        ) : (
                                            <RotateCcw className="w-4 h-4" />
                                        )}
                                        Restart
                                    </button>
                                </>
                            )}
                        </div>
                    </div>
                ))}
            </div>

            <div className="bg-muted/50 border rounded-xl p-6">
                <h3 className="font-semibold mb-3 flex items-center gap-2">
                    <Server className="w-5 h-5" />
                    About These Services
                </h3>
                <div className="grid gap-4 md:grid-cols-3 text-sm">
                    <div>
                        <h4 className="font-medium flex items-center gap-2"><Mail className="w-4 h-4 text-blue-600" /> KumoMTA</h4>
                        <p className="text-muted-foreground">High-performance open-source MTA for sending bulk emails. Required for email delivery.</p>
                    </div>
                    <div>
                        <h4 className="font-medium flex items-center gap-2"><Inbox className="w-4 h-4 text-purple-600" /> Dovecot</h4>
                        <p className="text-muted-foreground">IMAP/POP3 server for receiving bounce notifications and handling return emails.</p>
                    </div>
                    <div>
                        <h4 className="font-medium flex items-center gap-2"><ShieldCheck className="w-4 h-4 text-green-600" /> Reacher</h4>
                        <p className="text-muted-foreground">Email verification service to check if addresses are valid before sending.</p>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default ServiceManagerPage;
