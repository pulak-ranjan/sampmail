import React, { useState, useEffect, useRef } from 'react';
import DOMPurify from 'dompurify';
import {
  LayoutTemplate, Image, Type, Columns, Footprints, MousePointer2,
  Smartphone, Monitor, Eye, Save, Sparkles, X, ArrowUp, ArrowDown,
  Trash2, Code, GripVertical, Maximize, Facebook, Minus, Box, Layout
} from 'lucide-react';

const EmailTemplateBuilder = ({ template, onSave, onPreview }) => {
  const [blocks, setBlocks] = useState([]);
  const [selectedBlock, setSelectedBlock] = useState(null);
  const [viewMode, setViewMode] = useState('desktop'); // desktop, mobile
  const [showAIModal, setShowAIModal] = useState(false);
  const [showVariables, setShowVariables] = useState(false);
  const [htmlOutput, setHtmlOutput] = useState('');
  const editorRef = useRef(null);

  // Available content blocks
  const contentBlocks = {
    structure: [
      { type: 'header', label: 'Header', icon: LayoutTemplate, description: 'Logo and navigation' },
      { type: 'hero', label: 'Hero Section', icon: Image, description: 'Large image with text' },
      { type: 'content', label: 'Content Block', icon: Type, description: 'Text and image' },
      { type: 'columns-2', label: '2 Columns', icon: Columns, description: 'Two column layout' },
      { type: 'columns-3', label: '3 Columns', icon: Layout, description: 'Three column layout' },
      { type: 'footer', label: 'Footer', icon: Footprints, description: 'Footer with links' },
    ],
    content: [
      { type: 'text', label: 'Text', icon: Type, description: 'Rich text block' },
      { type: 'image', label: 'Image', icon: Image, description: 'Image with link' },
      { type: 'button', label: 'Button', icon: MousePointer2, description: 'Call to action' },
      { type: 'divider', label: 'Divider', icon: Minus, description: 'Horizontal line' },
      { type: 'spacer', label: 'Spacer', icon: Maximize, description: 'Empty space' },
      { type: 'social', label: 'Social Icons', icon: Facebook, description: 'Social media links' },
    ],
    advanced: [
      { type: 'video', label: 'Video', icon: Monitor, description: 'Video thumbnail' },
      { type: 'countdown', label: 'Countdown', icon: Box, description: 'Timer display' },
      { type: 'products', label: 'Products', icon: Box, description: 'Product grid' },
      { type: 'menu', label: 'Menu', icon: GripVertical, description: 'Navigation menu' },
    ],
  };

  // Template variables
  const variables = [
    { name: 'first_name', description: 'Contact first name', example: 'John' },
    { name: 'last_name', description: 'Contact last name', example: 'Doe' },
    { name: 'full_name', description: 'Full name', example: 'John Doe' },
    { name: 'email', description: 'Contact email', example: 'john@example.com' },
    { name: 'company', description: 'Company name', example: 'Acme Inc' },
    { name: 'company_name', description: 'Your company', example: 'SampMail' },
    { name: 'unsubscribe_url', description: 'Unsubscribe link', example: '#' },
    { name: 'webview_url', description: 'View in browser', example: '#' },
    { name: 'current_year', description: 'Current year', example: '2025' },
  ];

  // Add block to canvas
  const addBlock = (blockType) => {
    const newBlock = {
      id: `block_${Date.now()}`,
      type: blockType.type,
      label: blockType.label,
      content: getDefaultContent(blockType.type),
      styles: getDefaultStyles(blockType.type),
    };
    setBlocks([...blocks, newBlock]);
    setSelectedBlock(newBlock);
  };

  // Get default content for block type
  const getDefaultContent = (type) => {
    switch (type) {
      case 'header':
        return { logo: '', logoAlt: 'Logo', links: [] };
      case 'hero':
        return {
          image: 'https://via.placeholder.com/600x300',
          headline: 'Welcome {{first_name}}!',
          subheadline: 'Your headline here',
          buttonText: 'Get Started',
          buttonUrl: '#',
        };
      case 'text':
        return { html: '<p>Enter your text here. Use {{first_name}} for personalization.</p>' };
      case 'image':
        return { src: 'https://via.placeholder.com/600x400', alt: 'Image', link: '' };
      case 'button':
        return { text: 'Click Here', url: '#', align: 'center' };
      case 'divider':
        return { style: 'solid', color: '#e5e5e5' };
      case 'spacer':
        return { height: 20 };
      case 'social':
        return {
          icons: [
            { platform: 'facebook', url: '#' },
            { platform: 'twitter', url: '#' },
            { platform: 'linkedin', url: '#' },
          ]
        };
      case 'footer':
        return {
          companyName: '{{company_name}}',
          address: 'Your address here',
          unsubscribeText: 'Unsubscribe',
        };
      default:
        return {};
    }
  };

  // Get default styles for block type
  const getDefaultStyles = (type) => {
    return {
      backgroundColor: '#ffffff',
      padding: '20px',
      textColor: '#333333',
      fontSize: '16px',
    };
  };

  // Update block
  const updateBlock = (blockId, updates) => {
    setBlocks(blocks.map(b =>
      b.id === blockId ? { ...b, ...updates } : b
    ));
  };

  // Delete block
  const deleteBlock = (blockId) => {
    setBlocks(blocks.filter(b => b.id !== blockId));
    setSelectedBlock(null);
  };

  // Move block
  const moveBlock = (blockId, direction) => {
    const index = blocks.findIndex(b => b.id === blockId);
    if ((direction === 'up' && index === 0) ||
      (direction === 'down' && index === blocks.length - 1)) return;

    const newBlocks = [...blocks];
    const newIndex = direction === 'up' ? index - 1 : index + 1;
    [newBlocks[index], newBlocks[newIndex]] = [newBlocks[newIndex], newBlocks[index]];
    setBlocks(newBlocks);
  };

  // Generate HTML output
  const generateHTML = () => {
    let html = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Email</title>
</head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background-color:#f4f4f4;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f4;">
    <tr>
      <td align="center" style="padding:20px;">
        <table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;">
`;

    blocks.forEach(block => {
      html += renderBlockToHTML(block);
    });

    html += `
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`;

    return html;
  };

  // Render block to HTML
  const renderBlockToHTML = (block) => {
    const styles = block.styles || {};
    const content = block.content || {};

    switch (block.type) {
      case 'header':
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};text-align:center;">
              ${content.logo ? `<img src="${content.logo}" alt="${content.logoAlt}" style="max-width:200px;" />` : '<h1 style="margin:0;">{{company_name}}</h1>'}
            </td>
          </tr>`;

      case 'hero':
        return `
          <tr>
            <td style="padding:0;background-color:${styles.backgroundColor};">
              ${content.image ? `<img src="${content.image}" style="width:100%;display:block;" />` : ''}
              <div style="padding:30px;text-align:center;">
                <h1 style="margin:0 0 10px;font-size:28px;color:${styles.textColor};">${content.headline}</h1>
                <p style="margin:0 0 20px;font-size:18px;color:#666;">${content.subheadline}</p>
                ${content.buttonText ? `<a href="${content.buttonUrl}" style="display:inline-block;padding:12px 30px;background-color:#3B82F6;color:#ffffff;text-decoration:none;border-radius:6px;font-weight:bold;">${content.buttonText}</a>` : ''}
              </div>
            </td>
          </tr>`;

      case 'text':
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};color:${styles.textColor};font-size:${styles.fontSize};line-height:1.6;">
              ${content.html}
            </td>
          </tr>`;

      case 'image':
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};text-align:center;">
              ${content.link ? `<a href="${content.link}">` : ''}
              <img src="${content.src}" alt="${content.alt}" style="max-width:100%;display:block;margin:0 auto;" />
              ${content.link ? '</a>' : ''}
            </td>
          </tr>`;

      case 'button':
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};text-align:${content.align};">
              <a href="${content.url}" style="display:inline-block;padding:14px 30px;background-color:#3B82F6;color:#ffffff;text-decoration:none;border-radius:6px;font-weight:bold;">${content.text}</a>
            </td>
          </tr>`;

      case 'divider':
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};">
              <hr style="border:none;border-top:1px ${content.style} ${content.color};margin:0;" />
            </td>
          </tr>`;

      case 'spacer':
        return `
          <tr>
            <td style="height:${content.height}px;background-color:${styles.backgroundColor};">&nbsp;</td>
          </tr>`;

      case 'social':
        const iconUrls = {
          facebook: 'https://cdn-icons-png.flaticon.com/32/733/733547.png',
          twitter: 'https://cdn-icons-png.flaticon.com/32/733/733579.png',
          linkedin: 'https://cdn-icons-png.flaticon.com/32/733/733561.png',
          instagram: 'https://cdn-icons-png.flaticon.com/32/733/733558.png',
        };
        return `
          <tr>
            <td style="padding:${styles.padding};background-color:${styles.backgroundColor};text-align:center;">
              ${(content.icons || []).map(icon =>
          `<a href="${icon.url}" style="display:inline-block;margin:0 5px;"><img src="${iconUrls[icon.platform] || ''}" width="32" alt="${icon.platform}" /></a>`
        ).join('')}
            </td>
          </tr>`;

      case 'footer':
        return `
          <tr>
            <td style="padding:30px;background-color:#1F2937;text-align:center;">
              <p style="margin:0 0 10px;color:#9CA3AF;font-size:14px;">${content.companyName}</p>
              <p style="margin:0 0 10px;color:#6B7280;font-size:12px;">${content.address}</p>
              <p style="margin:0;"><a href="{{unsubscribe_url}}" style="color:#9CA3AF;font-size:12px;">${content.unsubscribeText}</a></p>
            </td>
          </tr>`;

      default:
        return '';
    }
  };

  // Handle save
  const handleSave = () => {
    const html = generateHTML();
    onSave({
      builder_json: JSON.stringify(blocks),
      html_content: html,
    });
  };

  // Insert variable
  const insertVariable = (varName) => {
    const text = `{{${varName}}}`;
    navigator.clipboard.writeText(text);
    // In a real implementation, this would insert at cursor position
  };

  return (
    <div className="flex h-full bg-gray-100">
      {/* Block Palette */}
      <div className="w-64 bg-white border-r overflow-y-auto">
        <div className="p-4">
          <h3 className="font-semibold text-gray-800 mb-4">Content Blocks</h3>

          {Object.entries(contentBlocks).map(([category, items]) => (
            <div key={category} className="mb-6">
              <h4 className="text-sm font-medium text-gray-500 uppercase mb-2">
                {category}
              </h4>
              <div className="grid grid-cols-2 gap-2">
                {items.map((block) => (
                  <button
                    key={block.type}
                    onClick={() => addBlock(block)}
                    className="p-3 bg-gray-50 rounded-lg hover:bg-white hover:shadow-md hover:ring-1 hover:ring-blue-500/20 text-center transition-all group border border-transparent"
                  >
                    <div className="flex justify-center mb-2 text-gray-400 group-hover:text-blue-600 transition-colors">
                      <block.icon className="w-6 h-6" />
                    </div>
                    <span className="text-xs font-medium text-gray-600 group-hover:text-gray-900">{block.label}</span>
                  </button>
                ))}
              </div>
            </div>
          ))}

          {/* Variables Button */}
          <button
            onClick={() => setShowVariables(!showVariables)}
            className="w-full p-3 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition mb-4 flex items-center justify-center gap-2 text-sm font-medium"
          >
            <Code className="w-4 h-4" /> Insert Variable
          </button>

          {showVariables && (
            <div className="bg-gray-50 rounded-lg p-3 mb-4">
              <h5 className="text-sm font-medium mb-2">Available Variables</h5>
              {variables.map((v) => (
                <button
                  key={v.name}
                  onClick={() => insertVariable(v.name)}
                  className="block w-full text-left p-2 hover:bg-white rounded text-sm"
                >
                  <code className="text-blue-600">{`{{${v.name}}}`}</code>
                  <span className="text-gray-500 text-xs block">{v.description}</span>
                </button>
              ))}
            </div>
          )}

          {/* AI Generate Button */}
          <button
            onClick={() => setShowAIModal(true)}
            className="w-full p-3 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition flex items-center justify-center gap-2 text-sm font-medium shadow-sm"
          >
            <Sparkles className="w-4 h-4" /> Generate with AI
          </button>
        </div>
      </div>

      {/* Canvas */}
      <div className="flex-1 overflow-auto">
        {/* Toolbar */}
        <div className="bg-white border-b p-3 flex items-center justify-between sticky top-0 z-10">
          <div className="flex items-center gap-2">
            <button
              onClick={() => setViewMode('desktop')}
              className={`p-2 rounded flex items-center gap-2 ${viewMode === 'desktop' ? 'bg-blue-100 text-blue-600' : 'text-gray-500 hover:bg-gray-100'}`}
            >
              <Monitor className="w-4 h-4" />
              <span className="text-sm font-medium">Desktop</span>
            </button>
            <button
              onClick={() => setViewMode('mobile')}
              className={`p-2 rounded flex items-center gap-2 ${viewMode === 'mobile' ? 'bg-blue-100 text-blue-600' : 'text-gray-500 hover:bg-gray-100'}`}
            >
              <Smartphone className="w-4 h-4" />
              <span className="text-sm font-medium">Mobile</span>
            </button>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => onPreview(generateHTML())}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded hover:bg-gray-200 flex items-center gap-2 text-sm font-medium"
            >
              <Eye className="w-4 h-4" /> Preview
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 flex items-center gap-2 text-sm font-medium shadow-sm transition-all"
            >
              <Save className="w-4 h-4" /> Save Template
            </button>
          </div>
        </div>

        {/* Email Preview */}
        <div className="p-8 flex justify-center">
          <div
            className={`bg-white shadow-lg transition-all ${viewMode === 'mobile' ? 'w-[375px]' : 'w-[600px]'
              }`}
            ref={editorRef}
          >
            {blocks.length === 0 ? (
              <div className="p-20 text-center text-gray-400">
                <div className="flex justify-center mb-4 opacity-50">
                  <LayoutTemplate className="w-16 h-16" />
                </div>
                <p className="text-xl font-semibold mb-2 text-gray-500">Start building your email</p>
                <p>Drag blocks from the left panel</p>
              </div>
            ) : (
              blocks.map((block, index) => (
                <div
                  key={block.id}
                  className={`relative group ${selectedBlock?.id === block.id ? 'ring-2 ring-blue-500' : ''}`}
                  onClick={() => setSelectedBlock(block)}
                >
                  {/* Block controls */}
                  <div className="absolute -right-10 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 flex flex-col gap-1">
                    <button
                      onClick={(e) => { e.stopPropagation(); moveBlock(block.id, 'up'); }}
                      className="w-8 h-8 bg-white shadow rounded flex items-center justify-center hover:bg-gray-100 text-gray-600"
                      disabled={index === 0}
                    >
                      <ArrowUp className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); moveBlock(block.id, 'down'); }}
                      className="w-8 h-8 bg-white shadow rounded flex items-center justify-center hover:bg-gray-100 text-gray-600"
                      disabled={index === blocks.length - 1}
                    >
                      <ArrowDown className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); deleteBlock(block.id); }}
                      className="w-8 h-8 bg-white border border-red-100 text-red-500 shadow rounded flex items-center justify-center hover:bg-red-50"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>

                  {/* Block preview */}
                  <BlockPreview block={block} />
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Block Editor Panel */}
      {selectedBlock && (
        <div className="w-80 bg-white border-l overflow-y-auto">
          <div className="p-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-semibold">Edit {selectedBlock.label}</h3>
              <button
                onClick={() => setSelectedBlock(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <BlockEditor
              block={selectedBlock}
              onUpdate={(updates) => {
                updateBlock(selectedBlock.id, updates);
                setSelectedBlock({ ...selectedBlock, ...updates });
              }}
            />
          </div>
        </div>
      )}

      {/* AI Modal */}
      {showAIModal && (
        <AIGenerateModal
          onClose={() => setShowAIModal(false)}
          onGenerate={(result) => {
            // Handle AI generated content
            setShowAIModal(false);
          }}
        />
      )}
    </div>
  );
};

