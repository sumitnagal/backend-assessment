package endpoints

import (
	"context"
	"encoding/json"
	"net/http"

	"backend-assessment/internal/datastore"
	"backend-assessment/internal/models"

	log "github.com/sirupsen/logrus"
)

// OrganizationHandler handles organization-related HTTP requests
type OrganizationHandler struct {
	db *datastore.PostgresDB
}

// NewOrganizationHandler creates a new organization handler
func NewOrganizationHandler(db *datastore.PostgresDB) *OrganizationHandler {
	return &OrganizationHandler{db: db}
}

// ListOrganizations returns a list of organizations
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := "SELECT id, name, settings, created_at, updated_at FROM organizations"
	
	rows, err := h.db.Query(context.Background(), query)
	if err != nil {
		log.Errorf("Failed to query organizations: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orgs []models.Organization
	for rows.Next() {
		var o models.Organization
		err := rows.Scan(&o.ID, &o.Name, &o.Settings, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			log.Errorf("Failed to scan organization: %v", err)
			continue
		}
		orgs = append(orgs, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orgs)
}