package types

// AppSpec defines the complete application specification
type AppSpec struct {
	App       AppConfig        `yaml:"app"`
	Services  []ServiceConfig  `yaml:"services"`
	Traffic   []TrafficConfig  `yaml:"traffic,omitempty"`
	Scenarios []ScenarioConfig `yaml:"scenarios,omitempty"`
}

// AppConfig defines application-level configuration
type AppConfig struct {
	Name         string         `yaml:"name"`
	Namespaces   []string       `yaml:"namespaces,omitempty"`
	Providers    ProviderConfig `yaml:"providers,omitempty"`
	MeshDefaults MeshConfig     `yaml:"meshDefaults,omitempty"`
}

// ProviderConfig defines which providers to use for ingress and mesh
type ProviderConfig struct {
	Ingress string `yaml:"ingress,omitempty"` // gateway-api, istio-gateway, k8s-ingress, openshift-routes, none
	Mesh    string `yaml:"mesh,omitempty"`    // istio, linkerd, gateway-api-mesh, none
}

// UpstreamRoute defines an upstream service with optional path-based routing
type UpstreamRoute struct {
	Name  string   `yaml:"name"`
	Match []string `yaml:"match,omitempty"` // Incoming paths that trigger routing to this upstream (HTTP callers only)
	Path  string   `yaml:"path,omitempty"`  // Explicit forward path to call on upstream (HTTP upstreams only), defaults to "/"
}

// ServiceConfig defines a service
type ServiceConfig struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Replicas    int               `yaml:"replicas,omitempty"`
	Type        string            `yaml:"type,omitempty"` // Deployment, StatefulSet, DaemonSet
	Protocols   []string          `yaml:"protocols,omitempty"`
	Ports       PortsConfig       `yaml:"ports,omitempty"`
	Upstreams   []UpstreamRoute   `yaml:"upstreams,omitempty"`
	Behavior    BehaviorConfig    `yaml:"behavior,omitempty"`
	Storage     StorageConfig     `yaml:"storage,omitempty"`
	Ingress     IngressConfig     `yaml:"ingress,omitempty"`
	Mesh        MeshConfig        `yaml:"mesh,omitempty"`
	Resources   ResourceConfig    `yaml:"resources,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// PortsConfig defines service ports
type PortsConfig struct {
	HTTP    int `yaml:"http,omitempty"`
	GRPC    int `yaml:"grpc,omitempty"`
	Metrics int `yaml:"metrics,omitempty"`
}

// BehaviorConfig defines default behavior for a service
type BehaviorConfig struct {
	Latency   string  `yaml:"latency,omitempty"`
	ErrorRate float64 `yaml:"errorRate,omitempty"`
	CPU       string  `yaml:"cpu,omitempty"`
	Memory    string  `yaml:"memory,omitempty"`
}

// StorageConfig defines storage requirements
type StorageConfig struct {
	Size         string `yaml:"size,omitempty"`
	StorageClass string `yaml:"storageClass,omitempty"`
}

// IngressConfig defines ingress settings
type IngressConfig struct {
	Enabled bool     `yaml:"enabled"`
	Host    string   `yaml:"host,omitempty"`
	TLS     bool     `yaml:"tls,omitempty"`
	Paths   []string `yaml:"paths,omitempty"`
}

// MeshConfig defines service mesh configuration
type MeshConfig struct {
	Enabled        *bool                 `yaml:"enabled,omitempty"` // pointer so we can distinguish unset from false
	Timeout        string                `yaml:"timeout,omitempty"` // e.g., "5s"
	Retries        *RetryConfig          `yaml:"retries,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuitBreaker,omitempty"`
	LoadBalancing  string                `yaml:"loadBalancing,omitempty"` // ROUND_ROBIN, LEAST_REQUEST, RANDOM, PASSTHROUGH
	TrafficSplit   []TrafficSplitConfig  `yaml:"trafficSplit,omitempty"`
	MTLS           string                `yaml:"mtls,omitempty"` // STRICT, PERMISSIVE, DISABLE
}

// RetryConfig defines retry policy
type RetryConfig struct {
	Attempts      int    `yaml:"attempts,omitempty"`
	PerTryTimeout string `yaml:"perTryTimeout,omitempty"`
	RetryOn       string `yaml:"retryOn,omitempty"` // e.g., "5xx,timeout,reset"
}

// CircuitBreakerConfig defines circuit breaker settings
type CircuitBreakerConfig struct {
	ConsecutiveErrors  int    `yaml:"consecutiveErrors,omitempty"`
	Interval           string `yaml:"interval,omitempty"`
	BaseEjectionTime   string `yaml:"baseEjectionTime,omitempty"`
	MaxEjectionPercent int    `yaml:"maxEjectionPercent,omitempty"`
}

// TrafficSplitConfig defines traffic splitting for canary/A-B testing
type TrafficSplitConfig struct {
	Version string `yaml:"version"`
	Weight  int    `yaml:"weight"`
	Subset  string `yaml:"subset,omitempty"`
}

// ResourceConfig defines resource requests and limits
type ResourceConfig struct {
	Requests ResourceValues `yaml:"requests,omitempty"`
	Limits   ResourceValues `yaml:"limits,omitempty"`
}

// ResourceValues defines CPU and memory values
type ResourceValues struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// TrafficConfig defines traffic generation
type TrafficConfig struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type,omitempty"` // load-generator
	Target      string   `yaml:"target"`
	Rate        string   `yaml:"rate,omitempty"`
	Pattern     string   `yaml:"pattern,omitempty"` // steady, spiky, diurnal
	Duration    string   `yaml:"duration,omitempty"`
	Paths       []string `yaml:"paths,omitempty"`       // List of paths to call
	PathPattern string   `yaml:"pathPattern,omitempty"` // round-robin, random, sequential
	Behavior    string   `yaml:"behavior,omitempty"`    // Behavior query param to inject
}