const BlockPreview = ({ block }) => {
  const { type, content = {}, styles = {} } = block;

  const baseStyle = {
    backgroundColor: styles.backgroundColor || '#ffffff',
    padding: styles.padding || '20px',
    color: styles.textColor || '#333333',
  };

  switch (type) {
    case 'header':
      return (
        <div style={baseStyle} className="text-center">
          {content.logo ? (
            <img src={content.logo} alt={content.logoAlt} className="max-w-[200px] mx-auto" />
          ) : (
            <h1 className="text-xl font-bold">{'{{company_name}}'}</h1>
          )}
        </div>
      );

    case 'hero':
      return (
        <div style={baseStyle}>
          {content.image && (
            <img src={content.image} alt="" className="w-full" />
          )}
          <div className="text-center py-6">
            <h1 className="text-2xl font-bold mb-2">{content.headline}</h1>
            <p className="text-gray-600 mb-4">{content.subheadline}</p>
            {content.buttonText && (
              <button className="px-6 py-3 bg-blue-600 text-white rounded-lg">
                {content.buttonText}
              </button>
            )}
          </div>
        </div>
      );

    case 'text':
      return (
        <div
          style={baseStyle}
          dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content.html || '', { USE_PROFILES: { html: true } }) }}
        />
      );

    case 'image':
      return (
        <div style={baseStyle} className="text-center">
          <img src={content.src} alt={content.alt} className="max-w-full mx-auto" />
        </div>
      );

    case 'button':
      return (
        <div style={{ ...baseStyle, textAlign: content.align || 'center' }}>
          <button className="px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold">
            {content.text}
          </button>
        </div>
      );

    case 'divider':
      return (
        <div style={baseStyle}>
          <hr style={{ borderTop: `1px ${content.style || 'solid'} ${content.color || '#e5e5e5'}` }} />
        </div>
      );

    case 'spacer':
      return <div style={{ ...baseStyle, height: content.height || 20 }} />;

    case 'social':
      return (
        <div style={baseStyle} className="text-center">
          <div className="flex justify-center gap-3">
            {(content.icons || []).map((icon, i) => (
              <span key={i} className="w-8 h-8 bg-gray-200 rounded-full flex items-center justify-center">
                {icon.platform[0].toUpperCase()}
              </span>
            ))}
          </div>
        </div>
      );

    case 'footer':
      return (
        <div style={{ ...baseStyle, backgroundColor: '#1F2937', color: '#9CA3AF', textAlign: 'center' }}>
          <p className="font-medium">{content.companyName}</p>
          <p className="text-sm text-gray-500">{content.address}</p>
          <p className="text-sm mt-2">
            <a href="#" className="text-gray-400 hover:text-gray-300">{content.unsubscribeText}</a>
          </p>
        </div>
      );

    default:
      return <div style={baseStyle}>Unknown block type: {type}</div>;
  }
};

