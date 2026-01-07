const API_BASE = "/api";

function getToken() {
  return localStorage.getItem("sampmail_token") || "";
}

// Helper for pages that make direct fetch calls - includes both token and org ID
export function getAuthHeaders() {
  const headers = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${getToken()}`
  };
  const orgId = localStorage.getItem("sampmail_org_id");
  if (orgId) headers["X-Organization-ID"] = orgId;
  return headers;
}

export async function apiRequest(path, options = {}) {
  const { method = "GET", body, auth = true } = options;
  const headers = { "Content-Type": "application/json" };

  if (auth) {
    const token = getToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;

    // Multi-tenancy: Organization Context
    const orgId = localStorage.getItem("sampmail_org_id");
    if (orgId) headers["X-Organization-ID"] = orgId;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined
  });

  const text = await res.text();
  let data;
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { raw: text };
  }

  if (!res.ok) {
    const msg = data.error || res.statusText || "Request failed";
    throw new Error(msg);
  }

  return data;
}

// Auth
export function registerAdmin(email, password) {
  return apiRequest("/auth/register", {
    method: "POST",
    body: { email, password },
    auth: false
  });
}

export function login(email, password) {
  return apiRequest("/auth/login", {
    method: "POST",
    body: { email, password },
    auth: false
  });
}

export function me() {
  return apiRequest("/auth/me");
}

// System
export function getSystemIPs() {
  return apiRequest("/system/ips");
}

export function addSystemIPs(cidr, list) {
  return apiRequest("/system/ips", {
    method: "POST",
    body: { cidr, list }
  });
}

// NEW: Configure IP on Server (Needed for IPsPage)
export function configureSystemIP(ip, netmask, iface) {
  return apiRequest("/system/ips/configure", {
    method: "POST",
    body: { ip, netmask, interface: iface }
  });
}

// Status & Dashboard
export function getStatus() {
  return apiRequest("/status", { auth: false });
}

export function getDashboardStats() {
  return apiRequest("/dashboard/stats");
}

export function getAIInsights() {
  return apiRequest("/system/ai-analyze", {
    method: "POST",
    body: { type: "logs" }
  });
}

// --- NEW: Chat Agent (Updated) ---
export function sendAIChat(payload) {
  // payload is { messages: [], new_msg: "..." }
  return apiRequest("/ai/chat", {
    method: "POST",
    body: payload
  });
}

// Settings
export function getSettings() {
  return apiRequest("/settings");
}

export function saveSettings(payload) {
  return apiRequest("/settings", { method: "POST", body: payload });
}

// Domains & senders
export function listDomains() {
  return apiRequest("/domains");
}

export function saveDomain(domain) {
  const method = domain.id ? "PUT" : "POST";
  const url = domain.id ? `/domains/${domain.id}` : "/domains";
  return apiRequest(url, { method, body: domain });
}

export function deleteDomain(id) {
  return apiRequest(`/domains/${id}`, { method: "DELETE" });
}

export function listSenders(domainID) {
  return apiRequest(`/domains/${domainID}/senders`);
}

export function saveSender(domainID, sender) {
  if (sender.id) {
    return apiRequest(`/senders/${sender.id}`, { method: "PUT", body: sender });
  }
  return apiRequest(`/domains/${domainID}/senders`, {
    method: "POST",
    body: sender
  });
}

export function deleteSender(id) {
  return apiRequest(`/senders/${id}`, { method: "DELETE" });
}

// Config
export function previewConfig() {
  return apiRequest("/config/preview");
}

export function applyConfig() {
  return apiRequest("/config/apply", { method: "POST" });
}

// DKIM
export function listDKIMRecords() {
  return apiRequest("/dkim/records");
}

export function generateDKIM(domain, localPart) {
  return apiRequest("/dkim/generate", {
    method: "POST",
    body: { domain, local_part: localPart }
  });
}

export function importSenders(file) {
  const formData = new FormData();
  formData.append("file", file);
  return fetch(`${API_BASE}/import/csv`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${getToken()}`
    },
    body: formData
  }).then(async (res) => {
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Import failed");
    return data;
  });
}

// Bounce
export function listBounces() {
  return apiRequest("/bounces");
}

export function saveBounce(b) {
  return apiRequest("/bounces", { method: "POST", body: b });
}

export function deleteBounce(id) {
  return apiRequest(`/bounces/${id}`, { method: "DELETE" });
}

export function applyBounces() {
  return apiRequest("/bounces/apply", { method: "POST" });
}

// Logs
export function getLogs(service, lines = 100) {
  return apiRequest(`/logs/${service}?lines=${lines}`);
}

// --- Tools & Security ---
export function sendTestEmail(payload) {
  return apiRequest("/tools/send-test", {
    method: "POST",
    body: payload
  });
}

export function blockIP(ip) {
  return apiRequest("/system/action/block-ip", {
    method: "POST",
    body: { ip }
  });
}

export function checkBlacklist() {
  return apiRequest("/system/check-blacklist", { method: "POST" });
}

export function checkSecurity() {
  return apiRequest("/system/check-security", { method: "POST" });
}

// --- Warmup ---
export function getWarmupList() {
  return apiRequest("/warmup");
}

export function updateWarmup(senderID, enabled, plan) {
  return apiRequest(`/warmup/${senderID}`, {
    method: "POST",
    body: { enabled, plan }
  });
}

// --- API Keys ---
export function listKeys() {
  return apiRequest("/keys");
}

