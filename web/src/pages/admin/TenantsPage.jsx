import React, { useState, useEffect } from "react";
import { adminListTenants, adminCreateTenant, adminDeleteTenant } from "../../api";
import { Plus, Trash2, ExternalLink, Building, Users } from "lucide-react";

export default function TenantsPage() {
    const [tenants, setTenants] = useState([]);
    const [loading, setLoading] = useState(true);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [formData, setFormData] = useState({ name: "", slug: "", plan: "free" });
    const [error, setError] = useState(null);

    useEffect(() => {
        loadTenants();
    }, []);

    const loadTenants = async () => {
        try {
            const data = await adminListTenants();
            setTenants(data);
        } catch (err) {
            console.error(err);
            setError("Failed to load tenants");
        } finally {
            setLoading(false);
        }
    };

    const handleCreate = async (e) => {
        e.preventDefault();
        try {
            await adminCreateTenant(formData);
            setIsModalOpen(false);
            setFormData({ name: "", slug: "", plan: "free" });
            loadTenants();
        } catch (err) {
            console.error(err);
            setError("Failed to create tenant");
        }
    };

    const handleDelete = async (id) => {
        if (!window.confirm("Are you sure? This will delete the organization and all its data!")) return;
        try {
            await adminDeleteTenant(id);
            loadTenants();
        } catch (err) {
            console.error(err);
            setError("Failed to delete tenant");
        }
    };

    if (loading) return <div className="p-8 text-center text-gray-400">Loading tenants...</div>;

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold text-white">Tenants</h1>
                    <p className="text-gray-400">Manage all organizations (Superadmin)</p>
                </div>
                <button
                    onClick={() => setIsModalOpen(true)}
                    className="flex items-center space-x-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                    <Plus size={18} />
                    <span>Create Tenant</span>
                </button>
            </div>

            {error && <div className="p-4 bg-red-900/20 text-red-400 rounded-lg border border-red-800">{error}</div>}

            <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
                <table className="w-full text-left">
                    <thead className="bg-gray-800/50">
                        <tr>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">ID</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">Organization</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">Slug</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">Plan</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">Limits</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase">Created</th>
                            <th className="px-6 py-3 text-xs font-medium text-gray-400 uppercase text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-800">
                        {tenants.map((t) => (
                            <tr key={t.id} className="hover:bg-gray-800/30 transition-colors">
                                <td className="px-6 py-4 text-sm text-gray-500">#{t.id}</td>
                                <td className="px-6 py-4">
                                    <div className="flex items-center space-x-3">
                                        <div className="w-8 h-8 rounded bg-gray-800 flex items-center justify-center text-gray-400">
                                            <Building size={16} />
                                        </div>
                                        <div>
                                            <div className="font-medium text-white">{t.name}</div>
                                            <div className="text-xs text-blue-400 flex items-center space-x-1">
                                                <Users size={12} />
                                                <span>{t.users_count || 1} Users</span>
                                            </div>
                                        </div>
                                    </div>
                                </td>
                                <td className="px-6 py-4 text-sm text-gray-400 font-mono">{t.slug}</td>
                                <td className="px-6 py-4">
                                    <span className={`px-2 py-1 text-xs rounded-full border ${t.plan === 'enterprise' ? 'bg-purple-900/20 border-purple-800 text-purple-400' :
                                            t.plan === 'pro' ? 'bg-blue-900/20 border-blue-800 text-blue-400' :
                                                'bg-gray-800 text-gray-400 border-gray-700'
                                        }`}>
                                        {t.plan.toUpperCase()}
                                    </span>
                                </td>
                                <td className="px-6 py-4 text-sm text-gray-400">
                                    <div>User Max: {t.max_users}</div>
                                    <div>Emails: {t.max_emails_per_month}</div>
                                </td>
                                <td className="px-6 py-4 text-sm text-gray-500">
                                    {new Date(t.created_at).toLocaleDateString()}
                                </td>
                                <td className="px-6 py-4 text-right">
                                    <div className="flex items-center justify-end space-x-2">
                                        <button
                                            onClick={() => handleDelete(t.id)}
                                            className="p-1 text-gray-400 hover:text-red-400 transition-colors"
                                            title="Delete Tenant"
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                        <button className="p-1 text-gray-400 hover:text-white transition-colors">
                                            <ExternalLink size={16} />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                        {tenants.length === 0 && (
                            <tr>
                                <td colSpan="7" className="px-6 py-8 text-center text-gray-500">
                                    No organizations found.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {/* Create Modal */}
            {isModalOpen && (
                <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                    <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 w-full max-w-md shadow-2xl">
                        <h2 className="text-xl font-bold mb-4 text-white">Create Organization</h2>
                        <form onSubmit={handleCreate} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-400 mb-1">Name</label>
                                <input
                                    type="text"
                                    required
                                    value={formData.name}
                                    onChange={(e) => setFormData({ ...formData, name: e.target.value, slug: e.target.value.toLowerCase().replace(/\s+/g, '-') })}
                                    className="w-full bg-gray-800 border border-gray-700 rounded p-2 text-white focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-400 mb-1">Slug</label>
                                <input
                                    type="text"
                                    required
                                    value={formData.slug}
                                    onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
                                    className="w-full bg-gray-800 border border-gray-700 rounded p-2 text-white focus:ring-2 focus:ring-blue-500 focus:outline-none font-mono"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-400 mb-1">Plan</label>
                                <select
                                    value={formData.plan}
                                    onChange={(e) => setFormData({ ...formData, plan: e.target.value })}
                                    className="w-full bg-gray-800 border border-gray-700 rounded p-2 text-white focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                >
                                    <option value="free">Free</option>
                                    <option value="starter">Starter</option>
                                    <option value="pro">Pro</option>
                                    <option value="enterprise">Enterprise</option>
                                </select>
                            </div>
                            <div className="flex justify-end space-x-3 mt-6">
                                <button
                                    type="button"
                                    onClick={() => setIsModalOpen(false)}
                                    className="px-4 py-2 text-gray-300 hover:text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-medium"
                                >
                                    Create
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