const BlockEditor = ({ block, onUpdate }) => {
  const [content, setContent] = useState(block.content || {});
  const [styles, setStyles] = useState(block.styles || {});

  const updateContent = (key, value) => {
    const newContent = { ...content, [key]: value };
    setContent(newContent);
    onUpdate({ content: newContent });
  };

  const updateStyles = (key, value) => {
    const newStyles = { ...styles, [key]: value };
    setStyles(newStyles);
    onUpdate({ styles: newStyles });
  };

  return (
    <div className="space-y-4">
      {/* Content editing based on block type */}
      {block.type === 'hero' && (
        <>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Headline</label>
            <input
              type="text"
              value={content.headline || ''}
              onChange={(e) => updateContent('headline', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Subheadline</label>
            <input
              type="text"
              value={content.subheadline || ''}
              onChange={(e) => updateContent('subheadline', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Image URL</label>
            <input
              type="text"
              value={content.image || ''}
              onChange={(e) => updateContent('image', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Button Text</label>
            <input
              type="text"
              value={content.buttonText || ''}
              onChange={(e) => updateContent('buttonText', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Button URL</label>
            <input
              type="text"
              value={content.buttonUrl || ''}
              onChange={(e) => updateContent('buttonUrl', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
        </>
      )}

      {block.type === 'text' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Content</label>
          <textarea
            value={content.html || ''}
            onChange={(e) => updateContent('html', e.target.value)}
            rows={6}
            className="w-full border rounded p-2"
          />
          <p className="text-xs text-gray-500 mt-1">
            Use {'{{first_name}}'} for personalization
          </p>
        </div>
      )}

      {block.type === 'button' && (
        <>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Button Text</label>
            <input
              type="text"
              value={content.text || ''}
              onChange={(e) => updateContent('text', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Button URL</label>
            <input
              type="text"
              value={content.url || ''}
              onChange={(e) => updateContent('url', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Alignment</label>
            <select
              value={content.align || 'center'}
              onChange={(e) => updateContent('align', e.target.value)}
              className="w-full border rounded p-2"
            >
              <option value="left">Left</option>
              <option value="center">Center</option>
              <option value="right">Right</option>
            </select>
          </div>
        </>
      )}

      {block.type === 'image' && (
        <>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Image URL</label>
            <input
              type="text"
              value={content.src || ''}
              onChange={(e) => updateContent('src', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Alt Text</label>
            <input
              type="text"
              value={content.alt || ''}
              onChange={(e) => updateContent('alt', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Link URL</label>
            <input
              type="text"
              value={content.link || ''}
              onChange={(e) => updateContent('link', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
        </>
      )}

      {/* Style editing */}
      <div className="border-t pt-4 mt-4">
        <h4 className="font-medium text-gray-800 mb-3">Styling</h4>
        <div className="space-y-3">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Background Color</label>
            <input
              type="color"
              value={styles.backgroundColor || '#ffffff'}
              onChange={(e) => updateStyles('backgroundColor', e.target.value)}
              className="w-full h-10 border rounded"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Padding</label>
            <input
              type="text"
              value={styles.padding || '20px'}
              onChange={(e) => updateStyles('padding', e.target.value)}
              className="w-full border rounded p-2"
            />
          </div>
        </div>
      </div>
    </div>
  );
};

const AIGenerateModal = ({ onClose, onGenerate }) => {
  const [prompt, setPrompt] = useState('');
  const [category, setCategory] = useState('newsletter');
  const [tone, setTone] = useState('professional');
  const [loading, setLoading] = useState(false);

  const handleGenerate = async () => {
    setLoading(true);
    try {
      const token = localStorage.getItem('sampmail_token');
      const orgId = localStorage.getItem('sampmail_org_id');
      const headers = {
        'Content-Type': 'application/json',
      };
      if (token) headers.Authorization = `Bearer ${token}`;
      if (orgId) headers['X-Organization-ID'] = orgId;

      const response = await fetch('/api/v2/templates/generate', {
        method: 'POST',
        headers,
        body: JSON.stringify({ prompt, category, tone }),
      });
      const result = await response.json();
      onGenerate(result);
    } catch (error) {
      console.error('AI generation failed:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl p-6 w-full max-w-lg">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-2">
          <Sparkles className="w-5 h-5 text-purple-600" />
          Generate with AI
        </h2>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Describe your email
            </label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="E.g., Welcome email for new SaaS customers with onboarding steps"
              rows={3}
              className="w-full border rounded-lg p-3"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                className="w-full border rounded-lg p-2"
              >
                <option value="welcome">Welcome</option>
                <option value="newsletter">Newsletter</option>
                <option value="promotional">Promotional</option>
                <option value="transactional">Transactional</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Tone</label>
              <select
                value={tone}
                onChange={(e) => setTone(e.target.value)}
                className="w-full border rounded-lg p-2"
              >
                <option value="professional">Professional</option>
                <option value="casual">Casual</option>
                <option value="friendly">Friendly</option>
                <option value="formal">Formal</option>
              </select>
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-600 hover:text-gray-800"
          >
            Cancel
          </button>
          <button
            onClick={handleGenerate}
            disabled={!prompt || loading}
            className="px-6 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 flex items-center gap-2"
          >
            {loading ? <Sparkles className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
            {loading ? 'Generating...' : 'Generate'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default EmailTemplateBuilder;