export function createKey(name, scopes) {
  return apiRequest("/keys", {
    method: "POST",
    body: { name, scopes }
  });
}

export function deleteKey(id) {
  return apiRequest(`/keys/${id}`, { method: "DELETE" });
}

// --- Proxies ---
export function listProxies() {
  return apiRequest("/proxies");
}

export function createProxy(proxy) {
  return apiRequest("/proxies", {
    method: "POST",
    body: proxy
  });
}

export function updateProxy(id, proxy) {
  return apiRequest(`/proxies/${id}`, {
    method: "PUT",
    body: proxy
  });
}

export function deleteProxy(id) {
  return apiRequest(`/proxies/${id}`, { method: "DELETE" });
}

export function testProxy(id) {
  return apiRequest(`/proxies/${id}/test`, { method: "POST" });
}

// --- Sending IPs ---
export function listSendingIPs() {
  return apiRequest("/sending-ips");
}

export function createSendingIP(ip) {
  return apiRequest("/sending-ips", {
    method: "POST",
    body: ip
  });
}

export function updateSendingIP(id, ip) {
  return apiRequest(`/sending-ips/${id}`, {
    method: "PUT",
    body: ip
  });
}

export function deleteSendingIP(id) {
  return apiRequest(`/sending-ips/${id}`, { method: "DELETE" });
}

// --- Queue (KumoMTA) ---
export function getQueue() {
  return apiRequest("/queue");
}

export function getQueueStats() {
  return apiRequest("/queue/stats");
}

export function flushQueue() {
  return apiRequest("/queue/flush", { method: "POST" });
}

export function deleteQueueMessage(id) {
  return apiRequest(`/queue/${id}`, { method: "DELETE" });
}

// --- DMARC ---
export function getDMARCReports() {
  return apiRequest("/dmarc/reports");
}

export function getDMARCStats() {
  return apiRequest("/dmarc/stats");
}

// --- Stats ---
export function getEmailStats(days = 30) {
  return apiRequest(`/stats?days=${days}`);
}

// --- Lists & Contacts ---
export function listContactLists() {
  return apiRequest("/lists");
}

export function createContactList(list) {
  return apiRequest("/lists", {
    method: "POST",
    body: list
  });
}

export function deleteContactList(id) {
  return apiRequest(`/lists/${id}`, { method: "DELETE" });
}

export function getContacts(listId, page = 1, limit = 50) {
  return apiRequest(`/lists/${listId}/contacts?page=${page}&limit=${limit}`);
}

export function addContact(listId, contact) {
  return apiRequest(`/lists/${listId}/contacts`, {
    method: "POST",
    body: contact
  });
}

// --- Campaigns ---
export function listCampaigns() {
  return apiRequest("/campaigns");
}

export function getCampaign(id) {
  return apiRequest(`/campaigns/${id}`);
}

export function createCampaign(campaign) {
  return apiRequest("/campaigns", {
    method: "POST",
    body: campaign
  });
}

export function updateCampaign(id, campaign) {
  return apiRequest(`/campaigns/${id}`, {
    method: "PUT",
    body: campaign
  });
}

export function sendCampaign(id) {
  return apiRequest(`/campaigns/${id}/send`, { method: "POST" });
}

// --- Templates ---
export function listTemplates() {
  return apiRequest("/templates");
}

export function getTemplate(id) {
  return apiRequest(`/templates/${id}`);
}

export function saveTemplate(template) {
  if (template.id) {
    return apiRequest(`/templates/${template.id}`, {
      method: "PUT",
      body: template
    });
  }
  return apiRequest("/templates", {
    method: "POST",
    body: template
  });
}

export function deleteTemplate(id) {
  return apiRequest(`/templates/${id}`, { method: "DELETE" });
}

// --- Webhooks ---
export function listWebhooks() {
  return apiRequest("/webhooks");
}

export function createWebhook(webhook) {
  return apiRequest("/webhooks", {
    method: "POST",
    body: webhook
  });
}

export function deleteWebhook(id) {
  return apiRequest(`/webhooks/${id}`, { method: "DELETE" });
}

export function testWebhook(id) {
  return apiRequest(`/webhooks/${id}/test`, { method: "POST" });
}

// --- Admin / Superadmin ---
export function adminListTenants() {
  return apiRequest("/v2/admin/organizations");
}

export function adminCreateTenant(data) {
  return apiRequest("/v2/admin/organizations", { method: "POST", body: data });
}

export function adminDeleteTenant(id) {
  return apiRequest(`/v2/admin/organizations/${id}`, { method: "DELETE" });
}

export function adminGetTenant(id) {
  return apiRequest(`/v2/organizations/${id}`);
}

export function adminUpdateTenant(id, data) {
  return apiRequest(`/v2/organizations/${id}`, { method: "PUT", body: data });
}

// --- System Health ---
export function getSystemHealth() {
  return apiRequest("/system/health");
}

export function getSystemServices() {
  return apiRequest("/system/services");
}

export function getSystemPorts() {
  return apiRequest("/system/ports");
}

// --- Default Export (Axios-like wrapper for compatibility) ---
const api = {
  get: (path, options = {}) => apiRequest(path, { ...options, method: "GET" }),
  post: (path, body, options = {}) => apiRequest(path, { ...options, method: "POST", body }),
  put: (path, body, options = {}) => apiRequest(path, { ...options, method: "PUT", body }),
  delete: (path, options = {}) => apiRequest(path, { ...options, method: "DELETE" }),
  apiRequest
};

export default api;
