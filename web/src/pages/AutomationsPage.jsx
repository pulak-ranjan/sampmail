import React, { useState, useEffect, useCallback } from 'react';
import {
    Zap, Plus, Play, Pause, Trash2, Edit2, MoreVertical,
    Users, Clock, Mail, Tag, Link2, FileText, Globe,
    ChevronRight, Search, Filter, X, CheckCircle, AlertCircle,
    Loader2, ArrowLeft, Copy
} from 'lucide-react';
import { cn } from '../lib/utils';
import { apiRequest } from '../api';
import AutomationBuilder from '../components/AutomationBuilder';

const triggerIcons = {
    trigger_contact_added: Users,
    trigger_tag_added: Tag,
    trigger_email_opened: Mail,
    trigger_link_clicked: Link2,
    trigger_form_submitted: FileText,
    trigger_webhook: Globe,
};

const triggerLabels = {
    trigger_contact_added: 'Contact Added',
    trigger_tag_added: 'Tag Added',
    trigger_email_opened: 'Email Opened',
    trigger_link_clicked: 'Link Clicked',
    trigger_form_submitted: 'Form Submitted',
    trigger_webhook: 'Webhook',
};

export default function AutomationsPage() {
    const [automations, setAutomations] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [showCreate, setShowCreate] = useState(false);
    const [selectedAutomation, setSelectedAutomation] = useState(null);
    const [searchTerm, setSearchTerm] = useState('');
    const [filterStatus, setFilterStatus] = useState('all');
    const [actionMenuId, setActionMenuId] = useState(null);

    useEffect(() => {
        fetchAutomations();
    }, [fetchAutomations]);

    const fetchAutomations = useCallback(async () => {
        try {
            setLoading(true);
            const data = await apiRequest('/v2/automations');
            setAutomations(data || []);
        } catch (err) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    }, []);

    const createAutomation = async (formData) => {
        try {
            const automation = await apiRequest('/v2/automations', {
                method: 'POST',
                body: {
                    ...formData,
                    nodes: '[]',
                    edges: '[]',
                    viewport: '{}',
                },
            });
            setAutomations([automation, ...automations]);
            setShowCreate(false);
            setSelectedAutomation(automation);
        } catch (err) {
            setError(err.message);
        }
    };

    const toggleAutomation = async (id, activate) => {
        try {
            const updated = await apiRequest(`/v2/automations/${id}/${activate ? 'activate' : 'pause'}`, {
                method: 'POST',
            });
            setAutomations((prev) => prev.map((a) => (a.id === id ? updated : a)));
            if (selectedAutomation?.id === id) {
                setSelectedAutomation(updated);
            }
        } catch (err) {
            setError(err.message);
        }
    };

    const deleteAutomation = async (id) => {
        if (!confirm('Are you sure you want to delete this automation?')) return;
        try {
            await apiRequest(`/v2/automations/${id}`, { method: 'DELETE' });
            setAutomations(automations.filter(a => a.id !== id));
            setActionMenuId(null);
        } catch (err) {
            setError(err.message);
        }
    };

    const duplicateAutomation = async (automation) => {
        try {
            const newAuto = await apiRequest('/v2/automations', {
                method: 'POST',
                body: {
                    name: `${automation.name} (Copy)`,
                    description: automation.description,
                    trigger_type: automation.trigger_type,
                    nodes: automation.nodes || '[]',
                    edges: automation.edges || '[]',
                    viewport: automation.viewport || '{}',
                    trigger_config: automation.trigger_config || {},
                    entry_filters: automation.entry_filters || {},
                    exit_conditions: automation.exit_conditions || {},
                    allow_reentry: !!automation.allow_reentry,
                    reentry_delay: automation.reentry_delay || 0,
                    goal_tracking: !!automation.goal_tracking,
                    goal_config: automation.goal_config || {},
                },
            });
            setAutomations([newAuto, ...automations]);
            setActionMenuId(null);
        } catch (err) {
            setError(err.message);
        }
    };

    const filteredAutomations = automations.filter(a => {
        const matchesSearch = a.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
            a.description?.toLowerCase().includes(searchTerm.toLowerCase());
        const matchesFilter = filterStatus === 'all' || a.status === filterStatus;
        return matchesSearch && matchesFilter;
    });

    // Detail View
    if (selectedAutomation) {
        return (
            <AutomationDetail
                automation={selectedAutomation}
                onBack={() => setSelectedAutomation(null)}
                onSave={async (data) => {
                    const updated = await apiRequest(`/v2/automations/${selectedAutomation.id}`, {
                        method: 'PUT',
                        body: data,
                    });
                    setSelectedAutomation(updated);
                    setAutomations((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
                }}
                onToggle={(activate) => toggleAutomation(selectedAutomation.id, activate)}
            />
        );
    }

    return (
        <div className="space-y-6 pb-20 md:pb-6">
            {/* Header */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                    <h1 className="text-2xl md:text-3xl font-bold text-foreground">Automations</h1>
                    <p className="text-muted-foreground text-sm md:text-base">Create automated email workflows</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="flex items-center justify-center gap-2 px-4 py-2.5 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors"
                >
                    <Plus className="w-5 h-5" />
                    <span>New Automation</span>
                </button>
            </div>

            {/* Error Banner */}
            {error && (
                <div className="p-4 bg-red-500/10 text-red-400 rounded-lg border border-red-500/20 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <AlertCircle className="w-5 h-5" />
                        {error}
                    </div>
                    <button onClick={() => setError(null)}><X className="w-4 h-4" /></button>
                </div>
            )}

            {/* Search & Filter */}
            <div className="flex flex-col sm:flex-row gap-3">
                <div className="relative flex-1">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <input
                        type="text"
                        placeholder="Search automations..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="w-full pl-10 pr-4 py-2.5 bg-card border border-border rounded-lg text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                    />
                </div>
                <div className="flex gap-2">
                    {['all', 'active', 'paused', 'draft'].map(status => (
                        <button
                            key={status}
                            onClick={() => setFilterStatus(status)}
                            className={cn(
                                "px-3 py-2 rounded-lg text-sm font-medium transition-colors capitalize",
                                filterStatus === status
                                    ? "bg-primary text-primary-foreground"
                                    : "bg-card border border-border text-muted-foreground hover:text-foreground"
                            )}
                        >
                            {status}
                        </button>
                    ))}
                </div>
            </div>

            {/* Loading */}
            {loading && (
                <div className="flex items-center justify-center py-20">
                    <Loader2 className="w-8 h-8 animate-spin text-primary" />
                </div>
            )}

            {/* Empty State */}
            {!loading && automations.length === 0 && (
                <div className="bg-card border border-border rounded-xl p-8 md:p-12 text-center">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-primary/10 flex items-center justify-center">
                        <Zap className="w-8 h-8 text-primary" />
                    </div>
                    <h3 className="text-xl font-semibold mb-2">No automations yet</h3>
                    <p className="text-muted-foreground mb-6 max-w-md mx-auto">
                        Automations help you send the right email at the right time, automatically.
                    </p>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-6 py-3 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors"
                    >
                        <Plus className="w-5 h-5 inline mr-2" />
                        Create Your First Automation
                    </button>
                </div>
            )}

            {/* Automations List - Card Grid */}
            {!loading && filteredAutomations.length > 0 && (
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                    {filteredAutomations.map((automation) => {
                        const TriggerIcon = triggerIcons[automation.trigger_type] || Zap;
                        const isActive = automation.status === 'active';
                        let nodeCount = 0;
                        try {
                            nodeCount = automation.nodes ? JSON.parse(automation.nodes).length : 0;
                        } catch {
                            nodeCount = 0;
                        }

                        return (
                            <div
                                key={automation.id}
                                className={cn(
                                    "bg-card border rounded-xl p-4 md:p-5 relative group cursor-pointer transition-all hover:border-primary/50",
                                    isActive ? "border-green-500/30" : "border-border"
                                )}
                                onClick={() => setSelectedAutomation(automation)}
                            >
                                {/* Status Indicator */}
                                <div className={cn(
                                    "absolute top-4 right-4 w-2.5 h-2.5 rounded-full",
                                    isActive ? "bg-green-500" : automation.status === 'paused' ? "bg-yellow-500" : "bg-muted"
                                )} />

                                {/* Icon & Name */}
                                <div className="flex items-start gap-3 mb-3">
                                    <div className={cn(
                                        "w-10 h-10 rounded-lg flex items-center justify-center shrink-0",
                                        isActive ? "bg-green-500/10" : "bg-muted"
                                    )}>
                                        <TriggerIcon className={cn(
                                            "w-5 h-5",
                                            isActive ? "text-green-500" : "text-muted-foreground"
                                        )} />
                                    </div>
                                    <div className="min-w-0 flex-1">
                                        <h3 className="font-semibold text-foreground truncate pr-6">{automation.name}</h3>
                                        <p className="text-sm text-muted-foreground truncate">
                                            {automation.description || triggerLabels[automation.trigger_type] || 'No trigger'}
                                        </p>
                                    </div>
                                </div>

                                {/* Stats */}
                                <div className="flex items-center gap-4 text-sm text-muted-foreground mb-4">
                                    <span className="flex items-center gap-1">
                                        <Users className="w-4 h-4" />
                                        {automation.total_entered ?? automation.entry_count ?? 0}
                                    </span>
                                    <span className="flex items-center gap-1">
                                        <Mail className="w-4 h-4" />
                                        {nodeCount} nodes
                                    </span>
                                </div>

                                {/* Actions */}
                                <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                                    <button
                                        onClick={() => toggleAutomation(automation.id, !isActive)}
                                        className={cn(
                                            "flex-1 py-2 rounded-lg text-sm font-medium transition-colors flex items-center justify-center gap-1.5",
                                            isActive
                                                ? "bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20"
                                                : "bg-green-500/10 text-green-500 hover:bg-green-500/20"
                                        )}
                                    >
                                        {isActive ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                                        {isActive ? 'Pause' : 'Activate'}
                                    </button>
                                    <button
                                        onClick={() => setSelectedAutomation(automation)}
                                        className="p-2 rounded-lg bg-muted hover:bg-muted/80 transition-colors"
                                    >
                                        <Edit2 className="w-4 h-4" />
                                    </button>
                                    <div className="relative">
                                        <button
                                            onClick={() => setActionMenuId(actionMenuId === automation.id ? null : automation.id)}
                                            className="p-2 rounded-lg bg-muted hover:bg-muted/80 transition-colors"
                                        >
                                            <MoreVertical className="w-4 h-4" />
                                        </button>
                                        {actionMenuId === automation.id && (
                                            <div className="absolute right-0 top-full mt-1 w-40 bg-card border border-border rounded-lg shadow-lg z-10 py-1">
                                                <button
                                                    onClick={() => duplicateAutomation(automation)}
                                                    className="w-full px-4 py-2 text-left text-sm hover:bg-muted flex items-center gap-2"
                                                >
                                                    <Copy className="w-4 h-4" /> Duplicate
                                                </button>
                                                <button
                                                    onClick={() => deleteAutomation(automation.id)}
                                                    className="w-full px-4 py-2 text-left text-sm text-red-500 hover:bg-red-500/10 flex items-center gap-2"
                                                >
                                                    <Trash2 className="w-4 h-4" /> Delete
                                                </button>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}

            {/* Create Modal */}
            {showCreate && (
                <CreateAutomationModal
                    onClose={() => setShowCreate(false)}
                    onCreate={createAutomation}
                />
            )}

            {/* Click outside to close menu */}
            {actionMenuId && (
                <div className="fixed inset-0 z-0" onClick={() => setActionMenuId(null)} />
            )}
        </div>
    );
}

// Create Automation Modal
function CreateAutomationModal({ onClose, onCreate }) {
    const [form, setForm] = useState({
        name: '',
        description: '',
        trigger_type: 'trigger_contact_added',
    });
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        await onCreate(form);
        setLoading(false);
    };

    const triggers = [
        { value: 'trigger_contact_added', label: 'Contact Added to List', icon: Users, desc: 'When a contact joins a list' },
        { value: 'trigger_tag_added', label: 'Tag Added', icon: Tag, desc: 'When a tag is applied' },
        { value: 'trigger_email_opened', label: 'Email Opened', icon: Mail, desc: 'When an email is opened' },
        { value: 'trigger_link_clicked', label: 'Link Clicked', icon: Link2, desc: 'When a link is clicked' },
        { value: 'trigger_form_submitted', label: 'Form Submitted', icon: FileText, desc: 'When a form is submitted' },
        { value: 'trigger_webhook', label: 'Webhook', icon: Globe, desc: 'Triggered by external webhook' },
    ];

    return (
        <div className="fixed inset-0 bg-black/60 flex items-end sm:items-center justify-center z-50 p-0 sm:p-4">
            <div className="bg-card rounded-t-2xl sm:rounded-xl w-full sm:max-w-lg max-h-[90vh] overflow-y-auto">
                {/* Header */}
                <div className="sticky top-0 bg-card border-b border-border px-4 sm:px-6 py-4 flex items-center justify-between">
                    <h2 className="text-lg font-semibold">Create Automation</h2>
                    <button onClick={onClose} className="p-2 rounded-lg hover:bg-muted">
                        <X className="w-5 h-5" />
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="p-4 sm:p-6 space-y-5">
                    {/* Name */}
                    <div>
                        <label className="block text-sm font-medium mb-1.5">Automation Name</label>
                        <input
                            type="text"
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            className="w-full h-11 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                            placeholder="e.g., Welcome Series"
                            required
                        />
                    </div>

                    {/* Description */}
                    <div>
                        <label className="block text-sm font-medium mb-1.5">Description (optional)</label>
                        <textarea
                            value={form.description}
                            onChange={(e) => setForm({ ...form, description: e.target.value })}
                            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary focus:outline-none resize-none"
                            rows={2}
                            placeholder="What does this automation do?"
                        />
                    </div>

                    {/* Trigger Selection */}
                    <div>
                        <label className="block text-sm font-medium mb-2">Start Trigger</label>
                        <div className="grid gap-2">
                            {triggers.map((trigger) => {
                                const Icon = trigger.icon;
                                const isSelected = form.trigger_type === trigger.value;
                                return (
                                    <button
                                        key={trigger.value}
                                        type="button"
                                        onClick={() => setForm({ ...form, trigger_type: trigger.value })}
                                        className={cn(
                                            "flex items-center gap-3 p-3 rounded-lg border text-left transition-all",
                                            isSelected
                                                ? "border-primary bg-primary/5"
                                                : "border-border hover:border-muted-foreground/50"
                                        )}
                                    >
                                        <div className={cn(
                                            "w-10 h-10 rounded-lg flex items-center justify-center shrink-0",
                                            isSelected ? "bg-primary/10" : "bg-muted"
                                        )}>
                                            <Icon className={cn("w-5 h-5", isSelected ? "text-primary" : "text-muted-foreground")} />
                                        </div>
                                        <div>
                                            <div className={cn("font-medium text-sm", isSelected && "text-primary")}>{trigger.label}</div>
                                            <div className="text-xs text-muted-foreground">{trigger.desc}</div>
                                        </div>
                                        {isSelected && <CheckCircle className="w-5 h-5 text-primary ml-auto" />}
                                    </button>
                                );
                            })}
                        </div>
                    </div>

                    {/* Actions */}
                    <div className="flex gap-3 pt-2">
                        <button
                            type="button"
                            onClick={onClose}
                            className="flex-1 py-3 rounded-lg border border-border text-sm font-medium hover:bg-muted transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={!form.name || loading}
                            className="flex-1 py-3 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
                        >
                            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                            Create
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

function AutomationDetail({ automation, onBack, onSave, onToggle }) {
    const [name, setName] = useState(automation.name || '');
    const [description, setDescription] = useState(automation.description || '');

    const TriggerIcon = triggerIcons[automation.trigger_type] || Zap;

    return (
        <div className="space-y-6 pb-20 md:pb-6">
            <button
                onClick={onBack}
                className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors w-fit"
            >
                <ArrowLeft className="w-5 h-5" />
                <span>Back</span>
            </button>

            <div className="grid lg:grid-cols-3 gap-6">
                <div className="lg:col-span-1 space-y-4">
                    <div className="bg-card border border-border rounded-xl p-5 space-y-4">
                        <div>
                            <label className="block text-sm font-medium mb-1.5">Name</label>
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1.5">Description</label>
                            <textarea
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary focus:outline-none resize-none"
                                rows={3}
                            />
                        </div>
                    </div>

                    <div className="bg-card border border-border rounded-xl p-5">
                        <h3 className="font-semibold mb-4">Trigger</h3>
                        <div className="flex items-center gap-3 p-3 bg-muted rounded-lg">
                            <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
                                <TriggerIcon className="w-5 h-5 text-primary" />
                            </div>
                            <div>
                                <div className="font-medium text-sm">{triggerLabels[automation.trigger_type]}</div>
                                <div className="text-xs text-muted-foreground">Starts the workflow</div>
                            </div>
                        </div>
                    </div>

                    <div className="bg-card border border-border rounded-xl p-5">
                        <h3 className="font-semibold mb-4">Statistics</h3>
                        <div className="grid grid-cols-2 gap-4">
                            <div className="text-center p-3 bg-muted rounded-lg">
                                <div className="text-2xl font-bold">{automation.total_entered || 0}</div>
                                <div className="text-xs text-muted-foreground">Entered</div>
                            </div>
                            <div className="text-center p-3 bg-muted rounded-lg">
                                <div className="text-2xl font-bold">{automation.total_completed || 0}</div>
                                <div className="text-xs text-muted-foreground">Completed</div>
                            </div>
                        </div>
                    </div>
                </div>

                <div className="lg:col-span-2 min-h-[60vh] border border-border rounded-xl overflow-hidden">
                    <AutomationBuilder
                        automation={{ ...automation, name, description }}
                        onSave={async (payload) => onSave({ name, description, ...payload })}
                        onActivate={() => onToggle(true)}
                        onPause={() => onToggle(false)}
                    />
                </div>
            </div>
        </div>
    );
}
