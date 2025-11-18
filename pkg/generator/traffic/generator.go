package traffic

import (
	"bytes"
	"embed"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/dsl/types"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Generator generates traffic generation manifests
type Generator struct {
	spec           *types.AppSpec
	templates      *template.Template
	currentTraffic *types.TrafficConfig
}

// Template data structures
type trafficJobData struct {
	Name            string
	Namespace       string
	TargetURL       string
	Pattern         string
	Rate            string
	Duration        string
	RateNumeric     int
	DurationSeconds int
	WrapperScript   string
	Paths           []string
	PathPattern     string
	Behavior        string
}

// NewGenerator creates a new traffic generator
func NewGenerator(spec *types.AppSpec) *Generator {
	// Parse templates with custom functions
	tmpl := template.Must(template.New("traffic").Funcs(template.FuncMap{
		"indent": func(spaces int, s string) string {
			indent := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = indent + line
				}
			}
			return strings.Join(lines, "\n")
		},
	}).ParseFS(templatesFS, "templates/*.tmpl"))

	return &Generator{
		spec:      spec,
		templates: tmpl,
	}
}

// GenerateAll generates all traffic generation manifests
func (g *Generator) GenerateAll() (map[string]string, error) {
	manifests := make(map[string]string)

	if len(g.spec.Traffic) == 0 {
		return manifests, nil
	}

	for _, traffic := range g.spec.Traffic {
		manifest, err := g.generateTrafficJob(&traffic)
		if err != nil {
			return nil, fmt.Errorf("failed to generate traffic job for %s: %w", traffic.Name, err)
		}
		manifests[fmt.Sprintf("30-traffic/%s-job.yaml", traffic.Name)] = manifest
	}

	return manifests, nil
}

