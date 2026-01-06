package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// AUTOMATION WORKFLOW ENGINE
// =====================================

// AutomationEngine executes automation workflows
type AutomationEngine struct {
	store   *store.Store
	mu      sync.RWMutex
	runners map[uint]*WorkflowRunner
	stopCh  chan struct{}

	// Services
	emailService    EmailSender
	webhookClient   *http.Client
	personalization *PersonalizationEngine
}

// EmailSender interface for sending emails
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// NewAutomationEngine creates a new automation engine
func NewAutomationEngine(st *store.Store, emailSender EmailSender) *AutomationEngine {
	return &AutomationEngine{
		store:           st,
		runners:         make(map[uint]*WorkflowRunner),
		stopCh:          make(chan struct{}),
		emailService:    emailSender,
		webhookClient:   &http.Client{Timeout: 30 * time.Second},
		personalization: NewPersonalizationEngine(),
	}
}

// Start begins the automation engine
func (ae *AutomationEngine) Start() error {
	log := logger.WithComponent("automation_engine")
	log.Info("starting automation engine")

	// Load active automations
	ae.loadActiveAutomations()

	ctx := context.Background()

	// Start scheduler for delayed actions
	go ae.runScheduler(ctx)

	// Start trigger listener
	go ae.runTriggerListener(ctx)

	return nil
}

// ProcessDelayedActions processes any pending delayed actions
func (ae *AutomationEngine) ProcessDelayedActions() {
	log := logger.WithComponent("automation_engine")
	log.Debug("processing delayed actions")

	// Find runs with pending delayed actions
	var runs []models.AutomationRunV2
	ae.store.DB.Where("status = ? AND next_action_at <= ?", "waiting", time.Now()).Find(&runs)

	for _, run := range runs {
		ae.processDelayedRun(&run)
	}
}

// processDelayedRun handles a single delayed run
func (ae *AutomationEngine) processDelayedRun(run *models.AutomationRunV2) {
	ae.mu.RLock()
	runner, exists := ae.runners[run.AutomationID]
	ae.mu.RUnlock()

	if !exists {
		return
	}

	runner.ContinueRun(run)
}

// Stop gracefully stops the automation engine
func (ae *AutomationEngine) Stop() {
	close(ae.stopCh)

	ae.mu.Lock()
	defer ae.mu.Unlock()

	for _, runner := range ae.runners {
		runner.Stop()
	}
}

// loadActiveAutomations loads all active workflows
func (ae *AutomationEngine) loadActiveAutomations() {
	var automations []models.AutomationV2
	if err := ae.store.DB.Where("status = ?", "active").Find(&automations).Error; err != nil {
		logger.WithComponent("automation").Error("failed to load automations", "error", err)
		return
	}

	for _, auto := range automations {
		ae.RegisterWorkflow(&auto)
	}
}

// RegisterWorkflow registers a workflow for execution
func (ae *AutomationEngine) RegisterWorkflow(automation *models.AutomationV2) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	runner := NewWorkflowRunner(ae, automation)
	ae.runners[automation.ID] = runner
}

// UnregisterWorkflow removes a workflow
func (ae *AutomationEngine) UnregisterWorkflow(automationID uint) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if runner, ok := ae.runners[automationID]; ok {
		runner.Stop()
		delete(ae.runners, automationID)
	}
}

// TriggerEvent triggers automations for a specific event
func (ae *AutomationEngine) TriggerEvent(eventType string, contactID uint, data map[string]interface{}) {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	for _, runner := range ae.runners {
		if runner.MatchesTrigger(eventType, data) {
			go runner.EnterContact(contactID, data)
		}
	}
}

// runScheduler processes delayed actions
func (ae *AutomationEngine) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ae.stopCh:
			return
		case <-ticker.C:
			ae.processDelayedActions()
		}
	}
}

