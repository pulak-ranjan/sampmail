import React, { useState, useEffect } from "react";
import { adminListTenants, adminCreateTenant, adminDeleteTenant } from "../../api";
import { Plus, Trash2, Building, Users, X, Loader2 } from "lucide-react";

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

export default function TenantsPage() {
    const [tenants, setTenants] = useState([]);
    const [loading, setLoading] = useState(true);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [formData, setFormData] = useState({ name: "", slug: "", plan: "free" });
    const [error, setError] = useState(null);
    const [saving, setSaving] = useState(false);

    // Members modal
    const [membersModalOrg, setMembersModalOrg] = useState(null);
    const [orgMembers, setOrgMembers] = useState([]);
    const [loadingMembers, setLoadingMembers] = useState(false);

    useEffect(() => {
        loadTenants();
    }, []);

    const loadTenants = async () => {
        try {
            const data = await adminListTenants();
            setTenants(Array.isArray(data) ? data : []);
        } catch (err) {
            console.error(err);
            setError("Failed to load tenants");
        } finally {
            setLoading(false);
        }
    };

    const loadOrgMembers = async (orgId) => {
        setLoadingMembers(true);
        try {
            const data = await apiRequest(`/v2/admin/organizations/${orgId}/members`);
            setOrgMembers(Array.isArray(data) ? data : []);
        } catch (err) {
            console.error(err);
            setOrgMembers([]);
        } finally {
            setLoadingMembers(false);
        }
    };

    const handleCreate = async (e) => {
        e.preventDefault();
        setSaving(true);
        try {
            await adminCreateTenant(formData);
            setIsModalOpen(false);
            setFormData({ name: "", slug: "", plan: "free" });
            loadTenants();
        } catch (err) {
            console.error(err);
            setError("Failed to create tenant");
        } finally {
            setSaving(false);
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

    const openMembersModal = (org) => {
        setMembersModalOrg(org);
        loadOrgMembers(org.id);
    };

    if (loading) return <div className="p-8 text-center text-gray-400">Loading tenants...</div>;

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold text-foreground">Organizations</h1>
                    <p className="text-muted-foreground">Manage all organizations (Superadmin)</p>
                </div>
                <button
                    onClick={() => setIsModalOpen(true)}
                    className="flex items-center space-x-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                    <Plus size={18} />
                    <span>Create Organization</span>
                </button>
            </div>

            {error && (
                <div className="p-4 bg-red-500/10 text-red-400 rounded-lg border border-red-500/20 flex items-center justify-between">
                    {error}
                    <button onClick={() => setError(null)}><X size={16} /></button>
                </div>
            )}

            <div className="bg-card border border-border rounded-lg overflow-hidden">
                <table className="w-full text-left">
                    <thead className="bg-muted/50">
                        <tr>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">ID</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Organization</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Slug</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Plan</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Limits</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Created</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                        {tenants.map((t) => (
                            <tr key={t.id} className="hover:bg-muted/30 transition-colors">
                                <td className="px-6 py-4 text-sm text-muted-foreground">#{t.id}</td>
                                <td className="px-6 py-4">
                                    <div className="flex items-center space-x-3">
                                        <div className="w-8 h-8 rounded bg-blue-600 flex items-center justify-center text-white font-medium">
                                            {t.name.charAt(0).toUpperCase()}
                                        </div>
                                        <div>
                                            <div className="font-medium text-foreground">{t.name}</div>
                                        </div>
                                    </div>
                                </td>
                                <td className="px-6 py-4 text-sm text-muted-foreground font-mono">{t.slug}</td>
                                <td className="px-6 py-4">
                                    <span className={`px-2 py-1 text-xs rounded-full border ${t.plan === 'enterprise' ? 'bg-purple-500/20 border-purple-500/30 text-purple-400' :
                                        t.plan === 'pro' ? 'bg-blue-500/20 border-blue-500/30 text-blue-400' :
                                            'bg-muted text-muted-foreground border-border'
                                        }`}>
                                        {(t.plan || 'free').toUpperCase()}
                                    </span>
                                </td>
                                <td className="px-6 py-4 text-sm text-muted-foreground">
                                    <div>Users: {t.max_users || 5}</div>
                                    <div>Emails: {t.max_emails_per_month || 1000}</div>
                                </td>
                                <td className="px-6 py-4 text-sm text-muted-foreground">
                                    {t.created_at ? new Date(t.created_at).toLocaleDateString() : '-'}
                                </td>
                                <td className="px-6 py-4 text-right">
                                    <div className="flex items-center justify-end space-x-2">
                                        <button
                                            onClick={() => openMembersModal(t)}
                                            className="p-2 text-muted-foreground hover:text-blue-400 hover:bg-blue-500/10 rounded transition-colors"
                                            title="View Members"
                                        >
                                            <Users size={16} />
                                        </button>
                                        <button
                                            onClick={() => handleDelete(t.id)}
                                            className="p-2 text-muted-foreground hover:text-red-400 hover:bg-red-500/10 rounded transition-colors"
                                            title="Delete Organization"
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                        {tenants.length === 0 && (
                            <tr>
                                <td colSpan="7" className="px-6 py-8 text-center text-muted-foreground">
                                    No organizations found. Create your first organization above.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {/* Create Modal */}
            {isModalOpen && (
                <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                    <div className="bg-card border border-border rounded-lg p-6 w-full max-w-md shadow-2xl">
                        <h2 className="text-xl font-bold mb-4 text-foreground">Create Organization</h2>
                        <form onSubmit={handleCreate} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-1">Name</label>
                                <input
                                    type="text"
                                    required
                                    value={formData.name}
                                    onChange={(e) => setFormData({ ...formData, name: e.target.value, slug: e.target.value.toLowerCase().replace(/\s+/g, '-') })}
                                    className="w-full bg-background border border-border rounded-lg p-2.5 text-foreground focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                    placeholder="My Organization"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-1">Slug</label>
                                <input
                                    type="text"
                                    required
                                    value={formData.slug}
                                    onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
                                    className="w-full bg-background border border-border rounded-lg p-2.5 text-foreground focus:ring-2 focus:ring-blue-500 focus:outline-none font-mono"
                                    placeholder="my-organization"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-1">Plan</label>
                                <select
                                    value={formData.plan}
                                    onChange={(e) => setFormData({ ...formData, plan: e.target.value })}
                                    className="w-full bg-background border border-border rounded-lg p-2.5 text-foreground focus:ring-2 focus:ring-blue-500 focus:outline-none"
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
                                    className="px-4 py-2 text-muted-foreground hover:text-foreground"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={saving}
                                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium flex items-center disabled:opacity-50"
                                >
                                    {saving && <Loader2 size={16} className="mr-2 animate-spin" />}
                                    Create
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* Members Modal */}
            {membersModalOrg && (
                <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                    <div className="bg-card border border-border rounded-lg p-6 w-full max-w-lg shadow-2xl">
                        <div className="flex items-center justify-between mb-4">
                            <h2 className="text-xl font-bold text-foreground flex items-center">
                                <Building size={20} className="mr-2 text-blue-400" />
                                {membersModalOrg.name} - Members
                            </h2>
                            <button onClick={() => setMembersModalOrg(null)} className="text-muted-foreground hover:text-foreground">
                                <X size={20} />
                            </button>
                        </div>

                        {loadingMembers ? (
                            <div className="text-center py-8 text-muted-foreground">
                                <Loader2 size={24} className="animate-spin mx-auto mb-2" />
                                Loading members...
                            </div>
                        ) : orgMembers.length === 0 ? (
                            <div className="text-center py-8 text-muted-foreground bg-muted/20 rounded-lg">
                                <Users size={32} className="mx-auto mb-2 opacity-50" />
                                No members assigned to this organization.
                                <p className="text-sm mt-2">Go to Users page to assign users.</p>
                            </div>
                        ) : (
                            <div className="space-y-2">
                                {orgMembers.map((m) => (
                                    <div key={m.id} className="flex items-center justify-between p-3 bg-muted/20 rounded-lg">
                                        <div className="flex items-center space-x-3">
                                            <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white text-sm font-medium">
                                                {m.email ? m.email.charAt(0).toUpperCase() : '?'}
                                            </div>
                                            <div>
                                                <div className="font-medium text-foreground">{m.email}</div>
                                                <span className="text-xs px-2 py-0.5 rounded bg-blue-500/20 text-blue-400">{m.role}</span>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}

                        <div className="mt-6 pt-4 border-t border-border">
                            <p className="text-sm text-muted-foreground">
                                To add users to this organization, go to <strong>Users</strong> page and click the
                                <Building size={14} className="inline mx-1" /> icon for any user.
                            </p>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
