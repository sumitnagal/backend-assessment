package endpoints

import (
	"context"
	"encoding/json"
	"net/http"

	"backend-assessment/internal/datastore"
	"backend-assessment/internal/models"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	db *datastore.PostgresDB
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *datastore.PostgresDB) *UserHandler {
	return &UserHandler{db: db}
}

// ListUsers returns a list of users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := "SELECT id, email, organization_id, role, created_at, updated_at FROM users"
	
	rows, err := h.db.Query(context.Background(), query)
	if err != nil {
		log.Errorf("Failed to query users: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Email, &u.OrganizationID, &u.Role, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			log.Errorf("Failed to scan user: %v", err)
			continue
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// GetUser returns a specific user by ID
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetUserID := vars["id"]

	currentUserID := r.Header.Get("X-User-ID")
	if currentUserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := "SELECT id, email, organization_id, role, created_at, updated_at FROM users WHERE id = $1"
	
	var u models.User
	err := h.db.QueryRow(context.Background(), query, targetUserID).Scan(&u.ID, &u.Email, &u.OrganizationID, 
		&u.Role, &u.CreatedAt, &u.UpdatedAt)
	
	if err != nil {
		if err.Error() == "no rows in result set" {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		log.Errorf("Failed to query user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}