// processDelayedActions executes actions whose delay has passed
// Uses Redis for O(log N) lookup if available, falls back to SQL
func (ae *AutomationEngine) processDelayedActions() {
	var runIDs []uint
	var runs []models.AutomationRunV2

	// Try Redis first (O(log N) - fast!)
	if IsDelayedQueueAvailable() {
		var err error
		runIDs, err = GetDelayedQueue().GetDue(500) // Process up to 500 at once
		if err != nil {
			logger.Warn("Redis delayed queue error, falling back to SQL", "error", err)
			runIDs = nil
		}
	}

	if len(runIDs) > 0 {
		// Fetch run details from DB in batch
		if err := ae.store.DB.Where("id IN ?", runIDs).Find(&runs).Error; err != nil {
			logger.Error("Failed to fetch runs by ID", "error", err)
			return
		}
	} else {
		// Fallback to SQL scan (slower but works without Redis)
		if err := ae.store.DB.
			Where("status = ? AND next_action_at <= ?", "active", time.Now()).
			Limit(100).
			Find(&runs).Error; err != nil {
			return
		}
	}

	// Process each due run
	processedIDs := make([]uint, 0, len(runs))
	for _, run := range runs {
		ae.mu.RLock()
		runner, ok := ae.runners[run.AutomationID]
		ae.mu.RUnlock()

		if ok {
			go runner.ContinueRun(&run)
			processedIDs = append(processedIDs, run.ID)
		}
	}

	// Remove processed items from Redis
	if IsDelayedQueueAvailable() && len(processedIDs) > 0 {
		GetDelayedQueue().RemoveBatch(processedIDs)
	}
}

// runTriggerListener listens for real-time triggers
func (ae *AutomationEngine) runTriggerListener(ctx context.Context) {
	// This would connect to a message queue in production
	// For now, triggers are called directly via TriggerEvent
}

// =====================================
// WORKFLOW RUNNER
// =====================================

// WorkflowRunner executes a single automation workflow
type WorkflowRunner struct {
	engine     *AutomationEngine
	automation *models.AutomationV2
	nodes      map[string]models.AutomationNode
	edges      []models.AutomationEdge
	stopCh     chan struct{}
}

// NewWorkflowRunner creates a new workflow runner
func NewWorkflowRunner(engine *AutomationEngine, automation *models.AutomationV2) *WorkflowRunner {
	wr := &WorkflowRunner{
		engine:     engine,
		automation: automation,
		nodes:      make(map[string]models.AutomationNode),
		stopCh:     make(chan struct{}),
	}

	// Parse nodes
	var nodes []models.AutomationNode
	if err := json.Unmarshal([]byte(automation.Nodes), &nodes); err == nil {
		for _, node := range nodes {
			wr.nodes[node.ID] = node
		}
	}

	// Parse edges
	json.Unmarshal([]byte(automation.Edges), &wr.edges)

	return wr
}

// Stop stops the workflow runner
func (wr *WorkflowRunner) Stop() {
	close(wr.stopCh)
}

// MatchesTrigger checks if this workflow matches the given trigger
func (wr *WorkflowRunner) MatchesTrigger(eventType string, data map[string]interface{}) bool {
	return wr.automation.TriggerType == eventType
}

