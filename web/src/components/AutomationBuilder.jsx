import React, { useState, useCallback, useRef } from 'react';

const AutomationBuilder = ({ automation, onSave, onActivate, onPause }) => {
  const [nodes, setNodes] = useState(automation?.nodes ? JSON.parse(automation.nodes) : []);
  const [edges, setEdges] = useState(automation?.edges ? JSON.parse(automation.edges) : []);
  const [selectedNode, setSelectedNode] = useState(null);
  const [showNodePalette, setShowNodePalette] = useState(true);
  const canvasRef = useRef(null);

  // Node types configuration
  const nodeTypes = {
    triggers: [
      { type: 'trigger_contact_added', label: 'Contact Added', icon: '👤', description: 'When contact is added to list' },
      { type: 'trigger_tag_added', label: 'Tag Added', icon: '🏷️', description: 'When tag is added to contact' },
      { type: 'trigger_email_opened', label: 'Email Opened', icon: '📬', description: 'When email is opened' },
      { type: 'trigger_link_clicked', label: 'Link Clicked', icon: '🔗', description: 'When link is clicked' },
      { type: 'trigger_form_submitted', label: 'Form Submitted', icon: '📝', description: 'When form is submitted' },
      { type: 'trigger_webhook', label: 'Webhook', icon: '🌐', description: 'External webhook trigger' },
      { type: 'trigger_schedule', label: 'Schedule', icon: '📅', description: 'Time-based trigger' },
    ],
    actions: [
      { type: 'action_send_email', label: 'Send Email', icon: '📧', description: 'Send an email' },
      { type: 'action_send_sms', label: 'Send SMS', icon: '💬', description: 'Send SMS message' },
      { type: 'action_add_tag', label: 'Add Tag', icon: '➕', description: 'Add tag to contact' },
      { type: 'action_remove_tag', label: 'Remove Tag', icon: '➖', description: 'Remove tag from contact' },
      { type: 'action_add_to_list', label: 'Add to List', icon: '📋', description: 'Add to subscriber list' },
      { type: 'action_update_field', label: 'Update Field', icon: '✏️', description: 'Update contact field' },
      { type: 'action_webhook', label: 'Call Webhook', icon: '🔌', description: 'Call external API' },
      { type: 'action_update_score', label: 'Update Score', icon: '⭐', description: 'Update lead score' },
      { type: 'action_notify', label: 'Notify Team', icon: '🔔', description: 'Send notification' },
    ],
    flow: [
      { type: 'delay', label: 'Delay', icon: '⏱️', description: 'Wait for time period' },
      { type: 'condition', label: 'Condition', icon: '🔀', description: 'If/else branch' },
      { type: 'ab_split', label: 'A/B Split', icon: '⚖️', description: 'Split traffic' },
      { type: 'goal', label: 'Goal', icon: '🎯', description: 'Conversion goal' },
      { type: 'exit', label: 'Exit', icon: '🚪', description: 'Exit automation' },
    ],
  };

  // Add node to canvas
  const addNode = (nodeType) => {
    const newNode = {
      id: `node_${Date.now()}`,
      type: nodeType.type.startsWith('trigger_') ? 'trigger' :
        nodeType.type.startsWith('action_') ? 'action' :
          nodeType.type === 'condition' ? 'condition' : 'delay',
      position: { x: 250 + Math.random() * 100, y: 100 + nodes.length * 100 },
      data: {
        label: nodeType.label,
        nodeType: nodeType.type,
        icon: nodeType.icon,
        description: nodeType.description,
        config: {},
      },
    };
    setNodes([...nodes, newNode]);
    setSelectedNode(newNode);
  };

  // Connect nodes
  const connectNodes = (sourceId, targetId, sourceHandle = null) => {
    const newEdge = {
      id: `edge_${Date.now()}`,
      source: sourceId,
      target: targetId,
      sourceHandle,
      animated: true,
      markerEnd: { type: 'arrowclosed' },
    };
    setEdges([...edges, newEdge]);
  };

  // Delete node
  const deleteNode = (nodeId) => {
    setNodes(nodes.filter(n => n.id !== nodeId));
    setEdges(edges.filter(e => e.source !== nodeId && e.target !== nodeId));
    setSelectedNode(null);
  };

  // Update node config
  const updateNodeConfig = (nodeId, config) => {
    setNodes(nodes.map(n =>
      n.id === nodeId ? { ...n, data: { ...n.data, config } } : n
    ));
  };

  // Save workflow
  const handleSave = () => {
    onSave({
      nodes: JSON.stringify(nodes),
      edges: JSON.stringify(edges),
    });
  };

  // Get node color based on type
  const getNodeColor = (type) => {
    if (type.startsWith('trigger_')) return 'bg-green-500 border-green-600';
    if (type.startsWith('action_')) return 'bg-blue-500 border-blue-600';
    if (type === 'condition' || type === 'ab_split') return 'bg-yellow-500 border-yellow-600';
    return 'bg-purple-500 border-purple-600';
  };

  return (
    <div className="flex h-full bg-gray-900">
      {/* Node Palette */}
      {showNodePalette && (
        <div className="w-64 bg-gray-800 border-r border-gray-700 overflow-y-auto">
          <div className="p-4">
            <h3 className="text-white font-semibold mb-4">Workflow Nodes</h3>

            {/* Triggers */}
            <div className="mb-6">
              <h4 className="text-green-400 text-sm font-medium mb-2">⚡ Triggers</h4>
              <div className="space-y-2">
                {nodeTypes.triggers.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-2 rounded bg-gray-700 hover:bg-gray-600 text-white text-sm flex items-center gap-2 transition"
                  >
                    <span>{node.icon}</span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Actions */}
            <div className="mb-6">
              <h4 className="text-blue-400 text-sm font-medium mb-2">📧 Actions</h4>
              <div className="space-y-2">
                {nodeTypes.actions.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-2 rounded bg-gray-700 hover:bg-gray-600 text-white text-sm flex items-center gap-2 transition"
                  >
                    <span>{node.icon}</span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Flow Control */}
            <div className="mb-6">
              <h4 className="text-purple-400 text-sm font-medium mb-2">🔀 Flow Control</h4>
              <div className="space-y-2">
                {nodeTypes.flow.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-2 rounded bg-gray-700 hover:bg-gray-600 text-white text-sm flex items-center gap-2 transition"
                  >
                    <span>{node.icon}</span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Canvas */}
      <div className="flex-1 relative" ref={canvasRef}>
        {/* Toolbar */}
        <div className="absolute top-4 left-4 z-10 flex gap-2">
          <button
            onClick={() => setShowNodePalette(!showNodePalette)}
            className="px-3 py-2 bg-gray-700 text-white rounded hover:bg-gray-600"
          >
            {showNodePalette ? '◀' : '▶'} Nodes
          </button>
          <button
            onClick={handleSave}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            💾 Save
          </button>
          {automation?.status === 'active' ? (
            <button
              onClick={onPause}
              className="px-4 py-2 bg-yellow-600 text-white rounded hover:bg-yellow-700"
            >
              ⏸️ Pause
            </button>
          ) : (
            <button
              onClick={onActivate}
              className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700"
            >
              ▶️ Activate
            </button>
          )}
        </div>

        {/* Workflow Canvas */}
        <div className="h-full bg-gray-900 bg-grid-pattern">
          {nodes.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center text-gray-500">
                <p className="text-xl mb-2">🚀 Start building your automation</p>
                <p>Drag nodes from the left panel or click to add</p>
              </div>
            </div>
          ) : (
            <svg className="w-full h-full">
              {/* Draw edges */}
              {edges.map((edge) => {
                const sourceNode = nodes.find(n => n.id === edge.source);
                const targetNode = nodes.find(n => n.id === edge.target);
                if (!sourceNode || !targetNode) return null;

                const x1 = sourceNode.position.x + 100;
                const y1 = sourceNode.position.y + 40;
                const x2 = targetNode.position.x + 100;
                const y2 = targetNode.position.y;

                return (
                  <g key={edge.id}>
                    <path
                      d={`M ${x1} ${y1} C ${x1} ${y1 + 50}, ${x2} ${y2 - 50}, ${x2} ${y2}`}
                      fill="none"
                      stroke="#4B5563"
                      strokeWidth="2"
                      markerEnd="url(#arrowhead)"
                    />
                  </g>
                );
              })}

              {/* Arrow marker */}
              <defs>
                <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="#4B5563" />
                </marker>
              </defs>
            </svg>
          )}

          {/* Render nodes */}
          {nodes.map((node) => (
            <div
              key={node.id}
              className={`absolute cursor-move ${getNodeColor(node.data.nodeType)} border-2 rounded-lg p-3 min-w-[200px] ${selectedNode?.id === node.id ? 'ring-2 ring-white' : ''
                }`}
              style={{ left: node.position.x, top: node.position.y }}
              onClick={() => setSelectedNode(node)}
            >
              <div className="flex items-center gap-2 text-white">
                <span className="text-lg">{node.data.icon}</span>
                <div>
                  <div className="font-semibold text-sm">{node.data.label}</div>
                  <div className="text-xs opacity-75">{node.data.description}</div>
                </div>
              </div>

              {/* Delete button */}
              <button
                onClick={(e) => { e.stopPropagation(); deleteNode(node.id); }}
                className="absolute -top-2 -right-2 w-5 h-5 bg-red-500 rounded-full text-white text-xs hover:bg-red-600"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Node Config Panel */}
      {selectedNode && (
        <div className="w-80 bg-gray-800 border-l border-gray-700 p-4 overflow-y-auto">
          <h3 className="text-white font-semibold mb-4">
            {selectedNode.data.icon} {selectedNode.data.label}
          </h3>

          <NodeConfigPanel
            node={selectedNode}
            onUpdate={(config) => updateNodeConfig(selectedNode.id, config)}
            onConnect={(targetId, handle) => connectNodes(selectedNode.id, targetId, handle)}
            availableNodes={nodes.filter(n => n.id !== selectedNode.id)}
          />
        </div>
      )}
    </div>
  );
};

const NodeConfigPanel = ({ node, onUpdate, onConnect, availableNodes }) => {
  const [config, setConfig] = useState(node.data.config || {});

  const handleChange = (key, value) => {
    const newConfig = { ...config, [key]: value };
    setConfig(newConfig);
    onUpdate(newConfig);
  };

  const renderConfig = () => {
    switch (node.data.nodeType) {
      case 'action_send_email':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Email Template</label>
              <select
                value={config.template_id || ''}
                onChange={(e) => handleChange('template_id', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="">Select template...</option>
                {/* Templates would be loaded from API */}
              </select>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Or enter subject</label>
              <input
                type="text"
                value={config.subject || ''}
                onChange={(e) => handleChange('subject', e.target.value)}
                placeholder="Email subject with {{first_name}}"
                className="w-full bg-gray-700 text-white rounded p-2"
              />
            </div>
          </>
        );

      case 'delay':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Delay Type</label>
              <select
                value={config.delay_type || 'hours'}
                onChange={(e) => handleChange('delay_type', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="minutes">Minutes</option>
                <option value="hours">Hours</option>
                <option value="days">Days</option>
              </select>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Duration</label>
              <input
                type="number"
                value={config.delay_value || 1}
                onChange={(e) => handleChange('delay_value', parseInt(e.target.value))}
                className="w-full bg-gray-700 text-white rounded p-2"
              />
            </div>
          </>
        );

      case 'condition':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Field</label>
              <select
                value={config.field || ''}
                onChange={(e) => handleChange('field', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="">Select field...</option>
                <option value="email">Email</option>
                <option value="first_name">First Name</option>
                <option value="lead_score">Lead Score</option>
                <option value="total_opens">Total Opens</option>
                <option value="total_clicks">Total Clicks</option>
              </select>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Operator</label>
              <select
                value={config.operator || 'equals'}
                onChange={(e) => handleChange('operator', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="equals">Equals</option>
                <option value="not_equals">Not Equals</option>
                <option value="contains">Contains</option>
                <option value="greater_than">Greater Than</option>
                <option value="less_than">Less Than</option>
                <option value="is_empty">Is Empty</option>
              </select>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Value</label>
              <input
                type="text"
                value={config.value || ''}
                onChange={(e) => handleChange('value', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              />
            </div>
          </>
        );

      case 'action_add_tag':
      case 'action_remove_tag':
        return (
          <div className="mb-4">
            <label className="block text-gray-300 text-sm mb-1">Tag</label>
            <input
              type="text"
              value={config.tag_name || ''}
              onChange={(e) => handleChange('tag_name', e.target.value)}
              placeholder="Enter tag name"
              className="w-full bg-gray-700 text-white rounded p-2"
            />
          </div>
        );

      case 'action_update_score':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Action</label>
              <select
                value={config.action || 'add'}
                onChange={(e) => handleChange('action', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="add">Add Points</option>
                <option value="subtract">Subtract Points</option>
                <option value="set">Set Score</option>
              </select>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Points</label>
              <input
                type="number"
                value={config.points || 0}
                onChange={(e) => handleChange('points', parseInt(e.target.value))}
                className="w-full bg-gray-700 text-white rounded p-2"
              />
            </div>
          </>
        );

      case 'action_webhook':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">URL</label>
              <input
                type="url"
                value={config.url || ''}
                onChange={(e) => handleChange('url', e.target.value)}
                placeholder="https://..."
                className="w-full bg-gray-700 text-white rounded p-2"
              />
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Method</label>
              <select
                value={config.method || 'POST'}
                onChange={(e) => handleChange('method', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2"
              >
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="GET">GET</option>
              </select>
            </div>
          </>
        );

      default:
        return (
          <p className="text-gray-400 text-sm">
            Configuration for this node type is not yet implemented.
          </p>
        );
    }
  };

  return (
    <div>
      {renderConfig()}

      {/* Connect to next node */}
      <div className="mt-6 pt-4 border-t border-gray-700">
        <label className="block text-gray-300 text-sm mb-2">Connect to:</label>
        <select
          onChange={(e) => e.target.value && onConnect(e.target.value)}
          className="w-full bg-gray-700 text-white rounded p-2"
          defaultValue=""
        >
          <option value="">Select node...</option>
          {availableNodes.map((n) => (
            <option key={n.id} value={n.id}>
              {n.data.icon} {n.data.label}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
};

export default AutomationBuilder;
