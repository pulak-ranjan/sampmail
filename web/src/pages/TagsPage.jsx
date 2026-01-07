import React, { useState, useEffect } from 'react';
import { Tag } from 'lucide-react';

const TagsPage = () => {
    const [tags, setTags] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [editTag, setEditTag] = useState(null);

    useEffect(() => {
        fetchTags();
    }, []);

    const fetchTags = async () => {
        try {
            const res = await fetch('/api/tags', {
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            const data = await res.json();
            setTags(data || []);
        } catch (error) {
            console.error('Failed to fetch tags:', error);
        } finally {
            setLoading(false);
        }
    };

    const createTag = async (data) => {
        try {
            await fetch('/api/tags', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
                },
                body: JSON.stringify(data),
            });
            setShowCreate(false);
            fetchTags();
        } catch (error) {
            console.error('Failed to create tag:', error);
        }
    };

    const updateTag = async (id, data) => {
        try {
            await fetch(`/api/tags/${id}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
                },
                body: JSON.stringify(data),
            });
            setEditTag(null);
            fetchTags();
        } catch (error) {
            console.error('Failed to update tag:', error);
        }
    };

    const deleteTag = async (id) => {
        if (!confirm('Delete this tag?')) return;
        try {
            await fetch(`/api/tags/${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
            });
            fetchTags();
        } catch (error) {
            console.error('Failed to delete tag:', error);
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Tags</h1>
                    <p className="text-gray-500">Organize contacts with tags</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + New Tag
                </button>
            </div>

            {tags.length === 0 ? (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-12 text-center">
                    <div className="flex justify-center mb-4"><Tag className="w-16 h-16 text-gray-400" /></div>
                    <h3 className="text-xl font-semibold mb-2 dark:text-white">No tags yet</h3>
                    <p className="text-gray-500 mb-4">Create tags to organize your contacts</p>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                        Create Tag
                    </button>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {tags.map((tag) => (
                        <div
                            key={tag.id}
                            className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow flex items-center justify-between"
                        >
                            <div className="flex items-center gap-3">
                                <div
                                    className="w-4 h-4 rounded-full"
                                    style={{ backgroundColor: tag.color || '#3B82F6' }}
                                ></div>
                                <div>
                                    <h3 className="font-medium text-gray-900 dark:text-white">{tag.name}</h3>
                                    <p className="text-sm text-gray-500">{tag.contact_count || 0} contacts</p>
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <button
                                    onClick={() => setEditTag(tag)}
                                    className="text-blue-600 hover:underline text-sm"
                                >
                                    Edit
                                </button>
                                <button
                                    onClick={() => deleteTag(tag.id)}
                                    className="text-red-600 hover:underline text-sm"
                                >
                                    Delete
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {(showCreate || editTag) && (
                <TagModal
                    tag={editTag}
                    onClose={() => { setShowCreate(false); setEditTag(null); }}
                    onSave={editTag ? (data) => updateTag(editTag.id, data) : createTag}
                />
            )}
        </div>
    );
};

const TagModal = ({ tag, onClose, onSave }) => {
    const [name, setName] = useState(tag?.name || '');
    const [color, setColor] = useState(tag?.color || '#3B82F6');

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4 dark:text-white">{tag ? 'Edit Tag' : 'Create Tag'}</h2>
                <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
                    <input
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        placeholder="e.g., VIP Customer"
                    />
                </div>
                <div className="mb-6">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Color</label>
                    <div className="flex items-center gap-3">
                        <input
                            type="color"
                            value={color}
                            onChange={(e) => setColor(e.target.value)}
                            className="w-12 h-10 rounded cursor-pointer"
                        />
                        <input
                            type="text"
                            value={color}
                            onChange={(e) => setColor(e.target.value)}
                            className="flex-1 border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        />
                    </div>
                </div>
                <div className="flex justify-end gap-3">
                    <button onClick={onClose} className="px-4 py-2 text-gray-600">Cancel</button>
                    <button
                        onClick={() => onSave({ name, color })}
                        disabled={!name}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg disabled:opacity-50"
                    >
                        {tag ? 'Update' : 'Create'}
                    </button>
                </div>
            </div>
        </div>
    );
};

export default TagsPage;

