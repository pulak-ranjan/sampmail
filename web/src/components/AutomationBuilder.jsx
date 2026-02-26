import React, { useState, useCallback, useRef } from 'react';
import {
  UserPlus, Tag, Mail, Link, FileText, Globe, Calendar,
  Send, MessageSquare, Plus, Minus, List, Edit2, Zap, Star, Bell,
  Clock, GitBranch, Scale, Target, LogOut,
  ChevronLeft, ChevronRight, Play, Pause, Save,
  MousePointer2, ZoomIn, ZoomOut, X
} from 'lucide-react';

const AutomationBuilder = ({ automation, onSave, onActivate, onPause }) => {
  const parseJSON = (input, fallback) => {
    if (!input) return fallback;
    if (typeof input !== 'string') return input;
    try {
      return JSON.parse(input);
    } catch {
      return fallback;
    }
  };

  const [nodes, setNodes] = useState(() => {
    const parsed = parseJSON(automation?.nodes, []);
    return Array.isArray(parsed) ? parsed : [];
  });
  const [edges, setEdges] = useState(() => {
    const parsed = parseJSON(automation?.edges, []);
    return Array.isArray(parsed) ? parsed : [];
  });
  const [selectedNode, setSelectedNode] = useState(null);
  const [showNodePalette, setShowNodePalette] = useState(true);
  const canvasRef = useRef(null);

  // Node types configuration
  const nodeTypes = {
    triggers: [
      { type: 'trigger_contact_added', label: 'Contact Added', icon: UserPlus, description: 'When contact is added to list' },
      { type: 'trigger_tag_added', label: 'Tag Added', icon: Tag, description: 'When tag is added to contact' },
      { type: 'trigger_email_opened', label: 'Email Opened', icon: Mail, description: 'When email is opened' },
      { type: 'trigger_link_clicked', label: 'Link Clicked', icon: Link, description: 'When link is clicked' },
      { type: 'trigger_form_submitted', label: 'Form Submitted', icon: FileText, description: 'When form is submitted' },
      { type: 'trigger_webhook', label: 'Webhook', icon: Globe, description: 'External webhook trigger' },
      { type: 'trigger_schedule', label: 'Schedule', icon: Calendar, description: 'Time-based trigger' },
    ],
    actions: [
      { type: 'action_send_email', label: 'Send Email', icon: Send, description: 'Send an email' },
      { type: 'action_send_sms', label: 'Send SMS', icon: MessageSquare, description: 'Send SMS message' },
      { type: 'action_add_tag', label: 'Add Tag', icon: Plus, description: 'Add tag to contact' },
      { type: 'action_remove_tag', label: 'Remove Tag', icon: Minus, description: 'Remove tag from contact' },
      { type: 'action_add_to_list', label: 'Add to List', icon: List, description: 'Add to subscriber list' },
      { type: 'action_update_field', label: 'Update Field', icon: Edit2, description: 'Update contact field' },
      { type: 'action_webhook', label: 'Call Webhook', icon: Zap, description: 'Call external API' },
      { type: 'action_update_score', label: 'Update Score', icon: Star, description: 'Update lead score' },
      { type: 'action_notify', label: 'Notify Team', icon: Bell, description: 'Send notification' },
    ],
    flow: [
      { type: 'delay', label: 'Delay', icon: Clock, description: 'Wait for time period' },
      { type: 'condition', label: 'Condition', icon: GitBranch, description: 'If/else branch' },
      { type: 'ab_split', label: 'A/B Split', icon: Scale, description: 'Split traffic' },
      { type: 'goal', label: 'Goal', icon: Target, description: 'Conversion goal' },
      { type: 'exit', label: 'Exit', icon: LogOut, description: 'Exit automation' },
    ],
  };

  // Canvas state
  const [scale, setScale] = useState(1);
  const [isConnectMode, setIsConnectMode] = useState(false);
  const [connectSource, setConnectSource] = useState(null);

  // Add node to canvas center
  const addNode = (nodeType) => {
    const newNode = {
      id: `node_${Date.now()}`,
      type: nodeType.type.startsWith('trigger_') ? 'trigger' :
        nodeType.type.startsWith('action_') ? 'action' :
          nodeType.type === 'condition' ? 'condition' : 'delay',
      position: { x: 400 + (nodes.length * 20), y: 100 + (nodes.length * 20) },
      data: {
        label: nodeType.label,
        nodeType: nodeType.type,
        iconData: nodeType.label, // Fallback identifier
        description: nodeType.description,
        config: {},
      },
    };
    setNodes([...nodes, newNode]);
    setSelectedNode(newNode);
  };

  // Connect nodes logic
  const handleNodeClick = (node) => {
    if (isConnectMode) {
      if (!connectSource) {
        setConnectSource(node.id);
      } else {
        if (connectSource !== node.id) {
          connectNodes(connectSource, node.id);
          setConnectSource(null);
          setIsConnectMode(false);
        }
      }
    } else {
      setSelectedNode(node);
    }
  };

  // Connect nodes
  const connectNodes = (sourceId, targetId, sourceHandle = null) => {
    // Prevent duplicates
    if (edges.some(e => e.source === sourceId && e.target === targetId)) return;

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

  const getNodeColor = (type) => {
    if (type.startsWith('trigger_')) return 'bg-emerald-500 border-emerald-600';
    if (type.startsWith('action_')) return 'bg-blue-500 border-blue-600';
    if (type === 'condition' || type === 'ab_split') return 'bg-amber-500 border-amber-600';
    return 'bg-violet-500 border-violet-600';
  };

  // Helper to find icon component from node type
  const getIconComponent = (nodeType) => {
    // Search in all categories
    const allTypes = [...nodeTypes.triggers, ...nodeTypes.actions, ...nodeTypes.flow];
    const def = allTypes.find(t => t.type === nodeType);
    return def ? def.icon : Zap;
  };

  return (
    <div className="flex h-full bg-gray-900 border-t border-gray-800">
      {/* Node Palette */}
      {showNodePalette && (
        <div className="w-64 bg-gray-800 border-r border-gray-700 overflow-y-auto shrink-0 z-20">
          <div className="p-4">
            <h3 className="text-white font-semibold mb-4 text-sm uppercase tracking-wide opacity-70">Workflow Components</h3>

            {/* Triggers */}
            <div className="mb-6">
              <h4 className="text-emerald-400 text-xs font-bold mb-3 uppercase">Triggers</h4>
              <div className="grid gap-2">
                {nodeTypes.triggers.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-3 rounded-lg bg-gray-700/50 hover:bg-gray-700 hover:ring-1 hover:ring-emerald-500/50 text-white text-sm flex items-center gap-3 transition-all group"
                  >
                    <span className="p-1.5 bg-gray-800 rounded group-hover:bg-gray-600 transition-colors">
                      <node.icon className="w-4 h-4" />
                    </span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Actions */}
            <div className="mb-6">
              <h4 className="text-blue-400 text-xs font-bold mb-3 uppercase">Actions</h4>
              <div className="grid gap-2">
                {nodeTypes.actions.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-3 rounded-lg bg-gray-700/50 hover:bg-gray-700 hover:ring-1 hover:ring-blue-500/50 text-white text-sm flex items-center gap-3 transition-all group"
                  >
                    <span className="p-1.5 bg-gray-800 rounded group-hover:bg-gray-600 transition-colors">
                      <node.icon className="w-4 h-4" />
                    </span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Flow Control */}
            <div className="mb-6">
              <h4 className="text-amber-400 text-xs font-bold mb-3 uppercase">Logic</h4>
              <div className="grid gap-2">
                {nodeTypes.flow.map((node) => (
                  <button
                    key={node.type}
                    onClick={() => addNode(node)}
                    className="w-full text-left p-3 rounded-lg bg-gray-700/50 hover:bg-gray-700 hover:ring-1 hover:ring-amber-500/50 text-white text-sm flex items-center gap-3 transition-all group"
                  >
                    <span className="p-1.5 bg-gray-800 rounded group-hover:bg-gray-600 transition-colors">
                      <node.icon className="w-4 h-4" />
                    </span>
                    <span>{node.label}</span>
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Canvas */}
      <div className="flex-1 relative overflow-hidden bg-dot-pattern" ref={canvasRef}>
        {/* Toolbar */}
        <div className="absolute top-4 left-4 right-4 z-10 flex justify-between items-center pointer-events-none">
          <div className="pointer-events-auto flex gap-2">
            <button
              onClick={() => setShowNodePalette(!showNodePalette)}
              className="p-2 bg-gray-800 text-white rounded-lg border border-gray-700 shadow-sm hover:bg-gray-700 transition"
              title="Toggle Sidebar"
            >
              {showNodePalette ? <ChevronLeft className="w-5 h-5" /> : <ChevronRight className="w-5 h-5" />}
            </button>
            <div className="flex items-center bg-gray-800 rounded-lg border border-gray-700 shadow-sm px-2">
              <button onClick={() => setScale(Math.max(0.2, scale - 0.1))} className="p-2 text-white hover:text-blue-400">
                <ZoomOut className="w-4 h-4" />
              </button>
              <span className="text-xs text-gray-400 min-w-[3rem] text-center">{Math.round(scale * 100)}%</span>
              <button onClick={() => setScale(Math.min(2, scale + 0.1))} className="p-2 text-white hover:text-blue-400">
                <ZoomIn className="w-4 h-4" />
              </button>
            </div>

            <button
              onClick={() => { setIsConnectMode(!isConnectMode); setConnectSource(null); }}
              className={`px-4 py-2 border border-gray-700 shadow-sm rounded-lg font-medium transition-all flex items-center gap-2 ${isConnectMode
                  ? 'bg-blue-600 text-white ring-2 ring-blue-400 ring-offset-2 ring-offset-gray-900 border-transparent animate-pulse'
                  : 'bg-gray-800 text-gray-300 hover:bg-gray-700'
                }`}
            >
              {isConnectMode ? (
                connectSource ? <><MousePointer2 className="w-4 h-4" /> Select Target...</> : <><MousePointer2 className="w-4 h-4" /> Select Source</>
              ) : (
                <><Link className="w-4 h-4" /> Connect</>
              )}
            </button>
          </div>

          <div className="pointer-events-auto flex gap-2">
            {automation?.status === 'active' ? (
              <button onClick={onPause} className="px-4 py-2 bg-amber-600 text-white rounded-lg hover:bg-amber-700 shadow font-medium flex items-center gap-2">
                <Pause className="w-4 h-4" /> Pause
              </button>
            ) : (
              <button onClick={onActivate} className="px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 shadow font-medium flex items-center gap-2">
                <Play className="w-4 h-4" /> Activate
              </button>
            )}
            <button onClick={handleSave} className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 shadow font-medium flex items-center gap-2">
              <Save className="w-4 h-4" /> Save
            </button>
          </div>
        </div>

        {/* Workflow Canvas */}
        <div
          className="w-full h-full relative cursor-grab active:cursor-grabbing origin-top-left"
          style={{ transform: `scale(${scale})`, transformOrigin: '0 0' }}
        >
          {nodes.length === 0 ? (
            <div className="flex items-center justify-center h-full pointer-events-none select-none">
              <div className="text-center text-gray-600">
                <div className="text-6xl mb-4 opacity-50 flex justify-center">
                  <Zap className="w-24 h-24" />
                </div>
                <h3 className="text-xl font-bold mb-2">Build your workflow</h3>
                <p>Add triggers and actions from the sidebar to get started.</p>
              </div>
            </div>
          ) : (
            <svg
              className="absolute inset-0 w-full h-full pointer-events-none"
              style={{ minWidth: '3000px', minHeight: '3000px' }} // SVG canvas needs substantial size
            >
              <defs>
                <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="#6B7280" />
                </marker>
              </defs>
              {edges.map((edge) => {
                const sourceNode = nodes.find(n => n.id === edge.source);
                const targetNode = nodes.find(n => n.id === edge.target);
                if (!sourceNode || !targetNode) return null;

                const x1 = sourceNode.position.x + 120; // Center width approx
                const y1 = sourceNode.position.y + 80;
                const x2 = targetNode.position.x + 120;
                const y2 = targetNode.position.y;

                return (
                  <path
                    key={edge.id}
                    d={`M ${x1} ${y1} C ${x1} ${y1 + 50}, ${x2} ${y2 - 50}, ${x2} ${y2}`}
                    fill="none"
                    stroke={isConnectMode && connectSource === edge.source ? '#3B82F6' : '#4B5563'}
                    strokeWidth="2"
                    markerEnd="url(#arrowhead)"
                    className="transition-all duration-300"
                  />
                );
              })}
              {isConnectMode && connectSource && nodes.find(n => n.id === connectSource) && (
                // Draw partial line from source if needed (skipped for simplicity in this SVG approach)
                <></>
              )}
            </svg>
          )}

          {nodes.map((node) => {
            const Icon = getIconComponent(node.data.nodeType);
            return (
              <div
                key={node.id}
                className={`
                  absolute cursor-pointer transition-all duration-200 group
                  w-[240px] rounded-xl border border-gray-700 bg-gray-800 shadow-lg
                  ${isConnectMode && connectSource === node.id ? 'ring-4 ring-blue-500/50 scale-105 z-30' : ''}
                  ${isConnectMode && !connectSource ? 'hover:ring-2 hover:ring-blue-400 hover:scale-105 z-20' : ''}
                  ${isConnectMode && connectSource && connectSource !== node.id ? 'hover:ring-2 hover:ring-green-400 hover:scale-105 z-20' : ''}
                  ${selectedNode?.id === node.id && !isConnectMode ? 'ring-2 ring-white z-20' : ''}
                `}
                style={{ left: node.position.x, top: node.position.y }}
                onClick={() => handleNodeClick(node)}
                onMouseDown={(e) => {
                  // Basic drag implementation (disabled in connect mode)
                  if (isConnectMode) return;
                  const startX = e.clientX - node.position.x;
                  const startY = e.clientY - node.position.y;

                  const handleMouseMove = (mv) => {
                    const newX = mv.clientX - startX;
                    const newY = mv.clientY - startY;
                    setNodes(ns => ns.map(n => n.id === node.id ? { ...n, position: { x: newX, y: newY } } : n));
                  };

                  const handleMouseUp = () => {
                    window.removeEventListener('mousemove', handleMouseMove);
                    window.removeEventListener('mouseup', handleMouseUp);
                  };

                  window.addEventListener('mousemove', handleMouseMove);
                  window.addEventListener('mouseup', handleMouseUp);
                }}
              >
                {/* Header */}
                <div className={`
                  p-3 rounded-t-xl flex items-center justify-between
                  ${getNodeColor(node.data.nodeType)} text-white overflow-hidden
                `}>
                  <div className="flex items-center gap-2 font-medium text-sm">
                    <Icon className="w-4 h-4" />
                    <span>{node.data.label}</span>
                  </div>
                  <button
                    onClick={(e) => { e.stopPropagation(); deleteNode(node.id); }}
                    className="opacity-0 group-hover:opacity-100 p-1 hover:bg-black/20 rounded transition-opacity"
                  >
                    <X className="w-3 h-3" />
                  </button>
                </div>

                {/* Body */}
                <div className="p-3">
                  <p className="text-xs text-gray-400 mb-2 line-clamp-2">{node.data.description}</p>
                  {node.data.config && Object.keys(node.data.config).length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {Object.keys(node.data.config).slice(0, 2).map(k => (
                        <span key={k} className="text-[10px] bg-gray-700 text-gray-300 px-1.5 py-0.5 rounded border border-gray-600">
                          {k}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Handles (Visual only) */}
                <div className="absolute -top-1.5 left-1/2 -translate-x-1/2 w-3 h-3 bg-gray-500 rounded-full border-2 border-gray-900 group-hover:bg-white transition-colors" />
                <div className="absolute -bottom-1.5 left-1/2 -translate-x-1/2 w-3 h-3 bg-gray-500 rounded-full border-2 border-gray-900 group-hover:bg-white transition-colors" />
              </div>
            );
          })}
        </div>
      </div>


      {/* Node Config Panel */}
      {selectedNode && (
        <div className="w-80 bg-gray-800 border-l border-gray-700 p-4 overflow-y-auto z-20 shadow-xl">
          <div className="flex items-center gap-3 mb-6 pb-4 border-b border-gray-700">
            <div className={`p-2 rounded-lg ${getNodeColor(selectedNode.data.nodeType)} text-white`}>
              {(() => {
                const Icon = getIconComponent(selectedNode.data.nodeType);
                return <Icon className="w-5 h-5" />;
              })()}
            </div>
            <div>
              <h3 className="text-white font-semibold">{selectedNode.data.label}</h3>
              <p className="text-xs text-gray-400">Configuration</p>
            </div>
          </div>

          <NodeConfigPanel
            node={selectedNode}
            onUpdate={(config) => updateNodeConfig(selectedNode.id, config)}
            onConnect={(targetId, handle) => connectNodes(selectedNode.id, targetId, handle)}
            availableNodes={nodes.filter(n => n.id !== selectedNode.id)}
            getIconComponent={getIconComponent}
          />
        </div>
      )}
    </div>
  );
};

const NodeConfigPanel = ({ node, onUpdate, onConnect, availableNodes, getIconComponent }) => {
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
              />
            </div>
          </>
        );

      case 'action_add_tag':
      case 'action_remove_tag':
        return (
          <div className="mb-4">
            <label className="block text-gray-300 text-sm mb-1">Tag</label>
            <div className="relative">
              <Tag className="absolute left-3 top-2.5 h-4 w-4 text-gray-500" />
              <input
                type="text"
                value={config.tag_name || ''}
                onChange={(e) => handleChange('tag_name', e.target.value)}
                placeholder="Enter tag name"
                className="w-full bg-gray-700 text-white rounded p-2 pl-9 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
              />
            </div>
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
              />
            </div>
          </>
        );

      case 'action_webhook':
        return (
          <>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">URL</label>
              <div className="relative">
                <Globe className="absolute left-3 top-2.5 h-4 w-4 text-gray-500" />
                <input
                  type="url"
                  value={config.url || ''}
                  onChange={(e) => handleChange('url', e.target.value)}
                  placeholder="https://..."
                  className="w-full bg-gray-700 text-white rounded p-2 pl-9 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
                />
              </div>
            </div>
            <div className="mb-4">
              <label className="block text-gray-300 text-sm mb-1">Method</label>
              <select
                value={config.method || 'POST'}
                onChange={(e) => handleChange('method', e.target.value)}
                className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
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
          <p className="text-gray-400 text-sm flex items-center gap-2">
            <Clock className="w-4 h-4" /> Configuration for this node type is not yet implemented.
          </p>
        );
    }
  };

  return (
    <div>
      {renderConfig()}

      {/* Connect to next node */}
      <div className="mt-6 pt-4 border-t border-gray-700">
        <label className="block text-gray-300 text-sm mb-2 flex items-center gap-2">
          <Link className="w-3 h-3" /> Connect manually to:
        </label>
        <select
          onChange={(e) => e.target.value && onConnect(e.target.value)}
          className="w-full bg-gray-700 text-white rounded p-2 border border-gray-600 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
          defaultValue=""
        >
          <option value="">Select node...</option>
          {availableNodes.map((n) => {
            const Icon = getIconComponent(n.data.nodeType);
            return (
              <option key={n.id} value={n.id}>
                {n.data.label} (ID: {n.id.substr(-4)})
              </option>
            );
          })}
        </select>
      </div>
    </div>
  );
};

export default AutomationBuilder;
