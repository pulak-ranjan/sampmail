import { useState, useEffect } from 'react';
import { Plus, Copy, Trash2, Eye, Edit, Search, Folder, File, Code, Layout } from 'lucide-react';
import api from '../api';
import ConfirmationModal from '../components/ConfirmationModal';

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
  const [deleteId, setDeleteId] = useState(null);

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
      console.error('Failed to clone template');
    }
  };

  const handleDeleteClick = (template) => {
    setDeleteId(template.id);
  };

  const confirmDelete = async () => {
    if (!deleteId) return;
    try {
      await api.delete(`/templates/${deleteId}`);
      loadTemplates();
    } catch (err) {
      console.error('Failed to delete template');
    }
    setDeleteId(null);
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
      console.error('Failed to save template');
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
      console.error('Failed to preview template');
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
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold dark:text-white">Email Templates</h1>
          <p className="text-gray-500">Create and manage email templates</p>
        </div>
        <button
          onClick={handleCreate}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm transition-all"
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
            className="w-full pl-10 pr-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 dark:bg-gray-800 dark:border-gray-700 dark:text-white"
          />
        </div>
        <select
          value={selectedCategory}
          onChange={(e) => setSelectedCategory(e.target.value)}
          className="px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 dark:bg-gray-800 dark:border-gray-700 dark:text-white"
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
          <div key={template.id} className="border rounded-lg overflow-hidden bg-white dark:bg-gray-800 dark:border-gray-700 shadow-sm hover:shadow-md transition-shadow group">
            {/* Thumbnail/Preview */}
            <div className="h-40 bg-gray-100 dark:bg-gray-900 flex items-center justify-center border-b dark:border-gray-700 relative overflow-hidden">
              {template.thumbnail_url ? (
                <img src={template.thumbnail_url} alt={template.name} className="h-full w-full object-cover" />
              ) : (
                <div className="text-gray-400 dark:text-gray-600 text-sm flex flex-col items-center">
                  <Layout className="w-8 h-8 mb-2 opacity-50" />
                  No preview
                </div>
              )}
              {/* Overlay Actions */}
              <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
                <button
                  onClick={() => handlePreview(template)}
                  className="p-2 bg-white rounded-full hover:bg-gray-100 text-gray-700 shadow-sm"
                  title="Preview"
                >
                  <Eye size={18} />
                </button>
                <button
                  onClick={() => handleEdit(template)}
                  className="p-2 bg-blue-600 rounded-full hover:bg-blue-700 text-white shadow-sm"
                  title="Edit"
                >
                  <Edit size={18} />
                </button>
              </div>
            </div>

            {/* Info */}
            <div className="p-4">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-semibold text-lg dark:text-white">{template.name}</h3>
                  <p className="text-sm text-gray-500 truncate">{template.subject}</p>
                </div>
                {template.is_built_in && (
                  <span className="px-2 py-1 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded">Built-in</span>
                )}
              </div>

              <div className="flex items-center gap-2 mt-2">
                <Folder size={14} className="text-gray-400" />
                <span className="text-sm text-gray-500 capitalize">{template.category}</span>
              </div>

              {/* Actions Footer */}
              <div className="flex gap-2 mt-4 pt-4 border-t dark:border-gray-700">
                <button
                  onClick={() => handleClone(template)}
                  className="flex-1 flex items-center justify-center gap-2 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors"
                >
                  <Copy size={16} /> Clone
                </button>
                {!template.is_built_in && (
                  <button
                    onClick={() => handleDeleteClick(template)}
                    className="flex-1 flex items-center justify-center gap-2 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
                  >
                    <Trash2 size={16} /> Delete
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {templates.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <File className="w-16 h-16 mx-auto text-gray-300 mb-4" />
          <p className="text-lg font-medium text-gray-600 dark:text-gray-400">No templates found</p>
          <button onClick={handleCreate} className="mt-4 text-blue-600 hover:underline">
            Create your first template
          </button>
        </div>
      )}

      <ConfirmationModal
        isOpen={!!deleteId}
        onClose={() => setDeleteId(null)}
        onConfirm={confirmDelete}
        title="Delete Template"
        message="Are you sure you want to delete this template? This cannot be undone."
        confirmText="Delete"
        type="danger"
      />

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
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white dark:bg-gray-800 rounded-xl w-full max-w-4xl max-h-[90vh] overflow-hidden shadow-2xl">
            <div className="flex justify-between items-center p-4 border-b dark:border-gray-700">
              <h2 className="font-semibold dark:text-white flex items-center gap-2"><Eye className="w-5 h-5" /> Template Preview</h2>
              <button onClick={() => setShowPreview(false)} className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">✕</button>
            </div>
            <iframe
              srcDoc={previewHtml}
              className="w-full h-[70vh] bg-white"
              sandbox="allow-same-origin"
              title="Preview"
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 backdrop-blur-sm animate-in zoom-in-95 duration-200">
      <div className="bg-white dark:bg-gray-800 rounded-xl w-full max-w-6xl max-h-[95vh] overflow-hidden flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex justify-between items-center p-4 border-b dark:border-gray-700">
          <h2 className="font-semibold text-lg dark:text-white">
            {template.id ? 'Edit Template' : 'New Template'}
          </h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 text-2xl">&times;</button>
        </div>

        {/* Form */}
        <div className="flex-1 overflow-auto p-6">
          <div className="grid grid-cols-3 gap-6 mb-6">
            <div>
              <label className="block text-sm font-medium mb-1 dark:text-gray-300">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => handleChange('name', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                placeholder="Template name"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1 dark:text-gray-300">Subject</label>
              <input
                type="text"
                value={form.subject}
                onChange={(e) => handleChange('subject', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                placeholder="Email subject line"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1 dark:text-gray-300">Category</label>
              <select
                value={form.category}
                onChange={(e) => handleChange('category', e.target.value)}
                className="w-full px-3 py-2 border rounded-lg dark:bg-gray-700 dark:border-gray-600 dark:text-white"
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
          <div className="flex gap-2 mb-4 border-b dark:border-gray-700">
            <button
              onClick={() => setActiveTab('html')}
              className={`px-4 py-2 -mb-px flex items-center gap-2 font-medium ${activeTab === 'html' ? 'border-b-2 border-blue-600 text-blue-600' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
            >
              <Code size={16} /> HTML
            </button>
            <button
              onClick={() => setActiveTab('text')}
              className={`px-4 py-2 -mb-px flex items-center gap-2 font-medium ${activeTab === 'text' ? 'border-b-2 border-blue-600 text-blue-600' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
            >
              <File size={16} /> Plain Text
            </button>
            <button
              onClick={() => setActiveTab('preview')}
              className={`px-4 py-2 -mb-px flex items-center gap-2 font-medium ${activeTab === 'preview' ? 'border-b-2 border-blue-600 text-blue-600' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
            >
              <Eye size={16} /> Preview
            </button>
          </div>

          {/* Variable Helper */}
          <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-300 rounded-lg text-sm border border-blue-100 dark:border-blue-800">
            <span className="font-medium mr-2">Variables:</span>
            {[
              '{{first_name}}', '{{last_name}}', '{{email}}', '{{unsubscribe_url}}', '{{company_name}}'
            ].map(v => (
              <code key={v} className="bg-white/50 dark:bg-black/20 px-1.5 py-0.5 rounded mx-1 text-xs font-mono border border-blue-200 dark:border-blue-700">{v}</code>
            ))}
          </div>

          {/* Content */}
          <div className="border rounded-lg overflow-hidden dark:border-gray-700">
            {activeTab === 'html' && (
              <textarea
                value={form.html_content}
                onChange={(e) => handleChange('html_content', e.target.value)}
                className="w-full h-96 px-4 py-3 font-mono text-sm focus:outline-none dark:bg-gray-900 dark:text-gray-300"
                placeholder="HTML content..."
              />
            )}

            {activeTab === 'text' && (
              <textarea
                value={form.text_content}
                onChange={(e) => handleChange('text_content', e.target.value)}
                className="w-full h-96 px-4 py-3 font-mono text-sm focus:outline-none dark:bg-gray-900 dark:text-gray-300"
                placeholder="Plain text version..."
              />
            )}

            {activeTab === 'preview' && (
              <iframe
                srcDoc={form.html_content}
                className="w-full h-96 bg-white"
                sandbox="allow-same-origin"
                title="Editor Preview"
              />
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-4 border-t dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50">
          <button
            onClick={onClose}
            className="px-4 py-2 border dark:border-gray-600 rounded-lg hover:bg-white dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 font-medium transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(form)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-medium shadow-sm transition-colors"
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
            background: #2563EB; 
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
        <p><a href="#" class="button">Click Me</a></p>
    </div>
    <div class="footer">
        <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
    </div>
</body>
</html>`;
