import { useState, useEffect } from 'react';
import { Plus, Copy, Trash2, Eye, Edit, Search, Folder } from 'lucide-react';
import api from '../api';

export default function TemplatesPage() {
  const [templates, setTemplates] = useState([]);
  const [categories, setCategories] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('');
  const [showEditor, setShowEditor] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState(null);
  const [showPreview, setShowPreview] = useState(false);
  const [previewHtml, setPreviewHtml] = useState('');

  useEffect(() => {
    loadTemplates();
    loadCategories();
  }, [search, selectedCategory]);

  const loadTemplates = async () => {
    try {
      const params = new URLSearchParams();
      if (search) params.append('search', search);
      if (selectedCategory) params.append('category', selectedCategory);

      const res = await api.get(`/templates?${params}`);
      setTemplates(res.data);
    } catch (err) {
      console.error('Failed to load templates:', err);
    } finally {
      setLoading(false);
    }
  };

  const loadCategories = async () => {
    try {
      const res = await api.get('/templates/categories');
      setCategories(res.data || []);
    } catch (err) {
      console.error('Failed to load categories:', err);
    }
  };

  const handleCreate = () => {
    setEditingTemplate({
      name: '',
      subject: '',
      html_content: defaultTemplate,
      text_content: '',
      category: 'newsletter'
    });
    setShowEditor(true);
  };

  const handleEdit = (template) => {
    setEditingTemplate(template);
    setShowEditor(true);
  };

  const handleClone = async (template) => {
    try {
      await api.post(`/templates/${template.id}/clone`);
      loadTemplates();
    } catch (err) {
      alert('Failed to clone template');
    }
  };

  const handleDelete = async (template) => {
    if (!confirm(`Delete template "${template.name}"?`)) return;
    try {
      await api.delete(`/templates/${template.id}`);
      loadTemplates();
    } catch (err) {
      alert('Failed to delete template');
    }
  };

  const handleSave = async (template) => {
    try {
      if (template.id) {
        await api.put(`/templates/${template.id}`, template);
      } else {
        await api.post('/templates', template);
      }
      setShowEditor(false);
      loadTemplates();
    } catch (err) {
      alert('Failed to save template');
    }
  };

  const handlePreview = async (template) => {
    try {
      const res = await api.post(`/templates/${template.id}/preview`, {
        first_name: 'John',
        last_name: 'Doe',
        email: 'john@example.com',
        company_name: 'Acme Inc',
        unsubscribe_url: '#'
      });
      setPreviewHtml(res.data.html);
      setShowPreview(true);
    } catch (err) {
      alert('Failed to preview template');
    }
  };

  if (loading) {
    return <div className="p-8 text-center">Loading templates...</div>;
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold">Email Templates</h1>
          <p className="text-gray-500">Create and manage email templates</p>
        </div>
        <button
          onClick={handleCreate}
          className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700"
        >
          <Plus size={20} />
          New Template
        </button>
      </div>

      {/* Filters */}
      <div className="flex gap-4 mb-6">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-2.5 text-gray-400" size={20} />
          <input
            type="text"
            placeholder="Search templates..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border rounded-lg focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <select
          value={selectedCategory}
          onChange={(e) => setSelectedCategory(e.target.value)}
          className="px-4 py-2 border rounded-lg focus:ring-2 focus:ring-indigo-500"
        >
          <option value="">All Categories</option>
          {categories.map(cat => (
            <option key={cat} value={cat}>{cat}</option>
          ))}
        </select>
      </div>

      {/* Template Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {templates.map(template => (
          <div key={template.id} className="border rounded-lg overflow-hidden bg-white shadow-sm hover:shadow-md transition-shadow">
            {/* Thumbnail/Preview */}
            <div className="h-40 bg-gray-100 flex items-center justify-center border-b">
              {template.thumbnail_url ? (
                <img src={template.thumbnail_url} alt={template.name} className="h-full w-full object-cover" />
              ) : (
                <div className="text-gray-400 text-sm">No preview</div>
              )}
            </div>

            {/* Info */}
            <div className="p-4">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-semibold text-lg">{template.name}</h3>
                  <p className="text-sm text-gray-500 truncate">{template.subject}</p>
                </div>
                {template.is_built_in && (
                  <span className="px-2 py-1 text-xs bg-gray-100 text-gray-600 rounded">Built-in</span>
                )}
              </div>

              <div className="flex items-center gap-2 mt-2">
                <Folder size={14} className="text-gray-400" />
                <span className="text-sm text-gray-500 capitalize">{template.category}</span>
              </div>

              {/* Actions */}
              <div className="flex gap-2 mt-4 pt-4 border-t">
                <button
                  onClick={() => handlePreview(template)}
                  className="flex-1 flex items-center justify-center gap-1 px-3 py-1.5 text-sm border rounded hover:bg-gray-50"
                >
                  <Eye size={16} />
                  Preview
                </button>
                <button
                  onClick={() => handleEdit(template)}
                  className="flex-1 flex items-center justify-center gap-1 px-3 py-1.5 text-sm border rounded hover:bg-gray-50"
                >
                  <Edit size={16} />
                  Edit
                </button>
                <button
                  onClick={() => handleClone(template)}
                  className="p-1.5 text-gray-500 hover:text-indigo-600 border rounded hover:bg-gray-50"
                  title="Clone"
                >
                  <Copy size={16} />
                </button>
                {!template.is_built_in && (
                  <button
                    onClick={() => handleDelete(template)}
                    className="p-1.5 text-gray-500 hover:text-red-600 border rounded hover:bg-gray-50"
                    title="Delete"
                  >
                    <Trash2 size={16} />
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {templates.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p>No templates found</p>
          <button onClick={handleCreate} className="mt-4 text-indigo-600 hover:underline">
            Create your first template
          </button>
        </div>
      )}

      {/* Editor Modal */}
      {showEditor && (
        <TemplateEditor
          template={editingTemplate}
          onSave={handleSave}
          onClose={() => setShowEditor(false)}
        />
      )}

      {/* Preview Modal */}
      {showPreview && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg w-full max-w-4xl max-h-[90vh] overflow-hidden">
            <div className="flex justify-between items-center p-4 border-b">
              <h2 className="font-semibold">Template Preview</h2>
              <button onClick={() => setShowPreview(false)} className="text-gray-500 hover:text-gray-700">✕</button>
            </div>
            <iframe
              srcDoc={previewHtml}
              className="w-full h-[70vh]"
              sandbox="allow-same-origin"
            />
          </div>
        </div>
      )}
    </div>
  );
}

