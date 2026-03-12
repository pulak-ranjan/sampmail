package core

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// SECURITY: Input validation patterns
var (
	validDomainPattern    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)
	validLocalPartPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	validIPPattern        = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
)

// SourceName returns egress source name
func SourceName(d models.Domain, s models.Sender) string {
	return fmt.Sprintf("%s__%s", sanitizeLuaString(d.Name), sanitizeLuaString(s.LocalPart))
}

// PoolName returns egress pool name
func PoolName(d models.Domain, s models.Sender) string {
	return fmt.Sprintf("%s__%s", sanitizeLuaString(d.Name), sanitizeLuaString(s.LocalPart))
}

// sanitizeLuaString escapes strings for safe Lua embedding
func sanitizeLuaString(s string) string {
	// Remove any characters that could break Lua syntax
	var safe strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			safe.WriteRune(r)
		default:
			// Skip invalid characters
		}
	}
	return safe.String()
}

// validateDomain checks domain name safety
func validateDomain(domain string) error {
	if domain == "" || len(domain) > 253 {
		return fmt.Errorf("domain name invalid length")
	}
	if !validDomainPattern.MatchString(domain) {
		return fmt.Errorf("domain contains invalid characters")
	}
	return nil
}

// validateLocalPart checks local part safety
func validateLocalPart(lp string) error {
	if lp == "" || len(lp) > 64 {
		return fmt.Errorf("local part invalid length")
	}
	if !validLocalPartPattern.MatchString(lp) {
		return fmt.Errorf("local part contains invalid characters")
	}
	return nil
}

// validateIP checks IP address format
func validateIP(ip string) error {
	if ip == "" {
		return nil // Empty is OK
	}
	if !validIPPattern.MatchString(ip) {
		return fmt.Errorf("invalid IP format")
	}
	return nil
}

// GenerateAuthTOML generates auth.toml
func GenerateAuthTOML(snap *Snapshot) string {
	var b strings.Builder
	b.WriteString("# KumoMTA SMTP Authentication Credentials\n")
	b.WriteString("# Format: username = \"password\"\n\n")

	for _, d := range snap.Domains {
		if err := validateDomain(d.Name); err != nil {
			logger.Warn("skipping invalid domain", "domain", d.Name, "error", err)
			continue
		}
		for _, s := range d.Senders {
			if s.SMTPPassword == "" {
				continue
			}
			if err := validateLocalPart(s.LocalPart); err != nil {
				logger.Warn("skipping invalid sender", "email", s.Email, "error", err)
				continue
			}
			// Escape quotes and backslashes for TOML
			safePass := strings.ReplaceAll(s.SMTPPassword, "\\", "\\\\")
			safePass = strings.ReplaceAll(safePass, "\"", "\\\"")
			safeEmail := sanitizeLuaString(s.Email)
			fmt.Fprintf(&b, "\"%s\" = \"%s\"\n", safeEmail, safePass)
		}
	}
	return b.String()
}

// GenerateSourcesTOML generates sources.toml
func GenerateSourcesTOML(snap *Snapshot) string {
	var b strings.Builder

	for _, d := range snap.Domains {
		if len(d.Senders) == 0 {
			continue
		}
		if err := validateDomain(d.Name); err != nil {
			continue
		}

		fmt.Fprintf(&b, "# ========================================\n")
		fmt.Fprintf(&b, "# %s Sources\n", sanitizeLuaString(d.Name))
		fmt.Fprintf(&b, "# ========================================\n\n")

		for _, s := range d.Senders {
			if err := validateLocalPart(s.LocalPart); err != nil {
				continue
			}
			if err := validateIP(s.IP); err != nil {
				continue
			}

			name := SourceName(d, s)
			ehloDomain := fmt.Sprintf("%s.%s", sanitizeLuaString(s.LocalPart), sanitizeLuaString(d.Name))

			fmt.Fprintf(&b, "[\"%s\"]\n", name)
			fmt.Fprintf(&b, "source_address = \"%s\"\n", sanitizeLuaString(s.IP))
			fmt.Fprintf(&b, "ehlo_domain = \"%s\"\n\n", ehloDomain)
		}
	}
	return b.String()
}

