import React, { useState, useEffect } from 'react';

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
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
          Authorization: `Bearer ${localStorage.getItem('token')}`,
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

  const deleteList = async (listId) => {
    if (!confirm('Are you sure you want to delete this list?')) return;

    try {
      await fetch(`/api/v2/lists/${listId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      });
      setLists(lists.filter(l => l.id !== listId));
      if (selectedList?.id === listId) {
        setSelectedList(null);
      }
    } catch (error) {
      console.error('Failed to delete list:', error);
    }
  };

  const addSubscriber = async (subscriberData) => {
    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('token')}`,
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

  const removeSubscriber = async (contactId) => {
    if (!confirm('Remove this subscriber from the list?')) return;

    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers/${contactId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      });
      fetchSubscribers();
      fetchLists();
    } catch (error) {
      console.error('Failed to remove subscriber:', error);
    }
  };

  const unsubscribeContact = async (contactId) => {
    try {
      await fetch(`/api/v2/lists/${selectedList.id}/subscribers/${contactId}/unsubscribe`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
    <div className="flex h-full">
      {/* Lists Sidebar */}
      <div className="w-80 bg-white border-r overflow-y-auto">
        <div className="p-4 border-b">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Subscriber Lists</h2>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-3 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
            >
              + New List
            </button>
          </div>
        </div>

        <div className="divide-y">
          {lists.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              <p>No lists yet</p>
              <button
                onClick={() => setShowCreateModal(true)}
                className="mt-2 text-blue-600 hover:underline"
              >
                Create your first list
              </button>
            </div>
          ) : (
            lists.map((list) => (
              <div
                key={list.id}
                onClick={() => setSelectedList(list)}
                className={`p-4 cursor-pointer hover:bg-gray-50 ${selectedList?.id === list.id ? 'bg-blue-50 border-l-4 border-blue-600' : ''
                  }`}
              >
                <div className="flex items-center justify-between">
                  <h3 className="font-medium">{list.name}</h3>
                  <span className="text-sm text-gray-500">
                    {list.subscriber_count || 0}
                  </span>
                </div>
                {list.description && (
                  <p className="text-sm text-gray-500 mt-1 truncate">{list.description}</p>
                )}
                <div className="flex items-center gap-2 mt-2 text-xs">
                  <span className="px-2 py-0.5 bg-green-100 text-green-700 rounded">
                    {list.active_count || 0} active
                  </span>
                  {list.unsubscribed_count > 0 && (
                    <span className="px-2 py-0.5 bg-gray-100 text-gray-600 rounded">
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
      <div className="flex-1 overflow-y-auto">
        {selectedList ? (
          <>
            {/* List Header */}
            <div className="bg-white border-b p-4">
              <div className="flex items-center justify-between">
                <div>
                  <h1 className="text-xl font-semibold">{selectedList.name}</h1>
                  {selectedList.description && (
                    <p className="text-gray-500 mt-1">{selectedList.description}</p>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowAddModal(true)}
                    className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                  >
                    + Add Subscriber
                  </button>
                  <button
                    onClick={() => setShowImportModal(true)}
                    className="px-4 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
                  >
                    📤 Import CSV
                  </button>
                  <button
                    onClick={exportList}
                    className="px-4 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
                  >
                    📥 Export
                  </button>
                  <button
                    onClick={() => deleteList(selectedList.id)}
                    className="px-4 py-2 bg-red-100 text-red-600 rounded hover:bg-red-200"
                  >
                    🗑️ Delete
                  </button>
                </div>
              </div>

              {/* Filters */}
              <div className="flex items-center gap-4 mt-4">
                <div className="flex-1">
                  <input
                    type="text"
                    placeholder="Search subscribers..."
                    value={search}
                    onChange={(e) => {
                      setSearch(e.target.value);
                      setPagination({ ...pagination, page: 1 });
                    }}
                    className="w-full px-4 py-2 border rounded-lg"
                  />
                </div>
                <select
                  value={statusFilter}
                  onChange={(e) => {
                    setStatusFilter(e.target.value);
                    setPagination({ ...pagination, page: 1 });
                  }}
                  className="px-4 py-2 border rounded-lg"
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
            <div className="p-4">
              <table className="w-full bg-white rounded-lg shadow">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Email</th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Name</th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Company</th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Status</th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Score</th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-gray-600">Subscribed</th>
                    <th className="px-4 py-3 text-right text-sm font-medium text-gray-600">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {subscribers.length === 0 ? (
                    <tr>
                      <td colSpan="7" className="px-4 py-8 text-center text-gray-500">
                        No subscribers found
                      </td>
                    </tr>
                  ) : (
                    subscribers.map((sub) => (
                      <tr key={sub.id} className="hover:bg-gray-50">
                        <td className="px-4 py-3 text-sm">{sub.email}</td>
                        <td className="px-4 py-3 text-sm">
                          {sub.first_name} {sub.last_name}
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">{sub.company || '-'}</td>
                        <td className="px-4 py-3">
                          <StatusBadge status={sub.status} />
                        </td>
                        <td className="px-4 py-3 text-sm">{sub.lead_score || 0}</td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {new Date(sub.subscribed_at).toLocaleDateString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-2">
                            {sub.status === 'active' && (
                              <button
                                onClick={() => unsubscribeContact(sub.contact_id)}
                                className="text-gray-400 hover:text-yellow-600"
                                title="Unsubscribe"
                              >
                                📤
                              </button>
                            )}
                            <button
                              onClick={() => removeSubscriber(sub.contact_id)}
                              className="text-gray-400 hover:text-red-600"
                              title="Remove"
                            >
                              🗑️
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>

              {/* Pagination */}
              {pagination.totalPages > 1 && (
                <div className="flex items-center justify-between mt-4">
                  <p className="text-sm text-gray-500">
                    Showing {(pagination.page - 1) * 50 + 1} to{' '}
                    {Math.min(pagination.page * 50, pagination.total)} of {pagination.total}
                  </p>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setPagination({ ...pagination, page: pagination.page - 1 })}
                      disabled={pagination.page === 1}
                      className="px-3 py-1 border rounded disabled:opacity-50"
                    >
                      Previous
                    </button>
                    <span className="text-sm">
                      Page {pagination.page} of {pagination.totalPages}
                    </span>
                    <button
                      onClick={() => setPagination({ ...pagination, page: pagination.page + 1 })}
                      disabled={pagination.page === pagination.totalPages}
                      className="px-3 py-1 border rounded disabled:opacity-50"
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
              <p className="text-xl mb-2">📋</p>
              <p>Select a list to view subscribers</p>
            </div>
          </div>
        )}
      </div>

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
    active: 'bg-green-100 text-green-700',
    unsubscribed: 'bg-gray-100 text-gray-600',
    bounced: 'bg-red-100 text-red-600',
    pending: 'bg-yellow-100 text-yellow-700',
  };

  return (
    <span className={`px-2 py-0.5 text-xs rounded ${colors[status] || colors.active}`}>
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl p-6 w-full max-w-md">
        <h2 className="text-xl font-bold mb-4">Create New List</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              List Name *
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Newsletter Subscribers"
              className="w-full border rounded-lg p-2"
              required
            />
          </div>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              className="w-full border rounded-lg p-2"
              rows={3}
            />
          </div>
          <div className="mb-6">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={doubleOptin}
                onChange={(e) => setDoubleOptin(e.target.checked)}
                className="rounded"
              />
              <span className="text-sm text-gray-700">Enable double opt-in</span>
            </label>
          </div>
          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-600 hover:text-gray-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!name}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl p-6 w-full max-w-md">
        <h2 className="text-xl font-bold mb-4">Add Subscriber</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Email *
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="subscriber@example.com"
              className="w-full border rounded-lg p-2"
              required
            />
          </div>
          <div className="grid grid-cols-2 gap-4 mb-6">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                First Name
              </label>
              <input
                type="text"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className="w-full border rounded-lg p-2"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Last Name
              </label>
              <input
                type="text"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="w-full border rounded-lg p-2"
              />
            </div>
          </div>
          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-600 hover:text-gray-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!email}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
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
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl p-6 w-full max-w-md">
        <h2 className="text-xl font-bold mb-4">Import Subscribers</h2>

        {result ? (
          <div>
            <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
              <h3 className="font-medium text-green-800 mb-2">Import Complete!</h3>
              <ul className="text-sm text-green-700 space-y-1">
                <li>✅ Imported: {result.imported}</li>
                <li>⏭️ Skipped (duplicates): {result.skipped}</li>
                <li>❌ Failed: {result.failed}</li>
              </ul>
            </div>
            <button
              onClick={onComplete}
              className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            >
              Done
            </button>
          </div>
        ) : (
          <>
            <div className="mb-4">
              <p className="text-sm text-gray-600 mb-3">
                Upload a CSV file with subscriber data. Required column: <code>email</code>
              </p>
              <p className="text-sm text-gray-500 mb-3">
                Optional columns: <code>first_name</code>, <code>last_name</code>, <code>company</code>, <code>phone</code>
              </p>
              <input
                type="file"
                accept=".csv"
                onChange={(e) => setFile(e.target.files[0])}
                className="w-full border rounded-lg p-2"
              />
            </div>
            <div className="flex justify-end gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 text-gray-600 hover:text-gray-800"
              >
                Cancel
              </button>
              <button
                onClick={handleImport}
                disabled={!file || importing}
                className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {importing ? 'Importing...' : 'Import'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default ListsPage;
