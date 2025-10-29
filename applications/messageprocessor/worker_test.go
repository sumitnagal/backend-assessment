package messageprocessor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// mockConn implements Conn
type mockConn struct{ execs int32 }

func (m *mockConn) Exec(ctx context.Context, sql string, args ...interface{}) error {
	atomic.AddInt32(&m.execs, 1)
	return nil
}
func (m *mockConn) Release() {}

// mockDB implements DB
type mockDB struct{ conn *mockConn }

func (m *mockDB) Acquire(ctx context.Context) (Conn, error) { return m.conn, nil }

func TestEnqueueMessage_Backpressure(t *testing.T) {
	w := NewWorker(&mockDB{conn: &mockConn{}}, 1, 1)
	// queue capacity 1: first ok, second ok (consumed later), third should error if not consumed
	if err := w.EnqueueMessage(GatewayMessage{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.EnqueueMessage(GatewayMessage{}); err == nil {
		// buffer size is 1: second send would block unless we consume; since we don't, this should error
		t.Fatalf("expected queue full error, got nil")
	}
}

func TestProcessAlert_ExecutesSQL(t *testing.T) {
	m := &mockConn{}
	w := NewWorker(&mockDB{conn: m}, 7, 10)

	go func() { w.Start() }()
	defer w.Stop()

	msg := GatewayMessage{
		GatewayID:   "30000000-0000-0000-0000-000000000001",
		MessageType: MessageTypeAlert,
		Timestamp:   time.Now(),
		Payload: map[string]interface{}{
			"severity": "warning",
			"message":  "test",
		},
	}
	if err := w.EnqueueMessage(msg); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	// Wait up to 1s for worker to handle the message
	deadline := time.Now().Add(1 * time.Second)
	for atomic.LoadInt32(&m.execs) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&m.execs) == 0 {
		t.Fatalf("expected Exec to be called at least once")
	}
}
