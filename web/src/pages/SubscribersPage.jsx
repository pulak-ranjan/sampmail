import { useState, useEffect } from 'react';
import { Users, Tag, Filter, Plus, Search, Upload, Download, Trash2, Edit, X, CheckCircle } from 'lucide-react';
import api from '../api';
import ConfirmationModal from '../components/ConfirmationModal';

export default function SubscribersPage() {
  const [activeTab, setActiveTab] = useState('subscribers');

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold dark:text-white">Subscriber Management</h1>
          <p className="text-gray-500">Manage contacts, tags, and segments</p>
        </div>
      </div>

      <div className="flex gap-1 mb-6 border-b dark:border-gray-700">
        {['subscribers', 'tags', 'segments'].map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 -mb-px flex items-center gap-2 capitalize font-medium transition-colors ${activeTab === tab
              ? 'border-b-2 border-blue-600 text-blue-600'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'
              }`}
          >
            {tab === 'subscribers' && <Users size={18} />}
            {tab === 'tags' && <Tag size={18} />}
            {tab === 'segments' && <Filter size={18} />}
            {tab}
          </button>
        ))}
      </div>

      {activeTab === 'subscribers' && <SubscribersTab />}
      {activeTab === 'tags' && <TagsTab />}
      {activeTab === 'segments' && <SegmentsTab />}
    </div>
  );
}

function SubscribersTab() {
  const [contacts, setContacts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState([]);

  useEffect(() => { loadContacts(); }, []);

  const loadContacts = async () => {
    try {
      const res = await api.get('/lists');
      let all = [];
      for (const list of res.data || []) {
        const c = await api.get(`/lists/${list.id}/contacts`);
        all = [...all, ...(c.data || [])];
      }
      setContacts(all);
    } catch (err) { console.error(err); }
    finally { setLoading(false); }
  };

  const filtered = contacts.filter(c =>
    c.email?.toLowerCase().includes(search.toLowerCase()) ||
    c.first_name?.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-4">
      <div className="flex gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-2.5 text-gray-400" size={20} />
          <input
            type="text"
            placeholder="Search..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
          />
        </div>
        <button className="flex items-center gap-2 px-4 py-2 border dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 font-medium">
          <Upload size={18} /> Import
        </button>
        <button className="flex items-center gap-2 px-4 py-2 border dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 font-medium">
          <Download size={18} /> Export
        </button>
      </div>

      <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg overflow-hidden shadow-sm">
        <table className="w-full">
          <thead className="bg-gray-50 dark:bg-gray-700/50 border-b dark:border-gray-700">
            <tr>
              <th className="w-10 p-4"><input type="checkbox" onChange={e => setSelected(e.target.checked ? filtered.map(c => c.id) : [])} className="rounded" /></th>
              <th className="p-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Email</th>
              <th className="p-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Name</th>
              <th className="p-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
              <th className="p-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Added</th>
            </tr>
          </thead>
          <tbody className="divide-y dark:divide-gray-700">
            {loading ? <tr><td colSpan="5" className="p-8 text-center text-gray-500">Loading...</td></tr> :
              filtered.length === 0 ? <tr><td colSpan="5" className="p-8 text-center text-gray-500">No subscribers found</td></tr> :
                filtered.slice(0, 50).map(c => (
                  <tr key={c.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                    <td className="p-4"><input type="checkbox" checked={selected.includes(c.id)}
                      onChange={e => setSelected(e.target.checked ? [...selected, c.id] : selected.filter(x => x !== c.id))} className="rounded" /></td>
                    <td className="p-3 font-medium text-gray-900 dark:text-white">{c.email}</td>
                    <td className="p-3 text-gray-600 dark:text-gray-300">{c.first_name} {c.last_name}</td>
                    <td className="p-3">
                      <span className={`px-2 py-1 text-xs font-medium rounded-full inline-flex items-center gap-1 ${c.is_valid
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                        : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'}`}>
                        {c.is_valid && <CheckCircle className="w-3 h-3" />}
                        {c.is_valid ? 'Valid' : 'Unverified'}</span>
                    </td>
                    <td className="p-3 text-sm text-gray-500 dark:text-gray-400">{new Date(c.created_at).toLocaleDateString()}</td>
                  </tr>
                ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function TagsTab() {
  const [tags, setTags] = useState([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(null);
  const [confirmDelete, setConfirmDelete] = useState(null);
  const colors = ['#EF4444', '#F59E0B', '#10B981', '#3B82F6', '#6366F1', '#8B5CF6', '#EC4899'];

  useEffect(() => { loadTags(); }, []);
  const loadTags = async () => {
    try { const r = await api.get('/tags'); setTags(r.data || []); }
    catch (e) { console.error(e); } finally { setLoading(false); }
  };

  const save = async (t) => {
    try {
      if (t.id) await api.put(`/tags/${t.id}`, t);
      else await api.post('/tags', t);
      setForm(null); loadTags();
    } catch (e) { console.error('Failed'); }
  };

  const del = async () => {
    if (!confirmDelete) return;
    try { await api.delete(`/tags/${confirmDelete.id}`); loadTags(); } catch (e) { console.error('Failed'); }
    setConfirmDelete(null);
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button onClick={() => setForm({ name: '', color: '#3B82F6' })}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm transition-colors">
          <Plus size={18} /> New Tag
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {loading ? <div className="col-span-full text-center py-8 text-gray-500">Loading...</div> :
          tags.map(t => (
            <div key={t.id} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg p-4 flex items-center justify-between shadow-sm hover:shadow-md transition-shadow">
              <div className="flex items-center gap-3">
                <div className="w-4 h-4 rounded-full ring-2 ring-white dark:ring-gray-800" style={{ backgroundColor: t.color }}></div>
                <div>
                  <div className="font-medium dark:text-white">{t.name}</div>
                  <div className="text-sm text-gray-500">{t.subscriber_count || 0} subscribers</div>
                </div>
              </div>
              <div className="flex gap-1">
                <button onClick={() => setForm(t)} className="p-2 text-gray-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors"><Edit size={16} /></button>
                <button onClick={() => setConfirmDelete(t)} className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"><Trash2 size={16} /></button>
              </div>
            </div>
          ))}
      </div>

      <ConfirmationModal
        isOpen={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={del}
        title="Delete Tag"
        message={`Are you sure you want to delete the tag "${confirmDelete?.name}"?`}
        confirmText="Delete Tag"
        type="danger"
      />

      {form && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-2xl animate-in zoom-in-95 duration-200">
            <div className="flex justify-between items-center mb-4">
              <h3 className="font-semibold text-lg dark:text-white">{form.id ? 'Edit' : 'New'} Tag</h3>
              <button onClick={() => setForm(null)} className="text-gray-400 hover:text-gray-600"><X size={20} /></button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Tag Name</label>
                <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none" placeholder="Tag name" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Color</label>
                <div className="flex gap-2 flex-wrap">
                  {colors.map(c => (
                    <button key={c} onClick={() => setForm({ ...form, color: c })}
                      className={`w-8 h-8 rounded-full transition-transform hover:scale-110 ${form.color === c ? 'ring-2 ring-offset-2 ring-blue-500 dark:ring-offset-gray-800' : ''}`}
                      style={{ backgroundColor: c }} />
                  ))}
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setForm(null)} className="px-4 py-2 border dark:border-gray-600 rounded-lg text-gray-600 dark:text-gray-300 font-medium">Cancel</button>
              <button onClick={() => save(form)} className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm">Save</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function SegmentsTab() {
  const [segments, setSegments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(null);
  const [confirmDelete, setConfirmDelete] = useState(null);

  useEffect(() => { loadSegments(); }, []);
  const loadSegments = async () => {
    try { const r = await api.get('/segments'); setSegments(r.data || []); }
    catch (e) { console.error(e); } finally { setLoading(false); }
  };

  const save = async (s) => {
    try {
      if (s.id) await api.put(`/segments/${s.id}`, s);
      else await api.post('/segments', s);
      setForm(null); loadSegments();
    } catch (e) { console.error('Failed'); }
  };

  const del = async () => {
    if (!confirmDelete) return;
    try { await api.delete(`/segments/${confirmDelete.id}`); loadSegments(); } catch (e) { console.error('Failed'); }
    setConfirmDelete(null);
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button onClick={() => setForm({ name: '', description: '', conditions: '[]' })}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm transition-colors">
          <Plus size={18} /> New Segment
        </button>
      </div>

      <div className="space-y-3">
        {loading ? <div className="text-center py-8 text-gray-500">Loading...</div> :
          segments.length === 0 ? <div className="text-center py-8 text-gray-500">No segments created yet</div> :
            segments.map(s => (
              <div key={s.id} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-lg p-4 flex items-center justify-between shadow-sm hover:shadow-md transition-shadow">
                <div>
                  <div className="font-medium dark:text-white">{s.name}</div>
                  <div className="text-sm text-gray-500">{s.description || 'No description'}</div>
                  <div className="text-sm text-blue-600 dark:text-blue-400 mt-1 font-medium">{s.cached_count} subscribers</div>
                </div>
                <div className="flex gap-1">
                  <button onClick={() => setForm(s)} className="p-2 text-gray-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors"><Edit size={16} /></button>
                  <button onClick={() => setConfirmDelete(s)} className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"><Trash2 size={16} /></button>
                </div>
              </div>
            ))}
      </div>

      <ConfirmationModal
        isOpen={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={del}
        title="Delete Segment"
        message={`Are you sure you want to delete the segment "${confirmDelete?.name}"?`}
        confirmText="Delete Segment"
        type="danger"
      />

      {form && <SegmentFormModal segment={form} onSave={save} onClose={() => setForm(null)} />}
    </div>
  );
}

function SegmentFormModal({ segment, onSave, onClose }) {
  const [form, setForm] = useState(segment);
  const [conditions, setConditions] = useState(JSON.parse(segment.conditions || '[]'));

  const fields = ['email', 'first_name', 'last_name', 'is_valid', 'created_at'];
  const operators = ['equals', 'not_equals', 'contains', 'not_contains', 'starts_with', 'ends_with'];

  const addCondition = () => setConditions([...conditions, { field: 'email', operator: 'contains', value: '', combiner: 'and' }]);
  const updateCondition = (i, k, v) => { const c = [...conditions]; c[i][k] = v; setConditions(c); };
  const removeCondition = (i) => setConditions(conditions.filter((_, x) => x !== i));

  const handleSave = () => {
    onSave({ ...form, conditions: JSON.stringify(conditions) });
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-2xl max-h-[90vh] overflow-auto shadow-2xl animate-in zoom-in-95 duration-200">
        <div className="flex justify-between items-center mb-4">
          <h3 className="font-semibold text-lg dark:text-white">{form.id ? 'Edit' : 'New'} Segment</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X size={20} /></button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
            <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
            <input type="text" value={form.description || ''} onChange={e => setForm({ ...form, description: e.target.value })}
              className="w-full px-3 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none" />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Conditions</label>
            <div className="space-y-2 p-4 bg-gray-50 dark:bg-gray-900/50 rounded-lg border dark:border-gray-700">
              {conditions.length === 0 && <p className="text-sm text-gray-500 text-center italic">No conditions added</p>}
              {conditions.map((c, i) => (
                <div key={i} className="flex gap-2 items-center flex-wrap sm:flex-nowrap">
                  {i > 0 && (
                    <select value={c.combiner} onChange={e => updateCondition(i, 'combiner', e.target.value)}
                      className="px-2 py-2 border dark:border-gray-600 rounded text-sm w-full sm:w-20 dark:bg-gray-700 dark:text-white">
                      <option value="and">AND</option>
                      <option value="or">OR</option>
                    </select>
                  )}
                  <select value={c.field} onChange={e => updateCondition(i, 'field', e.target.value)}
                    className="px-2 py-2 border dark:border-gray-600 rounded-lg flex-1 dark:bg-gray-700 dark:text-white">
                    {fields.map(f => <option key={f} value={f}>{f}</option>)}
                  </select>
                  <select value={c.operator} onChange={e => updateCondition(i, 'operator', e.target.value)}
                    className="px-2 py-2 border dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white">
                    {operators.map(o => <option key={o} value={o}>{o.replace('_', ' ')}</option>)}
                  </select>
                  <input type="text" value={c.value} onChange={e => updateCondition(i, 'value', e.target.value)}
                    className="px-2 py-2 border dark:border-gray-600 rounded-lg flex-1 dark:bg-gray-700 dark:text-white" placeholder="Value" />
                  <button onClick={() => removeCondition(i)} className="p-2 text-red-500 hover:bg-red-50 rounded"><X size={16} /></button>
                </div>
              ))}
            </div>
            <button onClick={addCondition} className="mt-2 text-sm text-blue-600 hover:text-blue-700 font-medium">+ Add condition</button>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onClose} className="px-4 py-2 border dark:border-gray-600 rounded-lg text-gray-600 dark:text-gray-300 font-medium">Cancel</button>
          <button onClick={handleSave} className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm">Save Segment</button>
        </div>
      </div>
    </div>
  );
}
