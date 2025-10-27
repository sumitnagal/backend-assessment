package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Port        int
	DatabaseURL string
	LogLevel    string
	JWTSecret   string
	RedisURL    string
    TracingEnabled bool
    JaegerEndpoint string
    ServiceName    string
    Environment    string
    // Rate limiting
    RateLimitRPM int
    RateLimitBurst int
    // Circuit breaker
    CBFailureThreshold int
    CBResetTimeoutSec  int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:        8080,
		DatabaseURL: "postgres://postgres:postgres@127.0.0.1:5432/backend_assessment_test?sslmode=disable",
		LogLevel:    "info",
		JWTSecret:   "secret-key-change-in-production",
		RedisURL:    "redis://127.0.0.1:6379",
        TracingEnabled: false,
        JaegerEndpoint: "http://127.0.0.1:14268/api/traces",
        ServiceName:    "backend-assessment",
        Environment:    "dev",
        RateLimitRPM:   600,
        RateLimitBurst: 50,
        CBFailureThreshold: 5,
        CBResetTimeoutSec:  30,
	}

	// Override with environment variables if present
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.DatabaseURL = dbURL
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		cfg.JWTSecret = jwtSecret
	}

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		cfg.RedisURL = redisURL
	}

    if tracing := os.Getenv("TRACING_ENABLED"); tracing != "" {
        if v, err := strconv.ParseBool(tracing); err == nil {
            cfg.TracingEnabled = v
        }
    }
    if je := os.Getenv("JAEGER_ENDPOINT"); je != "" {
        cfg.JaegerEndpoint = je
    }
    if sn := os.Getenv("SERVICE_NAME"); sn != "" {
        cfg.ServiceName = sn
    }
    if env := os.Getenv("ENVIRONMENT"); env != "" {
        cfg.Environment = env
    }

    if rpm := os.Getenv("RATE_LIMIT_RPM"); rpm != "" {
        if v, err := strconv.Atoi(rpm); err == nil {
            cfg.RateLimitRPM = v
        }
    }
    if rb := os.Getenv("RATE_LIMIT_BURST"); rb != "" {
        if v, err := strconv.Atoi(rb); err == nil {
            cfg.RateLimitBurst = v
        }
    }
    if cbf := os.Getenv("CB_FAILURE_THRESHOLD"); cbf != "" {
        if v, err := strconv.Atoi(cbf); err == nil {
            cfg.CBFailureThreshold = v
        }
    }
    if cbr := os.Getenv("CB_RESET_TIMEOUT_SEC"); cbr != "" {
        if v, err := strconv.Atoi(cbr); err == nil {
            cfg.CBResetTimeoutSec = v
        }
    }

	return cfg, nil
}