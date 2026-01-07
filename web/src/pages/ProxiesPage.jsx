import React, { useState, useEffect } from 'react';
import { Network, Plus, Trash2, Play, CheckCircle, XCircle, Globe, Shield, RefreshCw } from 'lucide-react';
import ConfirmationModal from '../components/ConfirmationModal';

const ProxiesPage = () => {
    const [proxies, setProxies] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [testing, setTesting] = useState(null);
    const [testResults, setTestResults] = useState({}); // { id: { success: true/false, message: '' } }
    const [deleteId, setDeleteId] = useState(null);

    useEffect(() => {
        fetchProxies();
    }, []);

    const fetchProxies = async () => {
        try {
            const res = await fetch('/api/proxies', {
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            const data = await res.json();
            setProxies(data || []);
        } catch (error) {
            console.error('Failed to fetch proxies:', error);
        } finally {
            setLoading(false);
        }
    };

    const createProxy = async (data) => {
        try {
            await fetch('/api/proxies', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
                },
                body: JSON.stringify(data),
            });
            setShowCreate(false);
            fetchProxies();
        } catch (error) {
            console.error('Failed to create proxy:', error);
        }
    };

    const confirmDelete = async () => {
        if (!deleteId) return;
        try {
            await fetch(`/api/proxies/${deleteId}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            fetchProxies();
        } catch (error) {
            console.error('Failed to delete proxy:', error);
        }
        setDeleteId(null);
    };

    const testProxy = async (id) => {
        setTesting(id);
        setTestResults(prev => ({ ...prev, [id]: null })); // Clear previous result
        try {
            const res = await fetch(`/api/proxies/${id}/test`, {
                method: 'POST',
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            const data = await res.json();
            setTestResults(prev => ({
                ...prev,
                [id]: {
                    success: data.success,
                    message: data.success ? 'Proxy is working' : (data.error || 'Connection failed')
                }
            }));
        } catch (error) {
            setTestResults(prev => ({
                ...prev,
                [id]: { success: false, message: 'Test failed' }
            }));
        } finally {
            setTesting(null);
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
                        <Network className="w-7 h-7 text-blue-600" />
                        Proxies
                    </h1>
                    <p className="text-gray-500">Configure SMTP and HTTP proxies for rotation</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2 shadow-sm transition-all"
                >
                    <Plus className="w-4 h-4" /> Add Proxy
                </button>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden border dark:border-gray-700">
                <table className="w-full">
                    <thead className="bg-gray-50 dark:bg-gray-700 border-b dark:border-gray-600">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                            <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Type</th>
                            <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Host</th>
                            <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                            <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Test Result</th>
                            <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                        {proxies.length === 0 ? (
                            <tr>
                                <td colSpan="6" className="px-6 py-12 text-center text-gray-500">
                                    <Globe className="w-12 h-12 mx-auto text-gray-300 mb-2" />
                                    No proxies configured
                                </td>
                            </tr>
                        ) : (
                            proxies.map((proxy) => (
                                <tr key={proxy.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                                    <td className="px-6 py-4 text-sm font-medium text-gray-900 dark:text-white">{proxy.name}</td>
                                    <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                                        <span className="px-2 py-1 bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 rounded text-xs font-medium uppercase border border-blue-100 dark:border-blue-800">
                                            {proxy.type}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 font-mono">
                                        {proxy.host}:{proxy.port}
                                    </td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2 py-1 text-xs font-medium rounded-full flex w-fit items-center gap-1 ${proxy.active
                                                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                                                : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
                                            }`}>
                                            <div className={`w-1.5 h-1.5 rounded-full ${proxy.active ? 'bg-green-500' : 'bg-gray-500'}`}></div>
                                            {proxy.active ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-sm">
                                        {testResults[proxy.id] && (
                                            <div className={`flex items-center gap-1.5 ${testResults[proxy.id].success
                                                    ? 'text-green-600 dark:text-green-400'
                                                    : 'text-red-600 dark:text-red-400'
                                                }`}>
                                                {testResults[proxy.id].success ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                                                <span className="text-xs font-medium">{testResults[proxy.id].message}</span>
                                            </div>
                                        )}
                                    </td>
                                    <td className="px-6 py-4 text-right space-x-2">
                                        <button
                                            onClick={() => testProxy(proxy.id)}
                                            disabled={testing === proxy.id}
                                            className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-blue-600 bg-blue-50 hover:bg-blue-100 rounded-md transition-colors disabled:opacity-50"
                                        >
                                            {testing === proxy.id ? <RefreshCw className="w-3 h-3 animate-spin" /> : <Play className="w-3 h-3" />}
                                            {testing === proxy.id ? 'Testing...' : 'Test'}
                                        </button>
                                        <button
                                            onClick={() => setDeleteId(proxy.id)}
                                            className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-gray-600 hover:text-red-600 hover:bg-red-50 rounded-md transition-colors"
                                        >
                                            <Trash2 className="w-3 h-3" />
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            <ConfirmationModal
                isOpen={!!deleteId}
                onClose={() => setDeleteId(null)}
                onConfirm={confirmDelete}
                title="Delete Proxy"
                message="Are you sure you want to remove this proxy configuration?"
                confirmText="Delete"
                type="danger"
            />

            {showCreate && <CreateProxyModal onClose={() => setShowCreate(false)} onCreate={createProxy} />}
        </div>
    );
};

const CreateProxyModal = ({ onClose, onCreate }) => {
    const [form, setForm] = useState({
        name: '',
        type: 'socks5',
        host: '',
        port: '',
        username: '',
        password: '',
    });

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-2xl border dark:border-gray-700 animate-in zoom-in-95 duration-200">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold dark:text-white flex items-center gap-2">
                        <Shield className="w-5 h-5 text-blue-600" />
                        Add Proxy
                    </h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">✕</button>
                </div>

                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
                        <input
                            type="text"
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                            placeholder="My Proxy"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Type</label>
                        <select
                            value={form.type}
                            onChange={(e) => setForm({ ...form, type: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                        >
                            <option value="socks5">SOCKS5</option>
                            <option value="http">HTTP</option>
                            <option value="https">HTTPS</option>
                        </select>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host</label>
                            <input
                                type="text"
                                value={form.host}
                                onChange={(e) => setForm({ ...form, host: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                                placeholder="proxy.example.com"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                            <input
                                type="number"
                                value={form.port}
                                onChange={(e) => setForm({ ...form, port: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                                placeholder="1080"
                            />
                        </div>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                            <input
                                type="text"
                                value={form.username}
                                onChange={(e) => setForm({ ...form, username: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                                placeholder="Optional"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                            <input
                                type="password"
                                value={form.password}
                                onChange={(e) => setForm({ ...form, password: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none transition-all"
                                placeholder="Optional"
                            />
                        </div>
                    </div>
                </div>
                <div className="flex justify-end gap-3 mt-8">
                    <button
                        onClick={onClose}
                        className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg font-medium transition-colors"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={() => onCreate(form)}
                        disabled={!form.name || !form.host || !form.port}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                    >
                        Add Proxy
                    </button>
                </div>
            </div>
        </div>
    );
};

export default ProxiesPage;
