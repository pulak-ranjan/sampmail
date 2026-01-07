import React, { useState, useEffect } from 'react';
import {
  Users, Plus, Search, Trash2, Upload, Download, UserMinus,
  FileText, CheckCircle, XCircle, AlertTriangle, UserPlus
} from 'lucide-react';
import ConfirmationModal from '../components/ConfirmationModal';
import { getAuthHeaders } from '../api';

const ListsPage = () => {
  const [lists, setLists] = useState([]);
  const [selectedList, setSelectedList] = useState(null);
  const [subscribers, setSubscribers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  const [showAddModal, setShowAddModal] = useState(false);
  const [pagination, setPagination] = useState({ page: 1, total: 0, totalPages: 0 });
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [confirmState, setConfirmState] = useState(null); // { type: 'deleteList' | 'removeSubscriber', id: string }

  // Fetch lists
  useEffect(() => {
    fetchLists();
  }, []);

  // Fetch subscribers when list changes
  useEffect(() => {
    if (selectedList) {
      fetchSubscribers();
    }
  }, [selectedList, pagination.page, search, statusFilter]);

  const fetchLists = async () => {
    try {
      const res = await fetch('/api/v2/lists', {
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
      });
      const data = await res.json();
      setLists(data);
    } catch (error) {
      console.error('Failed to fetch lists:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchSubscribers = async () => {
    if (!selectedList) return;

    try {
      const params = new URLSearchParams({
        page: pagination.page,
        limit: 50,
        ...(search && { search }),
        ...(statusFilter && { status: statusFilter }),
      });

      const res = await fetch(`/api/v2/lists/${selectedList.id}/subscribers?${params}`, {
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
      });
      const data = await res.json();
      setSubscribers(data.data || []);
      setPagination({
        page: data.page,
        total: data.total,
        totalPages: data.totalPages,
      });
    } catch (error) {
      console.error('Failed to fetch subscribers:', error);
    }
  };

  const createList = async (listData) => {
    try {
      const res = await fetch('/api/v2/lists', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
        },
        body: JSON.stringify(listData),
      });
      const data = await res.json();
      setLists([data, ...lists]);
      setShowCreateModal(false);
    } catch (error) {
      console.error('Failed to create list:', error);
    }
  };

  const deleteList = async () => {
    if (!confirmState?.id) return;
    const listId = confirmState.id;

    try {
      await fetch(`/api/v2/lists/${listId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
      });
      setLists(lists.filter(l => l.id !== listId));
      if (selectedList?.id === listId) {
        setSelectedList(null);
      }
    } catch (error) {
      console.error('Failed to delete list:', error);
    }
    setConfirmState(null);
  };

  const addSubscriber = async (subscriberData) => {
    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('sampmail_token')}`,
        },
        body: JSON.stringify(subscriberData),
      });
      setShowAddModal(false);
      fetchSubscribers();
      fetchLists(); // Refresh counts
    } catch (error) {
      console.error('Failed to add subscriber:', error);
    }
  };

  const removeSubscriber = async () => {
    if (!confirmState?.id) return;
    const contactId = confirmState.id;

    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers/${contactId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
      });
      fetchSubscribers();
      fetchLists();
    } catch (error) {
      console.error('Failed to remove subscriber:', error);
    }
    setConfirmState(null);
  };

  const unsubscribeContact = async (contactId) => {
    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers/${contactId}/unsubscribe`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
      });
      fetchSubscribers();
      fetchLists();
    } catch (error) {
      console.error('Failed to unsubscribe:', error);
    }
  };

  const exportList = () => {
    const params = statusFilter ? `?status=${statusFilter}` : '';
    window.open(`/api/v2/lists/${selectedList.id}/export${params}`, '_blank');
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="flex h-full bg-white dark:bg-gray-900">
      {/* Lists Sidebar */}
      <div className="w-80 border-r dark:border-gray-700 overflow-y-auto bg-gray-50/50 dark:bg-gray-800/50">
        <div className="p-4 border-b dark:border-gray-700 sticky top-0 bg-white/50 dark:bg-gray-800/50 backdrop-blur-sm z-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold dark:text-white flex items-center gap-2">
              <Users className="w-5 h-5 text-blue-600" /> Subscriber Lists
            </h2>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-3 py-1.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 flex items-center gap-1 shadow-sm"
            >
              <Plus className="w-4 h-4" /> New
            </button>
          </div>
        </div>

        <div className="divide-y dark:divide-gray-700">
          {lists.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              <p>No lists yet</p>
              <button
                onClick={() => setShowCreateModal(true)}
                className="mt-2 text-blue-600 hover:underline text-sm font-medium"
              >
                Create your first list
              </button>
            </div>
          ) : (
            lists.map((list) => (
              <div
                key={list.id}
                onClick={() => setSelectedList(list)}
                className={`p-4 cursor-pointer hover:bg-white dark:hover:bg-gray-700 transition-colors ${selectedList?.id === list.id ? 'bg-white dark:bg-gray-700 border-l-4 border-blue-600 shadow-sm' : ''
                  }`}
              >
                <div className="flex items-center justify-between">
                  <h3 className={`font-medium ${selectedList?.id === list.id ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}`}>{list.name}</h3>
                  <span className="text-sm text-gray-500 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded-full">
                    {list.subscriber_count || 0}
                  </span>
                </div>
                {list.description && (
                  <p className="text-sm text-gray-500 mt-1 truncate">{list.description}</p>
                )}
                <div className="flex items-center gap-2 mt-2 text-xs">
                  <span className="px-2 py-0.5 bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400 rounded border border-green-100 dark:border-green-800">
                    {list.active_count || 0} active
                  </span>
                  {list.unsubscribed_count > 0 && (
                    <span className="px-2 py-0.5 bg-gray-50 text-gray-600 dark:bg-gray-800 dark:text-gray-400 rounded border border-gray-200 dark:border-gray-700">
                      {list.unsubscribed_count} unsub
                    </span>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 overflow-y-auto bg-white dark:bg-gray-900">
        {selectedList ? (
          <>
            {/* List Header */}
            <div className="bg-white dark:bg-gray-800 border-b dark:border-gray-700 p-6 sticky top-0 z-10 shadow-sm">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h1 className="text-2xl font-bold dark:text-white mb-1">{selectedList.name}</h1>
                  {selectedList.description && (
                    <p className="text-gray-500 dark:text-gray-400">{selectedList.description}</p>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowAddModal(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2 shadow-sm font-medium transition-all"
                  >
                    <UserPlus className="w-4 h-4" /> Add Subscriber
                  </button>
                  <button
                    onClick={() => setShowImportModal(true)}
                    className="px-4 py-2 bg-white dark:bg-gray-700 border dark:border-gray-600 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 flex items-center gap-2 font-medium"
                  >
                    <Upload className="w-4 h-4" /> Import CSV
                  </button>
                  <button
                    onClick={exportList}
                    className="px-4 py-2 bg-white dark:bg-gray-700 border dark:border-gray-600 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 flex items-center gap-2 font-medium"
                  >
                    <Download className="w-4 h-4" /> Export
                  </button>
                  <button
                    onClick={() => setConfirmState({ type: 'deleteList', id: selectedList.id })}
                    className="px-4 py-2 bg-red-50 text-red-600 border border-red-100 rounded-lg hover:bg-red-100 flex items-center gap-2 font-medium transition-colors"
                  >
                    <Trash2 className="w-4 h-4" /> Delete
                  </button>
                </div>
              </div>

              {/* Filters */}
              <div className="flex items-center gap-4">
                <div className="flex-1 relative">
                  <Search className="absolute left-3 top-2.5 h-5 w-5 text-gray-400" />
                  <input
                    type="text"
                    placeholder="Search subscribers by email or name..."
                    value={search}
                    onChange={(e) => {
                      setSearch(e.target.value);
                      setPagination({ ...pagination, page: 1 });
                    }}
                    className="w-full pl-10 pr-4 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
                  />
                </div>
                <select
                  value={statusFilter}
                  onChange={(e) => {
                    setStatusFilter(e.target.value);
                    setPagination({ ...pagination, page: 1 });
                  }}
                  className="px-4 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
                >
                  <option value="">All Status</option>
                  <option value="active">Active</option>
                  <option value="unsubscribed">Unsubscribed</option>
                  <option value="bounced">Bounced</option>
                  <option value="pending">Pending</option>
                </select>
              </div>
            </div>

            {/* Subscribers Table */}
            <div className="p-6">
              <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border dark:border-gray-700 overflow-hidden">
                <table className="w-full">
                  <thead className="bg-gray-50 dark:bg-gray-700/50 border-b dark:border-gray-700">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Email</th>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Name</th>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Company</th>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Status</th>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Score</th>
                      <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Subscribed</th>
                      <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y dark:divide-gray-700">
                    {subscribers.length === 0 ? (
                      <tr>
                        <td colSpan="7" className="px-6 py-12 text-center text-gray-500">
                          <Users className="w-12 h-12 mx-auto text-gray-300 mb-2" />
                          No subscribers matching your filters
                        </td>
                      </tr>
                    ) : (
                      subscribers.map((sub) => (
                        <tr key={sub.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                          <td className="px-6 py-4 text-sm font-medium text-gray-900 dark:text-white">{sub.email}</td>
                          <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-300">
                            {sub.first_name} {sub.last_name}
                          </td>
                          <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">{sub.company || '-'}</td>
                          <td className="px-6 py-4">
                            <StatusBadge status={sub.status} />
                          </td>
                          <td className="px-6 py-4 text-sm text-gray-600 dark:text-gray-300">{sub.lead_score || 0}</td>
                          <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                            {new Date(sub.subscribed_at).toLocaleDateString()}
                          </td>
                          <td className="px-6 py-4 text-right">
                            <div className="flex items-center justify-end gap-2">
                              {sub.status === 'active' && (
                                <button
                                  onClick={() => unsubscribeContact(sub.contact_id)}
                                  className="text-gray-400 hover:text-yellow-600 p-1 rounded hover:bg-yellow-50 transition-colors"
                                  title="Unsubscribe"
                                >
                                  <UserMinus className="w-4 h-4" />
                                </button>
                              )}
                              <button
                                onClick={() => setConfirmState({ type: 'removeSubscriber', id: sub.contact_id })}
                                className="text-gray-400 hover:text-red-600 p-1 rounded hover:bg-red-50 transition-colors"
                                title="Remove"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              {pagination.totalPages > 1 && (
                <div className="flex items-center justify-between mt-6">
                  <p className="text-sm text-gray-500">
                    Showing {(pagination.page - 1) * 50 + 1} to{' '}
                    {Math.min(pagination.page * 50, pagination.total)} of {pagination.total} entries
                  </p>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setPagination({ ...pagination, page: pagination.page - 1 })}
                      disabled={pagination.page === 1}
                      className="px-3 py-1 border rounded disabled:opacity-50 hover:bg-gray-50 text-sm font-medium"
                    >
                      Previous
                    </button>
                    <span className="text-sm text-gray-600 font-medium">
                      Page {pagination.page} of {pagination.totalPages}
                    </span>
                    <button
                      onClick={() => setPagination({ ...pagination, page: pagination.page + 1 })}
                      disabled={pagination.page === pagination.totalPages}
                      className="px-3 py-1 border rounded disabled:opacity-50 hover:bg-gray-50 text-sm font-medium"
                    >
                      Next
                    </button>
                  </div>
                </div>
              )}
            </div>
          </>
        ) : (
          <div className="flex items-center justify-center h-full text-gray-500">
            <div className="text-center">
              <FileText className="w-16 h-16 mx-auto text-gray-200 mb-4" />
              <p className="text-lg font-medium text-gray-400">Select a list to view subscribers</p>
              <button
                onClick={() => setShowCreateModal(true)}
                className="mt-4 text-blue-600 hover:underline"
              >
                Or create a new list
              </button>
            </div>
          </div>
        )}
      </div>

      <ConfirmationModal
        isOpen={!!confirmState}
        onClose={() => setConfirmState(null)}
        onConfirm={confirmState?.type === 'deleteList' ? deleteList : removeSubscriber}
        title={confirmState?.type === 'deleteList' ? 'Delete List' : 'Remove Subscriber'}
        message={confirmState?.type === 'deleteList'
          ? 'Are you sure you want to delete this list? All subscribers in this list will be unlinked.'
          : 'Are you sure you want to remove this subscriber from the list?'}
        confirmText={confirmState?.type === 'deleteList' ? 'Delete List' : 'Remove'}
        type="danger"
      />

      {/* Create List Modal */}
      {showCreateModal && (
        <CreateListModal
          onClose={() => setShowCreateModal(false)}
          onCreate={createList}
        />
      )}

      {/* Import Modal */}
      {showImportModal && selectedList && (
        <ImportModal
          listId={selectedList.id}
          onClose={() => setShowImportModal(false)}
          onComplete={() => {
            setShowImportModal(false);
            fetchSubscribers();
            fetchLists();
          }}
        />
      )}

      {/* Add Subscriber Modal */}
      {showAddModal && (
        <AddSubscriberModal
          onClose={() => setShowAddModal(false)}
          onAdd={addSubscriber}
        />
      )}
    </div>
  );
};

const StatusBadge = ({ status }) => {
  const colors = {
    active: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 border-green-200 dark:border-green-800',
    unsubscribed: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 border-gray-200 dark:border-gray-700',
    bounced: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-800',
    pending: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400 border-yellow-200 dark:border-yellow-800',
  };

  return (
    <span className={`px-2 py-0.5 text-xs font-medium rounded border ${colors[status] || colors.active}`}>
      {status}
    </span>
  );
};

const CreateListModal = ({ onClose, onCreate }) => {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [doubleOptin, setDoubleOptin] = useState(false);

  const handleSubmit = (e) => {
    e.preventDefault();
    onCreate({ name, description, double_optin: doubleOptin });
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-2xl animate-in zoom-in-95 duration-200">
        <h2 className="text-xl font-bold mb-4 dark:text-white">Create New List</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              List Name *
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Newsletter Subscribers"
              className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              required
            />
          </div>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              rows={3}
            />
          </div>
          <div className="mb-6">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={doubleOptin}
                onChange={(e) => setDoubleOptin(e.target.checked)}
                className="rounded text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-700 dark:text-gray-300">Enable double opt-in</span>
            </label>
          </div>
          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg font-medium"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!name}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium shadow-sm"
            >
              Create List
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

const AddSubscriberModal = ({ onClose, onAdd }) => {
  const [email, setEmail] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    onAdd({ email, first_name: firstName, last_name: lastName, source: 'manual' });
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-2xl animate-in zoom-in-95 duration-200">
        <h2 className="text-xl font-bold mb-4 dark:text-white">Add Subscriber</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Email *
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="subscriber@example.com"
              className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              required
            />
          </div>
          <div className="grid grid-cols-2 gap-4 mb-6">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                First Name
              </label>
              <input
                type="text"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Last Name
              </label>
              <input
                type="text"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="w-full border dark:border-gray-600 rounded-lg p-2.5 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
          </div>
          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg font-medium"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!email}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium shadow-sm"
            >
              Add Subscriber
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

const ImportModal = ({ listId, onClose, onComplete }) => {
  const [file, setFile] = useState(null);
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState(null);

  const handleImport = async () => {
    if (!file) return;

    setImporting(true);
    try {
      const formData = new FormData();
      formData.append('file', file);

      const res = await fetch(`/api/v2/lists/${listId}/import`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${localStorage.getItem('sampmail_token')}` },
        body: formData,
      });
      const data = await res.json();
      setResult(data);
    } catch (error) {
      console.error('Import failed:', error);
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-2xl animate-in zoom-in-95 duration-200">
        <h2 className="text-xl font-bold mb-4 dark:text-white">Import Subscribers</h2>

        {result ? (
          <div>
            <div className="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4 mb-4">
              <h3 className="font-medium text-green-800 dark:text-green-400 mb-2 flex items-center gap-2">
                <CheckCircle className="w-5 h-5" /> Import Complete!
              </h3>
              <ul className="text-sm text-green-700 dark:text-green-300 space-y-1 ml-7">
                <li>Imported: <span className="font-bold">{result.imported}</span></li>
                <li>Skipped: <span className="font-bold">{result.skipped}</span></li>
                <li>Failed: <span className="font-bold">{result.failed}</span></li>
              </ul>
            </div>
            <button
              onClick={onComplete}
              className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm"
            >
              Done
            </button>
          </div>
        ) : (
          <>
            <div className="mb-6">
              <p className="text-sm text-gray-600 dark:text-gray-300 mb-3">
                Upload a CSV file with subscriber data. Required column: <code className="bg-gray-100 dark:bg-gray-700 px-1 rounded">email</code>
              </p>
              <div className="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-6 text-center hover:border-blue-500 transition-colors">
                <input
                  type="file"
                  accept=".csv"
                  onChange={(e) => setFile(e.target.files[0])}
                  className="hidden"
                  id="file-upload"
                />
                <label htmlFor="file-upload" className="cursor-pointer flex flex-col items-center">
                  <Upload className="w-8 h-8 text-gray-400 mb-2" />
                  <span className="text-sm font-medium text-blue-600 hover:text-blue-500">Click to upload</span>
                  <span className="text-xs text-gray-500 mt-1">{file ? file.name : 'or drag and drop CSV here'}</span>
                </label>
              </div>
            </div>
            <div className="flex justify-end gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg font-medium"
              >
                Cancel
              </button>
              <button
                onClick={handleImport}
                disabled={!file || importing}
                className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium shadow-sm"
              >
                {importing ? 'Importing...' : 'Import CSV'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default ListsPage;