// EnterContact starts a contact in this workflow
func (wr *WorkflowRunner) EnterContact(contactID uint, data map[string]interface{}) {
	log := logger.WithComponent("workflow").With("automation_id", wr.automation.ID, "contact_id", contactID)

	// Check if contact already in workflow
	if !wr.automation.AllowReentry {
		var count int64
		wr.engine.store.DB.Model(&models.AutomationRunV2{}).
			Where("automation_id = ? AND contact_id = ? AND status IN (?)",
				wr.automation.ID, contactID, []string{"active", "completed"}).
			Count(&count)

		if count > 0 {
			log.Debug("contact already in workflow, skipping")
			return
		}
	}

	// Find trigger node
	var triggerNodeID string
	for id, node := range wr.nodes {
		if strings.HasPrefix(node.Data.NodeType, "trigger_") {
			triggerNodeID = id
			break
		}
	}

	if triggerNodeID == "" {
		log.Error("no trigger node found")
		return
	}

	// Create run
	run := &models.AutomationRunV2{
		AutomationID:  wr.automation.ID,
		ContactID:     contactID,
		CurrentNodeID: triggerNodeID,
		Status:        "active",
		EnteredAt:     time.Now(),
		ContextData:   models.JSONMap(data),
		NodesVisited:  models.StringArray{triggerNodeID},
	}

	if err := wr.engine.store.DB.Create(run).Error; err != nil {
		log.Error("failed to create run", "error", err)
		return
	}

	// Update automation stats
	wr.engine.store.DB.Model(wr.automation).
		UpdateColumn("total_entered", wr.automation.TotalEntered+1)

	// Execute from trigger
	wr.executeFromNode(run, triggerNodeID)
}

// ContinueRun continues a paused run
func (wr *WorkflowRunner) ContinueRun(run *models.AutomationRunV2) {
	wr.executeFromNode(run, run.CurrentNodeID)
}

// executeFromNode executes the workflow starting from a node
func (wr *WorkflowRunner) executeFromNode(run *models.AutomationRunV2, nodeID string) {
	log := logger.WithComponent("workflow").
		With("automation_id", wr.automation.ID).
		With("run_id", run.ID).
		With("node_id", nodeID)

	node, ok := wr.nodes[nodeID]
	if !ok {
		log.Error("node not found")
		return
	}

	// Execute the node
	result, err := wr.executeNode(run, &node)

	// Log the action
	actionLog := &models.AutomationActionLog{
		RunID:      run.ID,
		NodeID:     nodeID,
		NodeType:   node.Data.NodeType,
		Action:     "completed",
		ExecutedAt: time.Now(),
	}
	if err != nil {
		actionLog.Action = "failed"
		actionLog.Result = models.JSONMap{"error": err.Error()}
		run.ErrorCount++
		run.LastError = err.Error()
	} else {
		actionLog.Result = result
	}
	wr.engine.store.DB.Create(actionLog)

	// Handle errors
	if err != nil {
		if run.ErrorCount >= 3 {
			run.Status = "failed"
			wr.engine.store.DB.Save(run)
		}
		return
	}

	// Find next node(s)
	nextNodes := wr.findNextNodes(nodeID, result)

	if len(nextNodes) == 0 {
		// Workflow complete
		run.Status = "completed"
		now := time.Now()
		run.CompletedAt = &now
		wr.engine.store.DB.Save(run)

		wr.engine.store.DB.Model(wr.automation).
			UpdateColumn("total_completed", wr.automation.TotalCompleted+1)
		return
	}

	// Execute next nodes
	for _, nextNodeID := range nextNodes {
		// Add to visited
		run.NodesVisited = append(run.NodesVisited, nextNodeID)
		run.CurrentNodeID = nextNodeID
		wr.engine.store.DB.Save(run)

		wr.executeFromNode(run, nextNodeID)
	}
}