// GenerateQueuesTOML generates queues.toml
func GenerateQueuesTOML(snap *Snapshot) string {
	var b strings.Builder

	for _, d := range snap.Domains {
		if len(d.Senders) == 0 {
			continue
		}
		if err := validateDomain(d.Name); err != nil {
			continue
		}

		fmt.Fprintf(&b, "# ========================================\n")
		fmt.Fprintf(&b, "# %s Tenants\n", sanitizeLuaString(d.Name))
		fmt.Fprintf(&b, "# ========================================\n\n")

		for _, s := range d.Senders {
			if err := validateLocalPart(s.LocalPart); err != nil {
				continue
			}

			pool := PoolName(d, s)
			tenantKey := fmt.Sprintf("tenant:%s", pool)

			fmt.Fprintf(&b, "[\"%s\"]\n", tenantKey)
			fmt.Fprintf(&b, "egress_pool = \"%s\"\n", pool)
			fmt.Fprintf(&b, "retry_interval = \"1m\"\n")
			fmt.Fprintf(&b, "max_age = \"3d\"\n")

			rate := GetSenderRate(s)
			if rate != "" {
				fmt.Fprintf(&b, "max_message_rate = \"%s\"\n", sanitizeLuaString(rate))
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	return b.String()
}

// GenerateListenerDomainsTOML generates listener_domains.toml
func GenerateListenerDomainsTOML(snap *Snapshot) string {
	var b strings.Builder
	for _, d := range snap.Domains {
		if err := validateDomain(d.Name); err != nil {
			continue
		}
		fmt.Fprintf(&b, "[\"%s\"]\n", sanitizeLuaString(d.Name))
		fmt.Fprintf(&b, "relay_to = true\n")
		fmt.Fprintf(&b, "log_oob = true\n")
		fmt.Fprintf(&b, "log_arf = true\n\n")
	}
	return b.String()
}

// GenerateDKIMDataTOML generates dkim_data.toml using config
func GenerateDKIMDataTOML(snap *Snapshot, dkimBasePath string) string {
	var b strings.Builder

	for _, d := range snap.Domains {
		if len(d.Senders) == 0 {
			continue
		}
		if err := validateDomain(d.Name); err != nil {
			continue
		}

		fmt.Fprintf(&b, "# ========================================\n")
		fmt.Fprintf(&b, "# %s DKIM\n", sanitizeLuaString(d.Name))
		fmt.Fprintf(&b, "# ========================================\n\n")

		safeDomain := sanitizeLuaString(d.Name)
		fmt.Fprintf(&b, "[domain.\"%s\"]\n", safeDomain)
		fmt.Fprintf(&b, "selector = \"default\"\n")
		fmt.Fprintf(&b, "headers = [\"From\", \"To\", \"Subject\", \"Date\", \"Message-ID\", \"List-Unsubscribe\"]\n\n")

		for _, s := range d.Senders {
			if err := validateLocalPart(s.LocalPart); err != nil {
				continue
			}

			selector := sanitizeLuaString(s.LocalPart)
			keyFile := fmt.Sprintf("%s/%s/%s.key",
				strings.TrimRight(dkimBasePath, "/"),
				safeDomain,
				selector)
			matchSender := sanitizeLuaString(s.Email)

			fmt.Fprintf(&b, "[[domain.\"%s\".policy]]\n", safeDomain)
			fmt.Fprintf(&b, "selector = \"%s\"\n", selector)
			fmt.Fprintf(&b, "filename = \"%s\"\n", keyFile)
			fmt.Fprintf(&b, "match_sender = \"%s\"\n\n", matchSender)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

// LuaConfig holds data for Lua template
type LuaConfig struct {
	MainHostname string
	ListenAddr   string
	RelayHosts   []string
	SpoolDir     string
	LogDir       string
	PolicyDir    string
	TLSCertPath  string
	TLSKeyPath   string
}

// initLuaTemplate is the secure template for init.lua
const initLuaTemplate = `local kumo = require 'kumo'

kumo.on('init', function()
  kumo.define_spool {
    name = 'data',
    path = '{{.SpoolDir}}/data',
    kind = 'LocalDisk',
  }

  kumo.define_spool {
    name = 'meta',
    path = '{{.SpoolDir}}/meta',
    kind = 'LocalDisk',
  }

  kumo.configure_local_logs {
    log_dir = '{{.LogDir}}',
    max_segment_duration = '10 seconds',
  }

  kumo.configure_bounce_classifier {
    files = {
      '/opt/kumomta/share/bounce_classifier/iana.toml',
    },
  }

  kumo.start_http_listener {
    listen = '127.0.0.1:8000',
    use_tls = false,
    trusted_hosts = { '127.0.0.1' },
  }

  -- Define Stealth Trace Settings (hides KumoMTA)
  local trace_settings = {
    received_header = false,
    supplemental_header = true,
    header_name = 'X-RefID',
  }

  -- TLS Configuration: Check if certificates exist, graceful fallback
  local tls_options = nil
  local cert_path = '{{.TLSCertPath}}'
  local key_path = '{{.TLSKeyPath}}'

  local function file_exists(path)
    local f = io.open(path, "r")
    if f then f:close() return true end
    return false
  end

  if file_exists(cert_path) and file_exists(key_path) then
    tls_options = {
      certificate = cert_path,
      private_key = key_path,
    }
  end

  -- Port 25: Main SMTP listener (relay from trusted IPs)
  kumo.start_esmtp_listener {
    listen = '{{.ListenAddr}}',
    hostname = '{{.MainHostname}}',
    banner = '220 ' .. '{{.MainHostname}}',
    relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
    trace_headers = trace_settings,
    tls_certificate = tls_options and tls_options.certificate or nil,
    tls_private_key = tls_options and tls_options.private_key or nil,
  }

  -- Port 587: Submission (STARTTLS + AUTH)
  kumo.start_esmtp_listener {
    listen = '0.0.0.0:587',
    hostname = '{{.MainHostname}}',
    banner = '220 ' .. '{{.MainHostname}}',
    relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
    trace_headers = trace_settings,
    tls_certificate = tls_options and tls_options.certificate or nil,
    tls_private_key = tls_options and tls_options.private_key or nil,
  }

  -- Port 465: SMTPS (Implicit TLS) - only if certificates exist
  if tls_options then
    kumo.start_esmtp_listener {
      listen = '0.0.0.0:465',
      hostname = '{{.MainHostname}}',
      banner = '220 ' .. '{{.MainHostname}}',
      relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
      trace_headers = trace_settings,
      tls_certificate = tls_options.certificate,
      tls_private_key = tls_options.private_key,
      tls_implicit = true,
    }
  end
end)

-- Load config files
local sources_data = kumo.toml_load('{{.PolicyDir}}/sources.toml')
local queues_data = kumo.toml_load('{{.PolicyDir}}/queues.toml')
local dkim_data = kumo.toml_load('{{.PolicyDir}}/dkim_data.toml')
local listener_domains = kumo.toml_load('{{.PolicyDir}}/listener_domains.toml')
local auth_users = kumo.toml_load('{{.PolicyDir}}/auth.toml')

-- =====================================================
-- SMTP AUTHENTICATION (PLAIN) with logging
-- =====================================================
kumo.on('smtp_server_auth_plain', function(authzid, authcid, password)
  if not auth_users or type(auth_users) ~= 'table' then
    kumo.log_error("SMTP AUTH: auth.toml not loaded or empty")
    return false
  end

  local valid_pass = auth_users[authcid]
  if valid_pass then
    if valid_pass == password then
      kumo.log_info("SMTP AUTH: Success for " .. authcid)
      return true
    else
      kumo.log_error("SMTP AUTH: Invalid password for " .. authcid)
    end
  else
    kumo.log_error("SMTP AUTH: Unknown user " .. authcid)
  end
  return false
end)

-- =====================================================
-- TENANT LOGIC (Double-Underscore Separator)
-- =====================================================
local function get_tenant_from_sender(sender_email)
  if sender_email then
    local localpart, domain = sender_email:match("([^@]+)@(.+)")
    if localpart and domain then
      return domain .. "__" .. localpart
    end
  end
  return "default"
end

-- =====================================================
-- LISTENER DOMAIN CONFIG
-- =====================================================
kumo.on('get_listener_domain', function(domain, listener, conn_meta)
  local authz_id = conn_meta:get_meta('authz_id')
  if authz_id then
    return kumo.make_listener_domain {
      relay_to = true,
      log_oob = true,
      log_arf = true,
    }
  end

  if listener_domains[domain] then
    local config = listener_domains[domain]
    return kumo.make_listener_domain {
      relay_to = config.relay_to or false,
      log_oob = config.log_oob or false,
      log_arf = config.log_arf or false,
    }
  end
  return kumo.make_listener_domain { relay_to = false }
end)

-- =====================================================
-- EGRESS POOLS / SOURCES
-- =====================================================
kumo.on('get_egress_pool', function(pool_name)
  if sources_data[pool_name] then
    return kumo.make_egress_pool {
      name = pool_name,
      entries = { { name = pool_name } },
    }
  end
  return kumo.make_egress_pool { name = pool_name, entries = {} }
end)

kumo.on('get_egress_source', function(source_name)
  local cfg = sources_data[source_name]
  if cfg then
    return kumo.make_egress_source {
      name = source_name,
      source_address = cfg.source_address,
      ehlo_domain = cfg.ehlo_domain,
    }
  end
  return kumo.make_egress_source { name = source_name }
end)

-- =====================================================
-- ISP TRAFFIC SHAPING (Protect sender reputation)
-- =====================================================
local google_limits = {
  max_message_rate = '50/h',
  max_connection_rate = '5/min',
  max_deliveries_per_connection = 20,
  connection_limit = 3,
}
local microsoft_limits = {
  max_message_rate = '50/h',
  max_connection_rate = '3/min',
  max_deliveries_per_connection = 10,
  connection_limit = 2,
}
local yahoo_limits = {
  max_message_rate = '100/h',
  max_connection_rate = '5/min',
  max_deliveries_per_connection = 20,
  connection_limit = 3,
}

local isp_patterns = {
  { pattern = 'google.com',     limits = google_limits },
  { pattern = 'google.co.',    limits = google_limits },
  { pattern = 'googlemail.com', limits = google_limits },
  { pattern = 'outlook.com',    limits = microsoft_limits },
  { pattern = 'hotmail.com',    limits = microsoft_limits },
  { pattern = 'live.com',       limits = microsoft_limits },
  { pattern = 'office365.com',  limits = microsoft_limits },
  { pattern = 'yahoodns.net',   limits = yahoo_limits },
  { pattern = 'yahoo.com',      limits = yahoo_limits },
  { pattern = 'aol.com',        limits = yahoo_limits },
}

local function get_isp_limit(site_name)
  local sn = site_name:lower()
  for _, entry in ipairs(isp_patterns) do
    if sn:find(entry.pattern, 1, true) then
      return entry.limits
    end
  end
  return nil
end

kumo.on('get_egress_path_config', function(domain, egress_source, site_name)
  local limits = get_isp_limit(site_name)
  if limits then
    return kumo.make_egress_path {
      enable_tls = 'OpportunisticInsecure',
      enable_mta_sts = false,
      max_message_rate = limits.max_message_rate,
      max_connection_rate = limits.max_connection_rate,
      max_deliveries_per_connection = limits.max_deliveries_per_connection,
      connection_limit = limits.connection_limit,
    }
  end

  -- Default limits
  return kumo.make_egress_path {
    enable_tls = 'OpportunisticInsecure',
    enable_mta_sts = false,
    max_connection_rate = '10/min',
    max_deliveries_per_connection = 50,
    connection_limit = 5,
  }
end)

-- =====================================================
-- QUEUE CONFIG
-- =====================================================
kumo.on('get_queue_config', function(domain, tenant, campaign, routing_domain)
  tenant = tenant or "default"
  local cfg = queues_data['tenant:' .. tenant] or {}
  return kumo.make_queue_config {
    egress_pool = cfg.egress_pool or tenant,
    retry_interval = cfg.retry_interval or '5m',
    max_age = cfg.max_age or '3d',
    max_message_rate = cfg.max_message_rate,
  }
end)

-- =====================================================
-- DKIM SIGNING
-- =====================================================
local function dkim_sign_message(msg)
  local sender = msg:from_header()
  if not sender then
    kumo.log_error("DKIM: missing From header")
    return
  end

  local sender_email = sender.email:lower()
  local sender_domain = sender.domain:lower()

  local domain_cfg = dkim_data.domain[sender_domain]
  if not domain_cfg or not domain_cfg.policy then
    kumo.log_error("DKIM: no DKIM config for domain " .. sender_domain)
    return
  end

  for _, policy in ipairs(domain_cfg.policy) do
    if sender_email == policy.match_sender:lower() then
      msg:dkim_sign(kumo.dkim.rsa_sha256_signer {
        domain = sender_domain,
        selector = policy.selector,
        headers = domain_cfg.headers,
        key = policy.filename,
      })
      return
    end
  end

  kumo.log_error("DKIM: no identity match for " .. sender_email)
end

-- =====================================================
-- HEADER SCRUBBING + SAFE RECEIVED HEADER
-- =====================================================
local function scrub_headers(msg)
  msg:remove_all_named_headers('User-Agent')
  msg:remove_all_named_headers('X-Mailer')
  msg:remove_all_named_headers('X-Originating-IP')
  msg:remove_all_named_headers('X-Report-Abuse')
  msg:remove_all_named_headers('X-EBS')
  msg:remove_x_headers { 'x-campaign', 'x-tenant', 'x-kumomta' }

  local remote_ip = msg:get_meta('received_from_ip') or '127.0.0.1'
  local timestamp = os.date("%a, %d %b %Y %H:%M:%S %z")
  local rcpt = msg:recipient() or "unknown"

  msg:prepend_header('Received', string.format(
    "from %s ([%s])\r\n\tby %s (Postfix) with ESMTPS\r\n\tfor <%s>; %s",
    msg:get_meta('received_from_name') or 'localhost',
    remote_ip,
    '{{.MainHostname}}',
    rcpt,
    timestamp
  ))
end

-- =====================================================
-- ENVELOPE-FROM SEPARATION (Bounce handling per sender)
-- =====================================================
local function set_envelope_from(msg)
  local sender = msg:from_header()
  if not sender then return end

  local localpart = sender.email:match("([^@]+)@")
  local domain = sender.domain
  if localpart and domain then
    local envelope = string.format('%s@%s.%s', localpart, localpart, domain)
    msg:set_sender(envelope)
  end
end

-- =====================================================
-- SMTP PATH (SMTP injection)
-- =====================================================
kumo.on('smtp_server_message_received', function(msg)
  local sender = msg:from_header()
  local sender_email = sender and sender.email or ""
  local tenant = get_tenant_from_sender(sender_email)
  msg:set_meta('tenant', tenant)

  local campaign = msg:get_first_named_header_value('X-Campaign')
  if campaign then msg:set_meta('campaign', campaign) end

  set_envelope_from(msg)
  scrub_headers(msg)
  dkim_sign_message(msg)
end)

-- =====================================================
-- HTTP / API PATH
-- =====================================================
kumo.on('http_message_generated', function(msg)
  local tenant = msg:get_first_named_header_value('X-Tenant')
  if not tenant then
    local sender = msg:from_header()
    local sender_email = sender and sender.email or ""
    tenant = get_tenant_from_sender(sender_email)
  end
  msg:set_meta('tenant', tenant)

  local campaign = msg:get_first_named_header_value('X-Campaign')
  if campaign then msg:set_meta('campaign', campaign) end

  set_envelope_from(msg)
  scrub_headers(msg)
  dkim_sign_message(msg)
end)

-- Custom hook for manual overrides
pcall(dofile, '{{.PolicyDir}}/custom.lua')
`

// GenerateInitLua generates init.lua using templates for security
func GenerateInitLua(snap *Snapshot) string {
	cfg := config.Get()

	mainHostname := "localhost"
	relayIPs := []string{"127.0.0.1"}
	listenAddr := "127.0.0.1:25"
	tlsCertPath := "/etc/ssl/certs/mail.crt"
	tlsKeyPath := "/etc/ssl/private/mail.key"

	if snap.Settings != nil {
		if snap.Settings.MainHostname != "" {
			mainHostname = sanitizeLuaString(snap.Settings.MainHostname)
		}
		if snap.Settings.SMTPListenAddr != "" {
			listenAddr = sanitizeLuaString(snap.Settings.SMTPListenAddr)
		}
		if snap.Settings.MailWizzIP != "" {
			parts := strings.Split(snap.Settings.MailWizzIP, ",")
			relayIPs = []string{"127.0.0.1"}
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" && validIPPattern.MatchString(p) {
					relayIPs = append(relayIPs, sanitizeLuaString(p))
				}
			}
		}
	}

	luaCfg := LuaConfig{
		MainHostname: mainHostname,
		ListenAddr:   listenAddr,
		RelayHosts:   relayIPs,
		SpoolDir:     cfg.SpoolDir,
		LogDir:       cfg.LogDir,
		PolicyDir:    cfg.PolicyPath(),
		TLSCertPath:  tlsCertPath,
		TLSKeyPath:   tlsKeyPath,
	}

	tmpl, err := template.New("initlua").Parse(initLuaTemplate)
	if err != nil {
		logger.Error("failed to parse init.lua template", "error", err)
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, luaCfg); err != nil {
		logger.Error("failed to execute init.lua template", "error", err)
		return ""
	}

	return buf.String()
}
