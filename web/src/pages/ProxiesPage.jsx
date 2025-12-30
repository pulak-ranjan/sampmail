import React, { useState, useEffect } from 'react';

const ProxiesPage = () => {
    const [proxies, setProxies] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [testing, setTesting] = useState(null);

    useEffect(() => {
        fetchProxies();
    }, []);

    const fetchProxies = async () => {
        try {
            const res = await fetch('/api/proxies', {
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                },
                body: JSON.stringify(data),
            });
            setShowCreate(false);
            fetchProxies();
        } catch (error) {
            console.error('Failed to create proxy:', error);
        }
    };

    const deleteProxy = async (id) => {
        if (!confirm('Delete this proxy?')) return;
        try {
            await fetch(`/api/proxies/${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            fetchProxies();
        } catch (error) {
            console.error('Failed to delete proxy:', error);
        }
    };

    const testProxy = async (id) => {
        setTesting(id);
        try {
            const res = await fetch(`/api/proxies/${id}/test`, {
                method: 'POST',
                headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            const data = await res.json();
            alert(data.success ? 'Proxy is working!' : `Proxy failed: ${data.error}`);
        } catch (error) {
            alert('Test failed');
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Proxies</h1>
                    <p className="text-gray-500">Configure SMTP and HTTP proxies</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + Add Proxy
                </button>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl shadow overflow-hidden">
                <table className="w-full">
                    <thead className="bg-gray-50 dark:bg-gray-700">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Type</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Host</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                        {proxies.length === 0 ? (
                            <tr>
                                <td colSpan="5" className="px-6 py-8 text-center text-gray-500">No proxies configured</td>
                            </tr>
                        ) : (
                            proxies.map((proxy) => (
                                <tr key={proxy.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                                    <td className="px-6 py-4 text-sm font-medium text-gray-900 dark:text-white">{proxy.name}</td>
                                    <td className="px-6 py-4 text-sm text-gray-500">
                                        <span className="px-2 py-1 bg-blue-100 text-blue-600 rounded text-xs">{proxy.type}</span>
                                    </td>
                                    <td className="px-6 py-4 text-sm text-gray-500">{proxy.host}:{proxy.port}</td>
                                    <td className="px-6 py-4">
                                        <span className={`px-2 py-1 text-xs rounded ${proxy.active ? 'bg-green-100 text-green-600' : 'bg-gray-100 text-gray-600'}`}>
                                            {proxy.active ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4 text-right space-x-2">
                                        <button
                                            onClick={() => testProxy(proxy.id)}
                                            disabled={testing === proxy.id}
                                            className="text-blue-600 hover:underline text-sm"
                                        >
                                            {testing === proxy.id ? 'Testing...' : 'Test'}
                                        </button>
                                        <button onClick={() => deleteProxy(proxy.id)} className="text-red-600 hover:underline text-sm">
                                            Delete
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

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
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Add Proxy</h2>
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
                        <input
                            type="text"
                            value={form.name}
                            onChange={(e) => setForm({ ...form, name: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                            placeholder="My Proxy"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Type</label>
                        <select
                            value={form.type}
                            onChange={(e) => setForm({ ...form, type: e.target.value })}
                            className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
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
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                                placeholder="proxy.example.com"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Port</label>
                            <input
                                type="number"
                                value={form.port}
                                onChange={(e) => setForm({ ...form, port: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
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
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                                placeholder="Optional"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                            <input
                                type="password"
                                value={form.password}
                                onChange={(e) => setForm({ ...form, password: e.target.value })}
                                className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                                placeholder="Optional"
                            />
                        </div>
                    </div>
                </div>
                <div className="flex justify-end gap-3 mt-6">
                    <button onClick={onClose} className="px-4 py-2 text-gray-600">Cancel</button>
                    <button
                        onClick={() => onCreate(form)}
                        disabled={!form.name || !form.host || !form.port}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg disabled:opacity-50"
                    >
                        Add
                    </button>
                </div>
            </div>
        </div>
    );
};

export default ProxiesPage;