// ScenarioConfig defines time-based scenarios
type ScenarioConfig struct {
	Name     string                 `yaml:"name"`
	At       string                 `yaml:"at"`                 // When to trigger
	Duration string                 `yaml:"duration,omitempty"` // How long it runs
	Action   string                 `yaml:"action"`             // What to do
	Params   map[string]interface{} `yaml:"params,omitempty"`
}

// Defaults returns a ServiceConfig with default values
func (s *ServiceConfig) Defaults() {
	if s.Replicas == 0 {
		s.Replicas = 1
	}
	if s.Type == "" {
		s.Type = "Deployment"
	}
	if len(s.Protocols) == 0 {
		s.Protocols = []string{"http"}
	}
	// HTTP port is always set (used for health checks even in gRPC-only services)
	if s.Ports.HTTP == 0 {
		s.Ports.HTTP = 8080
	}
	if s.Ports.GRPC == 0 && contains(s.Protocols, "grpc") {
		s.Ports.GRPC = 8080
	}
	if s.Ports.Metrics == 0 {
		s.Ports.Metrics = 9091
	}
	if s.Namespace == "" {
		s.Namespace = "default"
	}
	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	if s.Annotations == nil {
		s.Annotations = make(map[string]string)
	}
}

// HasHTTP returns true if the service supports HTTP
func (s *ServiceConfig) HasHTTP() bool {
	return contains(s.Protocols, "http")
}

// HasGRPC returns true if the service supports gRPC
func (s *ServiceConfig) HasGRPC() bool {
	return contains(s.Protocols, "grpc")
}

// NeedsIngress returns true if the service needs ingress
func (s *ServiceConfig) NeedsIngress() bool {
	return s.Ingress.Enabled
}

// IsStateful returns true if this is a StatefulSet
func (s *ServiceConfig) IsStateful() bool {
	return s.Type == "StatefulSet"
}

// MeshEnabled returns true if mesh is enabled for this service
func (s *ServiceConfig) MeshEnabled(appMeshProvider string) bool {
	// Explicit disable
	if s.Mesh.Enabled != nil && !*s.Mesh.Enabled {
		return false
	}
	// If app has mesh provider (not "none" or empty), default to enabled
	return appMeshProvider != "" && appMeshProvider != "none"
}

