import React, { useState, useEffect } from 'react';
import { Ban, Plus, Search, Trash2, AlertTriangle, UserX } from 'lucide-react';

const SuppressionsPage = () => {
    const [suppressions, setSuppressions] = useState([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState('');
    const [showAdd, setShowAdd] = useState(false);
    const [pagination, setPagination] = useState({ page: 1, total: 0, totalPages: 0 });

    useEffect(() => {
        fetchSuppressions();
    }, [pagination.page, search]);

    const fetchSuppressions = async () => {
        try {
            const params = new URLSearchParams({ page: pagination.page, limit: 50 });
            if (search) params.append('search', search);

            const res = await fetch(`/api/suppressions?${params}`, {
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            const data = await res.json();
            setSuppressions(data.data || []);
            setPagination({ page: data.page || 1, total: data.total || 0, totalPages: data.totalPages || 0 });
        } catch (error) {
            console.error('Failed to fetch suppressions:', error);
        } finally {
            setLoading(false);
        }
    };

    const addSuppression = async (email, reason) => {
        try {
            await fetch('/api/suppressions', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                },
                body: JSON.stringify({ email, reason }),
            });
            setShowAdd(false);
            fetchSuppressions();
        } catch (error) {
            console.error('Failed to add suppression:', error);
        }
    };

    const removeSuppression = async (id) => {
        if (!confirm('Remove this email from suppression list?')) return;
        try {
            await fetch(`/api/suppressions/${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            fetchSuppressions();
        } catch (error) {
            console.error('Failed to remove suppression:', error);
        }
    };

    const getReasonColor = (reason) => {
        const colors = {
            bounce: 'bg-red-100 text-red-600',
            complaint: 'bg-orange-100 text-orange-600',
            unsubscribe: 'bg-yellow-100 text-yellow-600',
            manual: 'bg-gray-100 text-gray-600',
        };
        return colors[reason] || colors.manual;
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
                        <Ban className="w-7 h-7 text-red-500" />
                        Suppression List
                    </h1>
                    <p className="text-gray-500">Emails that will not receive any campaigns</p>
                </div>
                <button
                    onClick={() => setShowAdd(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2 shadow-sm"
                >
                    <Plus className="w-4 h-4" /> Add Email
                </button>
            </div>

            <div className="mb-4 relative max-w-md">
                <Search className="absolute left-3 top-2.5 h-5 w-5 text-gray-400" />
                <input
                    type="text"
                    placeholder="Search emails..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="w-full pl-10 pr-4 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
                />
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden">
                <table className="w-full">
                    <thead className="bg-gray-50 dark:bg-gray-700">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Email</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Reason</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Added</th>
                            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                        {suppressions.length === 0 ? (
                            <tr>
                                <td colSpan="4" className="px-6 py-8 text-center text-gray-500">No suppressed emails</td>
                            </tr>
                        ) : (
                            suppressions.map((item) => (
                                <tr key={item.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                    <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">{item.email}</td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2 py-1 text-xs rounded ${getReasonColor(item.reason)}`}>
                                            {item.reason}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-500">{new Date(item.created_at).toLocaleDateString()}</td>
                                    <td className="px-6 py-4 text-right">
                                        <button onClick={() => removeSuppression(item.id)} className="text-red-600 hover:underline">
                                            Remove
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {showAdd && <AddSuppressionModal onClose={() => setShowAdd(false)} onAdd={addSuppression} />}
        </div>
    );
};

const AddSuppressionModal = ({ onClose, onAdd }) => {
    const [email, setEmail] = useState('');
    const [reason, setReason] = useState('manual');

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Add to Suppression List</h2>
                <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email</label>
                    <input
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        placeholder="email@example.com"
                    />
                </div>
                <div className="mb-6">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Reason</label>
                    <select
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                    >
                        <option value="manual">Manual</option>
                        <option value="bounce">Bounce</option>
                        <option value="complaint">Complaint</option>
                        <option value="unsubscribe">Unsubscribe</option>
                    </select>
                </div>
                <div className="flex justify-end gap-3">
                    <button onClick={onClose} className="px-4 py-2 text-gray-600">Cancel</button>
                    <button
                        onClick={() => onAdd(email, reason)}
                        disabled={!email}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg disabled:opacity-50"
                    >
                        Add
                    </button>
                </div>
            </div>
        </div>
    );
};

export default SuppressionsPage;