// executeNode executes a single node
func (wr *WorkflowRunner) executeNode(run *models.AutomationRunV2, node *models.AutomationNode) (models.JSONMap, error) {
	config := node.Data.Config

	switch node.Data.NodeType {
	// Triggers (just pass through)
	case models.NodeTypeTriggerContactAdded,
		models.NodeTypeTriggerTagAdded,
		models.NodeTypeTriggerEmailOpened,
		models.NodeTypeTriggerLinkClicked,
		models.NodeTypeTriggerFormSubmitted,
		models.NodeTypeTriggerWebhook,
		models.NodeTypeTriggerSchedule:
		return models.JSONMap{"triggered": true}, nil

	// Actions
	case models.NodeTypeActionSendEmail:
		return wr.executeSendEmail(run, config)

	case models.NodeTypeActionAddTag:
		return wr.executeAddTag(run, config)

	case models.NodeTypeActionRemoveTag:
		return wr.executeRemoveTag(run, config)

	case models.NodeTypeActionUpdateField:
		return wr.executeUpdateField(run, config)

	case models.NodeTypeActionWebhook:
		return wr.executeWebhook(run, config)

	case models.NodeTypeActionScore:
		return wr.executeUpdateScore(run, config)

	// Flow Control
	case models.NodeTypeDelay:
		return wr.executeDelay(run, config)

	case models.NodeTypeCondition:
		return wr.executeCondition(run, config)

	case models.NodeTypeABSplit:
		return wr.executeABSplit(run, config)

	case models.NodeTypeGoal:
		return wr.executeGoal(run, config)

	case models.NodeTypeExit:
		return models.JSONMap{"exited": true}, nil

	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Data.NodeType)
	}
}

// executeSendEmail sends an email
func (wr *WorkflowRunner) executeSendEmail(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	// Get contact
	var contact models.ContactV2
	if err := wr.engine.store.DB.First(&contact, run.ContactID).Error; err != nil {
		return nil, fmt.Errorf("contact not found: %w", err)
	}

	// Get template or content
	subject := getString(config, "subject")
	body := getString(config, "body")
	templateID := getUint(config, "template_id")

	if templateID > 0 {
		var template models.EmailTemplate
		if err := wr.engine.store.DB.First(&template, templateID).Error; err == nil {
			subject = template.Subject
			body = template.HTMLContent
		}
	}

	// Personalize
	ctx := &PersonalizationContext{
		Contact:     &contact,
		CurrentDate: time.Now(),
	}
	subject, _ = wr.engine.personalization.Render(subject, ctx)
	body, _ = wr.engine.personalization.Render(body, ctx)

	// Send
	if err := wr.engine.emailService.SendEmail(context.Background(), contact.Email, subject, body); err != nil {
		return nil, err
	}

	return models.JSONMap{"sent_to": contact.Email}, nil
}

// executeAddTag adds a tag to contact
func (wr *WorkflowRunner) executeAddTag(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	tagID := getUint(config, "tag_id")
	tagName := getString(config, "tag_name")

	// Create subscriber tag
	subscriberTag := &models.SubscriberTag{
		ContactID: run.ContactID,
		TagID:     tagID,
	}

	if err := wr.engine.store.DB.Create(subscriberTag).Error; err != nil {
		return nil, err
	}

	// Trigger tag added event
	go wr.engine.TriggerEvent(models.NodeTypeTriggerTagAdded, run.ContactID,
		map[string]interface{}{"tag_id": tagID, "tag_name": tagName})

	return models.JSONMap{"tag_id": tagID, "tag_name": tagName}, nil
}

// executeRemoveTag removes a tag from contact
func (wr *WorkflowRunner) executeRemoveTag(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	tagID := getUint(config, "tag_id")

	err := wr.engine.store.DB.
		Where("contact_id = ? AND tag_id = ?", run.ContactID, tagID).
		Delete(&models.SubscriberTag{}).Error

	return models.JSONMap{"tag_id": tagID, "removed": err == nil}, err
}

// executeUpdateField updates a contact field
func (wr *WorkflowRunner) executeUpdateField(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	fieldName := getString(config, "field")
	fieldValue := config["value"]

	updates := map[string]interface{}{fieldName: fieldValue}
	err := wr.engine.store.DB.Model(&models.ContactV2{}).
		Where("id = ?", run.ContactID).
		Updates(updates).Error

	return models.JSONMap{"field": fieldName, "value": fieldValue}, err
}

