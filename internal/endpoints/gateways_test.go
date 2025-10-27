package endpoints

import (
    "fmt"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"

    "backend-assessment/internal/models"
)

func TestListGateways_Unauthorized(t *testing.T) {
    h := NewGatewayHandler(nil)
    req := httptest.NewRequest(http.MethodGet, "/v1/gateways", nil)
    // no X-User-ID header
    rr := httptest.NewRecorder()

    http.HandlerFunc(h.ListGateways).ServeHTTP(rr, req)

    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rr.Code)
    }
}

func TestListGateways_ReturnsCached(t *testing.T) {
    h := NewGatewayHandler(nil)

    // Prepare cached value under expected cache key
    // key format: gateways_<search>
    search := "abc"
    cacheKey := "gateways_" + search

    expected := []models.Gateway{{
        ID:           "30000000-0000-0000-0000-000000000001",
        Serial:       "GW-HQ-001",
        OrganizationID: "00000000-0000-0000-0000-000000000001",
        SiteID:       "20000000-0000-0000-0000-000000000001",
        Name:         "HQ Gateway 1",
        HealthStatus: models.HealthStatusHealthy,
    }}

    gatewayCacheMu.Lock()
    gatewayCache = make(map[string][]models.Gateway)
    gatewayCache[cacheKey] = expected
    gatewayCacheMu.Unlock()

    req := httptest.NewRequest(http.MethodGet, "/v1/gateways?search="+search, nil)
    req.Header.Set("X-User-ID", "test-user")
    rr := httptest.NewRecorder()

    http.HandlerFunc(h.ListGateways).ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
        t.Fatalf("expected application/json content type, got %q", ct)
    }

    var got []models.Gateway
    if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    if len(got) != 1 || got[0].ID != expected[0].ID || got[0].Serial != expected[0].Serial {
        t.Fatalf("unexpected response: %+v", got)
    }
}

func TestListGateways_ConcurrentCachedReads_NoPanic(t *testing.T) {
    h := NewGatewayHandler(nil)

    // Seed cache
    cacheKey := "gateways_"
    gateways := make([]models.Gateway, 50)
    for i := range gateways {
        gateways[i] = models.Gateway{ID: "gw-"}
    }
    gatewayCacheMu.Lock()
    gatewayCache = make(map[string][]models.Gateway)
    gatewayCache[cacheKey] = gateways
    gatewayCacheMu.Unlock()

    var wg sync.WaitGroup
    num := 100
    wg.Add(num)
    errs := make(chan error, num)

    for i := 0; i < num; i++ {
        go func() {
            defer wg.Done()
            req := httptest.NewRequest(http.MethodGet, "/v1/gateways", nil)
            req.Header.Set("X-User-ID", "load-user")
            rr := httptest.NewRecorder()
            http.HandlerFunc(h.ListGateways).ServeHTTP(rr, req)
            if rr.Code != http.StatusOK {
                errs <- fmt.Errorf("unexpected status: %d", rr.Code)
            }
        }()
    }

    wg.Wait()
    close(errs)
    for err := range errs {
        if err != nil {
            t.Fatalf("request error: %v", err)
        }
    }
}

func TestGetGateway_Unauthorized(t *testing.T) {
    h := NewGatewayHandler(nil)
    req := httptest.NewRequest(http.MethodGet, "/v1/gateways/some-id", nil)
    rr := httptest.NewRecorder()
    http.HandlerFunc(h.GetGateway).ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rr.Code)
    }
}

func TestUpdateGateway_Unauthorized(t *testing.T) {
    h := NewGatewayHandler(nil)
    req := httptest.NewRequest(http.MethodPut, "/v1/gateways/some-id", strings.NewReader(`{"name":"x"}`))
    rr := httptest.NewRecorder()
    http.HandlerFunc(h.UpdateGateway).ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rr.Code)
    }
}

func TestRebootGateway_Unauthorized(t *testing.T) {
    h := NewGatewayHandler(nil)
    req := httptest.NewRequest(http.MethodPost, "/v1/gateways/some-id/reboot", nil)
    rr := httptest.NewRecorder()
    http.HandlerFunc(h.RebootGateway).ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rr.Code)
    }
}


