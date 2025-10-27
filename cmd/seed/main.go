package main

import (
	"context"
	"fmt"

	"backend-assessment/internal/config"
	"backend-assessment/internal/datastore"

	log "github.com/sirupsen/logrus"
)

func main() {
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

	log.Info("Seeding test data...")

	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to start transaction: %v", err)
	}

	// Seed organizations
	log.Info("Seeding organizations...")
	_, err = tx.Exec(ctx, `
		INSERT INTO organizations (id, name, settings) VALUES
		('00000000-0000-0000-0000-000000000001', 'Acme Corporation', '{"plan": "enterprise"}'),
		('00000000-0000-0000-0000-000000000002', 'TechStart Inc', '{"plan": "professional"}'),
		('00000000-0000-0000-0000-000000000003', 'Global Industries', '{"plan": "enterprise"}')
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		tx.Rollback(ctx)
		log.Fatalf("Failed to seed organizations: %v", err)
	}

	// Seed users
	log.Info("Seeding users...")
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, organization_id, role) VALUES
		('10000000-0000-0000-0000-000000000001', 'admin@acme.com', '00000000-0000-0000-0000-000000000001', 'admin'),
		('10000000-0000-0000-0000-000000000002', 'user@acme.com', '00000000-0000-0000-0000-000000000001', 'user'),
		('10000000-0000-0000-0000-000000000003', 'admin@techstart.com', '00000000-0000-0000-0000-000000000002', 'admin'),
		('10000000-0000-0000-0000-000000000004', 'admin@global.com', '00000000-0000-0000-0000-000000000003', 'admin')
		ON CONFLICT (email) DO NOTHING
	`)
	if err != nil {
		tx.Rollback(ctx)
		log.Fatalf("Failed to seed users: %v", err)
	}

	// Seed sites
	log.Info("Seeding sites...")
	_, err = tx.Exec(ctx, `
		INSERT INTO sites (id, name, organization_id, location) VALUES
		('20000000-0000-0000-0000-000000000001', 'Headquarters', '00000000-0000-0000-0000-000000000001', 'San Francisco, CA'),
		('20000000-0000-0000-0000-000000000002', 'Manufacturing Plant', '00000000-0000-0000-0000-000000000001', 'Austin, TX'),
		('20000000-0000-0000-0000-000000000003', 'Distribution Center', '00000000-0000-0000-0000-000000000001', 'Chicago, IL'),
		('20000000-0000-0000-0000-000000000004', 'Office', '00000000-0000-0000-0000-000000000002', 'Seattle, WA'),
		('20000000-0000-0000-0000-000000000005', 'Warehouse', '00000000-0000-0000-0000-000000000003', 'New York, NY')
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		tx.Rollback(ctx)
		log.Fatalf("Failed to seed sites: %v", err)
	}

	// Seed gateways
	log.Info("Seeding gateways...")
	_, err = tx.Exec(ctx, `
		INSERT INTO gateways (id, serial, organization_id, site_id, name, health_status, version, ip_address, location) VALUES
		('30000000-0000-0000-0000-000000000001', 'GW-HQ-001', '00000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'HQ Gateway 1', 'healthy', '2.1.0', '192.168.1.10', 'Building A, Floor 1'),
		('30000000-0000-0000-0000-000000000002', 'GW-HQ-002', '00000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'HQ Gateway 2', 'healthy', '2.1.0', '192.168.1.11', 'Building A, Floor 2'),
		('30000000-0000-0000-0000-000000000003', 'GW-MFG-001', '00000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000002', 'Manufacturing Gateway 1', 'warning', '2.0.5', '192.168.2.10', 'Production Line 1'),
		('30000000-0000-0000-0000-000000000004', 'GW-MFG-002', '00000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000002', 'Manufacturing Gateway 2', 'healthy', '2.1.0', '192.168.2.11', 'Production Line 2'),
		('30000000-0000-0000-0000-000000000005', 'GW-DC-001', '00000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000003', 'Distribution Gateway 1', 'critical', '1.9.2', '192.168.3.10', 'Warehouse Section A'),
		('30000000-0000-0000-0000-000000000006', 'GW-TS-001', '00000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000004', 'TechStart Gateway 1', 'healthy', '2.1.0', '192.168.4.10', 'Office Floor 3'),
		('30000000-0000-0000-0000-000000000007', 'GW-GI-001', '00000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000005', 'Global Gateway 1', 'offline', '2.0.0', '192.168.5.10', 'Warehouse Zone 1')
		ON CONFLICT (serial) DO NOTHING
	`)
	if err != nil {
		tx.Rollback(ctx)
		log.Fatalf("Failed to seed gateways: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	log.Info("Test data seeded successfully!")
	fmt.Println("\nSeeded data summary:")
	fmt.Println("  - 3 organizations")
	fmt.Println("  - 4 users")
	fmt.Println("  - 5 sites")
	fmt.Println("  - 7 gateways")
}
