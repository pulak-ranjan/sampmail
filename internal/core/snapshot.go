package core

import (
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// Snapshot represents a consistent view of configuration
// used to generate KumoMTA/Dovecot/firewall configs.
type Snapshot struct {
	Settings *models.AppSettings
	Domains  []models.Domain
}

// LoadSnapshot collects app settings + all domains (+ senders).
func LoadSnapshot(st *store.Store) (*Snapshot, error) {
	settings, err := st.GetSettings()
	if err != nil && err != store.ErrNotFound {
		return nil, err
	}
	// settings can be nil if not configured yet

	// orgID=0: snapshot covers all domains across all orgs (system-level view)
	domains, err := st.ListDomains(0)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Settings: settings,
		Domains:  domains,
	}, nil
}
