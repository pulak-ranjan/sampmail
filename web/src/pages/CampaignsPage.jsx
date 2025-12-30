import React, { useState, useEffect } from 'react';

const CampaignsPage = () => {
    const [campaigns, setCampaigns] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);

    useEffect(() => {
        fetchCampaigns();
    }, []);

    const fetchCampaigns = async () => {
        try {
            const res = await fetch('/api/v2/campaigns', {
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
            paused: 'bg-orange-100 text-orange-600',
            failed: 'bg-red-100 text-red-600',
        };
        return colors[status] || colors.draft;
    };

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
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + New Campaign
                </button>
            </div>

            {campaigns.length === 0 ? (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-12 text-center">
                    <div className="text-6xl mb-4">📧</div>
                    <h3 className="text-xl font-semibold mb-2 dark:text-white">No campaigns yet</h3>
                    <p className="text-gray-500 mb-4">Create your first email campaign to get started</p>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                        Create Campaign
                    </button>
                </div>
            ) : (
                <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden">
                    <table className="w-full">
                        <thead className="bg-gray-50 dark:bg-gray-700">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Subject</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Sent</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Opens</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Clicks</th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                            {campaigns.map((campaign) => (
                                <tr key={campaign.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                    <td className="px-6 py-4 whitespace-nowrap">
                                        <span className="font-medium text-gray-900 dark:text-white">{campaign.name}</span>
                                    </td>
                                    <td className="px-6 py-4 text-gray-500 dark:text-gray-300">{campaign.subject}</td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2 py-1 text-xs rounded ${getStatusColor(campaign.status)}`}>
                                            {campaign.status}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-gray-500 dark:text-gray-300">{campaign.sent_count || 0}</td>
                                    <td className="px-6 py-4 text-gray-500 dark:text-gray-300">{campaign.opened_count || 0}</td>
                                    <td className="px-6 py-4 text-gray-500 dark:text-gray-300">{campaign.clicked_count || 0}</td>
                                    <td className="px-6 py-4 text-right">
                                        <button className="text-blue-600 hover:underline mr-3">Edit</button>
                                        <button className="text-red-600 hover:underline">Delete</button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {showCreate && (
                <CreateCampaignModal
                    onClose={() => setShowCreate(false)}
                    onCreated={() => {
                        setShowCreate(false);
                        fetchCampaigns();
                    }}
                />
            )}
        </div>
    );
};

const CreateCampaignModal = ({ onClose, onCreated }) => {
    const [form, setForm] = useState({
        name: '',
        subject: '',
        from_name: '',
        from_email: '',
    });
    const [saving, setSaving] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setSaving(true);
        try {
            await fetch('/api/v2/campaigns', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                },
                body: JSON.stringify(form),
            });
            onCreated();
        } catch (error) {
            console.error('Failed to create campaign:', error);
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Create Campaign</h2>
                <form onSubmit={handleSubmit}>
                    <div className="mb-4">
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Campaign Name</label>
                        <input
                            type="text"
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="e.g., December Newsletter"
                            required
                        />
                    </div>
                    <div className="mb-4">
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject Line</label>
                        <input
                            type="text"
                            value={form.subject}
                            onChange={(e) => setForm({ ...form, subject: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="e.g., Your December update is here!"
                            required
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-4 mb-6">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Name</label>
                            <input
                                type="text"
                                value={form.from_name}
                                onChange={(e) => setForm({ ...form, from_name: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                                placeholder="Your Name"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Email</label>
                            <input
                                type="email"
                                value={form.from_email}
                                onChange={(e) => setForm({ ...form, from_email: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                                placeholder="you@domain.com"
                            />
                        </div>
                    </div>
                    <div className="flex justify-end gap-3">
                        <button type="button" onClick={onClose} className="px-4 py-2 text-gray-600 hover:text-gray-800">
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={saving || !form.name || !form.subject}
                            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
                        >
                            {saving ? 'Creating...' : 'Create'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
};

export default CampaignsPage;
