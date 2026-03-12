import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { 
  AlertCircle, CheckCircle, XCircle, RefreshCw, Brain, 
  Mail, ArrowRight, Filter, ChevronDown, ChevronUp
} from 'lucide-react';

const BouncesPage = () => {
  const [bounces, setBounces] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [analyzing, setAnalyzing] = useState(false);
  const [aiOnly, setAiOnly] = useState(false);
  const [expandedRow, setExpandedRow] = useState(null);
  const [pagination, setPagination] = useState({ limit: 25, offset: 0, total: 0 });

  const loadBounces = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.append('limit', String(pagination.limit));
      params.append('offset', String(pagination.offset));
      if (aiOnly) params.append('ai_only', 'true');

      const data = await api.get(`/v2/bounces?${params}`);
      setBounces(data.bounces || []);
      setPagination(prev => ({ ...prev, total: data.total || 0 }));
    } catch (err) {
      console.error('Failed to load bounces:', err);
    } finally {
      setLoading(false);
    }
  }, [pagination.limit, pagination.offset, aiOnly]);

  const loadStats = useCallback(async () => {
    try {
      const data = await api.get('/v2/bounces/stats');
      setStats(data);
    } catch (err) {
      console.error('Failed to load stats:', err);
    }
  }, []);

  useEffect(() => {
    loadBounces();
    loadStats();
  }, [loadBounces, loadStats]);

  const handleAnalyze = async () => {
    setAnalyzing(true);
    try {
      await api.post('/v2/bounces/analyze', {
        ollama_url: 'http://localhost:11434',
        model: 'qwen2.5:0.5b',
        batch_size: 10
      });
      await loadBounces();
      await loadStats();
    } catch (err) {
      console.error('Analysis failed:', err);
      alert('Analysis failed: ' + (err.message || 'Ollama not available'));
    } finally {
      setAnalyzing(false);
    }
  };

  const getSeverityColor = (severity) => {
    switch (severity) {
      case 'critical': return 'text-red-600 bg-red-50';
      case 'warning': return 'text-amber-600 bg-amber-50';
      case 'info': return 'text-blue-600 bg-blue-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  const getQualityBadge = (quality) => {
    switch (quality) {
      case 'valid': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-green-100 text-green-700"><CheckCircle className="w-3 h-3 inline mr-1" />Valid</span>;
      case 'risky': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-amber-100 text-amber-700"><AlertCircle className="w-3 h-3 inline mr-1" />Risky</span>;
      case 'invalid': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700"><XCircle className="w-3 h-3 inline mr-1" />Invalid</span>;
      default: return null;
    }
  };

  const getBounceTypeBadge = (type_) => {
    switch (type_) {
      case 'hard': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700">Hard</span>;
      case 'soft': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-amber-100 text-amber-700">Soft</span>;
      case 'complaint': return <span className="px-2 py-1 rounded-full text-xs font-medium bg-purple-100 text-purple-700">Complaint</span>;
      default: return <span className="px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">{type_}</span>;
    }
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Brain className="w-6 h-6 text-purple-600" />
            AI Bounce Analyzer
          </h1>
          <p className="text-muted-foreground mt-1">
            Analyze bounce reasons using AI to get actionable insights
          </p>
        </div>
        <button
          onClick={handleAnalyze}
          disabled={analyzing}
          className="flex items-center gap-2 px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${analyzing ? 'animate-spin' : ''}`} />
          {analyzing ? 'Analyzing...' : 'Analyze with AI'}
        </button>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="bg-card rounded-lg p-4 border">
            <div className="text-2xl font-bold">{stats.total?.toLocaleString() || 0}</div>
            <div className="text-sm text-muted-foreground">Total Bounces</div>
          </div>
          <div className="bg-card rounded-lg p-4 border">
            <div className="text-2xl font-bold text-red-600">{stats.hard_bounces?.toLocaleString() || 0}</div>
            <div className="text-sm text-muted-foreground">Hard Bounces</div>
          </div>
          <div className="bg-card rounded-lg p-4 border">
            <div className="text-2xl font-bold text-amber-600">{stats.soft_bounces?.toLocaleString() || 0}</div>
            <div className="text-sm text-muted-foreground">Soft Bounces</div>
          </div>
          <div className="bg-card rounded-lg p-4 border">
            <div className="text-2xl font-bold text-purple-600">{stats.ai_analyzed?.toLocaleString() || 0}</div>
            <div className="text-sm text-muted-foreground">AI Analyzed</div>
          </div>
        </div>
      )}

      {/* AI Stats */}
      {stats && stats.ai_analyzed > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="bg-green-50 rounded-lg p-4 border border-green-200">
            <div className="text-2xl font-bold text-green-700">{stats.ai_valid?.toLocaleString() || 0}</div>
            <div className="text-sm text-green-600">Valid Emails (retry)</div>
          </div>
          <div className="bg-amber-50 rounded-lg p-4 border border-amber-200">
            <div className="text-2xl font-bold text-amber-700">{stats.ai_risky?.toLocaleString() || 0}</div>
            <div className="text-sm text-amber-600">Risky (caution)</div>
          </div>
          <div className="bg-red-50 rounded-lg p-4 border border-red-200">
            <div className="text-2xl font-bold text-red-700">{stats.ai_invalid?.toLocaleString() || 0}</div>
            <div className="text-sm text-red-600">Invalid (remove)</div>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex items-center gap-4">
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={aiOnly}
            onChange={(e) => setAiOnly(e.target.checked)}
            className="w-4 h-4 rounded border-gray-300"
          />
          <span className="text-sm">Show AI analyzed only</span>
        </label>
      </div>

      {/* Table */}
      <div className="bg-card rounded-lg border overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-muted">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium">Email</th>
                <th className="px-4 py-3 text-left text-sm font-medium">Type</th>
                <th className="px-4 py-3 text-left text-sm font-medium">SMTP Code</th>
                <th className="px-4 py-3 text-left text-sm font-medium">AI Analysis</th>
                <th className="px-4 py-3 text-left text-sm font-medium">Quality</th>
                <th className="px-4 py-3 text-left text-sm font-medium">Date</th>
                <th className="px-4 py-3 text-left text-sm font-medium"></th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {loading ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                    Loading...
                  </td>
                </tr>
              ) : bounces.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                    No bounces found
                  </td>
                </tr>
              ) : (
                bounces.map((bounce, idx) => (
                  <React.Fragment key={bounce.id || idx}>
                    <tr className="hover:bg-muted/50">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Mail className="w-4 h-4 text-muted-foreground" />
                          <span className="font-mono text-sm">{bounce.email}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        {getBounceTypeBadge(bounce.bounce_type)}
                      </td>
                      <td className="px-4 py-3">
                        <code className="text-sm bg-muted px-2 py-1 rounded">{bounce.bounce_code || '-'}</code>
                      </td>
                      <td className="px-4 py-3">
                        {bounce.ai_category ? (
                          <div className="space-y-1">
                            <span className={`px-2 py-1 rounded text-xs font-medium ${getSeverityColor(bounce.ai_severity)}`}>
                              {bounce.ai_category}
                            </span>
                            {bounce.ai_explanation && (
                              <p className="text-xs text-muted-foreground line-clamp-2">
                                {bounce.ai_explanation}
                              </p>
                            )}
                          </div>
                        ) : (
                          <span className="text-sm text-muted-foreground">-</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {bounce.ai_email_quality ? getQualityBadge(bounce.ai_email_quality) : '-'}
                      </td>
                      <td className="px-4 py-3 text-sm text-muted-foreground">
                        {bounce.processed_at ? new Date(bounce.processed_at).toLocaleDateString() : '-'}
                      </td>
                      <td className="px-4 py-3">
                        {bounce.ai_category && (
                          <button
                            onClick={() => setExpandedRow(expandedRow === idx ? null : idx)}
                            className="p-1 hover:bg-muted rounded"
                          >
                            {expandedRow === idx ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                          </button>
                        )}
                      </td>
                    </tr>
                    {expandedRow === idx && bounce.ai_category && (
                      <tr>
                        <td colSpan={7} className="px-4 py-4 bg-muted/30">
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-3">
                              <div>
                                <h4 className="text-sm font-medium mb-1">AI Explanation</h4>
                                <p className="text-sm">{bounce.ai_explanation}</p>
                              </div>
                              <div>
                                <h4 className="text-sm font-medium mb-1">Recommended Action</h4>
                                <p className="text-sm">{bounce.ai_action}</p>
                              </div>
                            </div>
                            <div className="space-y-3">
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium">Retryable:</span>
                                {bounce.ai_is_retryable ? (
                                  <CheckCircle className="w-4 h-4 text-green-600" />
                                ) : (
                                  <XCircle className="w-4 h-4 text-red-600" />
                                )}
                                <span className="text-sm">{bounce.ai_is_retryable ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium">Permanent Failure:</span>
                                {bounce.ai_is_permanent_fail ? (
                                  <XCircle className="w-4 h-4 text-red-600" />
                                ) : (
                                  <CheckCircle className="w-4 h-4 text-green-600" />
                                )}
                                <span className="text-sm">{bounce.ai_is_permanent_fail ? 'Yes' : 'No'}</span>
                              </div>
                              {bounce.raw_message && (
                                <div>
                                  <h4 className="text-sm font-medium mb-1">Raw Message</h4>
                                  <pre className="text-xs bg-background p-2 rounded overflow-x-auto max-h-32">
                                    {bounce.raw_message}
                                  </pre>
                                </div>
                              )}
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {pagination.total > pagination.limit && (
          <div className="px-4 py-3 border-t flex items-center justify-between">
            <div className="text-sm text-muted-foreground">
              Showing {pagination.offset + 1} - {Math.min(pagination.offset + pagination.limit, pagination.total)} of {pagination.total}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPagination(p => ({ ...p, offset: Math.max(0, p.offset - p.limit) }))}
                disabled={pagination.offset === 0}
                className="px-3 py-1 text-sm border rounded disabled:opacity-50"
              >
                Previous
              </button>
              <button
                onClick={() => setPagination(p => ({ ...p, offset: p.offset + p.limit }))}
                disabled={pagination.offset + pagination.limit >= pagination.total}
                className="px-3 py-1 text-sm border rounded disabled:opacity-50"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default BouncesPage;
