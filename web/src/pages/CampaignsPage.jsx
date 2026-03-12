import React, { useState, useEffect } from 'react';
import EmailTemplateBuilder from '../components/EmailTemplateBuilder';
import { Mail, Rocket, Sparkles, Lightbulb, ArrowLeft, Plus, Send, Pause, Play, XCircle, AlertTriangle, CheckCircle, X } from 'lucide-react';
import ConfirmationModal from '../components/ConfirmationModal';
import { getAuthHeaders, sendCampaign } from '../api';

const CampaignsPage = () => {
    const [campaigns, setCampaigns] = useState([]);
    const [loading, setLoading] = useState(true);
    const [view, setView] = useState('list'); // list, create, edit
    const [selectedCampaign, setSelectedCampaign] = useState(null);
    const [deleteId, setDeleteId] = useState(null);
    const [sendConfirmCampaign, setSendConfirmCampaign] = useState(null);
    const [sending, setSending] = useState(false);
    const [actionLoading, setActionLoading] = useState(null); // 'pause', 'resume', 'cancel'

    useEffect(() => {
        fetchCampaigns();
    }, []);

    const fetchCampaigns = async () => {
        try {
            const res = await fetch('/api/v2/campaigns', {
                headers: getAuthHeaders(),
            });
            const data = await res.json();
            setCampaigns(data || []);
        } catch (error) {
            console.error('Failed to fetch campaigns:', error);
        } finally {
            setLoading(false);
        }
    };

    const getStatusColor = (status) => {
        const colors = {
            draft: 'bg-gray-100 text-gray-600',
            scheduled: 'bg-blue-100 text-blue-600',
            sending: 'bg-yellow-100 text-yellow-600',
            sent: 'bg-green-100 text-green-600',
            completed: 'bg-green-100 text-green-600',
            paused: 'bg-orange-100 text-orange-600',
            failed: 'bg-red-100 text-red-600',
        };
        return colors[status] || colors.draft;
    };

    const handleCreate = () => {
        setSelectedCampaign(null);
        setView('create');
    };

    const handleEdit = (campaign) => {
        setSelectedCampaign(campaign);
        setView('create'); // Re-use create view for editing
    };

    const handleDeleteClick = (id) => {
        setDeleteId(id);
    };

    const confirmDelete = async () => {
        if (!deleteId) return;
        try {
            await fetch(`/api/v2/campaigns/${deleteId}`, {
                method: 'DELETE',
                headers: getAuthHeaders(),
            });
            fetchCampaigns();
        } catch (error) {
            console.error(error);
        }
        setDeleteId(null);
    };

    const confirmSend = async () => {
        if (!sendConfirmCampaign) return;
        setSending(true);
        try {
            await sendCampaign(sendConfirmCampaign.id);
            fetchCampaigns();
        } catch (error) {
            console.error('Failed to send campaign:', error);
        }
        setSending(false);
        setSendConfirmCampaign(null);
    };

    const handlePause = async (campaign) => {
        setActionLoading(campaign.id);
        try {
            await fetch(`/api/campaigns/${campaign.id}/pause`, {
                method: 'POST',
                headers: getAuthHeaders(),
            });
            fetchCampaigns();
        } catch (error) {
            console.error('Failed to pause campaign:', error);
        }
        setActionLoading(null);
    };

    const handleResume = async (campaign) => {
        setActionLoading(campaign.id);
        try {
            await fetch(`/api/campaigns/${campaign.id}/resume`, {
                method: 'POST',
                headers: getAuthHeaders(),
            });
            fetchCampaigns();
        } catch (error) {
            console.error('Failed to resume campaign:', error);
        }
        setActionLoading(null);
    };

    const handleCancel = async (campaign) => {
        if (!window.confirm('Are you sure you want to cancel this campaign? This will stop sending immediately.')) return;
        setActionLoading(campaign.id);
        try {
            await fetch(`/api/campaigns/${campaign.id}/cancel`, {
                method: 'POST',
                headers: getAuthHeaders(),
            });
            fetchCampaigns();
        } catch (error) {
            console.error('Failed to cancel campaign:', error);
        }
        setActionLoading(null);
    };

    if (view === 'create') {
        return (
            <CampaignWizard
                campaign={selectedCampaign}
                onCancel={() => setView('list')}
                onSave={() => {
                    setView('list');
                    fetchCampaigns();
                }}
            />
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Campaigns</h1>
                    <p className="text-gray-500">Create and manage email campaigns</p>
                </div>
                <button
                    onClick={handleCreate}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm transition-all flex items-center gap-2"
                >
                    <Plus className="w-4 h-4" /> New Campaign
                </button>
            </div>

            {campaigns.length === 0 ? (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-12 text-center border dashed border-gray-300 dark:border-gray-700">
                    <div className="flex justify-center mb-4">
                        <Mail className="w-16 h-16 text-gray-300" />
                    </div>
                    <h3 className="text-xl font-semibold mb-2 dark:text-white">No campaigns yet</h3>
                    <p className="text-gray-500 mb-4">Create your first email campaign to get started</p>
                    <button
                        onClick={handleCreate}
                        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium inline-flex items-center gap-2"
                    >
                        <Plus className="w-4 h-4" /> Create Campaign
                    </button>
                </div>
            ) : (
                <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden">
                    <table className="w-full">
                        <thead className="bg-gray-50 dark:bg-gray-700 border-b dark:border-gray-600">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Subject</th>
                                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Stats</th>
                                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                            {campaigns.map((campaign) => (
                                <tr key={campaign.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                                    <td className="px-6 py-4 whitespace-nowrap">
                                        <span className="font-medium text-gray-900 dark:text-white">{campaign.name}</span>
                                        <div className="text-xs text-gray-500">Created {new Date(campaign.created_at).toLocaleDateString()}</div>
                                    </td>
                                    <td className="px-6 py-4 text-gray-500 dark:text-gray-300 max-w-[200px] truncate" title={campaign.subject}>
                                        {campaign.subject}
                                    </td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2.5 py-1 text-xs font-medium rounded-full ${getStatusColor(campaign.status)}`}>
                                            {campaign.status}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4">
                                        <div className="flex gap-4 text-xs">
                                            <div>
                                                <span className="block text-gray-500">Sent</span>
                                                <span className="font-semibold dark:text-gray-200">{campaign.total_sent || campaign.sent_count || 0}</span>
                                            </div>
                                            <div>
                                                <span className="block text-gray-500">Opens</span>
                                                <span className="font-semibold text-green-600">{campaign.total_opens || campaign.opened_count || 0}</span>
                                            </div>
                                            <div>
                                                <span className="block text-gray-500">Clicks</span>
                                                <span className="font-semibold text-blue-600">{campaign.total_clicks || campaign.clicked_count || 0}</span>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4 text-right space-x-3">
                                        {campaign.status === 'draft' && (
                                            <button
                                                onClick={() => setSendConfirmCampaign(campaign)}
                                                className="text-green-600 hover:text-green-800 font-medium inline-flex items-center gap-1"
                                            >
                                                <Send className="w-3 h-3" /> Send
                                            </button>
                                        )}
                                        {campaign.status === 'sending' && (
                                            <>
                                                <button
                                                    onClick={() => handlePause(campaign)}
                                                    disabled={actionLoading === campaign.id}
                                                    className="text-yellow-600 hover:text-yellow-800 font-medium inline-flex items-center gap-1 disabled:opacity-50"
                                                >
                                                    <Pause className="w-3 h-3" /> {actionLoading === campaign.id ? '...' : 'Pause'}
                                                </button>
                                                <button
                                                    onClick={() => handleCancel(campaign)}
                                                    disabled={actionLoading === campaign.id}
                                                    className="text-red-600 hover:text-red-800 font-medium inline-flex items-center gap-1 disabled:opacity-50"
                                                >
                                                    <XCircle className="w-3 h-3" /> {actionLoading === campaign.id ? '...' : 'Cancel'}
                                                </button>
                                            </>
                                        )}
                                        {campaign.status === 'paused' && (
                                            <>
                                                <button
                                                    onClick={() => handleResume(campaign)}
                                                    disabled={actionLoading === campaign.id}
                                                    className="text-green-600 hover:text-green-800 font-medium inline-flex items-center gap-1 disabled:opacity-50"
                                                >
                                                    <Play className="w-3 h-3" /> {actionLoading === campaign.id ? '...' : 'Resume'}
                                                </button>
                                                <button
                                                    onClick={() => handleCancel(campaign)}
                                                    disabled={actionLoading === campaign.id}
                                                    className="text-red-600 hover:text-red-800 font-medium inline-flex items-center gap-1 disabled:opacity-50"
                                                >
                                                    <XCircle className="w-3 h-3" /> Cancel
                                                </button>
                                            </>
                                        )}
                                        {campaign.status !== 'sending' && campaign.status !== 'paused' && (
                                            <button onClick={() => handleEdit(campaign)} className="text-blue-600 hover:text-blue-800 font-medium">Edit</button>
                                        )}
                                        <button onClick={() => handleDeleteClick(campaign.id)} className="text-red-600 hover:text-red-800 font-medium">Delete</button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            <ConfirmationModal
                isOpen={!!deleteId}
                onClose={() => setDeleteId(null)}
                onConfirm={confirmDelete}
                title="Delete Campaign"
                message="Are you sure you want to delete this campaign? This action cannot be undone."
                confirmText="Delete"
                type="danger"
            />

            <ConfirmationModal
                isOpen={!!sendConfirmCampaign}
                onClose={() => setSendConfirmCampaign(null)}
                onConfirm={confirmSend}
                title="Send Campaign"
                message={
                    <div>
                        <p className="mb-4">Are you sure you want to send "{sendConfirmCampaign?.name}"? This will immediately start sending to all subscribers in the campaign's list.</p>
                        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3 flex gap-3 items-start">
                            <AlertTriangle className="w-5 h-5 text-yellow-600 shrink-0" />
                            <div className="text-sm text-yellow-800 dark:text-yellow-300">
                                <strong>Tip:</strong> Make sure your list is verified! Go to <strong>Verify Emails</strong> to validate your subscriber list before sending to improve deliverability and reduce bounces.
                            </div>
                        </div>
                    </div>
                }
                confirmText={sending ? "Sending..." : "Send Now"}
                type="primary"
            />
        </div>
    );
};

const CampaignWizard = ({ campaign, onCancel, onSave }) => {
    const [step, setStep] = useState(1);
    const [form, setForm] = useState(campaign || {
        name: '',
        subject: '',
        from_name: '',
        from_email: '',
        html_content: '',
        builder_json: null
    });
    const [saving, setSaving] = useState(false);

    const handleSubmit = async () => {
        setSaving(true);
        try {
            const url = campaign ? `/api/v2/campaigns/${campaign.id}` : '/api/v2/campaigns';
            const method = campaign ? 'PUT' : 'POST';

            await fetch(url, {
                method,
                headers: {
                    ...getAuthHeaders(),
                },
                body: JSON.stringify(form),
            });
            onSave();
        } catch (error) {
            console.error('Failed to save campaign:', error);
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
            {/* Wizard Header */}
            <div className="bg-white dark:bg-gray-800 border-b px-6 py-4 flex items-center justify-between shadow-sm z-10">
                <div className="flex items-center gap-4">
                    <button onClick={onCancel} className="flex items-center gap-1 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300">
                        <ArrowLeft className="w-4 h-4" /> Back
                    </button>
                    <h2 className="text-lg font-bold dark:text-white">
                        {campaign ? 'Edit Campaign' : 'New Campaign'}
                    </h2>
                    <div className="h-6 w-px bg-gray-200 dark:bg-gray-700"></div>
                    <div className="flex gap-2">
                        {[1, 2, 3].map(s => (
                            <div key={s} className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${step === s
                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                                : 'text-gray-500'
                                }`}>
                                Step {s}
                            </div>
                        ))}
                    </div>
                </div>
                <div className="flex gap-2">
                    {step > 1 && (
                        <button
                            onClick={() => setStep(step - 1)}
                            className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg dark:text-gray-300 dark:hover:bg-gray-700"
                        >
                            Previous
                        </button>
                    )}
                    {step < 3 ? (
                        <button
                            onClick={() => {
                                if (step === 1 && (!form.name || !form.subject)) return alert('Please fill in required fields');
                                setStep(step + 1);
                            }}
                            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium"
                        >
                            Next Step
                        </button>
                    ) : (
                        <button
                            onClick={handleSubmit}
                            disabled={saving}
                            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-md flex items-center gap-2"
                        >
                            {saving ? 'Saving...' : <><Rocket className="w-4 h-4" /> Save & Finish</>}
                        </button>
                    )}
                </div>
            </div>

            {/* Wizard Content */}
            <div className="flex-1 overflow-hidden">
                {step === 1 && (
                    <div className="max-w-2xl mx-auto py-12 px-6">
                        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border p-8">
                            <h3 className="text-xl font-bold mb-6 dark:text-white">Campaign Details</h3>
                            <div className="space-y-6">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Campaign Name</label>
                                    <input
                                        type="text"
                                        value={form.name}
                                        onChange={(e) => setForm({ ...form, name: e.target.value })}
                                        className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                                        placeholder="e.g. December Newsletter"
                                        autoFocus
                                    />
                                    <p className="text-xs text-gray-500 mt-1">Internal name for your reference.</p>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject Line</label>
                                    <input
                                        type="text"
                                        value={form.subject}
                                        onChange={(e) => setForm({ ...form, subject: e.target.value })}
                                        className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                                        placeholder="e.g. You won't believe this update..."
                                    />
                                </div>
                                <div className="grid grid-cols-2 gap-6">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Sender Name</label>
                                        <input
                                            type="text"
                                            value={form.from_name}
                                            onChange={(e) => setForm({ ...form, from_name: e.target.value })}
                                            className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                                            placeholder="Your Name"
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Sender Email</label>
                                        <input
                                            type="email"
                                            value={form.from_email}
                                            onChange={(e) => setForm({ ...form, from_email: e.target.value })}
                                            className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                                            placeholder="you@yourdomain.com"
                                        />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {step === 2 && (
                    <div className="h-full">
                        <EmailTemplateBuilder
                            template={{
                                html_content: form.html_content,
                                builder_json: typeof form.builder_json === 'string' ? JSON.parse(form.builder_json) : form.builder_json
                            }}
                            onSave={(data) => {
                                setForm({
                                    ...form,
                                    html_content: data.html_content,
                                    builder_json: data.builder_json
                                });
                            }}
                            onPreview={(html) => {
                                const win = window.open('', '_blank');
                                win.document.write(html);
                            }}
                        />
                    </div>
                )}

                {step === 3 && (
                    <div className="max-w-3xl mx-auto py-12 px-6">
                        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border p-8 space-y-8">
                            <div className="text-center">
                                <div className="w-16 h-16 bg-green-100 text-green-600 rounded-full flex items-center justify-center mx-auto mb-4">
                                    <Sparkles className="w-8 h-8" />
                                </div>
                                <h3 className="text-2xl font-bold dark:text-white">Ready to Launch?</h3>
                                <p className="text-gray-500 mt-2">Review your campaign details before finalizing.</p>
                            </div>

                            <div className="grid grid-cols-2 gap-4 text-sm border-t border-b py-6 dark:border-gray-700">
                                <div>
                                    <span className="block text-gray-500 mb-1">Campaign</span>
                                    <span className="font-medium dark:text-white">{form.name}</span>
                                </div>
                                <div>
                                    <span className="block text-gray-500 mb-1">Subject</span>
                                    <span className="font-medium dark:text-white">{form.subject}</span>
                                </div>
                                <div>
                                    <span className="block text-gray-500 mb-1">From</span>
                                    <span className="font-medium dark:text-white">{form.from_name} &lt;{form.from_email}&gt;</span>
                                </div>
                                <div>
                                    <span className="block text-gray-500 mb-1">Content</span>
                                    <span className="font-medium dark:text-white">
                                        {form.html_content ? 'HTML Template Configured' : 'No Content Added'}
                                    </span>
                                </div>
                            </div>

                            <div className="bg-blue-50 dark:bg-blue-900/20 p-4 rounded-lg text-blue-800 dark:text-blue-300 text-sm flex gap-3 items-start">
                                <Lightbulb className="w-5 h-5 shrink-0" />
                                <div>
                                    <strong>Pro Tip:</strong> Ensure you have validated your sender domain in the Settings &gt; Domains page to improve deliverability.
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default CampaignsPage;

