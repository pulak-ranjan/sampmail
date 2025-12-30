package store

import (
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"gorm.io/gorm"
)

// =====================================
// MODULAR STORE ARCHITECTURE
// Splits the "God Object" store into domain-specific repositories
// =====================================

// BaseRepository provides common database operations
type BaseRepository struct {
	DB *gorm.DB
}

// =====================================
// AUTH REPOSITORY
// =====================================

// AuthRepository handles authentication-related data
type AuthRepository struct {
	BaseRepository
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{BaseRepository{DB: db}}
}

// FindAdminByEmail finds an admin by email
func (r *AuthRepository) FindAdminByEmail(email string) (*models.AdminUser, error) {
	var admin models.AdminUser
	if err := r.DB.Where("email = ?", email).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

// FindAdminByID finds an admin by ID
func (r *AuthRepository) FindAdminByID(id uint) (*models.AdminUser, error) {
	var admin models.AdminUser
	if err := r.DB.First(&admin, id).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

// CreateAdmin creates a new admin user
func (r *AuthRepository) CreateAdmin(admin *models.AdminUser) error {
	return r.DB.Create(admin).Error
}

// UpdateAdmin updates an admin user
func (r *AuthRepository) UpdateAdmin(admin *models.AdminUser) error {
	return r.DB.Save(admin).Error
}

// FindSessionByToken finds a session by token hash
func (r *AuthRepository) FindSessionByToken(tokenHash string) (*models.AuthSession, error) {
	var session models.AuthSession
	if err := r.DB.Where("token = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// CreateSession creates a new session
func (r *AuthRepository) CreateSession(session *models.AuthSession) error {
	return r.DB.Create(session).Error
}

// DeleteSession deletes a session
func (r *AuthRepository) DeleteSession(tokenHash string) error {
	return r.DB.Where("token = ?", tokenHash).Delete(&models.AuthSession{}).Error
}

// DeleteExpiredSessions removes expired sessions
func (r *AuthRepository) DeleteExpiredSessions() (int64, error) {
	result := r.DB.Where("expires_at < ?", time.Now()).Delete(&models.AuthSession{})
	return result.RowsAffected, result.Error
}

// ListUserSessions lists all sessions for a user
func (r *AuthRepository) ListUserSessions(adminID uint) ([]models.AuthSession, error) {
	var sessions []models.AuthSession
	err := r.DB.Where("admin_id = ?", adminID).Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// =====================================
// DOMAIN REPOSITORY
// =====================================

// DomainRepository handles domain-related data
type DomainRepository struct {
	BaseRepository
}

// NewDomainRepository creates a new domain repository
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{BaseRepository{DB: db}}
}

// List returns all domains
func (r *DomainRepository) List() ([]models.Domain, error) {
	var domains []models.Domain
	err := r.DB.Preload("Senders").Find(&domains).Error
	return domains, err
}

// FindByID finds a domain by ID
func (r *DomainRepository) FindByID(id uint) (*models.Domain, error) {
	var domain models.Domain
	if err := r.DB.Preload("Senders").First(&domain, id).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

// FindByName finds a domain by name
func (r *DomainRepository) FindByName(name string) (*models.Domain, error) {
	var domain models.Domain
	if err := r.DB.Where("name = ?", name).First(&domain).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

// Create creates a new domain
func (r *DomainRepository) Create(domain *models.Domain) error {
	return r.DB.Create(domain).Error
}

// Update updates a domain
func (r *DomainRepository) Update(domain *models.Domain) error {
	return r.DB.Save(domain).Error
}

// Delete deletes a domain and its senders
func (r *DomainRepository) Delete(id uint) error {
	return r.DB.Delete(&models.Domain{}, id).Error
}

// =====================================
// SENDER REPOSITORY
// =====================================

// SenderRepository handles sender-related data
type SenderRepository struct {
	BaseRepository
}

// NewSenderRepository creates a new sender repository
func NewSenderRepository(db *gorm.DB) *SenderRepository {
	return &SenderRepository{BaseRepository{DB: db}}
}

// List returns all senders for a domain
func (r *SenderRepository) List(domainID uint) ([]models.Sender, error) {
	var senders []models.Sender
	err := r.DB.Where("domain_id = ?", domainID).Find(&senders).Error
	return senders, err
}

// FindByID finds a sender by ID
func (r *SenderRepository) FindByID(id uint) (*models.Sender, error) {
	var sender models.Sender
	if err := r.DB.Preload("Domain").First(&sender, id).Error; err != nil {
		return nil, err
	}
	return &sender, nil
}

// FindByEmail finds a sender by email
func (r *SenderRepository) FindByEmail(email string) (*models.Sender, error) {
	var sender models.Sender
	if err := r.DB.Where("email = ?", email).Preload("Domain").First(&sender).Error; err != nil {
		return nil, err
	}
	return &sender, nil
}

// Create creates a new sender
func (r *SenderRepository) Create(sender *models.Sender) error {
	return r.DB.Create(sender).Error
}

// Update updates a sender
func (r *SenderRepository) Update(sender *models.Sender) error {
	return r.DB.Save(sender).Error
}

// Delete deletes a sender
func (r *SenderRepository) Delete(id uint) error {
	return r.DB.Delete(&models.Sender{}, id).Error
}

// =====================================
// CAMPAIGN REPOSITORY
// =====================================

// CampaignRepository handles campaign-related data
type CampaignRepository struct {
	BaseRepository
}

// NewCampaignRepository creates a new campaign repository
func NewCampaignRepository(db *gorm.DB) *CampaignRepository {
	return &CampaignRepository{BaseRepository{DB: db}}
}

// List returns campaigns with optional filters
func (r *CampaignRepository) List(status string, limit, offset int) ([]models.Campaign, error) {
	var campaigns []models.Campaign
	query := r.DB.Model(&models.Campaign{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&campaigns).Error
	return campaigns, err
}

// FindByID finds a campaign by ID
func (r *CampaignRepository) FindByID(id uint) (*models.Campaign, error) {
	var campaign models.Campaign
	if err := r.DB.Preload("Sender").Preload("Sender.Domain").First(&campaign, id).Error; err != nil {
		return nil, err
	}
	return &campaign, nil
}

// Create creates a new campaign
func (r *CampaignRepository) Create(campaign *models.Campaign) error {
	return r.DB.Create(campaign).Error
}

// Update updates a campaign
func (r *CampaignRepository) Update(campaign *models.Campaign) error {
	return r.DB.Save(campaign).Error
}

// Delete deletes a campaign and its recipients
func (r *CampaignRepository) Delete(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("campaign_id = ?", id).Delete(&models.CampaignRecipient{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Campaign{}, id).Error
	})
}

// CountRecipients counts recipients by status
func (r *CampaignRepository) CountRecipients(campaignID uint, status string) (int64, error) {
	var count int64
	query := r.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ?", campaignID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&count).Error
	return count, err
}

// GetScheduled returns campaigns ready to send
func (r *CampaignRepository) GetScheduled() ([]models.Campaign, error) {
	var campaigns []models.Campaign
	err := r.DB.Where("status = 'scheduled' AND scheduled_at <= ?", time.Now()).
		Preload("Sender").Preload("Sender.Domain").
		Find(&campaigns).Error
	return campaigns, err
}

// =====================================
// CONTACT REPOSITORY
// =====================================

// ContactRepository handles contact-related data
type ContactRepository struct {
	BaseRepository
}

// NewContactRepository creates a new contact repository
func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{BaseRepository{DB: db}}
}

// FindByEmail finds a contact by email
func (r *ContactRepository) FindByEmail(email string) (*models.Contact, error) {
	var contact models.Contact
	if err := r.DB.Where("email = ?", email).First(&contact).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

// List returns contacts with optional filters
func (r *ContactRepository) List(listID uint, status string, limit, offset int) ([]models.Contact, error) {
	var contacts []models.Contact
	query := r.DB.Model(&models.Contact{})
	if listID > 0 {
		query = query.Where("list_id = ?", listID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&contacts).Error
	return contacts, err
}

// Create creates a new contact
func (r *ContactRepository) Create(contact *models.Contact) error {
	return r.DB.Create(contact).Error
}

// CreateBatch creates contacts in batch
func (r *ContactRepository) CreateBatch(contacts []models.Contact, batchSize int) error {
	return r.DB.CreateInBatches(contacts, batchSize).Error
}

// Update updates a contact
func (r *ContactRepository) Update(contact *models.Contact) error {
	return r.DB.Save(contact).Error
}

// Delete deletes a contact
func (r *ContactRepository) Delete(id uint) error {
	return r.DB.Delete(&models.Contact{}, id).Error
}

// =====================================
// SUPPRESSION REPOSITORY
// =====================================

// SuppressionRepository handles suppression list data
type SuppressionRepository struct {
	BaseRepository
}

// NewSuppressionRepository creates a new suppression repository
func NewSuppressionRepository(db *gorm.DB) *SuppressionRepository {
	return &SuppressionRepository{BaseRepository{DB: db}}
}

// IsSuppressed checks if an email is suppressed
func (r *SuppressionRepository) IsSuppressed(email string) (bool, error) {
	var count int64
	err := r.DB.Model(&models.Suppression{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// BulkCheck checks multiple emails for suppression
func (r *SuppressionRepository) BulkCheck(emails []string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, e := range emails {
		result[e] = false
	}

	var suppressions []models.Suppression
	if err := r.DB.Where("email IN ?", emails).Find(&suppressions).Error; err != nil {
		return nil, err
	}

	for _, s := range suppressions {
		result[s.Email] = true
	}
	return result, nil
}

// Add adds an email to suppression list
func (r *SuppressionRepository) Add(email, reason, source string) error {
	suppression := models.Suppression{
		Email:     email,
		Reason:    reason,
		Source:    source,
		CreatedAt: time.Now(),
	}
	return r.DB.Create(&suppression).Error
}

// Remove removes an email from suppression list
func (r *SuppressionRepository) Remove(email string) error {
	return r.DB.Where("email = ?", email).Delete(&models.Suppression{}).Error
}

// =====================================
// STATS REPOSITORY
// =====================================

// StatsRepository handles statistics data
type StatsRepository struct {
	BaseRepository
}

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{BaseRepository{DB: db}}
}

// GetDailyStats returns stats for a date range
func (r *StatsRepository) GetDailyStats(domain string, from, to time.Time) ([]models.EmailStats, error) {
	var stats []models.EmailStats
	query := r.DB.Where("date BETWEEN ? AND ?", from, to)
	if domain != "" {
		query = query.Where("domain = ?", domain)
	}
	err := query.Order("date DESC").Find(&stats).Error
	return stats, err
}

// GetTotalStats returns aggregate stats
func (r *StatsRepository) GetTotalStats(domain string) (*models.EmailStats, error) {
	var stats models.EmailStats
	query := r.DB.Model(&models.EmailStats{}).
		Select("SUM(sent) as sent, SUM(delivered) as delivered, SUM(bounced) as bounced, SUM(deferred) as deferred")
	if domain != "" {
		query = query.Where("domain = ?", domain)
	}
	err := query.Scan(&stats).Error
	return &stats, err
}

// =====================================
// REPOSITORY REGISTRY
// =====================================

// Repositories holds all repository instances
type Repositories struct {
	Auth        *AuthRepository
	Domain      *DomainRepository
	Sender      *SenderRepository
	Campaign    *CampaignRepository
	Contact     *ContactRepository
	Suppression *SuppressionRepository
	Stats       *StatsRepository
	Atomic      *AtomicOps
}

// NewRepositories creates all repositories
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Auth:        NewAuthRepository(db),
		Domain:      NewDomainRepository(db),
		Sender:      NewSenderRepository(db),
		Campaign:    NewCampaignRepository(db),
		Contact:     NewContactRepository(db),
		Suppression: NewSuppressionRepository(db),
		Stats:       NewStatsRepository(db),
		Atomic:      NewAtomicOps(db),
	}
}
