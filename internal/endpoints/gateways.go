package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"strings"

	"backend-assessment/internal/datastore"
	"backend-assessment/internal/models"
    "backend-assessment/internal/reliability"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// GatewayHandler handles gateway-related HTTP requests
type GatewayHandler struct {
	db *datastore.PostgresDB
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(db *datastore.PostgresDB) *GatewayHandler {
	return &GatewayHandler{db: db}
}

// gatewayCache and mutex to protect concurrent access
var (
    gatewayCache   = make(map[string][]models.Gateway)
    gatewayCacheMu sync.RWMutex
)

// ListGateways returns a list of gateways
// BUG 3: Race condition in gateway cache - concurrent map read/write
func (h *GatewayHandler) ListGateways(w http.ResponseWriter, r *http.Request) {
	// Extract user info from request (simplified - normally from JWT)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get search parameter and use as cache key
	search := r.URL.Query().Get("search")
	cacheKey := fmt.Sprintf("gateways_%s", search)
	
	// Read from cache under read lock
	gatewayCacheMu.RLock()
	cached, exists := gatewayCache[cacheKey]
	gatewayCacheMu.RUnlock()
	if exists {
		log.Debugf("Returning cached gateways for key: %s", cacheKey)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cached)
		return
	}

	// Build query with proper parameterization
	query := "SELECT id, serial, organization_id, site_id, name, health_status, last_seen, version, ip_address, location, created_at, updated_at FROM gateways"
	args := []interface{}{}
	
	if search != "" {
		query += " WHERE name LIKE $1 OR serial LIKE $1"
		args = append(args, "%"+search+"%")
	}
	
	// Note: In production, should filter by organization_id for proper tenant isolation
	
	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		log.Errorf("Failed to query gateways: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var gateways []models.Gateway
	for rows.Next() {
		var g models.Gateway
		err := rows.Scan(&g.ID, &g.Serial, &g.OrganizationID, &g.SiteID, &g.Name, 
			&g.HealthStatus, &g.LastSeen, &g.Version, &g.IPAddress, &g.Location, 
			&g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			log.Errorf("Failed to scan gateway: %v", err)
			continue
		}
		gateways = append(gateways, g)
	}

	// Write to cache under write lock
	gatewayCacheMu.Lock()
	gatewayCache[cacheKey] = gateways
	gatewayCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gateways)
}

// GetGateway returns a specific gateway by ID
func (h *GatewayHandler) GetGateway(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gatewayID := vars["id"]

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Note: In production, should verify user has access to this organization's gateway
	query := "SELECT id, serial, organization_id, site_id, name, health_status, last_seen, version, ip_address, location, created_at, updated_at FROM gateways WHERE id = $1"
	
	var g models.Gateway
	err := h.db.QueryRow(context.Background(), query, gatewayID).Scan(&g.ID, &g.Serial, &g.OrganizationID, 
		&g.SiteID, &g.Name, &g.HealthStatus, &g.LastSeen, &g.Version, &g.IPAddress, 
		&g.Location, &g.CreatedAt, &g.UpdatedAt)
	
	if err != nil {
		if err.Error() == "no rows in result set" {
			http.Error(w, "Gateway not found", http.StatusNotFound)
			return
		}
		log.Errorf("Failed to query gateway: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// UpdateGateway updates a gateway
func (h *GatewayHandler) UpdateGateway(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gatewayID := vars["id"]

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var updateData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build dynamic update query (simplified)
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	for field, value := range updateData {
		// Basic validation (incomplete)
		if field == "name" || field == "location" {
			setParts = append(setParts, fmt.Sprintf("%s = $%d", field, argIndex))
			args = append(args, value)
			argIndex++
		}
	}

	if len(setParts) == 0 {
		http.Error(w, "No valid fields to update", http.StatusBadRequest)
		return
	}

	// Note: In production, should verify user has access to this organization's gateway
	query := fmt.Sprintf("UPDATE gateways SET %s WHERE id = $%d", 
		strings.Join(setParts, ", "), argIndex)
	args = append(args, gatewayID)

	_, err := h.db.Exec(context.Background(), query, args...)
	if err != nil {
		log.Errorf("Failed to update gateway: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"updated"}`))
}

// RebootGateway sends a reboot command to a gateway
func (h *GatewayHandler) RebootGateway(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gatewayID := vars["id"]

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

    // Note: In production, should verify user has access to this organization's gateway

    // Wrap reboot with circuit breaker + retry
    breaker := reliability.DefaultBreaker()
    err := breaker.Do(func() error {
        // Simulate sending reboot command (external call)
        log.Infof("User %s requested reboot for gateway %s", userID, gatewayID)
        return nil
    })
    if err != nil {
        http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
        return
    }

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"reboot_initiated"}`))
}