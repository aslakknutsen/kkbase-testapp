package service

import (
	"fmt"
	"os"
	"strconv"
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
	Name        string   // Unique ID for this upstream entry (used for behavior targeting)
	URL         string
	Protocol    string   // "http" or "grpc"
	Match       []string // Incoming paths that trigger routing to this upstream (empty = match all)
	Path        string   // Explicit forward path to call on upstream (empty = "/")
	Group       string   // Weighted selection group - upstreams in same group are mutually exclusive
	Probability float64  // Independent call probability (0.0-1.0), only for ungrouped upstreams
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

	// Parse upstreams: id=url:match=/a,/b:path=/forward:group=name|id2=url2
	// Format: id=protocol://host:port[:match=/a,/b][:path=/forward][:group=name]
	// Examples:
	//   - product-api=http://product.ns.svc.cluster.local:8080
	//   - order-api=http://order.ns.svc.cluster.local:8080:match=/orders,/cart
	//   - message-bus=http://message-bus.ns.svc.cluster.local:8080:path=/events/OrderCreated
	//   - gateway=http://gateway:8080:match=/api:path=/v2/api
	//   - payment-ok=http://bus:8080:path=/events/PaymentProcessed:group=payment-outcome
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

			var name, url, path, group string
			var match []string
			var prob float64

			// Check for new format (name=url) vs old format (name:url)
			if strings.Contains(upstream, "=") {
				// New format: id=url[:match=...][:path=...][:group=...][:prob=0.5]
				eqIdx := strings.Index(upstream, "=")
				name = upstream[:eqIdx]
				rest := upstream[eqIdx+1:]

				// Parse URL and optional match/path/group/prob parameters
				// URL format: protocol://host:port
				// Full format: protocol://host:port:match=/a,/b:path=/forward:group=name:prob=0.5
				url, match, path, group, prob = parseUpstreamParams(rest)
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
				Name:        name,
				URL:         url,
				Protocol:    protocol,
				Match:       match,
				Path:        path,
				Group:       group,
				Probability: prob,
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

// parseUpstreamParams parses URL and optional match/path/group/prob from upstream string
// Format: protocol://host:port[:match=/a,/b][:path=/forward][:group=name][:prob=0.5]
func parseUpstreamParams(s string) (url string, match []string, path string, group string, prob float64) {
	// Find where URL ends (after port number)
	// URL format: protocol://host:port
	// We need to find the port, then check for parameters after

	// Find the :// in the protocol
	protoEnd := strings.Index(s, "://")
	if protoEnd == -1 {
		return s, nil, "", "", 0
	}

	// Find the next colon after ://, which should be the port
	afterProto := s[protoEnd+3:]
	portColonIdx := strings.Index(afterProto, ":")
	if portColonIdx == -1 {
		// No port specified, return whole string as URL
		return s, nil, "", "", 0
	}

	// Find where the port number ends
	portStart := protoEnd + 3 + portColonIdx + 1

	// Look for all parameter markers after the port
	paramMarkers := []string{":match=", ":path=", ":group=", ":prob="}
	paramIndices := make(map[string]int)

	for _, marker := range paramMarkers {
		idx := strings.Index(s[portStart:], marker)
		if idx != -1 {
			paramIndices[marker] = idx + portStart
		} else {
			paramIndices[marker] = -1
		}
	}

	// Determine where URL ends (first parameter marker)
	portEnd := len(s)
	for _, idx := range paramIndices {
		if idx != -1 && idx < portEnd {
			portEnd = idx
		}
	}

	url = s[:portEnd]

	// Helper to find end of a parameter value
	findParamEnd := func(start int) int {
		end := len(s)
		for _, idx := range paramIndices {
			if idx > start && idx < end {
				end = idx
			}
		}
		return end
	}

	// Parse match parameter
	if idx := paramIndices[":match="]; idx != -1 {
		start := idx + len(":match=")
		end := findParamEnd(start)
		matchStr := s[start:end]
		for _, p := range strings.Split(matchStr, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				match = append(match, trimmed)
			}
		}
	}

	// Parse path parameter
	if idx := paramIndices[":path="]; idx != -1 {
		start := idx + len(":path=")
		end := findParamEnd(start)
		path = strings.TrimSpace(s[start:end])
	}

	// Parse group parameter
	if idx := paramIndices[":group="]; idx != -1 {
		start := idx + len(":group=")
		end := findParamEnd(start)
		group = strings.TrimSpace(s[start:end])
	}

	// Parse prob parameter
	if idx := paramIndices[":prob="]; idx != -1 {
		start := idx + len(":prob=")
		end := findParamEnd(start)
		probStr := strings.TrimSpace(s[start:end])
		if p, err := strconv.ParseFloat(probStr, 64); err == nil {
			prob = p
		}
	}

	return url, match, path, group, prob
}
