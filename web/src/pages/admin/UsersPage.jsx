import React, { useState, useEffect } from "react";
import { Plus, Trash2, Edit2, Shield, User, X, Check, Loader2, Building, Link } from "lucide-react";

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

export default function UsersPage() {
    const [users, setUsers] = useState([]);
    const [organizations, setOrganizations] = useState([]);
    const [loading, setLoading] = useState(true);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingUser, setEditingUser] = useState(null);
    const [formData, setFormData] = useState({ email: "", password: "", is_super_admin: false });
    const [error, setError] = useState(null);
    const [saving, setSaving] = useState(false);

    // Organization assignment state
    const [assignModalUser, setAssignModalUser] = useState(null);
    const [userOrgs, setUserOrgs] = useState([]);
    const [selectedOrgId, setSelectedOrgId] = useState("");
    const [selectedRole, setSelectedRole] = useState("admin");
    const [loadingOrgs, setLoadingOrgs] = useState(false);

    useEffect(() => {
        loadUsers();
        loadOrganizations();
    }, []);

    const loadUsers = async () => {
        try {
            const data = await apiRequest("/admin/users");
            setUsers(Array.isArray(data) ? data : []);
        } catch (err) {
            console.error(err);
            setError("Failed to load users");
        } finally {
            setLoading(false);
        }
    };

    const loadOrganizations = async () => {
        try {
            const data = await apiRequest("/v2/admin/organizations");
            setOrganizations(Array.isArray(data) ? data : []);
        } catch (err) {
            console.error(err);
        }
    };

    const loadUserOrgs = async (userId) => {
        setLoadingOrgs(true);
        try {
            const data = await apiRequest(`/admin/users/${userId}/organizations`);
            setUserOrgs(Array.isArray(data) ? data : []);
        } catch (err) {
            console.error(err);
            setUserOrgs([]);
        } finally {
            setLoadingOrgs(false);
        }
    };

    const handleCreate = async (e) => {
        e.preventDefault();
        setSaving(true);
        try {
            await apiRequest("/admin/users", { method: "POST", body: formData });
            setIsModalOpen(false);
            setFormData({ email: "", password: "", is_super_admin: false });
            loadUsers();
        } catch (err) {
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    const handleUpdate = async (e) => {
        e.preventDefault();
        setSaving(true);
        try {
            const updateData = { ...formData };
            if (!updateData.password) delete updateData.password;
            await apiRequest(`/admin/users/${editingUser.id}`, { method: "PUT", body: updateData });
            setEditingUser(null);
            setFormData({ email: "", password: "", is_super_admin: false });
            loadUsers();
        } catch (err) {
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    const handleDelete = async (id) => {
        if (!window.confirm("Are you sure you want to delete this user?")) return;
        try {
            await apiRequest(`/admin/users/${id}`, { method: "DELETE" });
            loadUsers();
        } catch (err) {
            setError(err.message);
        }
    };

    const openEditModal = (user) => {
        setEditingUser(user);
        setFormData({ email: user.email, password: "", is_super_admin: user.is_super_admin });
    };

    const closeModal = () => {
        setIsModalOpen(false);
        setEditingUser(null);
        setFormData({ email: "", password: "", is_super_admin: false });
        setError(null);
    };

    const openAssignModal = (user) => {
        setAssignModalUser(user);
        loadUserOrgs(user.id);
        setSelectedOrgId("");
        setSelectedRole("admin");
    };

    const closeAssignModal = () => {
        setAssignModalUser(null);
        setUserOrgs([]);
        setSelectedOrgId("");
    };

    const handleAssignOrg = async () => {
        if (!selectedOrgId) return;
        setSaving(true);
        try {
            await apiRequest(`/admin/users/${assignModalUser.id}/organizations`, {
                method: "POST",
                body: { organization_id: parseInt(selectedOrgId), role: selectedRole }
            });
            loadUserOrgs(assignModalUser.id);
            setSelectedOrgId("");
        } catch (err) {
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    const handleRemoveOrg = async (orgId) => {
        if (!window.confirm("Remove user from this organization?")) return;
        try {
            await apiRequest(`/admin/users/${assignModalUser.id}/organizations/${orgId}`, { method: "DELETE" });
            loadUserOrgs(assignModalUser.id);
        } catch (err) {
            setError(err.message);
        }
    };

    if (loading) return <div className="p-8 text-center text-muted-foreground">Loading users...</div>;

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold text-foreground">Users</h1>
                    <p className="text-muted-foreground">Manage admin users and their organizations</p>
                </div>
                <button
                    onClick={() => setIsModalOpen(true)}
                    className="flex items-center space-x-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                    <Plus size={18} />
                    <span>Create User</span>
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
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Email</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">Role</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase">2FA</th>
                            <th className="px-6 py-3 text-xs font-medium text-muted-foreground uppercase text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                        {users.map((u) => (
                            <tr key={u.id} className="hover:bg-muted/30 transition-colors">
                                <td className="px-6 py-4 text-sm text-muted-foreground">#{u.id}</td>
                                <td className="px-6 py-4">
                                    <div className="flex items-center space-x-3">
                                        <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white font-medium">
                                            {u.email.charAt(0).toUpperCase()}
                                        </div>
                                        <span className="font-medium text-foreground">{u.email}</span>
                                    </div>
                                </td>
                                <td className="px-6 py-4">
                                    {u.is_super_admin ? (
                                        <span className="inline-flex items-center px-2 py-1 text-xs rounded-full bg-purple-500/20 text-purple-400 border border-purple-500/30">
                                            <Shield size={12} className="mr-1" /> Super Admin
                                        </span>
                                    ) : (
                                        <span className="inline-flex items-center px-2 py-1 text-xs rounded-full bg-gray-500/20 text-gray-400 border border-gray-500/30">
                                            <User size={12} className="mr-1" /> Admin
                                        </span>
                                    )}
                                </td>
                                <td className="px-6 py-4">
                                    {u.two_fa_enabled ? (
                                        <span className="text-green-400"><Check size={16} /></span>
                                    ) : (
                                        <span className="text-muted-foreground">-</span>
                                    )}
                                </td>
                                <td className="px-6 py-4 text-right">
                                    <div className="flex items-center justify-end space-x-2">
                                        <button
                                            onClick={() => openAssignModal(u)}
                                            className="p-2 text-muted-foreground hover:text-blue-400 hover:bg-blue-500/10 rounded transition-colors"
                                            title="Manage Organizations"
                                        >
                                            <Building size={16} />
                                        </button>
                                        <button
                                            onClick={() => openEditModal(u)}
                                            className="p-2 text-muted-foreground hover:text-foreground hover:bg-muted rounded transition-colors"
                                            title="Edit User"
                                        >
                                            <Edit2 size={16} />
                                        </button>
                                        <button
                                            onClick={() => handleDelete(u.id)}
                                            className="p-2 text-muted-foreground hover:text-red-400 hover:bg-red-500/10 rounded transition-colors"
                                            title="Delete User"
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                        {users.length === 0 && (
                            <tr>
                                <td colSpan="5" className="px-6 py-8 text-center text-muted-foreground">
                                    No users found. Create your first user above.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {/* Create/Edit Modal */}
            {(isModalOpen || editingUser) && (
                <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                    <div className="bg-card border border-border rounded-lg p-6 w-full max-w-md shadow-2xl">
                        <h2 className="text-xl font-bold mb-4 text-foreground">
                            {editingUser ? "Edit User" : "Create User"}
                        </h2>
                        <form onSubmit={editingUser ? handleUpdate : handleCreate} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-1">Email</label>
                                <input
                                    type="email"
                                    required
                                    value={formData.email}
                                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                                    className="w-full bg-background border border-border rounded-lg p-2.5 text-foreground focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                    placeholder="user@example.com"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-1">
                                    Password {editingUser && "(leave blank to keep current)"}
                                </label>
                                <input
                                    type="password"
                                    required={!editingUser}
                                    value={formData.password}
                                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                    className="w-full bg-background border border-border rounded-lg p-2.5 text-foreground focus:ring-2 focus:ring-blue-500 focus:outline-none"
                                    placeholder="••••••••"
                                />
                            </div>
                            <div className="flex items-center space-x-2">
                                <input
                                    type="checkbox"
                                    id="is_super_admin"
                                    checked={formData.is_super_admin}
                                    onChange={(e) => setFormData({ ...formData, is_super_admin: e.target.checked })}
                                    className="w-4 h-4 rounded border-border"
                                />
                                <label htmlFor="is_super_admin" className="text-sm text-muted-foreground">
                                    Grant Super Admin privileges
                                </label>
                            </div>
                            <div className="flex justify-end space-x-3 mt-6">
                                <button
                                    type="button"
                                    onClick={closeModal}
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
                                    {editingUser ? "Save Changes" : "Create User"}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* Organization Assignment Modal */}
            {assignModalUser && (
                <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
                    <div className="bg-card border border-border rounded-lg p-6 w-full max-w-lg shadow-2xl">
                        <div className="flex items-center justify-between mb-4">
                            <h2 className="text-xl font-bold text-foreground">
                                Manage Organizations for {assignModalUser.email}
                            </h2>
                            <button onClick={closeAssignModal} className="text-muted-foreground hover:text-foreground">
                                <X size={20} />
                            </button>
                        </div>

                        {/* Current Organizations */}
                        <div className="mb-6">
                            <h3 className="text-sm font-medium text-muted-foreground mb-2">Current Organizations</h3>
                            {loadingOrgs ? (
                                <div className="text-center py-4 text-muted-foreground">Loading...</div>
                            ) : userOrgs.length === 0 ? (
                                <div className="text-center py-4 text-muted-foreground bg-muted/20 rounded-lg">
                                    User is not assigned to any organizations
                                </div>
                            ) : (
                                <div className="space-y-2">
                                    {userOrgs.map((membership) => (
                                        <div key={membership.id} className="flex items-center justify-between p-3 bg-muted/20 rounded-lg">
                                            <div className="flex items-center space-x-3">
                                                <Building size={16} className="text-blue-400" />
                                                <span className="font-medium">{membership.organization?.name || `Org #${membership.organization_id}`}</span>
                                                <span className="text-xs px-2 py-0.5 rounded bg-blue-500/20 text-blue-400">{membership.role}</span>
                                            </div>
                                            <button
                                                onClick={() => handleRemoveOrg(membership.organization_id)}
                                                className="p-1 text-muted-foreground hover:text-red-400"
                                                title="Remove from organization"
                                            >
                                                <X size={16} />
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>

                        {/* Add to Organization */}
                        <div className="border-t border-border pt-4">
                            <h3 className="text-sm font-medium text-muted-foreground mb-2">Add to Organization</h3>
                            <div className="flex items-center space-x-2">
                                <select
                                    value={selectedOrgId}
                                    onChange={(e) => setSelectedOrgId(e.target.value)}
                                    className="flex-1 bg-background border border-border rounded-lg p-2.5 text-foreground"
                                >
                                    <option value="">Select organization...</option>
                                    {organizations
                                        .filter(org => !userOrgs.some(m => m.organization_id === org.id))
                                        .map(org => (
                                            <option key={org.id} value={org.id}>{org.name}</option>
                                        ))
                                    }
                                </select>
                                <select
                                    value={selectedRole}
                                    onChange={(e) => setSelectedRole(e.target.value)}
                                    className="w-32 bg-background border border-border rounded-lg p-2.5 text-foreground"
                                >
                                    <option value="owner">Owner</option>
                                    <option value="admin">Admin</option>
                                    <option value="editor">Editor</option>
                                    <option value="viewer">Viewer</option>
                                </select>
                                <button
                                    onClick={handleAssignOrg}
                                    disabled={!selectedOrgId || saving}
                                    className="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg disabled:opacity-50 flex items-center"
                                >
                                    {saving ? <Loader2 size={16} className="animate-spin" /> : <Link size={16} />}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
