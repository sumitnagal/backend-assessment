package api

import (
	"net/http"
    "sync"
    "time"
    "expvar"

	"backend-assessment/internal/config"
	"backend-assessment/internal/datastore"
	"backend-assessment/internal/endpoints"

	"github.com/gorilla/mux"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server represents the API server
type Server struct {
	config *config.Config
	db     *datastore.PostgresDB
    // rate limiting state
    rlMu   sync.Mutex
    tokens map[string]*tokenBucket
    rlDropped *expvar.Int
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, db *datastore.PostgresDB) *Server {
	return &Server{
		config: cfg,
        db:     db,
        tokens: make(map[string]*tokenBucket),
        rlDropped: expvar.NewInt("rate_limit_dropped"),
	}
}

// Router returns the configured HTTP router
func (s *Server) Router() *mux.Router {
    r := mux.NewRouter()
    // Expose expvar metrics at /debug/vars
    r.Handle("/debug/vars", http.DefaultServeMux)

	// Health check endpoint
    r.Handle("/health", otelhttp.NewHandler(http.HandlerFunc(s.healthCheck), "health")).Methods("GET")

	// API v1 routes
    api := r.PathPrefix("/v1").Subrouter()

	// Gateway endpoints
	gatewayHandler := endpoints.NewGatewayHandler(s.db)
    api.Handle("/gateways", otelhttp.NewHandler(http.HandlerFunc(gatewayHandler.ListGateways), "list_gateways")).Methods("GET")
    api.Handle("/gateways/{id}", otelhttp.NewHandler(http.HandlerFunc(gatewayHandler.GetGateway), "get_gateway")).Methods("GET")
    api.Handle("/gateways/{id}", otelhttp.NewHandler(http.HandlerFunc(gatewayHandler.UpdateGateway), "update_gateway")).Methods("PUT")
    api.Handle("/gateways/{id}/reboot", otelhttp.NewHandler(s.withRateLimit(http.HandlerFunc(gatewayHandler.RebootGateway)), "reboot_gateway")).Methods("POST")

	// User endpoints
	userHandler := endpoints.NewUserHandler(s.db)
    api.Handle("/users", otelhttp.NewHandler(http.HandlerFunc(userHandler.ListUsers), "list_users")).Methods("GET")
    api.Handle("/users/{id}", otelhttp.NewHandler(http.HandlerFunc(userHandler.GetUser), "get_user")).Methods("GET")

	// Organization endpoints
	orgHandler := endpoints.NewOrganizationHandler(s.db)
    api.Handle("/organizations", otelhttp.NewHandler(http.HandlerFunc(orgHandler.ListOrganizations), "list_organizations")).Methods("GET")

	return r
}

// tokenBucket implements a simple token bucket per key
type tokenBucket struct {
    capacity int
    tokens   float64
    fillRate float64 // tokens per second
    lastFill time.Time
}

func (b *tokenBucket) allow() bool {
    now := time.Now()
    elapsed := now.Sub(b.lastFill).Seconds()
    b.tokens = minFloat(float64(b.capacity), b.tokens+elapsed*b.fillRate)
    b.lastFill = now
    if b.tokens >= 1 {
        b.tokens -= 1
        return true
    }
    return false
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID := r.Header.Get("X-User-ID")
        if userID == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        key := userID
        s.rlMu.Lock()
        b, ok := s.tokens[key]
        if !ok {
            rpm := s.config.RateLimitRPM
            burst := s.config.RateLimitBurst
            b = &tokenBucket{
                capacity: burst,
                tokens:   float64(burst),
                fillRate: float64(rpm) / 60.0,
                lastFill: time.Now(),
            }
            s.tokens[key] = b
        }
        allowed := b.allow()
        s.rlMu.Unlock()
        if !allowed {
            s.rlDropped.Add(1)
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func minFloat(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}