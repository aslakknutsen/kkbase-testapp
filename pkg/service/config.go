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
	Protocol string   // "http" or "grpc"
	Paths    []string // Path prefixes this upstream handles (empty = match all)
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

	// Parse upstreams: name:url:path1,path2|name2:url2
	// Old format (without paths): product-api:http://product:8080,cart-api:grpc://cart:9090
	// New format (with paths): order-api:http://order:8080:/orders,/cart|product-api:http://product:8080:/products
	upstreamsStr := os.Getenv("UPSTREAMS")
	if upstreamsStr != "" {
		// Use | as delimiter to support commas in path lists
		delimiter := "|"
		if !strings.Contains(upstreamsStr, "|") {
			// Backward compatibility: if no | found, use comma
			delimiter = ","
		}

		for _, upstream := range strings.Split(upstreamsStr, delimiter) {
			parts := strings.SplitN(strings.TrimSpace(upstream), ":", 3)
			if len(parts) >= 2 {
				name := parts[0]
				url := parts[1]
				protocol := "http"
				if strings.HasPrefix(url, "grpc://") {
					protocol = "grpc"
				}

				var paths []string
				if len(parts) == 3 && parts[2] != "" {
					// Split paths by comma
					for _, p := range strings.Split(parts[2], ",") {
						if trimmed := strings.TrimSpace(p); trimmed != "" {
							paths = append(paths, trimmed)
						}
					}
				}

				cfg.Upstreams[name] = &UpstreamConfig{
					Name:     name,
					URL:      url,
					Protocol: protocol,
					Paths:    paths,
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
