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
	validDomainPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)
	validLocalPartPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	validIPPattern       = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
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

  local trace_settings = {
    received_header = false,
    supplemental_header = true,
    header_name = 'X-RefID',
  }

  kumo.start_esmtp_listener {
    listen = '{{.ListenAddr}}',
    hostname = '{{.MainHostname}}',
    banner = '220 ' .. '{{.MainHostname}}',
    relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
    trace_headers = trace_settings,
  }

  kumo.start_esmtp_listener {
    listen = '0.0.0.0:587',
    hostname = '{{.MainHostname}}',
    banner = '220 ' .. '{{.MainHostname}}',
    relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
    trace_headers = trace_settings,
  }

  kumo.start_esmtp_listener {
    listen = '0.0.0.0:465',
    hostname = '{{.MainHostname}}',
    banner = '220 ' .. '{{.MainHostname}}',
    relay_hosts = { {{range $i, $h := .RelayHosts}}{{if $i}}, {{end}}'{{$h}}'{{end}} },
    trace_headers = trace_settings,
  }
end)

-- Load config files
local sources_data = kumo.toml_load('{{.PolicyDir}}/sources.toml')
local queues_data = kumo.toml_load('{{.PolicyDir}}/queues.toml')
local dkim_data = kumo.toml_load('{{.PolicyDir}}/dkim_data.toml')
local listener_domains = kumo.toml_load('{{.PolicyDir}}/listener_domains.toml')
local auth_users = kumo.toml_load('{{.PolicyDir}}/auth.toml')

kumo.on('smtp_server_auth_plain', function(auth_user, auth_password)
  local valid_pass = auth_users[auth_user]
  if valid_pass and valid_pass == auth_password then
    return true
  end
  return false
end)

local function get_tenant_from_sender(sender_email)
  if sender_email then
    local localpart, domain = sender_email:match("([^@]+)@(.+)")
    if localpart and domain then
      return domain .. "__" .. localpart
    end
  end
  return "default"
end

kumo.on('get_listener_domain', function(domain, listener, conn_meta)
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

kumo.on('get_egress_path_config', function(domain, egress_source, site_name)
  return kumo.make_egress_path {
    enable_tls = 'OpportunisticInsecure',
    enable_mta_sts = false,
  }
end)

kumo.on('get_queue_config', function(domain, tenant, campaign, routing_domain)
  tenant = tenant or "default"
  local cfg = queues_data['tenant:' .. tenant] or {}
  return kumo.make_queue_config {
    egress_pool = cfg.egress_pool or tenant,
    retry_interval = cfg.retry_interval or '1m',
    max_age = cfg.max_age or '3d',
    max_message_rate = cfg.max_message_rate,
  }
end)

local function dkim_sign_message(msg)
  local sender = msg:from_header()
  if not sender then return end

  local sender_email = sender.email:lower()
  local sender_domain = sender.domain:lower()

  local domain_cfg = dkim_data.domain[sender_domain]
  if not domain_cfg or not domain_cfg.policy then return end

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
end

local function scrub_headers(msg)
  msg:remove_all_named_headers('User-Agent')
  msg:remove_all_named_headers('X-Mailer')
  msg:remove_all_named_headers('X-Originating-IP')
  msg:remove_x_headers { 'x-campaign', 'x-tenant', 'x-kumomta' }
end

kumo.on('smtp_server_message_received', function(msg)
  local sender = msg:from_header()
  local sender_email = sender and sender.email or ""
  local tenant = get_tenant_from_sender(sender_email)
  msg:set_meta('tenant', tenant)

  local campaign = msg:get_first_named_header_value('X-Campaign')
  if campaign then msg:set_meta('campaign', campaign) end

  scrub_headers(msg)
  dkim_sign_message(msg)
end)

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

  scrub_headers(msg)
  dkim_sign_message(msg)
end)

pcall(dofile, '{{.PolicyDir}}/custom.lua')
`

// GenerateInitLua generates init.lua using templates for security
func GenerateInitLua(snap *Snapshot) string {
	cfg := config.Get()

	mainHostname := "localhost"
	relayIPs := []string{"127.0.0.1"}
	listenAddr := "127.0.0.1:25"

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