// executeWebhook calls an external webhook
func (wr *WorkflowRunner) executeWebhook(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	url := getString(config, "url")
	method := getString(config, "method")
	if method == "" {
		method = "POST"
	}

	// Get contact data
	var contact models.ContactV2
	wr.engine.store.DB.First(&contact, run.ContactID)

	// Build payload
	payload := map[string]interface{}{
		"contact_id":    run.ContactID,
		"contact_email": contact.Email,
		"automation_id": run.AutomationID,
		"run_id":        run.ID,
		"context":       run.ContextData,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := wr.engine.webhookClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return models.JSONMap{"status_code": resp.StatusCode, "url": url}, nil
}

// executeUpdateScore updates contact's lead score
func (wr *WorkflowRunner) executeUpdateScore(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	action := getString(config, "action") // add, subtract, set
	points := getInt(config, "points")

	var update string
	switch action {
	case "add":
		update = fmt.Sprintf("lead_score + %d", points)
	case "subtract":
		update = fmt.Sprintf("lead_score - %d", points)
	case "set":
		update = fmt.Sprintf("%d", points)
	default:
		update = fmt.Sprintf("lead_score + %d", points)
	}

	err := wr.engine.store.DB.Model(&models.ContactV2{}).
		Where("id = ?", run.ContactID).
		Update("lead_score", wr.engine.store.DB.Raw(update)).Error

	return models.JSONMap{"action": action, "points": points}, err
}

// executeDelay pauses the workflow
// Uses Redis for O(log N) scheduling when available
func (wr *WorkflowRunner) executeDelay(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	delayType := getString(config, "delay_type") // minutes, hours, days, until_date, until_time
	delayValue := getInt(config, "delay_value")

	var nextAction time.Time

	switch delayType {
	case "minutes":
		nextAction = time.Now().Add(time.Duration(delayValue) * time.Minute)
	case "hours":
		nextAction = time.Now().Add(time.Duration(delayValue) * time.Hour)
	case "days":
		nextAction = time.Now().Add(time.Duration(delayValue) * 24 * time.Hour)
	case "until_date":
		// Parse date from config
		dateStr := getString(config, "date")
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			nextAction = t
		}
	default:
		nextAction = time.Now().Add(time.Duration(delayValue) * time.Minute)
	}

	// Schedule in Redis for fast lookup (O(log N))
	if IsDelayedQueueAvailable() {
		if err := GetDelayedQueue().Schedule(run.ID, nextAction); err != nil {
			logger.Warn("Failed to schedule in Redis, using DB", "error", err)
		}
	}

	// Always save to DB as source of truth
	run.NextActionAt = &nextAction
	run.Status = "waiting"
	wr.engine.store.DB.Save(run)

	return models.JSONMap{"next_action_at": nextAction, "delay_type": delayType}, nil
}

// executeCondition evaluates a condition
func (wr *WorkflowRunner) executeCondition(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	field := getString(config, "field")
	operator := getString(config, "operator")
	value := config["value"]

	// Get contact
	var contact models.ContactV2
	if err := wr.engine.store.DB.First(&contact, run.ContactID).Error; err != nil {
		return nil, err
	}

	// Get field value from contact (simplified)
	var fieldValue interface{}
	switch field {
	case "email":
		fieldValue = contact.Email
	case "first_name":
		fieldValue = contact.FirstName
	case "lead_score":
		fieldValue = contact.LeadScore
	case "total_opens":
		fieldValue = contact.TotalOpens
	default:
		if contact.CustomFields != nil {
			fieldValue = contact.CustomFields[field]
		}
	}

	// Evaluate condition
	result := wr.evaluateCondition(fieldValue, operator, value)

	return models.JSONMap{"field": field, "result": result, "branch": branchName(result)}, nil
}

