package edgehealth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend-assessment/internal/datastore"
	"backend-assessment/internal/models"

	log "github.com/sirupsen/logrus"
)

// HealthStatus represents the health state of a gateway
type HealthStatus struct {
	GatewayID    string
	Status       string
	LastChecked  time.Time
	ErrorCount   int
	ResponseTime time.Duration
}

// HealthProcessor monitors and processes gateway health status
type HealthProcessor struct {
	db            *datastore.PostgresDB
	healthCache   map[string]*HealthStatus
	pendingChecks map[string]bool
	checkInterval time.Duration
	stopChan      chan bool
	// BUG 2: Missing mutex for concurrent access to shared state
	// Should have: mu sync.RWMutex
	mu sync.RWMutex
}

// NewHealthProcessor creates a new health processor
func NewHealthProcessor(db *datastore.PostgresDB, checkInterval time.Duration) *HealthProcessor {
	return &HealthProcessor{
		db:            db,
		healthCache:   make(map[string]*HealthStatus),
		pendingChecks: make(map[string]bool),
		checkInterval: checkInterval,
		stopChan:      make(chan bool),
	}
}

// Start begins the health monitoring process
func (p *HealthProcessor) Start() {
	log.Info("Starting health processor")

	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.processHealthChecks()
		case <-p.stopChan:
			log.Info("Stopping health processor")
			return
		}
	}
}

// Stop gracefully stops the health processor
func (p *HealthProcessor) Stop() {
	p.stopChan <- true
}

// processHealthChecks retrieves all gateways and checks their health
func (p *HealthProcessor) processHealthChecks() {
	ctx := context.Background()

	query := `SELECT id, serial, organization_id, last_seen FROM gateways`
	rows, err := p.db.Query(ctx, query)
	if err != nil {
		log.Errorf("Failed to query gateways: %v", err)
		return
	}
	defer rows.Close()

	var gateways []struct {
		ID             string
		Serial         string
		OrganizationID string
		LastSeen       time.Time
	}

	for rows.Next() {
		var g struct {
			ID             string
			Serial         string
			OrganizationID string
			LastSeen       time.Time
		}
		if err := rows.Scan(&g.ID, &g.Serial, &g.OrganizationID, &g.LastSeen); err != nil {
			log.Errorf("Failed to scan gateway: %v", err)
			continue
		}
		gateways = append(gateways, g)
	}

	// Process health checks concurrently
	// BUG 2: This creates concurrent goroutines that access shared state without synchronization
	var wg sync.WaitGroup
	for _, gateway := range gateways {
		wg.Add(1)
		go func(gw struct {
			ID             string
			Serial         string
			OrganizationID string
			LastSeen       time.Time
		}) {
			defer wg.Done()
			p.checkGatewayHealth(gw.ID, gw.LastSeen)
		}(gateway)
	}

	wg.Wait()
	log.Debugf("Completed health checks for %d gateways", len(gateways))
}

// checkGatewayHealth checks the health of a single gateway
// BUG 2: Deadlock - concurrent access to shared maps without proper locking
func (p *HealthProcessor) checkGatewayHealth(gatewayID string, lastSeen time.Time) {
	// BUG 2: Reading from shared map without lock
	if p.pendingChecks[gatewayID] {
		log.Debugf("Health check already pending for gateway %s", gatewayID)
		return
	}

	// BUG 2: Writing to shared map without lock - causes race condition
	p.mu.Lock()
	if p.pendingChecks[gatewayID] {
		p.mu.Unlock()
		return
	}
	p.pendingChecks[gatewayID] = true
	p.mu.Unlock()


	// Read cached status under read lock
	p.mu.RLock()
	cachedStatus := p.healthCache[gatewayID]
	p.mu.RUnlock()

	// Simulate health check logic
	status := p.determineHealthStatus(gatewayID, lastSeen)
	if cachedStatus != nil {
		status.ErrorCount = cachedStatus.ErrorCount
		if status.Status != models.HealthStatusHealthy {
			status.ErrorCount++
		} else {
			status.ErrorCount = 0
		}
	}

	// Commit new cache value quickly under write lock
	p.mu.Lock()
	p.healthCache[gatewayID] = status
	p.mu.Unlock()

	// Persist to database
	p.updateHealthInDatabase(status)

	// Trigger alerts if needed
	if status.Status == models.HealthStatusCritical {
		p.triggerHealthAlert(status)
	}

	// Clear pending
	p.mu.Lock()
	delete(p.pendingChecks, gatewayID)
	p.mu.Unlock()
}

