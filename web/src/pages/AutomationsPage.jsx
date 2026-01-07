import React, { useState, useEffect } from 'react';
import AutomationBuilder from '../components/AutomationBuilder';
import { getAuthHeaders } from '../api';

const AutomationsPage = () => {
    const [automations, setAutomations] = useState([]);
    const [loading, setLoading] = useState(true);
    const [selectedAutomation, setSelectedAutomation] = useState(null);
    const [showBuilder, setShowBuilder] = useState(false);
    const [showCreate, setShowCreate] = useState(false);

    useEffect(() => {
        fetchAutomations();
    }, []);

    const fetchAutomations = async () => {
        try {
            const res = await fetch('/api/v2/automations', {
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            const data = await res.json();
            setAutomations(data || []);
        } catch (error) {
            console.error('Failed to fetch automations:', error);
        } finally {
            setLoading(false);
        }
    };

    const createAutomation = async (data) => {
        try {
            const res = await fetch('/api/v2/automations', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
                },
                body: JSON.stringify(data),
            });
            const automation = await res.json();
            setAutomations([automation, ...automations]);
            setShowCreate(false);
            setSelectedAutomation(automation);
            setShowBuilder(true);
        } catch (error) {
            console.error('Failed to create automation:', error);
        }
    };

    const saveAutomation = async (data) => {
        try {
            await fetch(`/api/v2/automations/${selectedAutomation.id}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
                },
                body: JSON.stringify({ ...selectedAutomation, ...data }),
            });
            fetchAutomations();
        } catch (error) {
            console.error('Failed to save automation:', error);
        }
    };

    const toggleAutomation = async (id, active) => {
        try {
            await fetch(`/api/v2/automations/${id}/${active ? 'activate' : 'pause'}`, {
                method: 'POST',
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            fetchAutomations();
        } catch (error) {
            console.error('Failed to toggle automation:', error);
        }
    };

    const deleteAutomation = async (id) => {
        if (!confirm('Delete this automation?')) return;
        try {
            await fetch(`/api/v2/automations/${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            setAutomations(automations.filter(a => a.id !== id));
        } catch (error) {
            console.error('Failed to delete automation:', error);
        }
    };

    if (showBuilder && selectedAutomation) {
        return (
            <div className="h-screen">
                <div className="bg-gray-800 px-4 py-2 flex items-center justify-between">
                    <button
                        onClick={() => setShowBuilder(false)}
                        className="text-gray-300 hover:text-white"
                    >
                        ← Back to Automations
                    </button>
                    <h2 className="text-white font-medium">{selectedAutomation.name}</h2>
                    <div></div>
                </div>
                <AutomationBuilder
                    automation={selectedAutomation}
                    onSave={saveAutomation}
                    onActivate={() => toggleAutomation(selectedAutomation.id, true)}
                    onPause={() => toggleAutomation(selectedAutomation.id, false)}
                />
            </div>
        );
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    return (
        <div className="p-6">
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Automations</h1>
                    <p className="text-gray-500">Create automated email workflows</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + New Automation
                </button>
            </div>

            {automations.length === 0 ? (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-12 text-center">
                    <div className="text-6xl mb-4">🤖</div>
                    <h3 className="text-xl font-semibold mb-2 dark:text-white">No automations yet</h3>
                    <p className="text-gray-500 mb-4">Create your first automation workflow</p>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                        Create Automation
                    </button>
                </div>
            ) : (
                <div className="grid gap-4">
                    {automations.map((automation) => (
                        <div
                            key={automation.id}
                            className="bg-white dark:bg-gray-800 rounded-xl p-6 shadow flex items-center justify-between"
                        >
                            <div>
                                <h3 className="font-semibold text-gray-900 dark:text-white">{automation.name}</h3>
                                <p className="text-sm text-gray-500">{automation.description || 'No description'}</p>
                                <div className="flex items-center gap-4 mt-2 text-sm">
                                    <span className={`px-2 py-0.5 rounded ${automation.status === 'active' ? 'bg-green-100 text-green-600' : 'bg-gray-100 text-gray-600'}`}>
                                        {automation.status || 'draft'}
                                    </span>
                                    <span className="text-gray-400">
                                        {automation.entry_count || 0} enrollments
                                    </span>
                                </div>
                            </div>
                            <div className="flex items-center gap-3">
                                <button
                                    onClick={() => {
                                        setSelectedAutomation(automation);
                                        setShowBuilder(true);
                                    }}
                                    className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200"
                                >
                                    Edit
                                </button>
                                {automation.status === 'active' ? (
                                    <button
                                        onClick={() => toggleAutomation(automation.id, false)}
                                        className="px-4 py-2 bg-yellow-100 text-yellow-600 rounded hover:bg-yellow-200"
                                    >
                                        Pause
                                    </button>
                                ) : (
                                    <button
                                        onClick={() => toggleAutomation(automation.id, true)}
                                        className="px-4 py-2 bg-green-100 text-green-600 rounded hover:bg-green-200"
                                    >
                                        Activate
                                    </button>
                                )}
                                <button
                                    onClick={() => deleteAutomation(automation.id)}
                                    className="px-4 py-2 bg-red-100 text-red-600 rounded hover:bg-red-200"
                                >
                                    Delete
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {showCreate && (
                <CreateAutomationModal
                    onClose={() => setShowCreate(false)}
                    onCreate={createAutomation}
                />
            )}
        </div>
    );
};

const CreateAutomationModal = ({ onClose, onCreate }) => {
    const [form, setForm] = useState({
        name: '',
        description: '',
        trigger_type: 'trigger_contact_added',
    });

    const handleSubmit = (e) => {
        e.preventDefault();
        onCreate(form);
    };

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Create Automation</h2>
                <form onSubmit={handleSubmit}>
                    <div className="mb-4">
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
                        <input
                            type="text"
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="e.g., Welcome Series"
                            required
                        />
                    </div>
                    <div className="mb-4">
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
                        <textarea
                            value={form.description}
                            onChange={(e) => setForm({ ...form, description: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            rows={3}
                            placeholder="Optional description"
                        />
                    </div>
                    <div className="mb-6">
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Trigger</label>
                        <select
                            value={form.trigger_type}
                            onChange={(e) => setForm({ ...form, trigger_type: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        >
                            <option value="trigger_contact_added">Contact Added to List</option>
                            <option value="trigger_tag_added">Tag Added</option>
                            <option value="trigger_email_opened">Email Opened</option>
                            <option value="trigger_link_clicked">Link Clicked</option>
                            <option value="trigger_form_submitted">Form Submitted</option>
                            <option value="trigger_webhook">Webhook</option>
                        </select>
                    </div>
                    <div className="flex justify-end gap-3">
                        <button type="button" onClick={onClose} className="px-4 py-2 text-gray-600 hover:text-gray-800">
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={!form.name}
                            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
                        >
                            Create
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
};

export default AutomationsPage;

