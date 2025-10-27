package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"backend-assessment/internal/config"
	"backend-assessment/internal/datastore"

	log "github.com/sirupsen/logrus"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// migrations contains all database migrations in order
var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up: `
-- Create organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create sites table
CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    location VARCHAR(500),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create gateways table
CREATE TABLE IF NOT EXISTS gateways (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial VARCHAR(255) NOT NULL UNIQUE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    health_status VARCHAR(50) NOT NULL DEFAULT 'offline',
    last_seen TIMESTAMP NOT NULL DEFAULT NOW(),
    version VARCHAR(50),
    ip_address VARCHAR(50),
    location VARCHAR(500),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create schema_migrations table to track applied migrations
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_sites_organization_id ON sites(organization_id);
CREATE INDEX IF NOT EXISTS idx_gateways_organization_id ON gateways(organization_id);
CREATE INDEX IF NOT EXISTS idx_gateways_site_id ON gateways(site_id);
CREATE INDEX IF NOT EXISTS idx_gateways_serial ON gateways(serial);
CREATE INDEX IF NOT EXISTS idx_gateways_health_status ON gateways(health_status);
CREATE INDEX IF NOT EXISTS idx_gateways_last_seen ON gateways(last_seen);
`,
		Down: `
DROP TABLE IF EXISTS gateways CASCADE;
DROP TABLE IF EXISTS sites CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;
`,
	},
	{
		Version: 2,
		Name:    "worker_tables",
		Up: `
-- Deployments table referenced by message processor
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- App status per gateway
CREATE TABLE IF NOT EXISTS gateway_app_status (
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    app_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gateway_id, app_id)
);

-- Metrics from gateways
CREATE TABLE IF NOT EXISTS gateway_metrics (
    id BIGSERIAL PRIMARY KEY,
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    metrics JSONB NOT NULL,
    timestamp TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_metrics_gateway_id ON gateway_metrics(gateway_id);
CREATE INDEX IF NOT EXISTS idx_gateway_metrics_timestamp ON gateway_metrics(timestamp);

-- Alerts from gateways
CREATE TABLE IF NOT EXISTS gateway_alerts (
    id BIGSERIAL PRIMARY KEY,
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    severity VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_alerts_gateway_id ON gateway_alerts(gateway_id);
CREATE INDEX IF NOT EXISTS idx_gateway_alerts_timestamp ON gateway_alerts(timestamp);

-- Health history
CREATE TABLE IF NOT EXISTS gateway_health_history (
    id BIGSERIAL PRIMARY KEY,
    gateway_id UUID NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    error_count INTEGER NOT NULL DEFAULT 0,
    checked_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_health_history_gateway_id ON gateway_health_history(gateway_id);
CREATE INDEX IF NOT EXISTS idx_gateway_health_history_checked_at ON gateway_health_history(checked_at);
`,
		Down: `
DROP TABLE IF EXISTS gateway_health_history CASCADE;
DROP TABLE IF EXISTS gateway_alerts CASCADE;
DROP TABLE IF EXISTS gateway_metrics CASCADE;
DROP TABLE IF EXISTS gateway_app_status CASCADE;
DROP TABLE IF EXISTS deployments CASCADE;
`,
	},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <up|down|status>")
		os.Exit(1)
	}

	command := os.Args[1]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := datastore.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	switch command {
	case "up":
		if err := migrateUp(ctx, db); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Info("Migrations applied successfully")
	case "down":
		if err := migrateDown(ctx, db); err != nil {
			log.Fatalf("Migration rollback failed: %v", err)
		}
		log.Info("Migrations rolled back successfully")
	case "status":
		if err := showStatus(ctx, db); err != nil {
			log.Fatalf("Failed to show status: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: migrate <up|down|status>")
		os.Exit(1)
	}
}

func migrateUp(ctx context.Context, db *datastore.PostgresDB) error {
	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, migration := range migrations {
		// Check if migration is already applied
		var count int
		err := db.QueryRow(ctx, `
			SELECT COUNT(*) FROM schema_migrations WHERE version = $1
		`, migration.Version).Scan(&count)

		// If schema_migrations table doesn't exist yet, create it
		if err != nil {
			// Try to create the schema_migrations table
			_, err = db.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS schema_migrations (
					version INTEGER PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					applied_at TIMESTAMP NOT NULL DEFAULT NOW()
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create schema_migrations table: %w", err)
			}
			count = 0
		}

		if count > 0 {
			log.Infof("Migration %d (%s) already applied, skipping", migration.Version, migration.Name)
			continue
		}

		log.Infof("Applying migration %d: %s", migration.Version, migration.Name)

		// Start transaction
		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		// Execute migration
		_, err = tx.Exec(ctx, migration.Up)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		// Record migration
		_, err = tx.Exec(ctx, `
			INSERT INTO schema_migrations (version, name) VALUES ($1, $2)
		`, migration.Version, migration.Name)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		log.Infof("Migration %d applied successfully", migration.Version)
	}

	return nil
}

func migrateDown(ctx context.Context, db *datastore.PostgresDB) error {
	// Sort migrations by version in reverse order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version > migrations[j].Version
	})

	for _, migration := range migrations {
		// Check if migration is applied
		var count int
		err := db.QueryRow(ctx, `
			SELECT COUNT(*) FROM schema_migrations WHERE version = $1
		`, migration.Version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if count == 0 {
			log.Infof("Migration %d (%s) not applied, skipping", migration.Version, migration.Name)
			continue
		}

		log.Infof("Rolling back migration %d: %s", migration.Version, migration.Name)

		// Start transaction
		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		// Execute rollback
		_, err = tx.Exec(ctx, migration.Down)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
		}

		// Remove migration record
		_, err = tx.Exec(ctx, `
			DELETE FROM schema_migrations WHERE version = $1
		`, migration.Version)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to remove migration record %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit rollback %d: %w", migration.Version, err)
		}

		log.Infof("Migration %d rolled back successfully", migration.Version)
	}

	return nil
}

func showStatus(ctx context.Context, db *datastore.PostgresDB) error {
	// Check if schema_migrations table exists
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check schema_migrations table: %w", err)
	}

	if !exists {
		fmt.Println("No migrations have been applied yet")
		return nil
	}

	// Get applied migrations
	rows, err := db.Query(ctx, `
		SELECT version, name, applied_at 
		FROM schema_migrations 
		ORDER BY version
	`)
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	fmt.Println("Applied migrations:")
	for rows.Next() {
		var version int
		var name string
		var appliedAt string
		if err := rows.Scan(&version, &name, &appliedAt); err != nil {
			return fmt.Errorf("failed to scan migration: %w", err)
		}
		fmt.Printf("  %d: %s (applied at %s)\n", version, name, appliedAt)
	}

	return nil
}
