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

	// Upstream services (slice to support multiple entries with same name)
	Upstreams []*UpstreamConfig

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
	Match    []string // Incoming paths that trigger routing to this upstream (empty = match all)
	Path     string   // Explicit forward path to call on upstream (empty = "/")
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
		GRPCPort:        getEnvInt("GRPC_PORT", 8080),
		MetricsPort:     getEnvInt("METRICS_PORT", 9091),
		DefaultBehavior: getEnv("DEFAULT_BEHAVIOR", ""),
		OTELEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		ClientTimeout:   time.Duration(getEnvInt("CLIENT_TIMEOUT_MS", 30000)) * time.Millisecond,
		Upstreams:       []*UpstreamConfig{},
	}

	// Parse upstreams: name=url:match=/a,/b:path=/forward|name2=url2
	// Format: name=protocol://host:port[:match=/a,/b][:path=/forward]
	// Examples:
	//   - product-api=http://product.ns.svc.cluster.local:8080
	//   - order-api=http://order.ns.svc.cluster.local:8080:match=/orders,/cart
	//   - message-bus=http://message-bus.ns.svc.cluster.local:8080:path=/events/OrderCreated
	//   - gateway=http://gateway:8080:match=/api:path=/v2/api
	// Old format (backward compat): name:url (no = sign)
	upstreamsStr := os.Getenv("UPSTREAMS")
	if upstreamsStr != "" {
		// Determine delimiter:
		// - New format (with =): Use | to separate multiple upstreams
		// - Old format (with : only): Use , to separate upstreams
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

			var name, url, path string
			var match []string

			// Check for new format (name=url) vs old format (name:url)
			if strings.Contains(upstream, "=") {
				// New format: name=url[:match=...][:path=...]
				eqIdx := strings.Index(upstream, "=")
				name = upstream[:eqIdx]
				rest := upstream[eqIdx+1:]

				// Parse URL and optional match/path parameters
				// URL format: protocol://host:port
				// Full format: protocol://host:port:match=/a,/b:path=/forward
				url, match, path = parseUpstreamParams(rest)
			} else {
				// Old format: name:url
				parts := strings.SplitN(upstream, ":", 2)
				if len(parts) < 2 {
					continue
				}
				name = parts[0]
				url = parts[1]
			}

			// Skip malformed entries
			if name == "" || url == "" {
				continue
			}

			protocol := "http"
			if strings.HasPrefix(url, "grpc://") {
				protocol = "grpc"
			}

			cfg.Upstreams = append(cfg.Upstreams, &UpstreamConfig{
				Name:     name,
				URL:      url,
				Protocol: protocol,
				Match:    match,
				Path:     path,
			})
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

// parseUpstreamParams parses URL and optional match/path from upstream string
// Format: protocol://host:port[:match=/a,/b][:path=/forward]
func parseUpstreamParams(s string) (url string, match []string, path string) {
	// Find where URL ends (after port number)
	// URL format: protocol://host:port
	// We need to find the port, then check for :match= or :path= after

	// Find the :// in the protocol
	protoEnd := strings.Index(s, "://")
	if protoEnd == -1 {
		return s, nil, ""
	}

	// Find the next colon after ://, which should be the port
	afterProto := s[protoEnd+3:]
	portColonIdx := strings.Index(afterProto, ":")
	if portColonIdx == -1 {
		// No port specified, return whole string as URL
		return s, nil, ""
	}

	// Find where the port number ends (either at :match=, :path=, or end of string)
	portStart := protoEnd + 3 + portColonIdx + 1
	portEnd := len(s)

	// Look for :match= or :path= after the port
	matchIdx := strings.Index(s[portStart:], ":match=")
	pathIdx := strings.Index(s[portStart:], ":path=")

	if matchIdx != -1 {
		matchIdx += portStart
	}
	if pathIdx != -1 {
		pathIdx += portStart
	}

	// Determine where URL ends
	if matchIdx != -1 && (pathIdx == -1 || matchIdx < pathIdx) {
		portEnd = matchIdx
	} else if pathIdx != -1 {
		portEnd = pathIdx
	}

	url = s[:portEnd]

	// Parse match parameter
	if matchIdx != -1 {
		matchStart := matchIdx + len(":match=")
		matchEnd := len(s)
		if pathIdx != -1 && pathIdx > matchIdx {
			matchEnd = pathIdx
		}
		matchStr := s[matchStart:matchEnd]
		for _, p := range strings.Split(matchStr, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				match = append(match, trimmed)
			}
		}
	}

	// Parse path parameter
	if pathIdx != -1 {
		pathStart := pathIdx + len(":path=")
		pathEnd := len(s)
		if matchIdx != -1 && matchIdx > pathIdx {
			pathEnd = matchIdx
		}
		path = strings.TrimSpace(s[pathStart:pathEnd])
	}

	return url, match, path
}
