package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"gorm.io/gorm/clause"
)

// Standard KumoMTA paths
const (
	SourcesPath = "/opt/kumomta/etc/policy/sources.toml"
	InitLuaPath = "/opt/kumomta/etc/policy/init.lua"
	DBPath      = "/var/lib/sampmail/sampmail.db"
)

func main() {
	// Safety check: verify KumoMTA config exists
	if _, err := os.Stat(SourcesPath); os.IsNotExist(err) {
		fmt.Printf("Warning: KumoMTA configuration not found at %s\n", SourcesPath)
		fmt.Println("Skipping migration (run this only on a server with KumoMTA installed)")
		return
	}

	// Open database
	fmt.Printf("Opening database at %s...\n", DBPath)
	st, err := store.NewStore(DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Parse global settings from init.lua
	parseInitLua(st)

	// Parse domains, senders and IPs from sources.toml
	parseSourcesToml(st)

	fmt.Println("\nMigration complete. Configuration and IPs have been imported.")

	// Safe restart (preserves queue, applies new config)
	resetKumoMTA()
}

func resetKumoMTA() {
	fmt.Println("\nRestarting KumoMTA to apply new configuration...")
	fmt.Println("(This preserves the mail queue and retries delivery with new settings)")

	cmd := exec.Command("systemctl", "restart", "kumomta")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Error restarting service: %v\nOutput: %s\n", err, string(out))
		fmt.Println("Please check logs manually: journalctl -u kumomta -n 50")
		return
	}

	fmt.Println("Service restarted successfully.")

	time.Sleep(2 * time.Second)

	fmt.Println("\nVerifying service status...")
	verify := exec.Command("systemctl", "is-active", "kumomta")
	if out, err := verify.CombinedOutput(); err != nil {
		fmt.Printf("Service status: %s\n", strings.TrimSpace(string(out)))
	} else {
		fmt.Println("Status: Active")
		fmt.Println("Queue processing has resumed with the new configuration.")
	}
}

func parseInitLua(st *store.Store) {
	fmt.Println("Reading init.lua for settings...")
	file, err := os.Open(InitLuaPath)
	if err != nil {
		fmt.Printf("Warning: Could not read init.lua: %v\n", err)
		return
	}
	defer file.Close()

	settings := &models.AppSettings{
		AIProvider: "openai",
	}

	reHostname := regexp.MustCompile(`hostname\s*=\s*'([^']+)'`)
	reRelay := regexp.MustCompile(`relay_hosts\s*=\s*\{\s*'([^']+)'`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if matches := reHostname.FindStringSubmatch(line); len(matches) > 1 {
			if settings.MainHostname == "" {
				settings.MainHostname = matches[1]
				fmt.Printf("Found hostname: %s\n", settings.MainHostname)
			}
		}

		if matches := reRelay.FindStringSubmatch(line); len(matches) > 1 {
			if !strings.Contains(settings.MailWizzIP, matches[1]) {
				if settings.MailWizzIP != "" {
					settings.MailWizzIP += ", "
				}
				settings.MailWizzIP += matches[1]
			}
		}
	}

	if settings.MainServerIP == "" {
		settings.MainServerIP = getOutboundIP()
		fmt.Printf("Auto-detected server IP: %s\n", settings.MainServerIP)
	}

	existing, _ := st.GetSettings()
	if existing != nil {
		settings.ID = existing.ID
	}
	st.UpsertSettings(settings)
}

func parseSourcesToml(st *store.Store) {
	fmt.Println("Reading sources.toml for domains and IPs...")
	file, err := os.Open(SourcesPath)
	if err != nil {
		log.Fatalf("Failed to open sources.toml: %v", err)
	}
	defer file.Close()

	reEhlo := regexp.MustCompile(`ehlo_domain\s*=\s*"([^"]+)"`)
	reSource := regexp.MustCompile(`source_address\s*=\s*"([^"]+)"`)

	var currentIP string
	count := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if matches := reSource.FindStringSubmatch(line); len(matches) > 1 {
			currentIP = matches[1]

			if currentIP != "" {
				sysIP := &models.SystemIP{
					Value:     currentIP,
					CreatedAt: time.Now(),
					Interface: "eth0",
				}
				st.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(sysIP)
			}
		}

		if matches := reEhlo.FindStringSubmatch(line); len(matches) > 1 {
			ehlo := matches[1]

			parts := strings.SplitN(ehlo, ".", 2)
			if len(parts) != 2 {
				continue
			}
			localPart := parts[0]
			domainName := parts[1]

			domain := ensureDomain(st, domainName)

			bounceUser := "b-" + localPart
			ensureBounceAccount(st, bounceUser, domainName)

			sender := &models.Sender{
				DomainID:       domain.ID,
				LocalPart:      localPart,
				Email:          localPart + "@" + domainName,
				IP:             currentIP,
				BounceUsername: bounceUser,
			}

			var existing models.Sender
			if err := st.DB.Where("email = ?", sender.Email).First(&existing).Error; err == nil {
				existing.IP = currentIP
				existing.BounceUsername = bounceUser
				st.DB.Save(&existing)
			} else {
				st.CreateSender(sender)
				count++
			}
		}
	}
	fmt.Printf("Imported %d senders.\n", count)
}

func ensureDomain(st *store.Store, name string) *models.Domain {
	d, err := st.GetDomainByName(name)
	if err == nil {
		return d
	}
	newD := &models.Domain{
		Name:            name,
		MailHost:        "mail." + name,
		BounceHost:      "bounce." + name,
		DMARCPolicy:     "none",
		DMARCPercentage: 100,
	}
	st.CreateDomain(newD)
	return newD
}

func ensureBounceAccount(st *store.Store, user, domain string) {
	var count int64
	st.DB.Model(&models.BounceAccount{}).Where("username = ?", user).Count(&count)
	if count == 0 {
		st.CreateBounceAccount(&models.BounceAccount{
			Username: user,
			Domain:   domain,
			Notes:    "Auto-imported",
		})
	}
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
