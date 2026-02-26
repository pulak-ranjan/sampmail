import React, { useState, useEffect } from 'react';
import api from '../api';

const SegmentsPage = () => {
    const [segments, setSegments] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);

    useEffect(() => {
        fetchSegments();
    }, []);

    const fetchSegments = async () => {
        try {
            const data = await api.get('/v2/segments');
            setSegments(data || []);
        } catch (error) {
            console.error('Failed to fetch segments:', error);
        } finally {
            setLoading(false);
        }
    };

    const createSegment = async (data) => {
        try {
            await api.post('/v2/segments', data);
            setShowCreate(false);
            fetchSegments();
        } catch (error) {
            console.error('Failed to create segment:', error);
        }
    };

    const deleteSegment = async (id) => {
        if (!confirm('Delete this segment?')) return;
        try {
            await api.delete(`/v2/segments/${id}`);
            fetchSegments();
        } catch (error) {
            console.error('Failed to delete segment:', error);
        }
    };

    const refreshSegment = async (id) => {
        try {
            await api.post(`/v2/segments/${id}/refresh`, {});
            fetchSegments();
        } catch (error) {
            console.error('Failed to refresh segment:', error);
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
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Segments</h1>
                    <p className="text-gray-500">Dynamic contact groups based on conditions</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                    + New Segment
                </button>
            </div>

            {segments.length === 0 ? (
                <div className="bg-white dark:bg-gray-800 rounded-xl p-12 text-center">
                    <div className="text-6xl mb-4">📊</div>
                    <h3 className="text-xl font-semibold mb-2 dark:text-white">No segments yet</h3>
                    <p className="text-gray-500 mb-4">Create segments to target specific contacts</p>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                        Create Segment
                    </button>
                </div>
            ) : (
                <div className="grid gap-4">
                    {segments.map((segment) => (
                        <div
                            key={segment.id}
                            className="bg-white dark:bg-gray-800 rounded-xl p-6 shadow"
                        >
                            <div className="flex items-center justify-between">
                                <div>
                                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{segment.name}</h3>
                                    <p className="text-sm text-gray-500">{segment.description || 'No description'}</p>
                                    <div className="flex items-center gap-4 mt-2 text-sm">
                                        <span className="text-blue-600">{segment.subscriber_count || 0} contacts</span>
                                        <span className="text-gray-400">
                                            Updated: {segment.last_refreshed ? new Date(segment.last_refreshed).toLocaleString() : 'Never'}
                                        </span>
                                    </div>
                                </div>
                                <div className="flex items-center gap-2">
                                    <button
                                        onClick={() => refreshSegment(segment.id)}
                                        className="px-3 py-1 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200"
                                    >
                                        🔄 Refresh
                                    </button>
                                    <button
                                        onClick={() => deleteSegment(segment.id)}
                                        className="px-3 py-1 bg-red-100 text-red-600 rounded hover:bg-red-200"
                                    >
                                        Delete
                                    </button>
                                </div>
                            </div>
                            {segment.conditions && (
                                <div className="mt-4 p-3 bg-gray-50 dark:bg-gray-700 rounded text-sm">
                                    <span className="text-gray-500 dark:text-gray-400">Conditions: </span>
                                    <code className="text-gray-700 dark:text-gray-300">{segment.conditions}</code>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}

            {showCreate && <CreateSegmentModal onClose={() => setShowCreate(false)} onCreate={createSegment} />}
        </div>
    );
};

const CreateSegmentModal = ({ onClose, onCreate }) => {
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [conditions, setConditions] = useState('');

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-lg">
                <h2 className="text-xl font-bold mb-4 dark:text-white">Create Segment</h2>
                <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
                    <input
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        placeholder="e.g., Active Subscribers"
                    />
                </div>
                <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
                    <input
                        type="text"
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white"
                        placeholder="Optional description"
                    />
                </div>
                <div className="mb-6">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Conditions (JSON)</label>
                    <textarea
                        value={conditions}
                        onChange={(e) => setConditions(e.target.value)}
                        className="w-full border dark:border-gray-600 rounded-lg p-2 dark:bg-gray-700 dark:text-white font-mono text-sm"
                        rows={4}
                        placeholder='{"field": "lead_score", "operator": ">", "value": 50}'
                    />
                    <p className="text-xs text-gray-500 mt-1">Define conditions to filter contacts dynamically</p>
                </div>
                <div className="flex justify-end gap-3">
                    <button onClick={onClose} className="px-4 py-2 text-gray-600">Cancel</button>
                    <button
                        onClick={() => onCreate({ name, description, conditions })}
                        disabled={!name}
                        className="px-6 py-2 bg-blue-600 text-white rounded-lg disabled:opacity-50"
                    >
                        Create
                    </button>
                </div>
            </div>
        </div>
    );
};

export default SegmentsPage;
