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
	Name       string   `yaml:"name"`
	Namespaces []string `yaml:"namespaces,omitempty"`
}

// ServiceConfig defines a service
type ServiceConfig struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Replicas    int               `yaml:"replicas,omitempty"`
	Type        string            `yaml:"type,omitempty"` // Deployment, StatefulSet, DaemonSet
	Protocols   []string          `yaml:"protocols,omitempty"`
	Ports       PortsConfig       `yaml:"ports,omitempty"`
	Upstreams   []string          `yaml:"upstreams,omitempty"`
	Behavior    BehaviorConfig    `yaml:"behavior,omitempty"`
	Storage     StorageConfig     `yaml:"storage,omitempty"`
	Ingress     IngressConfig     `yaml:"ingress,omitempty"`
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
	Name     string `yaml:"name"`
	Type     string `yaml:"type,omitempty"` // load-generator
	Target   string `yaml:"target"`
	Rate     string `yaml:"rate,omitempty"`
	Pattern  string `yaml:"pattern,omitempty"` // steady, spiky, diurnal
	Duration string `yaml:"duration,omitempty"`
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
		s.Ports.GRPC = 9090
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