// Template Editor Component
function TemplateEditor({ template, onSave, onClose }) {
  const [form, setForm] = useState(template);
  const [activeTab, setActiveTab] = useState('html');

  const handleChange = (field, value) => {
    setForm(prev => ({ ...prev, [field]: value }));
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg w-full max-w-6xl max-h-[95vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex justify-between items-center p-4 border-b">
          <h2 className="font-semibold text-lg">
            {template.id ? 'Edit Template' : 'New Template'}
          </h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-700 text-2xl">&times;</button>
        </div>

        {/* Form */}
        <div className="flex-1 overflow-auto p-4">
          <div className="grid grid-cols-3 gap-4 mb-4">
            <div>
              <label className="block text-sm font-medium mb-1">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => handleChange('name', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg"
                placeholder="Template name"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Subject</label>
              <input
                type="text"
                value={form.subject}
                onChange={(e) => handleChange('subject', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg"
                placeholder="Email subject line"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Category</label>
              <select
                value={form.category}
                onChange={(e) => handleChange('category', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg"
              >
                <option value="newsletter">Newsletter</option>
                <option value="welcome">Welcome</option>
                <option value="promotional">Promotional</option>
                <option value="transactional">Transactional</option>
                <option value="minimal">Minimal</option>
                <option value="other">Other</option>
              </select>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex gap-2 mb-2 border-b">
            <button
              onClick={() => setActiveTab('html')}
              className={`px-4 py-2 -mb-px ${activeTab === 'html' ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-gray-500'}`}
            >
              HTML
            </button>
            <button
              onClick={() => setActiveTab('text')}
              className={`px-4 py-2 -mb-px ${activeTab === 'text' ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-gray-500'}`}
            >
              Plain Text
            </button>
            <button
              onClick={() => setActiveTab('preview')}
              className={`px-4 py-2 -mb-px ${activeTab === 'preview' ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-gray-500'}`}
            >
              Preview
            </button>
          </div>

          {/* Variable Helper */}
          <div className="mb-2 p-2 bg-gray-50 rounded text-sm">
            <span className="font-medium">Variables:</span>
            {' '}
            <code className="bg-gray-200 px-1 rounded">{'{{first_name}}'}</code>
            {' '}
            <code className="bg-gray-200 px-1 rounded">{'{{last_name}}'}</code>
            {' '}
            <code className="bg-gray-200 px-1 rounded">{'{{email}}'}</code>
            {' '}
            <code className="bg-gray-200 px-1 rounded">{'{{unsubscribe_url}}'}</code>
          </div>

          {/* Content */}
          {activeTab === 'html' && (
            <textarea
              value={form.html_content}
              onChange={(e) => handleChange('html_content', e.target.value)}
              className="w-full h-96 px-3 py-2 border rounded-lg font-mono text-sm"
              placeholder="HTML content..."
            />
          )}

          {activeTab === 'text' && (
            <textarea
              value={form.text_content}
              onChange={(e) => handleChange('text_content', e.target.value)}
              className="w-full h-96 px-3 py-2 border rounded-lg font-mono text-sm"
              placeholder="Plain text version..."
            />
          )}

          {activeTab === 'preview' && (
            <iframe
              srcDoc={form.html_content}
              className="w-full h-96 border rounded-lg"
              sandbox="allow-same-origin"
            />
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-4 border-t">
          <button
            onClick={onClose}
            className="px-4 py-2 border rounded-lg hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(form)}
            className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700"
          >
            Save Template
          </button>
        </div>
      </div>
    </div>
  );
}

// Default template HTML
const defaultTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            line-height: 1.6; 
            color: #333; 
            max-width: 600px; 
            margin: 0 auto; 
            padding: 20px; 
        }
        .header { text-align: center; padding: 20px 0; }
        .content { padding: 20px 0; }
        .footer { 
            text-align: center; 
            padding: 20px 0; 
            border-top: 1px solid #eee; 
            font-size: 12px; 
            color: #666; 
        }
        .button { 
            display: inline-block; 
            padding: 12px 24px; 
            background: #4F46E5; 
            color: white; 
            text-decoration: none; 
            border-radius: 6px; 
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Your Title Here</h1>
    </div>
    <div class="content">
        <p>Hi {{first_name}},</p>
        <p>Your email content goes here.</p>
    </div>
    <div class="footer">
        <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
    </div>
</body>
</html>`;
