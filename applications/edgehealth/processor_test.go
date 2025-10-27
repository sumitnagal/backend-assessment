package edgehealth

import (
    "testing"
    "time"

    "backend-assessment/internal/models"
)

func TestDetermineHealthStatus_Thresholds(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    gatewayID := "30000000-0000-0000-0000-000000000001"

    // Healthy: < 5m
    s := p.determineHealthStatus(gatewayID, time.Now().Add(-4*time.Minute))
    if s.Status != models.HealthStatusHealthy {
        t.Fatalf("expected healthy, got %s", s.Status)
    }

    // Warning: < 15m
    s = p.determineHealthStatus(gatewayID, time.Now().Add(-10*time.Minute))
    if s.Status != models.HealthStatusWarning {
        t.Fatalf("expected warning, got %s", s.Status)
    }

    // Critical: < 30m
    s = p.determineHealthStatus(gatewayID, time.Now().Add(-20*time.Minute))
    if s.Status != models.HealthStatusCritical {
        t.Fatalf("expected critical, got %s", s.Status)
    }

    // Offline: >= 30m
    s = p.determineHealthStatus(gatewayID, time.Now().Add(-40*time.Minute))
    if s.Status != models.HealthStatusOffline {
        t.Fatalf("expected offline, got %s", s.Status)
    }
}

func TestDetermineHealthStatus_UsesCachedErrorCount(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    gatewayID := "30000000-0000-0000-0000-000000000001"

    // Seed cache with previous errors
    p.healthCache[gatewayID] = &HealthStatus{
        GatewayID:   gatewayID,
        Status:       models.HealthStatusWarning,
        LastChecked:  time.Now().Add(-1 * time.Hour),
        ErrorCount:   2,
    }

    // Move to non-healthy to increment error count
    s := p.determineHealthStatus(gatewayID, time.Now().Add(-20*time.Minute))
    if s.ErrorCount != 3 {
        t.Fatalf("expected error count to increment to 3, got %d", s.ErrorCount)
    }

    // Move to healthy resets errors
    s = p.determineHealthStatus(gatewayID, time.Now().Add(-1*time.Minute))
    if s.Status != models.HealthStatusHealthy || s.ErrorCount != 0 {
        t.Fatalf("expected healthy with 0 errors, got status=%s errors=%d", s.Status, s.ErrorCount)
    }
}

func TestGetHealthStatus_NotFound(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    if _, err := p.GetHealthStatus("missing"); err == nil {
        t.Fatalf("expected error for missing gateway")
    }
}

func TestGetHealthStatus_ReturnsCopy(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    gatewayID := "g1"
    p.healthCache[gatewayID] = &HealthStatus{GatewayID: gatewayID, Status: models.HealthStatusWarning}

    s, err := p.GetHealthStatus(gatewayID)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // mutate returned value, ensure cache not affected
    s.Status = models.HealthStatusHealthy
    if p.healthCache[gatewayID].Status != models.HealthStatusWarning {
        t.Fatalf("expected original cache to remain warning, got %s", p.healthCache[gatewayID].Status)
    }
}

func TestGetAllHealthStatuses_ReturnsCopies(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    p.healthCache["g1"] = &HealthStatus{GatewayID: "g1", Status: models.HealthStatusHealthy}
    p.healthCache["g2"] = &HealthStatus{GatewayID: "g2", Status: models.HealthStatusWarning}

    all := p.GetAllHealthStatuses()
    if len(all) != 2 {
        t.Fatalf("expected 2 statuses, got %d", len(all))
    }
    // mutate response
    all[0].Status = models.HealthStatusCritical
    // ensure cache unchanged
    if p.healthCache["g1"].Status != models.HealthStatusHealthy {
        t.Fatalf("expected cache to remain healthy, got %s", p.healthCache["g1"].Status)
    }
}

func TestClearCache(t *testing.T) {
    p := NewHealthProcessor(nil, time.Second)
    p.healthCache["g1"] = &HealthStatus{GatewayID: "g1"}
    p.pendingChecks["g1"] = true

    p.ClearCache()
    if len(p.healthCache) != 0 || len(p.pendingChecks) != 0 {
        t.Fatalf("expected caches cleared, got health=%d pending=%d", len(p.healthCache), len(p.pendingChecks))
    }
}