// generateTrafficJob generates a single traffic Job manifest
func (g *Generator) generateTrafficJob(traffic *types.TrafficConfig) (string, error) {
	// Find target service
	targetService := g.findService(traffic.Target)
	if targetService == nil {
		return "", fmt.Errorf("target service %s not found", traffic.Target)
	}

	// Determine protocol and port
	protocol := "http"
	port := 8080
	if targetService.HasHTTP() {
		protocol = "http"
		port = targetService.Ports.HTTP
		if port == 0 {
			port = 8080
		}
	} else if targetService.HasGRPC() {
		protocol = "grpc"
		port = targetService.Ports.GRPC
		if port == 0 {
			port = 9090
		}
	}

	// Construct target URL
	targetURL := fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d",
		protocol, targetService.Name, targetService.Namespace, port)

	// Parse rate (e.g., "100/s" -> 100)
	rateNumeric := parseRate(traffic.Rate)

	// Parse duration (e.g., "1h" -> 3600)
	durationSeconds := parseDuration(traffic.Duration)

	// Determine namespace for traffic job (use target service namespace)
	namespace := targetService.Namespace

	// Set default path pattern if not specified
	pathPattern := traffic.PathPattern
	if pathPattern == "" && len(traffic.Paths) > 0 {
		pathPattern = "round-robin"
	}

	// Store current traffic for script generation
	g.currentTraffic = traffic

	// Generate wrapper script based on pattern
	wrapperScript := g.generateWrapperScript(traffic, rateNumeric, durationSeconds, targetURL)

	data := trafficJobData{
		Name:            traffic.Name,
		Namespace:       namespace,
		TargetURL:       targetURL,
		Pattern:         traffic.Pattern,
		Rate:            traffic.Rate,
		Duration:        traffic.Duration,
		RateNumeric:     rateNumeric,
		DurationSeconds: durationSeconds,
		WrapperScript:   wrapperScript,
		Paths:           traffic.Paths,
		PathPattern:     pathPattern,
		Behavior:        traffic.Behavior,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "traffic-job.yaml.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateWrapperScript creates the bash script based on pattern
func (g *Generator) generateWrapperScript(traffic *types.TrafficConfig, rate, duration int, targetURL string) string {
	pattern := traffic.Pattern
	if pattern == "" {
		pattern = "steady"
	}

	// Append behavior query param if specified
	url := targetURL
	if traffic.Behavior != "" {
		url = fmt.Sprintf("%s?behavior=%s", targetURL, traffic.Behavior)
	}

	switch pattern {
	case "steady":
		return g.generateSteadyScript(rate, duration, url)
	case "spiky":
		return g.generateSpikyScript(rate, duration, url)
	case "diurnal":
		return g.generateDiurnalScript(rate, duration, url)
	default:
		return g.generateSteadyScript(rate, duration, url)
	}
}

// generateSteadyScript generates a steady traffic pattern
func (g *Generator) generateSteadyScript(rate, duration int, targetURL string) string {
	durationStr := fmt.Sprintf("%ds", duration)
	if duration == 0 {
		durationStr = "0" // Run indefinitely
	}

	if g.currentTraffic != nil && len(g.currentTraffic.Paths) > 0 {
		return g.generateMultiPathScript(rate, duration, targetURL, g.currentTraffic.Paths, g.currentTraffic.PathPattern, "steady")
	}

	return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting steady traffic generation"
echo "Target: %s"
echo "Rate: %d qps"
echo "Duration: %s"

fortio load -qps %d -t %s -c 8 %s
`, targetURL, rate, durationStr, rate, durationStr, targetURL)
}

// generateSpikyScript generates a spiky traffic pattern
func (g *Generator) generateSpikyScript(rate, duration int, targetURL string) string {
	if g.currentTraffic != nil && len(g.currentTraffic.Paths) > 0 {
		return g.generateMultiPathScript(rate, duration, targetURL, g.currentTraffic.Paths, g.currentTraffic.PathPattern, "spiky")
	}

	highRate := int(float64(rate) * 3.0) // 3x spike
	lowRate := int(float64(rate) * 0.2)  // 20% baseline
	burstDuration := 5                   // 5 second bursts
	pauseDuration := 25                  // 25 second pauses

	return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting spiky traffic generation"
echo "Target: %s"
echo "High rate: %d qps (burst), Low rate: %d qps (baseline)"
echo "Pattern: %ds burst every %ds"
echo "Duration: %ds"

END_TIME=$(($(date +%%s) + %d))

while [ $(date +%%s) -lt $END_TIME ]; do
    echo "$(date): Burst phase - %d qps for %ds"
    timeout %ds fortio load -qps %d -c 8 %s || true
    
    REMAINING=$((END_TIME - $(date +%%s)))
    if [ $REMAINING -le 0 ]; then
        break
    fi
    
    PAUSE_TIME=%d
    if [ $REMAINING -lt $PAUSE_TIME ]; then
        PAUSE_TIME=$REMAINING
    fi
    
    echo "$(date): Baseline phase - %d qps for ${PAUSE_TIME}s"
    timeout ${PAUSE_TIME}s fortio load -qps %d -c 2 %s || true
done

echo "$(date): Spiky traffic complete"
`, targetURL, highRate, lowRate, burstDuration, burstDuration+pauseDuration, duration,
		duration, highRate, burstDuration, burstDuration, highRate, targetURL,
		pauseDuration, lowRate, lowRate, targetURL)
}

