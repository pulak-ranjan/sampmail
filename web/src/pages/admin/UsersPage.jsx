import React, { useState, useEffect } from "react";
import { Plus, Trash2, Edit2, Shield, User, X, Check, Loader2 } from "lucide-react";

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
    const [loading, setLoading] = useState(true);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingUser, setEditingUser] = useState(null);
    const [formData, setFormData] = useState({ email: "", password: "", is_super_admin: false });
    const [error, setError] = useState(null);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        loadUsers();
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

    if (loading) return <div className="p-8 text-center text-muted-foreground">Loading users...</div>;

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold text-foreground">Users</h1>
                    <p className="text-muted-foreground">Manage admin users and their permissions</p>
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
        </div>
    );
}
