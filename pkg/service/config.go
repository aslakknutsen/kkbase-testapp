package service

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds the service configuration
type Config struct {
	// Service identification
	Name      string
	Version   string
	Namespace string
	PodName   string
	NodeName  string

	// Server ports
	HTTPPort    int
	GRPCPort    int
	MetricsPort int

	// Upstream services
	Upstreams map[string]*UpstreamConfig

	// Default behavior
	DefaultBehavior string

	// Observability
	OTELEndpoint string
	LogLevel     string

	// Client settings
	ClientTimeout time.Duration
}

// UpstreamConfig defines an upstream service
type UpstreamConfig struct {
	Name     string
	URL      string
	Protocol string // "http" or "grpc"
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() *Config {
	cfg := &Config{
		Name:            getEnv("SERVICE_NAME", "testservice"),
		Version:         getEnv("SERVICE_VERSION", "1.0.0"),
		Namespace:       getEnv("NAMESPACE", os.Getenv("POD_NAMESPACE")),
		PodName:         getEnv("POD_NAME", os.Getenv("HOSTNAME")),
		NodeName:        getEnv("NODE_NAME", ""),
		HTTPPort:        getEnvInt("HTTP_PORT", 8080),
		GRPCPort:        getEnvInt("GRPC_PORT", 9090),
		MetricsPort:     getEnvInt("METRICS_PORT", 9091),
		DefaultBehavior: getEnv("DEFAULT_BEHAVIOR", ""),
		OTELEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		ClientTimeout:   time.Duration(getEnvInt("CLIENT_TIMEOUT_MS", 30000)) * time.Millisecond,
		Upstreams:       make(map[string]*UpstreamConfig),
	}

	// Parse upstreams: product-api:http://product:8080,cart-api:grpc://cart:9090
	upstreamsStr := os.Getenv("UPSTREAMS")
	if upstreamsStr != "" {
		for _, upstream := range strings.Split(upstreamsStr, ",") {
			parts := strings.SplitN(strings.TrimSpace(upstream), ":", 2)
			if len(parts) == 2 {
				name := parts[0]
				url := parts[1]
				protocol := "http"
				if strings.HasPrefix(url, "grpc://") {
					protocol = "grpc"
				}
				cfg.Upstreams[name] = &UpstreamConfig{
					Name:     name,
					URL:      url,
					Protocol: protocol,
				}
			}
		}
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