// generateDiurnalScript generates a diurnal (daily cycle) traffic pattern
func (g *Generator) generateDiurnalScript(rate, duration int, targetURL string) string {
	if g.currentTraffic != nil && len(g.currentTraffic.Paths) > 0 {
		return g.generateMultiPathScript(rate, duration, targetURL, g.currentTraffic.Paths, g.currentTraffic.PathPattern, "diurnal")
	}

	// Sample every 5 minutes
	sampleInterval := 300

	return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting diurnal traffic generation"
echo "Target: %s"
echo "Base rate: %d qps"
echo "Duration: %ds"
echo "Pattern: 24-hour sine wave"

BASE_RATE=%d
DURATION=%d
SAMPLE_INTERVAL=%d
END_TIME=$(($(date +%%s) + DURATION))

while [ $(date +%%s) -lt $END_TIME ]; do
    # Calculate current time in seconds since midnight
    CURRENT_HOUR=$(date +%%H)
    CURRENT_MIN=$(date +%%M)
    SECONDS_SINCE_MIDNIGHT=$((CURRENT_HOUR * 3600 + CURRENT_MIN * 60))
    
    # Calculate sine wave position (peak at hour 12, trough at hour 0/24)
    # Using basic integer math approximation
    # sin(2π * t/86400) where t is seconds since midnight
    # Approximation: multiply by 100 for precision, divide later
    ANGLE=$((SECONDS_SINCE_MIDNIGHT * 628 / 86400))  # 628 ≈ 2π * 100
    
    # Rough sine approximation using case statement for common values
    # Maps to range [0.3, 1.7] where 1.0 is baseline
    if [ $CURRENT_HOUR -ge 9 ] && [ $CURRENT_HOUR -le 17 ]; then
        # Daytime hours (9 AM - 5 PM): 1.3x - 1.7x rate
        MULTIPLIER=160
    elif [ $CURRENT_HOUR -ge 6 ] && [ $CURRENT_HOUR -le 20 ]; then
        # Morning/Evening (6-9 AM, 5-8 PM): 1.0x - 1.3x rate
        MULTIPLIER=120
    else
        # Night (8 PM - 6 AM): 0.3x - 0.7x rate
        MULTIPLIER=50
    fi
    
    CURRENT_RATE=$((BASE_RATE * MULTIPLIER / 100))
    
    REMAINING=$((END_TIME - $(date +%%s)))
    if [ $REMAINING -le 0 ]; then
        break
    fi
    
    INTERVAL=$SAMPLE_INTERVAL
    if [ $REMAINING -lt $INTERVAL ]; then
        INTERVAL=$REMAINING
    fi
    
    echo "$(date): Rate ${CURRENT_RATE} qps for ${INTERVAL}s (hour: $CURRENT_HOUR, multiplier: ${MULTIPLIER}%%)"
    timeout ${INTERVAL}s fortio load -qps $CURRENT_RATE -c 8 %s || true
done

echo "$(date): Diurnal traffic complete"
`, targetURL, rate, duration, rate, duration, sampleInterval, targetURL)
}

// generateMultiPathScript generates a script that distributes traffic across multiple paths
func (g *Generator) generateMultiPathScript(rate, duration int, baseURL string, paths []string, pathPattern, trafficPattern string) string {
	// Extract behavior query param if present
	behaviorParam := ""
	if strings.Contains(baseURL, "?behavior=") {
		parts := strings.SplitN(baseURL, "?", 2)
		baseURL = parts[0]
		behaviorParam = "?" + parts[1]
	}
	durationStr := fmt.Sprintf("%ds", duration)
	if duration == 0 {
		durationStr = "0"
	}

	// Build paths list for script
	pathsList := ""
	for _, path := range paths {
		pathsList += fmt.Sprintf("  \"%s\"\n", path)
	}

	switch pathPattern {
	case "random":
		return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting %s traffic generation with random path selection"
echo "Base URL: %s"
echo "Paths: %d"
echo "Rate: %d qps total"
echo "Duration: %s"

# Define paths
PATHS="
%s"

END_TIME=$(($(date +%%s) + %d))

while [ $(date +%%s) -lt $END_TIME ]; do
    # Pick random path
    PATH_COUNT=$(echo "$PATHS" | wc -l)
    RANDOM_INDEX=$(($(od -An -N2 -i /dev/urandom) %% PATH_COUNT + 1))
    SELECTED_PATH=$(echo "$PATHS" | sed -n "${RANDOM_INDEX}p" | tr -d ' "')
    
    FULL_URL="%s${SELECTED_PATH}%s"
    
    REMAINING=$((END_TIME - $(date +%%s)))
    if [ $REMAINING -le 0 ]; then
        break
    fi
    
    INTERVAL=5
    if [ $REMAINING -lt $INTERVAL ]; then
        INTERVAL=$REMAINING
    fi
    
    echo "$(date): Calling $SELECTED_PATH at %d qps for ${INTERVAL}s"
    timeout ${INTERVAL}s fortio load -qps %d -c 4 "$FULL_URL" || true
done

echo "$(date): Multi-path traffic complete"
`, trafficPattern, baseURL, len(paths), rate, durationStr, pathsList, duration, baseURL, behaviorParam, rate, rate)

	case "sequential":
		return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting %s traffic generation with sequential path pattern"
