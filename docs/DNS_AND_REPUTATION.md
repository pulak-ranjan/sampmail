# DNS Management & IP Reputation Guide

This guide covers essential DNS configuration and IP reputation management for successful email delivery with SampMail.

---

## DNS Records Overview

| Record | Purpose | Priority |
|--------|---------|----------|
| **SPF** | Authorize sending IPs | Critical |
| **DKIM** | Sign emails cryptographically | Critical |
| **DMARC** | Define policy for failures | Critical |
| **rDNS/PTR** | Reverse DNS for IP | Critical |
| **MX** | Receive bounce emails | Important |

---

## SPF (Sender Policy Framework)

SPF tells receivers which IPs can send email for your domain.

### Basic SPF Record

```dns
v=spf1 ip4:YOUR_SERVER_IP -all
```

### With Multiple IPs

```dns
v=spf1 ip4:192.168.1.100 ip4:192.168.1.101 ~all
```

### Best Practices
- Use `-all` (hard fail) not `~all` (soft fail) for production
- Keep under 10 DNS lookups
- Include all sending IPs

---

## DKIM (DomainKeys Identified Mail)

DKIM adds a cryptographic signature to your emails.

### Setup in SampMail
1. Go to **Domains** → **Add Domain**
2. SampMail generates DKIM keys automatically
3. Add the provided DNS record:

```dns
selector._domainkey.example.com. IN TXT "v=DKIM1; k=rsa; p=YOUR_PUBLIC_KEY"
```

### DKIM Tips
- Use 2048-bit keys (minimum)
- Rotate keys every 6-12 months
- Keep the private key secure

---

## DMARC (Domain-based Message Authentication)

DMARC tells receivers what to do when SPF/DKIM fails.

### Start with Monitoring

```dns
_dmarc.example.com. IN TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
```

### Move to Quarantine

```dns
_dmarc.example.com. IN TXT "v=DMARC1; p=quarantine; pct=25; rua=mailto:dmarc@example.com"
```

### Full Enforcement

```dns
_dmarc.example.com. IN TXT "v=DMARC1; p=reject; rua=mailto:dmarc@example.com"
```

### DMARC Progression
1. Start with `p=none` (monitor only)
2. Review reports for 2-4 weeks
3. Move to `p=quarantine` at 25%, then 50%, then 100%
4. Finally set `p=reject` when ready

---

## Reverse DNS (PTR/rDNS)

**Critical**: Most major providers (Gmail, Outlook) reject mail from IPs without proper rDNS.

### What You Need
- PTR record set by your hosting provider
- Should match your mail hostname

### Example
```
192.168.1.100 → mail.example.com
mail.example.com → 192.168.1.100
```

### How to Set
1. **VPS Providers**: Usually in control panel (Vultr, DigitalOcean, Linode)
2. **Dedicated**: Contact hosting provider
3. **Cloud**: AWS/GCP have specific processes

---

## IP Reputation Management

### New IP Warmup

Never send high volume from a new IP immediately. Follow SampMail's warmup schedules:

| Day | Conservative | Standard | Aggressive |
|-----|-------------|----------|------------|
| 1 | 10/hr | 25/hr | 50/hr |
| 2 | 20/hr | 50/hr | 100/hr |
| 3 | 40/hr | 100/hr | 250/hr |
| 4 | 80/hr | 200/hr | 500/hr |
| 5 | 150/hr | 400/hr | 1000/hr |
| ... | ... | ... | ... |

### Monitoring Reputation

Check these RBLs regularly:
- [MXToolbox](https://mxtoolbox.com/blacklists.aspx)
- [MultiRBL](http://multirbl.valli.org/)

SampMail's webhook service can alert you automatically.

### Reputation Killers

| Issue | Impact | Fix |
|-------|--------|-----|
| High bounce rate (>3%) | Severe | Clean your lists |
| Spam complaints (>0.1%) | Severe | Improve content, add unsubscribe |
| Spam traps | Severe | Remove old/purchased lists |
| Missing rDNS | High | Configure PTR record |
| Missing authentication | High | Add SPF/DKIM/DMARC |

---

## Complete DNS Example

For domain `example.com` with sending IP `192.168.1.100`:

```dns
; SPF
example.com.          IN TXT "v=spf1 ip4:192.168.1.100 -all"

; DKIM
default._domainkey.example.com. IN TXT "v=DKIM1; k=rsa; p=MIGf..."

; DMARC
_dmarc.example.com.   IN TXT "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"

; MX for bounces
bounce.example.com.   IN MX 10 mail.example.com.
mail.example.com.     IN A 192.168.1.100

; Don't forget PTR (set via hosting provider)
; 100.1.168.192.in-addr.arpa. IN PTR mail.example.com.
```

---

## Troubleshooting

### Check SPF
```bash
dig TXT example.com +short
```

### Check DKIM
```bash
dig TXT default._domainkey.example.com +short
```

### Check DMARC
```bash
dig TXT _dmarc.example.com +short
```

### Check rDNS
```bash
dig -x YOUR_IP +short
```

### Test Full Authentication
Use [mail-tester.com](https://www.mail-tester.com/) or send to Gmail and check headers.

---

## Resources

- [Google Postmaster Tools](https://postmaster.google.com/)
- [Microsoft SNDS](https://sendersupport.olc.protection.outlook.com/snds/)
- [MXToolbox](https://mxtoolbox.com/)
- [DMARC Analyzer](https://www.dmarcanalyzer.com/)
