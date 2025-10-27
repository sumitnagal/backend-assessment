package main

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	mrand "math/rand"
	"sync"
	"time"

	"backend-assessment/applications/messageprocessor"
	"backend-assessment/internal/config"
	"backend-assessment/internal/datastore"
    "backend-assessment/internal/observability"

	log "github.com/sirupsen/logrus"
)

// dbAdapter adapts *datastore.PostgresDB to messageprocessor.DB
type dbAdapter struct{ inner *datastore.PostgresDB }

func (a *dbAdapter) Acquire(ctx context.Context) (messageprocessor.Conn, error) {
    return a.inner.Acquire(ctx)
}

func init() {
	// Seed math/rand for synthetic data generation
	mrand.Seed(time.Now().UnixNano())
}

func runWorkerLoadImpl(ratePerSec, durationSec, queueSize, workerID int) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

    db, err := datastore.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer db.Close()

    // initialize tracing for CLI load if enabled
    shutdown, err := observability.InitTracing(context.Background(), cfg)
    if err != nil {
        log.Fatalf("failed to init tracing: %v", err)
    }
    defer func() {
        if err := shutdown(context.Background()); err != nil {
            log.Errorf("tracing shutdown error: %v", err)
        }
    }()

    worker := messageprocessor.NewWorker(&dbAdapter{inner: db}, workerID, queueSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fetch real gateway UUIDs to satisfy FK constraints
	gatewayIDs, err := fetchGatewayIDs(db)
	if err != nil {
		log.Fatalf("failed to fetch gateway IDs: %v", err)
	}
	if len(gatewayIDs) == 0 {
		log.Fatalf("no gateways found. Run 'make seed' first, then retry")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.Start()
	}()

	// Producer: generate messages at target rate (per second)
	ticker := time.NewTicker(1 * time.Second)
	end := time.After(time.Duration(durationSec) * time.Second)
	produced := 0
	for {
		select {
		case <-end:
			cancel()
			worker.Stop()
			wg.Wait()
			log.Infof("Produced %d messages. Queue size now: %d", produced, worker.GetQueueSize())
			return
		case <-ticker.C:
			for i := 0; i < max(1, ratePerSec); i++ {
				_ = worker.EnqueueMessage(randomGatewayMessage(gatewayIDs))
				produced++
			}
		case <-ctx.Done():
			worker.Stop()
			wg.Wait()
			return
		}
	}
}

func randomGatewayMessage(gatewayIDs []string) messageprocessor.GatewayMessage {
	types := []messageprocessor.MessageType{
		messageprocessor.MessageTypeHeartbeat,
		messageprocessor.MessageTypeAppStatus,
		messageprocessor.MessageTypeMetrics,
		messageprocessor.MessageTypeAlert,
		messageprocessor.MessageTypeDeployment,
	}
	mt := types[mrand.Intn(len(types))]

	payload := map[string]interface{}{}
	switch mt {
	case messageprocessor.MessageTypeHeartbeat:
		payload["last_seen"] = time.Now().Add(-time.Duration(mrand.Intn(600)) * time.Second).Format(time.RFC3339)
	case messageprocessor.MessageTypeAppStatus:
		payload["app_id"] = generateUUIDv4()
		payload["status"] = []string{"running", "stopped", "crashed"}[mrand.Intn(3)]
	case messageprocessor.MessageTypeMetrics:
		payload["cpu"] = mrand.Float64() * 100
		payload["mem"] = mrand.Float64() * 100
	case messageprocessor.MessageTypeAlert:
		payload["severity"] = []string{"info", "warning", "critical"}[mrand.Intn(3)]
		payload["message"] = "synthetic alert"
	case messageprocessor.MessageTypeDeployment:
		payload["deployment_id"] = generateUUIDv4()
		payload["status"] = []string{"pending", "in_progress", "completed", "failed"}[mrand.Intn(4)]
	}

	return messageprocessor.GatewayMessage{
		GatewayID:   gatewayIDs[mrand.Intn(len(gatewayIDs))],
		MessageType: mt,
		Timestamp:   time.Now(),
		Payload:     payload,
	}
}

func rndID(prefix string) string {
	return prefix + "-" + time.Now().Format("150405") + "-" + randomString(5)
}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[mrand.Intn(len(letters))]
	}
	return string(b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fetchGatewayIDs(db *datastore.PostgresDB) ([]string, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx, `SELECT id FROM gateways`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// generateUUIDv4 creates a RFC4122-ish UUID v4 string using crypto/rand
func generateUUIDv4() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		// fallback to math/rand if crypto fails
		for i := range b {
			b[i] = byte(mrand.Intn(256))
		}
	}
	// Set version (4) and variant (RFC 4122)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexs := make([]byte, 36)
	hex.Encode(hexs[0:8], b[0:4])
	hexs[8] = '-'
	hex.Encode(hexs[9:13], b[4:6])
	hexs[13] = '-'
	hex.Encode(hexs[14:18], b[6:8])
	hexs[18] = '-'
	hex.Encode(hexs[19:23], b[8:10])
	hexs[23] = '-'
	hex.Encode(hexs[24:36], b[10:16])
	return string(hexs)
}