echo "Base URL: %s"
echo "Paths: %d"
echo "Rate: %d qps total"
echo "Duration: %s"

# Define paths
PATHS="
%s"

PATH_ARRAY=$(echo "$PATHS" | tr -d ' "')
PATH_COUNT=$(echo "$PATH_ARRAY" | wc -l)

END_TIME=$(($(date +%%s) + %d))
PATH_INDEX=1

while [ $(date +%%s) -lt $END_TIME ]; do
    SELECTED_PATH=$(echo "$PATH_ARRAY" | sed -n "${PATH_INDEX}p")
    FULL_URL="%s${SELECTED_PATH}%s"
    
    REMAINING=$((END_TIME - $(date +%%s)))
    if [ $REMAINING -le 0 ]; then
        break
    fi
    
    INTERVAL=5
    if [ $REMAINING -lt $INTERVAL ]; then
        INTERVAL=$REMAINING
    fi
    
    echo "$(date): Calling $SELECTED_PATH at %d qps for ${INTERVAL}s"
    timeout ${INTERVAL}s fortio load -qps %d -c 4 "$FULL_URL" || true
    
    # Move to next path
    PATH_INDEX=$((PATH_INDEX + 1))
    if [ $PATH_INDEX -gt $PATH_COUNT ]; then
        PATH_INDEX=1
    fi
done

echo "$(date): Multi-path traffic complete"
`, trafficPattern, baseURL, len(paths), rate, durationStr, pathsList, duration, baseURL, behaviorParam, rate, rate)

	default: // round-robin
		return fmt.Sprintf(`#!/bin/sh
set -e

echo "Starting %s traffic generation with round-robin path pattern"
echo "Base URL: %s"
echo "Paths: %d"
echo "Rate: %d qps total"
echo "Duration: %s"

# Define paths
PATHS="
%s"

PATH_ARRAY=$(echo "$PATHS" | tr -d ' "')
PATH_COUNT=$(echo "$PATH_ARRAY" | wc -l)
RATE_PER_PATH=$((%d / PATH_COUNT))

if [ $RATE_PER_PATH -lt 1 ]; then
    RATE_PER_PATH=1
fi

echo "Rate per path: ${RATE_PER_PATH} qps"

# Build fortio command with all paths
FORTIO_CMD="fortio load -qps $RATE_PER_PATH -t %s -c 2"
for path in $PATH_ARRAY; do
    FULL_URL="%s${path}%s"
    echo "  Adding path: $path"
    FORTIO_CMD="$FORTIO_CMD $FULL_URL &"
done

echo "Starting parallel load generation..."
eval $FORTIO_CMD
wait

echo "$(date): Multi-path traffic complete"
`, trafficPattern, baseURL, len(paths), rate, durationStr, pathsList, rate, durationStr, baseURL, behaviorParam)
	}
}

// findService finds a service by name in the spec
func (g *Generator) findService(name string) *types.ServiceConfig {
	for i := range g.spec.Services {
		if g.spec.Services[i].Name == name {
			return &g.spec.Services[i]
		}
	}
	return nil
}

// parseRate extracts numeric rate from string like "100/s"
func parseRate(rate string) int {
	if rate == "" {
		return 100 // Default rate
	}

	// Remove "/s" or "/sec" suffix
	rate = strings.TrimSuffix(rate, "/s")
	rate = strings.TrimSuffix(rate, "/sec")
	rate = strings.TrimSpace(rate)

	val, err := strconv.Atoi(rate)
	if err != nil {
		return 100 // Default on parse error
	}
	return val
}

// parseDuration converts duration string to seconds
func parseDuration(durationStr string) int {
	if durationStr == "" {
		return 0 // Infinite
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 3600 // Default 1 hour on parse error
	}

	seconds := int(math.Round(duration.Seconds()))
	return seconds
}
