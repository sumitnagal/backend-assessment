package models

import (
	"time"
)

// Gateway represents an IoT gateway device
type Gateway struct {
	ID             string    `json:"id" db:"id"`
	Serial         string    `json:"serial" db:"serial"`
	OrganizationID string    `json:"organization_id" db:"organization_id"`
	SiteID         string    `json:"site_id" db:"site_id"`
	Name           string    `json:"name" db:"name"`
	HealthStatus   string    `json:"health_status" db:"health_status"`
	LastSeen       time.Time `json:"last_seen" db:"last_seen"`
	Version        string    `json:"version" db:"version"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	Location       string    `json:"location" db:"location"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// HealthStatus constants
const (
	HealthStatusHealthy  = "healthy"
	HealthStatusWarning  = "warning"
	HealthStatusCritical = "critical"
	HealthStatusOffline  = "offline"
)

// User represents a system user
type User struct {
	ID             string    `json:"id" db:"id"`
	Email          string    `json:"email" db:"email"`
	OrganizationID string    `json:"organization_id" db:"organization_id"`
	Role           string    `json:"role" db:"role"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Organization represents a customer organization
type Organization struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Settings  string    `json:"settings" db:"settings"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Site represents a physical location containing gateways
type Site struct {
	ID             string    `json:"id" db:"id"`
	Name           string    `json:"name" db:"name"`
	OrganizationID string    `json:"organization_id" db:"organization_id"`
	Location       string    `json:"location" db:"location"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}