package istio

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/aslakknutsen/kkbase/testapp/pkg/dsl/types"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// MeshGenerator generates Istio mesh manifests (VirtualService, DestinationRule)
type MeshGenerator struct {
	spec      *types.AppSpec
	templates *template.Template
}

// NewMeshGenerator creates a new Istio mesh generator
func NewMeshGenerator(spec *types.AppSpec) *MeshGenerator {
	tmpl := template.Must(template.New("istio-mesh").Funcs(funcMap()).ParseFS(templatesFS, "templates/*.tmpl"))
	return &MeshGenerator{
		spec:      spec,
		templates: tmpl,
	}
}

// Name returns the generator name
func (g *MeshGenerator) Name() string {
	return "istio-mesh"
}

// Generate generates Istio mesh manifests
func (g *MeshGenerator) Generate() (map[string]string, error) {
	manifests := make(map[string]string)

	meshProvider := g.spec.App.Providers.Mesh
	if meshProvider != "istio" {
		return manifests, nil
	}

	// Generate VirtualService and DestinationRule for each service
	for _, svc := range g.spec.Services {
		// Check if mesh is enabled for this service
		if !svc.MeshEnabled(meshProvider) {
			continue
		}

		// Get effective mesh config (app defaults + service overrides)
		meshConfig := svc.EffectiveMeshConfig(g.spec.App.MeshDefaults)

		// Generate VirtualService if service has upstreams or traffic splitting
		if len(svc.Upstreams) > 0 || len(meshConfig.TrafficSplit) > 0 {
			vs, err := g.generateVirtualService(svc, meshConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to generate VirtualService for %s: %w", svc.Name, err)
			}
			manifests[fmt.Sprintf("40-mesh/%s-virtualservice.yaml", svc.Name)] = vs
		}

		// Generate DestinationRule for circuit breaking, load balancing, subsets
		dr, err := g.generateDestinationRule(svc, meshConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to generate DestinationRule for %s: %w", svc.Name, err)
		}
		manifests[fmt.Sprintf("40-mesh/%s-destinationrule.yaml", svc.Name)] = dr
	}

	return manifests, nil
}

// virtualServiceData holds data for VirtualService template
type virtualServiceData struct {
	Name            string
	Namespace       string
	AppName         string
	Hosts           []string
	HTTPRoutes      []httpRoute
	Timeout         string
	Retries         *retryPolicy
	HasTrafficSplit bool
	TrafficSplit    []trafficSplit
	DefaultPort     int
}

type httpRoute struct {
	Name        string
	Match       []matchCondition
	Destination destination
	Rewrite     *rewrite
}

type matchCondition struct {
	URIPrefix string
}

type destination struct {
	Host      string
	Namespace string
	Port      int
}

type rewrite struct {
	URI string
}

type retryPolicy struct {
	Attempts      int
	PerTryTimeout string
	RetryOn       string
}

type trafficSplit struct {
	Subset string
	Weight int
}

func (g *MeshGenerator) generateVirtualService(svc types.ServiceConfig, mesh types.MeshConfig) (string, error) {
	// Build host list - service's own FQDN
	host := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)
	hosts := []string{host}

	var httpRoutes []httpRoute
	var trafficSplits []trafficSplit

	// If traffic splitting is configured
	if len(mesh.TrafficSplit) > 0 {
		for _, split := range mesh.TrafficSplit {
			trafficSplits = append(trafficSplits, trafficSplit{
				Subset: split.Subset,
				Weight: split.Weight,
			})
		}
	}
	// Note: VirtualServices route incoming traffic to the service itself.
	// Upstreams are used by the application code for outbound calls, not for mesh routing.

	// Build retry policy
	var retries *retryPolicy
	if mesh.Retries != nil {
		retryOn := mesh.Retries.RetryOn
		if retryOn == "" {
			retryOn = "5xx,reset,connect-failure,refused-stream"
		}
		retries = &retryPolicy{
			Attempts:      mesh.Retries.Attempts,
			PerTryTimeout: mesh.Retries.PerTryTimeout,
			RetryOn:       retryOn,
		}
	}

	// Determine default port based on service protocol configuration
	defaultPort := svc.Ports.HTTP
	if svc.HasHTTP() && svc.HasGRPC() {
		// Dual-protocol service: use unified HTTP port
		defaultPort = svc.Ports.HTTP
	} else if svc.HasGRPC() && !svc.HasHTTP() {
		// gRPC-only service: use gRPC port
		defaultPort = svc.Ports.GRPC
	}

	data := virtualServiceData{
		Name:            svc.Name,
		Namespace:       svc.Namespace,
		AppName:         g.spec.App.Name,
		Hosts:           hosts,
		HTTPRoutes:      httpRoutes,
		Timeout:         mesh.Timeout,
		Retries:         retries,
		HasTrafficSplit: len(trafficSplits) > 0,
		TrafficSplit:    trafficSplits,
		DefaultPort:     defaultPort,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "virtualservice.yaml.tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// destinationRuleData holds data for DestinationRule template
type destinationRuleData struct {
	Name             string
	Namespace        string
	AppName          string
	Host             string
	LoadBalancer     string
	ConnectionPool   *connectionPool
	OutlierDetection *outlierDetection
	TLSMode          string
	Subsets          []subset
}

type connectionPool struct {
	MaxConnections int
	MaxRequests    int
}

type outlierDetection struct {
	ConsecutiveErrors  int
	Interval           string
	BaseEjectionTime   string
	MaxEjectionPercent int
}

type subset struct {
	Name    string
	Version string
	Labels  map[string]string
}

func (g *MeshGenerator) generateDestinationRule(svc types.ServiceConfig, mesh types.MeshConfig) (string, error) {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)

	// Map load balancing strategy
	loadBalancer := mesh.LoadBalancing
	if loadBalancer == "" {
		loadBalancer = "ROUND_ROBIN"
	}

	// Build outlier detection (circuit breaker)
	var outlier *outlierDetection
	if mesh.CircuitBreaker != nil {
		interval := mesh.CircuitBreaker.Interval
		if interval == "" {
			interval = "30s"
		}
		baseEjectionTime := mesh.CircuitBreaker.BaseEjectionTime
		if baseEjectionTime == "" {
			baseEjectionTime = "30s"
		}
		maxEjectionPercent := mesh.CircuitBreaker.MaxEjectionPercent
		if maxEjectionPercent == 0 {
			maxEjectionPercent = 100
		}

		outlier = &outlierDetection{
			ConsecutiveErrors:  mesh.CircuitBreaker.ConsecutiveErrors,
			Interval:           interval,
			BaseEjectionTime:   baseEjectionTime,
			MaxEjectionPercent: maxEjectionPercent,
		}
	}

	// Build TLS mode
	tlsMode := mesh.MTLS
	if tlsMode == "" {
		tlsMode = "ISTIO_MUTUAL" // Default to mutual TLS in Istio
	}

	// Build subsets for traffic splitting
	var subsets []subset
	for _, split := range mesh.TrafficSplit {
		subsets = append(subsets, subset{
			Name:    split.Subset,
			Version: split.Version,
			Labels: map[string]string{
				"version": split.Version,
			},
		})
	}

	data := destinationRuleData{
		Name:             svc.Name,
		Namespace:        svc.Namespace,
		AppName:          g.spec.App.Name,
		Host:             host,
		LoadBalancer:     loadBalancer,
		OutlierDetection: outlier,
		TLSMode:          tlsMode,
		Subsets:          subsets,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "destinationrule.yaml.tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// funcMap returns template helper functions
func funcMap() template.FuncMap {
	return template.FuncMap{
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
	}
}
