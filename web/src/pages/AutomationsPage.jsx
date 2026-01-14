import React, { useState, useEffect } from 'react';
import {
    Zap, Plus, Play, Pause, Trash2, Edit2, MoreVertical,
    Users, Clock, Mail, Tag, Link2, FileText, Globe,
    ChevronRight, Search, Filter, X, CheckCircle, AlertCircle,
    Loader2, ArrowLeft, Settings, Copy
} from 'lucide-react';
import { cn } from '../lib/utils';

const API_BASE = "/api";

async function apiRequest(endpoint, options = {}) {
    const token = localStorage.getItem("sampmail_token");
    const res = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            Authorization: token ? `Bearer ${token}` : "",
            ...options.headers,
        },
        body: options.body ? JSON.stringify(options.body) : undefined,
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || "Request failed");
    }
    return res.json();
}

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
    }, []);

    const fetchAutomations = async () => {
        try {
            setLoading(true);
            const data = await apiRequest('/v2/automations');
            setAutomations(data || []);
        } catch (err) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    };

    const createAutomation = async (formData) => {
        try {
            const automation = await apiRequest('/v2/automations', {
                method: 'POST',
                body: formData,
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
            await apiRequest(`/v2/automations/${id}/${activate ? 'activate' : 'pause'}`, {
                method: 'POST',
            });
            setAutomations(automations.map(a =>
                a.id === id ? { ...a, status: activate ? 'active' : 'paused' } : a
            ));
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
                    steps: automation.steps,
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
                    await apiRequest(`/v2/automations/${selectedAutomation.id}`, {
                        method: 'PUT',
                        body: data,
                    });
                    fetchAutomations();
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
                                        {automation.entry_count || 0}
                                    </span>
                                    <span className="flex items-center gap-1">
                                        <Mail className="w-4 h-4" />
                                        {automation.steps?.length || 0} steps
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

// Automation Detail View with working step selector
function AutomationDetail({ automation, onBack, onSave, onToggle }) {
    const [name, setName] = useState(automation.name);
    const [description, setDescription] = useState(automation.description || '');
    const [steps, setSteps] = useState(automation.steps || []);
    const [saving, setSaving] = useState(false);
    const [showAddStep, setShowAddStep] = useState(false);
    const [editingStep, setEditingStep] = useState(null);

    const isActive = automation.status === 'active';
    const TriggerIcon = triggerIcons[automation.trigger_type] || Zap;

    const handleSave = async () => {
        setSaving(true);
        await onSave({ ...automation, name, description, steps });
        setSaving(false);
    };

    const addStep = (stepType) => {
        const newStep = {
            id: `step_${Date.now()}`,
            type: stepType.type,
            name: stepType.label,
            config: stepType.defaultConfig || {},
            order: steps.length + 1,
        };
        setSteps([...steps, newStep]);
        setShowAddStep(false);
        setEditingStep(newStep);
    };

    const updateStep = (stepId, updates) => {
        setSteps(steps.map(s => s.id === stepId ? { ...s, ...updates } : s));
        setEditingStep(null);
    };

    const deleteStep = (stepId) => {
        if (confirm('Delete this step?')) {
            setSteps(steps.filter(s => s.id !== stepId));
        }
    };

    const stepTypes = [
        { type: 'send_email', label: 'Send Email', icon: Mail, color: 'bg-blue-500', desc: 'Send an email to the contact', defaultConfig: { subject: '', template_id: null } },
        { type: 'wait', label: 'Wait', icon: Clock, color: 'bg-yellow-500', desc: 'Wait for a period of time', defaultConfig: { duration: 1, unit: 'days' } },
        { type: 'add_tag', label: 'Add Tag', icon: Tag, color: 'bg-green-500', desc: 'Add a tag to the contact', defaultConfig: { tag_id: null } },
        { type: 'remove_tag', label: 'Remove Tag', icon: Tag, color: 'bg-red-500', desc: 'Remove a tag from the contact', defaultConfig: { tag_id: null } },
        { type: 'update_field', label: 'Update Field', icon: Edit2, color: 'bg-purple-500', desc: 'Update a contact field', defaultConfig: { field: '', value: '' } },
        { type: 'webhook', label: 'Webhook', icon: Globe, color: 'bg-orange-500', desc: 'Call an external URL', defaultConfig: { url: '', method: 'POST' } },
    ];

    const getStepIcon = (type) => {
        const stepType = stepTypes.find(s => s.type === type);
        return stepType?.icon || Zap;
    };

    const getStepColor = (type) => {
        const stepType = stepTypes.find(s => s.type === type);
        return stepType?.color || 'bg-muted';
    };

    return (
        <div className="space-y-6 pb-20 md:pb-6">
            {/* Header */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-4">
                <button
                    onClick={onBack}
                    className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors w-fit"
                >
                    <ArrowLeft className="w-5 h-5" />
                    <span>Back</span>
                </button>
                <div className="flex-1" />
                <div className="flex items-center gap-2">
                    <button
                        onClick={handleSave}
                        disabled={saving}
                        className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium flex items-center gap-2 transition-colors hover:bg-primary/90"
                    >
                        {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Settings className="w-4 h-4" />}
                        Save
                    </button>
                    <button
                        onClick={() => onToggle(!isActive)}
                        className={cn(
                            "px-4 py-2 rounded-lg text-sm font-medium flex items-center gap-2 transition-colors",
                            isActive
                                ? "bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20"
                                : "bg-green-500/10 text-green-500 hover:bg-green-500/20"
                        )}
                    >
                        {isActive ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                        {isActive ? 'Pause' : 'Activate'}
                    </button>
                </div>
            </div>

            {/* Main Content */}
            <div className="grid lg:grid-cols-3 gap-6">
                {/* Settings Panel */}
                <div className="lg:col-span-1 space-y-4">
                    <div className="bg-card border border-border rounded-xl p-5">
                        <h3 className="font-semibold mb-4">Settings</h3>

                        <div className="space-y-4">
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
                    </div>

                    {/* Trigger Info */}
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

                    {/* Stats */}
                    <div className="bg-card border border-border rounded-xl p-5">
                        <h3 className="font-semibold mb-4">Statistics</h3>
                        <div className="grid grid-cols-2 gap-4">
                            <div className="text-center p-3 bg-muted rounded-lg">
                                <div className="text-2xl font-bold">{automation.entry_count || 0}</div>
                                <div className="text-xs text-muted-foreground">Enrolled</div>
                            </div>
                            <div className="text-center p-3 bg-muted rounded-lg">
                                <div className="text-2xl font-bold">{automation.completed_count || 0}</div>
                                <div className="text-xs text-muted-foreground">Completed</div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Workflow Steps */}
                <div className="lg:col-span-2">
                    <div className="bg-card border border-border rounded-xl p-5 min-h-[400px]">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="font-semibold">Workflow Steps ({steps.length})</h3>
                            <button
                                onClick={() => setShowAddStep(true)}
                                className="text-sm text-primary hover:underline flex items-center gap-1"
                            >
                                <Plus className="w-4 h-4" />
                                Add Step
                            </button>
                        </div>

                        {steps.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-16 text-center">
                                <div className="w-16 h-16 rounded-full bg-muted flex items-center justify-center mb-4">
                                    <Mail className="w-8 h-8 text-muted-foreground" />
                                </div>
                                <h4 className="font-medium mb-2">No steps yet</h4>
                                <p className="text-sm text-muted-foreground mb-4 max-w-sm">
                                    Add actions like sending emails, waiting, or adding tags to build your workflow.
                                </p>
                                <button
                                    onClick={() => setShowAddStep(true)}
                                    className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium"
                                >
                                    <Plus className="w-4 h-4 inline mr-1" />
                                    Add First Step
                                </button>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {/* Trigger */}
                                <div className="flex items-center gap-3 p-3 bg-green-500/10 border border-green-500/30 rounded-lg">
                                    <div className="w-8 h-8 rounded bg-green-500/20 flex items-center justify-center">
                                        <TriggerIcon className="w-4 h-4 text-green-500" />
                                    </div>
                                    <div className="flex-1">
                                        <div className="font-medium text-sm text-green-500">Start: {triggerLabels[automation.trigger_type]}</div>
                                    </div>
                                </div>

                                {/* Connector */}
                                <div className="flex justify-center">
                                    <div className="w-0.5 h-6 bg-border" />
                                </div>

                                {/* Steps */}
                                {steps.map((step, index) => {
                                    const StepIcon = getStepIcon(step.type);
                                    return (
                                        <React.Fragment key={step.id || index}>
                                            <div className="flex items-center gap-3 p-3 bg-muted rounded-lg group">
                                                <div className={cn("w-8 h-8 rounded flex items-center justify-center", getStepColor(step.type) + '/20')}>
                                                    <StepIcon className={cn("w-4 h-4", getStepColor(step.type).replace('bg-', 'text-'))} />
                                                </div>
                                                <div className="flex-1 min-w-0">
                                                    <div className="font-medium text-sm">{step.name || step.type}</div>
                                                    <div className="text-xs text-muted-foreground truncate">
                                                        {step.config?.subject || step.config?.duration ? `${step.config.duration} ${step.config.unit}` : 'Click to configure'}
                                                    </div>
                                                </div>
                                                <button
                                                    onClick={() => setEditingStep(step)}
                                                    className="p-1.5 rounded hover:bg-background transition-opacity"
                                                >
                                                    <Edit2 className="w-4 h-4" />
                                                </button>
                                                <button
                                                    onClick={() => deleteStep(step.id)}
                                                    className="p-1.5 rounded hover:bg-red-500/10 text-red-500 transition-opacity"
                                                >
                                                    <Trash2 className="w-4 h-4" />
                                                </button>
                                            </div>
                                            {index < steps.length - 1 && (
                                                <div className="flex justify-center">
                                                    <div className="w-0.5 h-6 bg-border" />
                                                </div>
                                            )}
                                        </React.Fragment>
                                    );
                                })}

                                {/* Add more steps */}
                                <div className="flex justify-center">
                                    <div className="w-0.5 h-6 bg-border" />
                                </div>
                                <button
                                    onClick={() => setShowAddStep(true)}
                                    className="w-full p-3 border-2 border-dashed border-border rounded-lg text-muted-foreground hover:border-primary hover:text-primary transition-colors flex items-center justify-center gap-2"
                                >
                                    <Plus className="w-4 h-4" />
                                    Add Step
                                </button>
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Add Step Modal */}
            {showAddStep && (
                <div className="fixed inset-0 bg-black/60 flex items-end sm:items-center justify-center z-50 p-0 sm:p-4">
                    <div className="bg-card rounded-t-2xl sm:rounded-xl w-full sm:max-w-lg max-h-[90vh] overflow-y-auto">
                        <div className="sticky top-0 bg-card border-b border-border px-4 sm:px-6 py-4 flex items-center justify-between">
                            <h2 className="text-lg font-semibold">Add Step</h2>
                            <button onClick={() => setShowAddStep(false)} className="p-2 rounded-lg hover:bg-muted">
                                <X className="w-5 h-5" />
                            </button>
                        </div>
                        <div className="p-4 sm:p-6 space-y-2">
                            {stepTypes.map((stepType) => {
                                const Icon = stepType.icon;
                                return (
                                    <button
                                        key={stepType.type}
                                        onClick={() => addStep(stepType)}
                                        className="w-full flex items-center gap-3 p-3 rounded-lg border border-border hover:border-primary hover:bg-primary/5 transition-all text-left"
                                    >
                                        <div className={cn("w-10 h-10 rounded-lg flex items-center justify-center", stepType.color + '/20')}>
                                            <Icon className={cn("w-5 h-5", stepType.color.replace('bg-', 'text-'))} />
                                        </div>
                                        <div>
                                            <div className="font-medium text-sm">{stepType.label}</div>
                                            <div className="text-xs text-muted-foreground">{stepType.desc}</div>
                                        </div>
                                        <ChevronRight className="w-5 h-5 text-muted-foreground ml-auto" />
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                </div>
            )}

            {/* Edit Step Modal */}
            {editingStep && (
                <StepEditor
                    step={editingStep}
                    onSave={(updates) => updateStep(editingStep.id, updates)}
                    onClose={() => setEditingStep(null)}
                />
            )}
        </div>
    );
}

// Step Editor Modal
function StepEditor({ step, onSave, onClose }) {
    const [config, setConfig] = useState(step.config || {});

    const handleSave = () => {
        onSave({ config });
    };

    return (
        <div className="fixed inset-0 bg-black/60 flex items-end sm:items-center justify-center z-50 p-0 sm:p-4">
            <div className="bg-card rounded-t-2xl sm:rounded-xl w-full sm:max-w-lg max-h-[90vh] overflow-y-auto">
                <div className="sticky top-0 bg-card border-b border-border px-4 sm:px-6 py-4 flex items-center justify-between">
                    <h2 className="text-lg font-semibold">Configure: {step.name}</h2>
                    <button onClick={onClose} className="p-2 rounded-lg hover:bg-muted">
                        <X className="w-5 h-5" />
                    </button>
                </div>
                <div className="p-4 sm:p-6 space-y-4">
                    {step.type === 'send_email' && (
                        <>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">Email Subject</label>
                                <input
                                    type="text"
                                    value={config.subject || ''}
                                    onChange={(e) => setConfig({ ...config, subject: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    placeholder="Enter email subject"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">Template ID</label>
                                <input
                                    type="text"
                                    value={config.template_id || ''}
                                    onChange={(e) => setConfig({ ...config, template_id: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    placeholder="Enter template ID or leave blank"
                                />
                            </div>
                        </>
                    )}
                    {step.type === 'wait' && (
                        <div className="flex gap-4">
                            <div className="flex-1">
                                <label className="block text-sm font-medium mb-1.5">Duration</label>
                                <input
                                    type="number"
                                    value={config.duration || 1}
                                    onChange={(e) => setConfig({ ...config, duration: parseInt(e.target.value) })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    min={1}
                                />
                            </div>
                            <div className="flex-1">
                                <label className="block text-sm font-medium mb-1.5">Unit</label>
                                <select
                                    value={config.unit || 'days'}
                                    onChange={(e) => setConfig({ ...config, unit: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                >
                                    <option value="minutes">Minutes</option>
                                    <option value="hours">Hours</option>
                                    <option value="days">Days</option>
                                    <option value="weeks">Weeks</option>
                                </select>
                            </div>
                        </div>
                    )}
                    {(step.type === 'add_tag' || step.type === 'remove_tag') && (
                        <div>
                            <label className="block text-sm font-medium mb-1.5">Tag Name</label>
                            <input
                                type="text"
                                value={config.tag_name || ''}
                                onChange={(e) => setConfig({ ...config, tag_name: e.target.value })}
                                className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                placeholder="Enter tag name"
                            />
                        </div>
                    )}
                    {step.type === 'webhook' && (
                        <>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">Webhook URL</label>
                                <input
                                    type="url"
                                    value={config.url || ''}
                                    onChange={(e) => setConfig({ ...config, url: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    placeholder="https://example.com/webhook"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">Method</label>
                                <select
                                    value={config.method || 'POST'}
                                    onChange={(e) => setConfig({ ...config, method: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                >
                                    <option value="POST">POST</option>
                                    <option value="GET">GET</option>
                                </select>
                            </div>
                        </>
                    )}
                    {step.type === 'update_field' && (
                        <>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">Field Name</label>
                                <input
                                    type="text"
                                    value={config.field || ''}
                                    onChange={(e) => setConfig({ ...config, field: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    placeholder="e.g., custom_field_1"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium mb-1.5">New Value</label>
                                <input
                                    type="text"
                                    value={config.value || ''}
                                    onChange={(e) => setConfig({ ...config, value: e.target.value })}
                                    className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
                                    placeholder="Value to set"
                                />
                            </div>
                        </>
                    )}
                </div>
                <div className="p-4 sm:p-6 pt-0 flex gap-3">
                    <button
                        onClick={onClose}
                        className="flex-1 py-3 rounded-lg border border-border text-sm font-medium hover:bg-muted transition-colors"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleSave}
                        className="flex-1 py-3 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
                    >
                        Save Step
                    </button>
                </div>
            </div>
        </div>
    );
}
