package messageprocessor

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "backend-assessment/internal/models"

    log "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

// MessageType represents different types of messages from gateways
type MessageType string

const (
	MessageTypeHeartbeat  MessageType = "heartbeat"
	MessageTypeAppStatus  MessageType = "app_status"
	MessageTypeMetrics    MessageType = "metrics"
	MessageTypeAlert      MessageType = "alert"
	MessageTypeDeployment MessageType = "deployment"
)

// GatewayMessage represents a message received from a gateway
type GatewayMessage struct {
	GatewayID   string                 `json:"gateway_id"`
	MessageType MessageType            `json:"message_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
}

// Worker processes messages from IoT gateways
// DB abstracts the minimal database operations needed by the worker for testability
type DB interface {
    Acquire(ctx context.Context) (Conn, error)
}

// Conn abstracts a single connection acquired from the DB pool
type Conn interface {
    Exec(ctx context.Context, sql string, args ...interface{}) error
    Release()
}

type Worker struct {
    db           DB
	messageQueue chan GatewayMessage
	workerID     int
	stopChan     chan bool
}

// NewWorker creates a new message processor worker
func NewWorker(db DB, workerID int, queueSize int) *Worker {
	return &Worker{
		db:           db,
		messageQueue: make(chan GatewayMessage, queueSize),
		workerID:     workerID,
		stopChan:     make(chan bool),
	}
}

// Start begins processing messages
func (w *Worker) Start() {
	log.Infof("Worker %d starting message processing", w.workerID)

	for {
		select {
		case msg := <-w.messageQueue:
			w.processMessage(msg)
		case <-w.stopChan:
			log.Infof("Worker %d stopping", w.workerID)
			return
		}
	}
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	w.stopChan <- true
}

// EnqueueMessage adds a message to the processing queue
func (w *Worker) EnqueueMessage(msg GatewayMessage) error {
	select {
	case w.messageQueue <- msg:
		return nil
	default:
		return fmt.Errorf("message queue full")
	}
}

// processMessage handles a single gateway message
// BUG 1: Database connections not properly closed, causing memory leaks
func (w *Worker) processMessage(msg GatewayMessage) {
    tracer := otel.Tracer("worker")
    _, span := tracer.Start(context.Background(), "process_message")
    span.SetAttributes(
        attribute.Int("worker.id", w.workerID),
        attribute.String("gateway.id", msg.GatewayID),
        attribute.String("message.type", string(msg.MessageType)),
    )
    defer span.End()

    log.Debugf("Worker %d processing message type %s from gateway %s",
        w.workerID, msg.MessageType, msg.GatewayID)

	switch msg.MessageType {
	case MessageTypeHeartbeat:
		w.processHeartbeat(msg)
	case MessageTypeAppStatus:
		w.processAppStatus(msg)
	case MessageTypeMetrics:
		w.processMetrics(msg)
	case MessageTypeAlert:
		w.processAlert(msg)
	case MessageTypeDeployment:
		w.processDeployment(msg)
	default:
        span.SetStatus(codes.Error, "unknown message type")
        log.Warnf("Unknown message type: %s", msg.MessageType)
	}
}

// processHeartbeat updates gateway last_seen timestamp
// BUG 1: Connection leak - Acquire() without Release()
func (w *Worker) processHeartbeat(msg GatewayMessage) {
    ctx, span := otel.Tracer("worker").Start(context.Background(), "process_heartbeat")
    span.SetAttributes(attribute.String("gateway.id", msg.GatewayID))
    defer span.End()
	// BUG 1: Acquiring a connection directly from the pool without releasing it
	// This causes connections to leak and eventually exhaust the pool
	conn, err := w.db.Acquire(context.Background())
	if err != nil {
		log.Errorf("Failed to acquire connection: %v", err)
		return
	}
	// MISSING: defer conn.Release() - this is the bug!
	defer conn.Release()

	query := `UPDATE gateways SET last_seen = $1, health_status = $2 WHERE id = $3`

	healthStatus := models.HealthStatusHealthy
	if lastSeenStr, ok := msg.Payload["last_seen"].(string); ok {
		lastSeen, err := time.Parse(time.RFC3339, lastSeenStr)
		if err == nil && time.Since(lastSeen) > 5*time.Minute {
			healthStatus = models.HealthStatusWarning
		}
	}

    err = conn.Exec(ctx, query, msg.Timestamp, healthStatus, msg.GatewayID)
	if err != nil {
        span.RecordError(err)
		log.Errorf("Failed to update gateway heartbeat: %v", err)
		return
	}

	log.Debugf("Updated heartbeat for gateway %s", msg.GatewayID)
}

// processAppStatus updates application status on gateway
// BUG 1: Another connection leak
func (w *Worker) processAppStatus(msg GatewayMessage) {
    ctx, span := otel.Tracer("worker").Start(context.Background(), "process_app_status")
    span.SetAttributes(attribute.String("gateway.id", msg.GatewayID))
    defer span.End()
	// BUG 1: Another connection leak - no Release() call
	conn, err := w.db.Acquire(context.Background())
	if err != nil {
		log.Errorf("Failed to acquire connection: %v", err)
		return
	}
	// MISSING: defer conn.Release() - this is the bug!
	defer conn.Release()

	appID, ok := msg.Payload["app_id"].(string)
	if !ok {
		log.Warn("Missing app_id in app_status message")
		return
	}

	status, ok := msg.Payload["status"].(string)
	if !ok {
		log.Warn("Missing status in app_status message")
		return
	}

	query := `
		INSERT INTO gateway_app_status (gateway_id, app_id, status, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (gateway_id, app_id) 
		DO UPDATE SET status = $3, updated_at = $4
	`

    err = conn.Exec(ctx, query, msg.GatewayID, appID, status, msg.Timestamp)
	if err != nil {
        span.RecordError(err)
		log.Errorf("Failed to update app status: %v", err)
		return
	}

	log.Debugf("Updated app status for gateway %s, app %s: %s", msg.GatewayID, appID, status)
}

// processMetrics stores gateway metrics
// BUG 1: Yet another connection leak
func (w *Worker) processMetrics(msg GatewayMessage) {
    ctx, span := otel.Tracer("worker").Start(context.Background(), "process_metrics")
    span.SetAttributes(attribute.String("gateway.id", msg.GatewayID))
    defer span.End()
	// BUG 1: Connection leak - acquiring without releasing
	conn, err := w.db.Acquire(context.Background())
	if err != nil {
		log.Errorf("Failed to acquire connection: %v", err)
		return
	}
	// MISSING: defer conn.Release() - this is the bug!
	defer conn.Release()

	metricsJSON, err := json.Marshal(msg.Payload)
	if err != nil {
		log.Errorf("Failed to marshal metrics: %v", err)
		return
	}

	query := `
		INSERT INTO gateway_metrics (gateway_id, metrics, timestamp)
		VALUES ($1, $2, $3)
	`

    err = conn.Exec(ctx, query, msg.GatewayID, metricsJSON, msg.Timestamp)
	if err != nil {
        span.RecordError(err)
		log.Errorf("Failed to insert metrics: %v", err)
		return
	}

	log.Debugf("Stored metrics for gateway %s", msg.GatewayID)
}

// processAlert handles alert messages from gateways
func (w *Worker) processAlert(msg GatewayMessage) {
    ctx, span := otel.Tracer("worker").Start(context.Background(), "process_alert")
    span.SetAttributes(attribute.String("gateway.id", msg.GatewayID))
    defer span.End()
	// This one is implemented correctly for comparison
	conn, err := w.db.Acquire(context.Background())
	if err != nil {
		log.Errorf("Failed to acquire connection: %v", err)
		return
	}
	defer conn.Release() // Correct: connection is released

	severity, ok := msg.Payload["severity"].(string)
	if !ok {
		severity = "info"
	}

	message, ok := msg.Payload["message"].(string)
	if !ok {
		log.Warn("Missing message in alert")
		return
	}

	query := `
		INSERT INTO gateway_alerts (gateway_id, severity, message, timestamp)
		VALUES ($1, $2, $3, $4)
	`

    err = conn.Exec(ctx, query, msg.GatewayID, severity, message, msg.Timestamp)
	if err != nil {
        span.RecordError(err)
		log.Errorf("Failed to insert alert: %v", err)
		return
	}

	log.Infof("Alert from gateway %s [%s]: %s", msg.GatewayID, severity, message)
}

// processDeployment handles deployment status updates
// BUG 1: One more connection leak
func (w *Worker) processDeployment(msg GatewayMessage) {
    ctx, span := otel.Tracer("worker").Start(context.Background(), "process_deployment")
    span.SetAttributes(attribute.String("gateway.id", msg.GatewayID))
    defer span.End()
	// BUG 1: Connection leak
	conn, err := w.db.Acquire(context.Background())
	if err != nil {
		log.Errorf("Failed to acquire connection: %v", err)
		return
	}
	// MISSING: defer conn.Release() - this is the bug!
	defer conn.Release()

	deploymentID, ok := msg.Payload["deployment_id"].(string)
	if !ok {
		log.Warn("Missing deployment_id in deployment message")
		return
	}

	status, ok := msg.Payload["status"].(string)
	if !ok {
		log.Warn("Missing status in deployment message")
		return
	}

	query := `
		UPDATE deployments 
		SET status = $1, updated_at = $2
		WHERE id = $3 AND gateway_id = $4
	`

    err = conn.Exec(ctx, query, status, msg.Timestamp, deploymentID, msg.GatewayID)
	if err != nil {
        span.RecordError(err)
		log.Errorf("Failed to update deployment status: %v", err)
		return
	}

	log.Infof("Deployment %s on gateway %s: %s", deploymentID, msg.GatewayID, status)
}

// GetQueueSize returns the current queue size
func (w *Worker) GetQueueSize() int {
	return len(w.messageQueue)
}
