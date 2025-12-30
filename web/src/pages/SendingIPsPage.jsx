import React, { useState, useEffect } from 'react';

const SendingIPsPage = () => {
    const [ips, setIps] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);

    useEffect(() => {
        fetchIPs();
    }, []);

    const fetchIPs = async () => {
        try {
            const res = await fetch('/api/sending-ips', {
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            const data = await res.json();
            setIps(data || []);
        } catch (error) {
            console.error('Failed to fetch IPs:', error);
        } finally {
            setLoading(false);
        }
    };

    const createIP = async (data) => {
        try {
            await fetch('/api/sending-ips', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                },
                body: JSON.stringify(data),
            });
            setShowCreate(false);
            fetchIPs();
        } catch (error) {
            console.error('Failed to create IP:', error);
        }
    };

    const deleteIP = async (id) => {
        if (!confirm('Delete this sending IP?')) return;
        try {
            await fetch(`/api/sending-ips/${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            fetchIPs();
        } catch (error) {
            console.error('Failed to delete IP:', error);
        }
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Sending IPs</h1>
                    <p className="text-gray-500">Configure outgoing email IP addresses</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + Add Sending IP
                </button>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden">
                <table className="w-full">
                    <thead className="bg-gray-50 dark:bg-gray-700">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">IP Address</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Hostname</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Pool</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Daily Limit</th>
                            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                        {ips.length === 0 ? (
                            <tr>
                                <td colSpan="6" className="px-6 py-8 text-center text-gray-500">No sending IPs configured</td>
                            </tr>
                        ) : (
                            ips.map((ip) => (
                                <tr key={ip.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                    <td className="px-6 py-4 text-sm font-mono text-gray-900 dark:text-white">{ip.ip_address}</td>
                                    <td className="px-6 py-4 text-sm text-gray-500">{ip.hostname || '-'}</td>
                                    <td className="px-6 py-4 text-sm text-gray-500">{ip.pool || 'default'}</td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2 py-1 text-xs rounded ${ip.active ? 'bg-green-100 text-green-600' : 'bg-gray-100 text-gray-600'}`}>
                                            {ip.active ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-500">{ip.daily_limit || '∞'}</td>
                                    <td className="px-6 py-4 text-right">
                                        <button onClick={() => deleteIP(ip.id)} className="text-red-600 hover:underline text-sm">
                                            Delete
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {showCreate && <CreateIPModal onClose={() => setShowCreate(false)} onCreate={createIP} />}
        </div>
    );
};

const CreateIPModal = ({ onClose, onCreate }) => {
    const [form, setForm] = useState({
        ip_address: '',
        hostname: '',
        pool: 'default',
        daily_limit: '',
    });

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Add Sending IP</h2>
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">IP Address</label>
                        <input
                            type="text"
                            value={form.ip_address}
                            onChange={(e) => setForm({ ...form, ip_address: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white font-mono"
                            placeholder="192.168.1.1"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Hostname (rDNS)</label>
                        <input
                            type="text"
                            value={form.hostname}
                            onChange={(e) => setForm({ ...form, hostname: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="mail.example.com"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Pool</label>
                        <input
                            type="text"
                            value={form.pool}
                            onChange={(e) => setForm({ ...form, pool: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="default"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Daily Limit</label>
                        <input
                            type="number"
                            value={form.daily_limit}
                            onChange={(e) => setForm({ ...form, daily_limit: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="Leave empty for unlimited"
                        />
                    </div>
                </div>
                <div className="flex justify-end gap-3 mt-6">
                    <button onClick={onClose} className="px-4 py-2 text-gray-600">Cancel</button>
                    <button
                        onClick={() => onCreate(form)}
                        disabled={!form.ip_address}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg disabled:opacity-50"
                    >
                        Add
                    </button>
                </div>
            </div>
        </div>
    );
};

export default SendingIPsPage;