// EffectiveMeshConfig returns the effective mesh configuration for this service
// by merging app defaults with service-specific overrides
func (s *ServiceConfig) EffectiveMeshConfig(appDefaults MeshConfig) MeshConfig {
	// Start with app defaults
	config := MeshConfig{
		Enabled:        appDefaults.Enabled,
		Timeout:        appDefaults.Timeout,
		Retries:        appDefaults.Retries,
		CircuitBreaker: appDefaults.CircuitBreaker,
		LoadBalancing:  appDefaults.LoadBalancing,
		TrafficSplit:   appDefaults.TrafficSplit,
		MTLS:           appDefaults.MTLS,
	}

	// Override with service-specific settings (if provided)
	if s.Mesh.Enabled != nil {
		config.Enabled = s.Mesh.Enabled
	}
	if s.Mesh.Timeout != "" {
		config.Timeout = s.Mesh.Timeout
	}
	if s.Mesh.Retries != nil {
		config.Retries = s.Mesh.Retries
	}
	if s.Mesh.CircuitBreaker != nil {
		config.CircuitBreaker = s.Mesh.CircuitBreaker
	}
	if s.Mesh.LoadBalancing != "" {
		config.LoadBalancing = s.Mesh.LoadBalancing
	}
	if len(s.Mesh.TrafficSplit) > 0 {
		config.TrafficSplit = s.Mesh.TrafficSplit
	}
	if s.Mesh.MTLS != "" {
		config.MTLS = s.Mesh.MTLS
	}

	return config
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// UnmarshalYAML implements custom unmarshaling to support both old []string and new []UpstreamRoute formats
func (s *ServiceConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Define an aux struct with all fields explicit
	aux := &struct {
		Name        string            `yaml:"name"`
		Namespace   string            `yaml:"namespace,omitempty"`
		Replicas    int               `yaml:"replicas,omitempty"`
		Type        string            `yaml:"type,omitempty"`
		Protocols   []string          `yaml:"protocols,omitempty"`
		Ports       PortsConfig       `yaml:"ports,omitempty"`
		Upstreams   interface{}       `yaml:"upstreams,omitempty"`
		Behavior    BehaviorConfig    `yaml:"behavior,omitempty"`
		Storage     StorageConfig     `yaml:"storage,omitempty"`
		Ingress     IngressConfig     `yaml:"ingress,omitempty"`
		Mesh        MeshConfig        `yaml:"mesh,omitempty"`
		Resources   ResourceConfig    `yaml:"resources,omitempty"`
		Labels      map[string]string `yaml:"labels,omitempty"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	}{}

	if err := unmarshal(aux); err != nil {
		return err
	}

	// Copy all fields
	s.Name = aux.Name
	s.Namespace = aux.Namespace
	s.Replicas = aux.Replicas
	s.Type = aux.Type
	s.Protocols = aux.Protocols
	s.Ports = aux.Ports
	s.Behavior = aux.Behavior
	s.Storage = aux.Storage
	s.Ingress = aux.Ingress
	s.Mesh = aux.Mesh
	s.Resources = aux.Resources
	s.Labels = aux.Labels
	s.Annotations = aux.Annotations

	// Process upstreams based on type
	if aux.Upstreams != nil {
		switch v := aux.Upstreams.(type) {
		case []interface{}:
			if len(v) > 0 {
				// Check first element to determine format
				switch v[0].(type) {
				case string:
					// Simple format: []string (just upstream names)
					s.Upstreams = make([]UpstreamRoute, 0, len(v))
					for _, item := range v {
						if str, ok := item.(string); ok {
							s.Upstreams = append(s.Upstreams, UpstreamRoute{
								Name:  str,
								Match: nil,
								Path:  "",
							})
						}
					}
				case map[string]interface{}:
					// Full format: []UpstreamRoute with match/path
					s.Upstreams = make([]UpstreamRoute, 0, len(v))
					for _, item := range v {
						if m, ok := item.(map[string]interface{}); ok {
							route := UpstreamRoute{}
							if name, ok := m["name"].(string); ok {
								route.Name = name
							}
							if match, ok := m["match"].([]interface{}); ok {
								route.Match = make([]string, 0, len(match))
								for _, p := range match {
									if pathStr, ok := p.(string); ok {
										route.Match = append(route.Match, pathStr)
									}
								}
							}
							if path, ok := m["path"].(string); ok {
								route.Path = path
							}
							s.Upstreams = append(s.Upstreams, route)
						}
					}
				}
			}
		}
	}

	return nil
}
