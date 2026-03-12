import { useState, useEffect, useCallback } from 'react';
import {
  BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, PieChart, Pie, Cell
} from 'recharts';
import {
  TrendingUp, TrendingDown, Mail, MousePointer,
  AlertTriangle, CheckCircle, Users, Send, Eye, Target
} from 'lucide-react';
import api from '../api';

export default function AnalyticsDashboard() {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [period, setPeriod] = useState(7);
  const [deliverability, setDeliverability] = useState(null);

  const loadDashboard = useCallback(async () => {
    try {
      setLoading(true);
      const [dashData, delivData] = await Promise.all([
        api.get(`/v2/analytics/dashboard?days=${period}`),
        api.get(`/v2/analytics/deliverability?days=${period}`)
      ]);
      setData(dashData);
      setDeliverability(delivData);
    } catch (err) {
      console.error('Failed to load analytics:', err);
    } finally {
      setLoading(false);
    }
  }, [period]);

  useEffect(() => {
    loadDashboard();
  }, [period, loadDashboard]);

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600"></div>
      </div>
    );
  }

  if (!data) {
    return <div className="p-8 text-center text-gray-500">Failed to load analytics</div>;
  }

  const COLORS = ['#4F46E5', '#10B981', '#F59E0B', '#EF4444'];

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold">Analytics Dashboard</h1>
          <p className="text-gray-500">Email performance overview</p>
        </div>
        <select
          value={period}
          onChange={(e) => setPeriod(Number(e.target.value))}
          className="px-4 py-2 border rounded-lg"
        >
          <option value={7}>Last 7 days</option>
          <option value={14}>Last 14 days</option>
          <option value={30}>Last 30 days</option>
          <option value={90}>Last 90 days</option>
        </select>
      </div>

      {/* Deliverability Score */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-1 bg-white rounded-xl shadow-sm p-6 border">
          <h3 className="text-lg font-semibold mb-4">Deliverability Score</h3>
          <div className="flex items-center justify-center">
            <div className="relative w-40 h-40">
              <svg className="w-full h-full" viewBox="0 0 100 100">
                <circle
                  className="text-gray-200"
                  strokeWidth="10"
                  stroke="currentColor"
                  fill="transparent"
                  r="40"
                  cx="50"
                  cy="50"
                />
                <circle
                  className={`${data.deliverability_score >= 80 ? 'text-green-500' :
                    data.deliverability_score >= 60 ? 'text-yellow-500' : 'text-red-500'
                    }`}
                  strokeWidth="10"
                  strokeDasharray={`${data.deliverability_score * 2.51} 251`}
                  strokeLinecap="round"
                  stroke="currentColor"
                  fill="transparent"
                  r="40"
                  cx="50"
                  cy="50"
                  transform="rotate(-90 50 50)"
                />
              </svg>
              <div className="absolute inset-0 flex items-center justify-center">
                <span className="text-3xl font-bold">{Math.round(data.deliverability_score)}</span>
              </div>
            </div>
          </div>
          <div className="mt-4 text-center text-sm text-gray-500">
            {data.deliverability_score >= 80 ? 'Excellent' :
              data.deliverability_score >= 60 ? 'Good' : 'Needs Improvement'}
          </div>
        </div>

        {/* Health Status */}
        <div className="lg:col-span-2 bg-white rounded-xl shadow-sm p-6 border">
          <h3 className="text-lg font-semibold mb-4">Health Status</h3>
          {deliverability?.health && (
            <div className="space-y-4">
              <div className={`flex items-center gap-3 p-4 rounded-lg ${deliverability.health.status === 'good' ? 'bg-green-50 text-green-700' :
                deliverability.health.status === 'warning' ? 'bg-yellow-50 text-yellow-700' :
                  'bg-red-50 text-red-700'
                }`}>
                {deliverability.health.status === 'good' ? (
                  <CheckCircle size={24} />
                ) : (
                  <AlertTriangle size={24} />
                )}
                <span className="font-medium capitalize">
                  {deliverability.health.status === 'good' ? 'All systems healthy' :
                    `${deliverability.health.issues?.length || 0} issues detected`}
                </span>
              </div>

              {deliverability.health.issues?.length > 0 && (
                <ul className="space-y-2">
                  {deliverability.health.issues.map((issue, i) => (
                    <li key={i} className="flex items-start gap-2 text-sm text-gray-600">
                      <span className="text-red-500">•</span>
                      {issue}
                    </li>
                  ))}
                </ul>
              )}

              {deliverability.recommendations?.length > 0 && (
                <div className="mt-4 pt-4 border-t">
                  <h4 className="font-medium text-sm mb-2">Recommendations</h4>
                  <ul className="space-y-1">
                    {deliverability.recommendations.map((rec, i) => (
                      <li key={i} className="flex items-start gap-2 text-sm text-gray-600">
                        <span className="text-indigo-500">→</span>
                        {rec}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          title="Total Sent"
          value={data.period.sent.toLocaleString()}
          icon={<Send className="text-indigo-600" />}
          trend={null}
        />
        <StatCard
          title="Opens"
          value={data.period.opens.toLocaleString()}
          subtitle={`${data.rates.open_rate.toFixed(1)}% rate`}
          icon={<Eye className="text-green-600" />}
          trend={data.rates.open_rate > 20 ? 'up' : 'down'}
        />
        <StatCard
          title="Clicks"
          value={data.period.clicks.toLocaleString()}
          subtitle={`${data.rates.click_rate.toFixed(1)}% rate`}
          icon={<MousePointer className="text-blue-600" />}
          trend={data.rates.click_rate > 2 ? 'up' : 'down'}
        />
        <StatCard
          title="Bounces"
          value={data.period.bounces.toLocaleString()}
          subtitle={`${data.rates.bounce_rate.toFixed(2)}% rate`}
          icon={<AlertTriangle className="text-red-600" />}
          trend={data.rates.bounce_rate < 2 ? 'up' : 'down'}
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Daily Stats Chart */}
        <div className="bg-white rounded-xl shadow-sm p-6 border">
          <h3 className="text-lg font-semibold mb-4">Daily Performance</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={data.daily_stats}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis
                dataKey="date"
                tickFormatter={(v) => new Date(v).toLocaleDateString('en', { month: 'short', day: 'numeric' })}
                tick={{ fontSize: 12 }}
              />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip
                labelFormatter={(v) => new Date(v).toLocaleDateString()}
                formatter={(v) => [v.toLocaleString(), '']}
              />
              <Line type="monotone" dataKey="sent" stroke="#4F46E5" strokeWidth={2} name="Sent" />
              <Line type="monotone" dataKey="opens" stroke="#10B981" strokeWidth={2} name="Opens" />
              <Line type="monotone" dataKey="clicks" stroke="#F59E0B" strokeWidth={2} name="Clicks" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Bounce Breakdown */}
        <div className="bg-white rounded-xl shadow-sm p-6 border">
          <h3 className="text-lg font-semibold mb-4">Bounce Breakdown</h3>
          {deliverability && (
            <div className="flex items-center">
              <ResponsiveContainer width="50%" height={200}>
                <PieChart>
                  <Pie
                    data={[
                      { name: 'Hard Bounces', value: deliverability.bounces.hard },
                      { name: 'Soft Bounces', value: deliverability.bounces.soft },
                      { name: 'Complaints', value: deliverability.bounces.complaints },
                    ]}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    dataKey="value"
                  >
                    {COLORS.map((color, index) => (
                      <Cell key={`cell-${index}`} fill={color} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <div className="w-3 h-3 rounded-full bg-indigo-600"></div>
                  <span className="text-sm">Hard Bounces: {deliverability.bounces.hard}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-3 h-3 rounded-full bg-green-500"></div>
                  <span className="text-sm">Soft Bounces: {deliverability.bounces.soft}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
                  <span className="text-sm">Complaints: {deliverability.bounces.complaints}</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-white rounded-xl shadow-sm p-4 border">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-indigo-100 rounded-lg">
              <Users className="text-indigo-600" size={20} />
            </div>
            <div>
              <div className="text-2xl font-bold">{data.overview.total_subscribers.toLocaleString()}</div>
              <div className="text-sm text-gray-500">Total Subscribers</div>
            </div>
          </div>
        </div>
        <div className="bg-white rounded-xl shadow-sm p-4 border">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-green-100 rounded-lg">
              <Mail className="text-green-600" size={20} />
            </div>
            <div>
              <div className="text-2xl font-bold">{data.overview.total_campaigns.toLocaleString()}</div>
              <div className="text-sm text-gray-500">Total Campaigns</div>
            </div>
          </div>
        </div>
        <div className="bg-white rounded-xl shadow-sm p-4 border">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-blue-100 rounded-lg">
              <Send className="text-blue-600" size={20} />
            </div>
            <div>
              <div className="text-2xl font-bold">{data.overview.total_sent.toLocaleString()}</div>
              <div className="text-sm text-gray-500">Total Emails Sent</div>
            </div>
          </div>
        </div>
        <div className="bg-white rounded-xl shadow-sm p-4 border">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-purple-100 rounded-lg">
              <Target className="text-purple-600" size={20} />
            </div>
            <div>
              <div className="text-2xl font-bold">{data.overview.total_domains}</div>
              <div className="text-sm text-gray-500">Active Domains</div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Campaigns */}
      {data.recent_campaigns?.length > 0 && (
        <div className="bg-white rounded-xl shadow-sm p-6 border">
          <h3 className="text-lg font-semibold mb-4">Recent Campaigns</h3>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="text-left text-sm text-gray-500 border-b">
                  <th className="pb-3 font-medium">Campaign</th>
                  <th className="pb-3 font-medium">Status</th>
                  <th className="pb-3 font-medium">Sent</th>
                  <th className="pb-3 font-medium">Created</th>
                </tr>
              </thead>
              <tbody>
                {data.recent_campaigns.map(campaign => (
                  <tr key={campaign.id} className="border-b last:border-0">
                    <td className="py-3">
                      <div className="font-medium">{campaign.name}</div>
                      <div className="text-sm text-gray-500">{campaign.subject}</div>
                    </td>
                    <td className="py-3">
                      <span className={`px-2 py-1 text-xs rounded-full ${campaign.status === 'sent' ? 'bg-green-100 text-green-700' :
                        campaign.status === 'sending' ? 'bg-blue-100 text-blue-700' :
                          campaign.status === 'scheduled' ? 'bg-yellow-100 text-yellow-700' :
                            'bg-gray-100 text-gray-700'
                        }`}>
                        {campaign.status}
                      </span>
                    </td>
                    <td className="py-3">{campaign.sent_count?.toLocaleString() || '-'}</td>
                    <td className="py-3 text-sm text-gray-500">
                      {new Date(campaign.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ title, value, subtitle, icon, trend }) {
  return (
    <div className="bg-white rounded-xl shadow-sm p-4 border">
      <div className="flex items-center justify-between">
        <div className="p-2 bg-gray-100 rounded-lg">{icon}</div>
        {trend && (
          <div className={`flex items-center text-sm ${trend === 'up' ? 'text-green-600' : 'text-red-600'}`}>
            {trend === 'up' ? <TrendingUp size={16} /> : <TrendingDown size={16} />}
          </div>
        )}
      </div>
      <div className="mt-3">
        <div className="text-2xl font-bold">{value}</div>
        <div className="text-sm text-gray-500">{title}</div>
        {subtitle && <div className="text-xs text-gray-400 mt-1">{subtitle}</div>}
      </div>
    </div>
  );
}
