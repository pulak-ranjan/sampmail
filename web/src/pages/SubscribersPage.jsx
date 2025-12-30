import { useState, useEffect } from 'react';
import { Users, Tag, Filter, Plus, Search, Upload, Download, Trash2, Edit, MoreHorizontal, X } from 'lucide-react';
import api from '../api';

export default function SubscribersPage() {
  const [activeTab, setActiveTab] = useState('subscribers');

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold">Subscriber Management</h1>
          <p className="text-gray-500">Manage contacts, tags, and segments</p>
        </div>
      </div>

      <div className="flex gap-1 mb-6 border-b">
        {['subscribers', 'tags', 'segments'].map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 -mb-px flex items-center gap-2 capitalize ${activeTab === tab ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-gray-500'
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
          <input type="text" placeholder="Search..." value={search} onChange={e => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border rounded-lg" />
        </div>
        <button className="flex items-center gap-2 px-4 py-2 border rounded-lg hover:bg-gray-50">
          <Upload size={18} /> Import
        </button>
        <button className="flex items-center gap-2 px-4 py-2 border rounded-lg hover:bg-gray-50">
          <Download size={18} /> Export
        </button>
      </div>

      <div className="bg-white border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="w-10 p-3"><input type="checkbox" onChange={e => setSelected(e.target.checked ? filtered.map(c => c.id) : [])} /></th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Email</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Name</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Status</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Added</th>
            </tr>
          </thead>
          <tbody>
            {loading ? <tr><td colSpan="5" className="p-8 text-center">Loading...</td></tr> :
              filtered.length === 0 ? <tr><td colSpan="5" className="p-8 text-center">No subscribers</td></tr> :
                filtered.slice(0, 50).map(c => (
                  <tr key={c.id} className="border-t hover:bg-gray-50">
                    <td className="p-3"><input type="checkbox" checked={selected.includes(c.id)}
                      onChange={e => setSelected(e.target.checked ? [...selected, c.id] : selected.filter(x => x !== c.id))} /></td>
                    <td className="p-3 font-medium">{c.email}</td>
                    <td className="p-3">{c.first_name} {c.last_name}</td>
                    <td className="p-3"><span className={`px-2 py-1 text-xs rounded-full ${c.is_valid ? 'bg-green-100 text-green-700' : 'bg-gray-100'}`}>
                      {c.is_valid ? 'Valid' : 'Unverified'}</span></td>
                    <td className="p-3 text-sm text-gray-500">{new Date(c.created_at).toLocaleDateString()}</td>
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
    } catch (e) { alert('Failed'); }
  };

  const del = async (t) => {
    if (!confirm(`Delete "${t.name}"?`)) return;
    try { await api.delete(`/tags/${t.id}`); loadTags(); } catch (e) { alert('Failed'); }
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button onClick={() => setForm({ name: '', color: '#6366F1' })}
          className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg">
          <Plus size={18} /> New Tag
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {loading ? <div className="col-span-full text-center py-8">Loading...</div> :
          tags.map(t => (
            <div key={t.id} className="bg-white border rounded-lg p-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="w-4 h-4 rounded-full" style={{ backgroundColor: t.color }}></div>
                <div><div className="font-medium">{t.name}</div>
                  <div className="text-sm text-gray-500">{t.subscriber_count || 0} subscribers</div></div>
              </div>
              <div className="flex gap-1">
                <button onClick={() => setForm(t)} className="p-2 hover:bg-gray-100 rounded"><Edit size={16} /></button>
                <button onClick={() => del(t)} className="p-2 hover:bg-gray-100 rounded"><Trash2 size={16} /></button>
              </div>
            </div>
          ))}
      </div>

      {form && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <div className="flex justify-between mb-4">
              <h3 className="font-semibold">{form.id ? 'Edit' : 'New'} Tag</h3>
              <button onClick={() => setForm(null)}><X size={20} /></button>
            </div>
            <div className="space-y-4">
              <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                className="w-full px-3 py-2 border rounded-lg" placeholder="Tag name" />
              <div className="flex gap-2">
                {colors.map(c => (
                  <button key={c} onClick={() => setForm({ ...form, color: c })}
                    className={`w-8 h-8 rounded-full ${form.color === c ? 'ring-2 ring-offset-2 ring-indigo-500' : ''}`}
                    style={{ backgroundColor: c }} />
                ))}
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setForm(null)} className="px-4 py-2 border rounded-lg">Cancel</button>
              <button onClick={() => save(form)} className="px-4 py-2 bg-indigo-600 text-white rounded-lg">Save</button>
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
    } catch (e) { alert('Failed'); }
  };

  const del = async (s) => {
    if (!confirm(`Delete "${s.name}"?`)) return;
    try { await api.delete(`/segments/${s.id}`); loadSegments(); } catch (e) { alert('Failed'); }
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button onClick={() => setForm({ name: '', description: '', conditions: '[]' })}
          className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg">
          <Plus size={18} /> New Segment
        </button>
      </div>

      <div className="space-y-3">
        {loading ? <div className="text-center py-8">Loading...</div> :
          segments.length === 0 ? <div className="text-center py-8 text-gray-500">No segments</div> :
            segments.map(s => (
              <div key={s.id} className="bg-white border rounded-lg p-4 flex items-center justify-between">
                <div>
                  <div className="font-medium">{s.name}</div>
                  <div className="text-sm text-gray-500">{s.description || 'No description'}</div>
                  <div className="text-sm text-indigo-600 mt-1">{s.cached_count} subscribers</div>
                </div>
                <div className="flex gap-1">
                  <button onClick={() => setForm(s)} className="p-2 hover:bg-gray-100 rounded"><Edit size={16} /></button>
                  <button onClick={() => del(s)} className="p-2 hover:bg-gray-100 rounded"><Trash2 size={16} /></button>
                </div>
              </div>
            ))}
      </div>

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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-auto">
        <div className="flex justify-between mb-4">
          <h3 className="font-semibold text-lg">{form.id ? 'Edit' : 'New'} Segment</h3>
          <button onClick={onClose}><X size={20} /></button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Name</label>
            <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
              className="w-full px-3 py-2 border rounded-lg" />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Description</label>
            <input type="text" value={form.description || ''} onChange={e => setForm({ ...form, description: e.target.value })}
              className="w-full px-3 py-2 border rounded-lg" />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Conditions</label>
            <div className="space-y-2">
              {conditions.map((c, i) => (
                <div key={i} className="flex gap-2 items-center">
                  {i > 0 && (
                    <select value={c.combiner} onChange={e => updateCondition(i, 'combiner', e.target.value)}
                      className="px-2 py-1 border rounded text-sm w-16">
                      <option value="and">AND</option>
                      <option value="or">OR</option>
                    </select>
                  )}
                  <select value={c.field} onChange={e => updateCondition(i, 'field', e.target.value)}
                    className="px-2 py-2 border rounded-lg flex-1">
                    {fields.map(f => <option key={f} value={f}>{f}</option>)}
                  </select>
                  <select value={c.operator} onChange={e => updateCondition(i, 'operator', e.target.value)}
                    className="px-2 py-2 border rounded-lg">
                    {operators.map(o => <option key={o} value={o}>{o.replace('_', ' ')}</option>)}
                  </select>
                  <input type="text" value={c.value} onChange={e => updateCondition(i, 'value', e.target.value)}
                    className="px-2 py-2 border rounded-lg flex-1" placeholder="Value" />
                  <button onClick={() => removeCondition(i)} className="p-2 text-red-500"><X size={16} /></button>
                </div>
              ))}
            </div>
            <button onClick={addCondition} className="mt-2 text-sm text-indigo-600">+ Add condition</button>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onClose} className="px-4 py-2 border rounded-lg">Cancel</button>
          <button onClick={handleSave} className="px-4 py-2 bg-indigo-600 text-white rounded-lg">Save Segment</button>
        </div>
      </div>
    </div>
  );
}