// determineHealthStatus calculates the health status based on various factors
func (p *HealthProcessor) determineHealthStatus(gatewayID string, lastSeen time.Time) *HealthStatus {
	status := &HealthStatus{
		GatewayID:   gatewayID,
		LastChecked: time.Now(),
		ErrorCount:  0,
	}

	timeSinceLastSeen := time.Since(lastSeen)

	// Determine status based on last seen time
	switch {
	case timeSinceLastSeen < 5*time.Minute:
		status.Status = models.HealthStatusHealthy
	case timeSinceLastSeen < 15*time.Minute:
		status.Status = models.HealthStatusWarning
	case timeSinceLastSeen < 30*time.Minute:
		status.Status = models.HealthStatusCritical
	default:
		status.Status = models.HealthStatusOffline
	}

	// BUG 2: Reading from shared map without lock
	if cached, exists := p.healthCache[gatewayID]; exists {
		status.ErrorCount = cached.ErrorCount
		if status.Status != models.HealthStatusHealthy {
			status.ErrorCount++
		} else {
			status.ErrorCount = 0
		}
	}

	return status
}

// updateHealthInDatabase persists health status to the database
func (p *HealthProcessor) updateHealthInDatabase(status *HealthStatus) {
	ctx := context.Background()

	query := `
		UPDATE gateways 
		SET health_status = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := p.db.Exec(ctx, query, status.Status, time.Now(), status.GatewayID)
	if err != nil {
		log.Errorf("Failed to update health status for gateway %s: %v", status.GatewayID, err)
		return
	}

	// Also insert into health history
	historyQuery := `
		INSERT INTO gateway_health_history (gateway_id, status, error_count, checked_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err = p.db.Exec(ctx, historyQuery, status.GatewayID, status.Status, status.ErrorCount, status.LastChecked)
	if err != nil {
		log.Errorf("Failed to insert health history for gateway %s: %v", status.GatewayID, err)
	}
}

// triggerHealthAlert sends an alert for critical health status
func (p *HealthProcessor) triggerHealthAlert(status *HealthStatus) {
	log.Warnf("ALERT: Gateway %s is in %s state (errors: %d)",
		status.GatewayID, status.Status, status.ErrorCount)

	// In production, this would send notifications via email, Slack, PagerDuty, etc.
	ctx := context.Background()

	alertQuery := `
		INSERT INTO gateway_alerts (gateway_id, severity, message, timestamp)
		VALUES ($1, $2, $3, $4)
	`

	message := fmt.Sprintf("Gateway health is %s with %d consecutive errors",
		status.Status, status.ErrorCount)

	_, err := p.db.Exec(ctx, alertQuery, status.GatewayID, "critical", message, time.Now())
	if err != nil {
		log.Errorf("Failed to insert health alert: %v", err)
	}
}

// GetHealthStatus returns the cached health status for a gateway
// BUG 2: Race condition - reading from shared map without lock
func (p *HealthProcessor) GetHealthStatus(gatewayID string) (*HealthStatus, error) {
	// BUG 2: Concurrent read without lock
	status, exists := p.healthCache[gatewayID]
	if !exists {
		return nil, fmt.Errorf("no health status found for gateway %s", gatewayID)
	}

	// Return a copy to avoid external modifications
	return &HealthStatus{
		GatewayID:    status.GatewayID,
		Status:       status.Status,
		LastChecked:  status.LastChecked,
		ErrorCount:   status.ErrorCount,
		ResponseTime: status.ResponseTime,
	}, nil
}

// GetAllHealthStatuses returns all cached health statuses
// BUG 2: Race condition - reading from shared map without lock
func (p *HealthProcessor) GetAllHealthStatuses() []*HealthStatus {
	statuses := make([]*HealthStatus, 0, len(p.healthCache))

	// BUG 2: Iterating over shared map without lock
	for _, status := range p.healthCache {
		statuses = append(statuses, &HealthStatus{
			GatewayID:    status.GatewayID,
			Status:       status.Status,
			LastChecked:  status.LastChecked,
			ErrorCount:   status.ErrorCount,
			ResponseTime: status.ResponseTime,
		})
	}

	return statuses
}

// ClearCache clears the health cache
// BUG 2: Race condition - modifying shared map without lock
func (p *HealthProcessor) ClearCache() {
	// BUG 2: Clearing map without lock while other goroutines may be accessing it
	p.healthCache = make(map[string]*HealthStatus)
	p.pendingChecks = make(map[string]bool)
	log.Info("Health cache cleared")
}