// evaluateCondition evaluates a single condition
func (wr *WorkflowRunner) evaluateCondition(fieldValue interface{}, operator string, compareValue interface{}) bool {
	switch operator {
	case "equals", "eq", "==":
		return fmt.Sprint(fieldValue) == fmt.Sprint(compareValue)
	case "not_equals", "ne", "!=":
		return fmt.Sprint(fieldValue) != fmt.Sprint(compareValue)
	case "contains":
		return strings.Contains(fmt.Sprint(fieldValue), fmt.Sprint(compareValue))
	case "not_contains":
		return !strings.Contains(fmt.Sprint(fieldValue), fmt.Sprint(compareValue))
	case "starts_with":
		return strings.HasPrefix(fmt.Sprint(fieldValue), fmt.Sprint(compareValue))
	case "ends_with":
		return strings.HasSuffix(fmt.Sprint(fieldValue), fmt.Sprint(compareValue))
	case "is_empty":
		return fmt.Sprint(fieldValue) == ""
	case "is_not_empty":
		return fmt.Sprint(fieldValue) != ""
	case "greater_than", "gt", ">":
		return toFloat(fieldValue) > toFloat(compareValue)
	case "less_than", "lt", "<":
		return toFloat(fieldValue) < toFloat(compareValue)
	case "greater_equal", "gte", ">=":
		return toFloat(fieldValue) >= toFloat(compareValue)
	case "less_equal", "lte", "<=":
		return toFloat(fieldValue) <= toFloat(compareValue)
	default:
		return false
	}
}

// executeABSplit randomly splits traffic
func (wr *WorkflowRunner) executeABSplit(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	// Get split percentages
	splitA := getInt(config, "split_a")
	if splitA == 0 {
		splitA = 50
	}

	// Determine branch based on run ID (deterministic)
	branch := "A"
	if int(run.ID)%100 >= splitA {
		branch = "B"
	}

	return models.JSONMap{"branch": branch, "split_a": splitA, "split_b": 100 - splitA}, nil
}

// executeGoal marks a goal as achieved
func (wr *WorkflowRunner) executeGoal(run *models.AutomationRunV2, config models.JSONMap) (models.JSONMap, error) {
	goalName := getString(config, "goal_name")

	// Update conversion rate
	wr.engine.store.DB.Model(wr.automation).
		UpdateColumn("conversion_rate",
			wr.engine.store.DB.Raw("(total_completed * 100.0 / NULLIF(total_entered, 0))"))

	return models.JSONMap{"goal_achieved": goalName}, nil
}

// findNextNodes finds the next nodes to execute based on edges and result
func (wr *WorkflowRunner) findNextNodes(currentNodeID string, result models.JSONMap) []string {
	var nextNodes []string

	branch := ""
	if result != nil {
		if b, ok := result["branch"].(string); ok {
			branch = b
		}
	}

	for _, edge := range wr.edges {
		if edge.Source == currentNodeID {
			// Check if edge matches branch for conditions
			if branch != "" && edge.SourceHandle != "" {
				if edge.SourceHandle == branch || edge.SourceHandle == strings.ToLower(branch) {
					nextNodes = append(nextNodes, edge.Target)
				}
			} else if edge.SourceHandle == "" {
				nextNodes = append(nextNodes, edge.Target)
			}
		}
	}

	return nextNodes
}

// Helper functions
func getString(m models.JSONMap, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func getInt(m models.JSONMap, key string) int {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		}
	}
	return 0
}

func getUint(m models.JSONMap, key string) uint {
	return uint(getInt(m, key))
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	}
	return 0
}

func branchName(result bool) string {
	if result {
		return "yes"
	}
	return "no"
}

// Global automation engine
var globalAutomationEngine *AutomationEngine

// InitAutomationEngine initializes the global automation engine
func InitAutomationEngine(st *store.Store, emailSender EmailSender) {
	globalAutomationEngine = NewAutomationEngine(st, emailSender)
}

// GetAutomationEngine returns the global automation engine
func GetAutomationEngine() *AutomationEngine {
	return globalAutomationEngine
}

// TriggerAutomation is a convenience function to trigger automations
func TriggerAutomation(eventType string, contactID uint, data map[string]interface{}) {
	if globalAutomationEngine != nil {
		globalAutomationEngine.TriggerEvent(eventType, contactID, data)
	}
}
