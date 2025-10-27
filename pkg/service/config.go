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

	// Parse upstreams: name=url:path1,path2|name2=url2
	// Format: name=protocol://host:port or name=protocol://host:port:path1,path2
	// Examples:
	//   - product-api=http://product.ns.svc.cluster.local:8080
	//   - order-api=http://order.ns.svc.cluster.local:8080:/orders,/cart
	// Old format (backward compat): name:url (no = sign, no colons in URL)
	upstreamsStr := os.Getenv("UPSTREAMS")
	if upstreamsStr != "" {
		// Determine delimiter:
		// - New format (with =): Use | to separate multiple upstreams, allows commas in paths
		// - Old format (with : only): Use , to separate upstreams, no path support
		delimiter := "|"
		isNewFormat := strings.Contains(upstreamsStr, "=")

		if !isNewFormat {
			// Old format backward compatibility: use comma delimiter
			delimiter = ","
		} else if !strings.Contains(upstreamsStr, "|") {
			// New format with single upstream only - no delimiter needed
			delimiter = "\x00" // Use null byte as delimiter (won't be in the string)
		}

		for _, upstream := range strings.Split(upstreamsStr, delimiter) {
			upstream = strings.TrimSpace(upstream)
			if upstream == "" {
				continue
			}

			var name, url string
			var paths []string

			// Check for new format (name=url) vs old format (name:url)
			if strings.Contains(upstream, "=") {
				// New format: name=url or name=url:paths
				eqIdx := strings.Index(upstream, "=")
				name = upstream[:eqIdx]
				rest := upstream[eqIdx+1:]

				// Check if there are paths (colon after port number)
				// URL format: protocol://host:port
				// Paths format: protocol://host:port:/path1,/path2
				// Strategy: Find last colon, check if what follows is a path (starts with /)
				colonIdx := strings.LastIndex(rest, ":")
				if colonIdx > 0 && !strings.HasPrefix(rest[colonIdx:], "://") {
					// Check if this is a path separator (next char is /) or port number (next char is digit)
					afterColon := rest[colonIdx+1:]
					if len(afterColon) > 0 && afterColon[0] == '/' {
						// This is a paths separator
						url = rest[:colonIdx]
						pathsStr := afterColon
						for _, p := range strings.Split(pathsStr, ",") {
							if trimmed := strings.TrimSpace(p); trimmed != "" {
								paths = append(paths, trimmed)
							}
						}
					} else {
						// This is a port number, no paths
						url = rest
					}
				} else {
					// No colon or colon is part of ://, no paths
					url = rest
				}
			} else {
				// Old format: name:url or name:url:paths
				parts := strings.SplitN(upstream, ":", 3)
				if len(parts) < 2 {
					continue
				}
				name = parts[0]
				url = parts[1]
				if len(parts) == 3 && parts[2] != "" {
					for _, p := range strings.Split(parts[2], ",") {
						if trimmed := strings.TrimSpace(p); trimmed != "" {
							paths = append(paths, trimmed)
						}
					}
				}
			}

			// Skip malformed entries
			if name == "" || url == "" {
				continue
			}

			protocol := "http"
			if strings.HasPrefix(url, "grpc://") {
				protocol = "grpc"
			}

			cfg.Upstreams[name] = &UpstreamConfig{
				Name:     name,
				URL:      url,
				Protocol: protocol,
				Paths:    paths,
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
