package k8s

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Generator generates Kubernetes manifests
type Generator struct {
	spec      *types.AppSpec
	image     string // TestService container image
	templates *template.Template
}

// Template data structures
type namespaceData struct {
	AppName    string
	Namespaces []string
}

type workloadData struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Replicas  int
	Image     string
	Ports     []portData
	EnvVars   []envVarData
	Resources resourcesData
	Probes    *probesData
	Storage   *storageData
}

type portData struct {
	ContainerPort int
	Name          string
	Protocol      string
}

type envVarData struct {
	Name      string
	Value     string
	ValueFrom string
}

type resourcesData struct {
	Requests resourceQuantity
	Limits   resourceQuantity
}

type resourceQuantity struct {
	CPU    string
	Memory string
}

type probesData struct {
	Liveness  probeConfig
	Readiness probeConfig
}

type probeConfig struct {
	Path                string
	Port                int
	InitialDelaySeconds int
	PeriodSeconds       int
}

type storageData struct {
	Size string
}

type serviceData struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Ports     []servicePortData
}

type servicePortData struct {
	Name       string
	Port       int
	TargetPort string
	Protocol   string
}

type serviceMonitorData struct {
	Name      string
	Namespace string
	Labels    map[string]string
}

// NewGenerator creates a new Kubernetes manifest generator
func NewGenerator(spec *types.AppSpec, image string) *Generator {
	if image == "" {
		image = "testservice:latest"
	}

	// Parse templates with custom functions
	tmpl := template.Must(template.New("k8s").Funcs(template.FuncMap{
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
		image:     image,
		templates: tmpl,
	}
}

// GenerateAll generates all Kubernetes manifests
func (g *Generator) GenerateAll() (map[string]string, error) {
	manifests := make(map[string]string)

	// Generate namespaces
	if len(g.spec.App.Namespaces) > 0 {
		ns := g.GenerateNamespaces()
		manifests["00-namespaces.yaml"] = ns
	}

	// Generate service manifests
	for _, svc := range g.spec.Services {
		prefix := fmt.Sprintf("10-services/%s", svc.Name)

		// Workload (Deployment/StatefulSet/DaemonSet)
		workload := g.GenerateWorkload(&svc)
		manifests[fmt.Sprintf("%s-%s.yaml", prefix, strings.ToLower(svc.Type))] = workload

		// Service
		service := g.GenerateService(&svc)
		manifests[fmt.Sprintf("%s-service.yaml", prefix)] = service

		// ServiceMonitor
		monitor := g.GenerateServiceMonitor(&svc)
		manifests[fmt.Sprintf("%s-servicemonitor.yaml", prefix)] = monitor
	}

	return manifests, nil
}

// GenerateNamespaces generates namespace manifests
func (g *Generator) GenerateNamespaces() string {
	data := namespaceData{
		AppName:    g.spec.App.Name,
		Namespaces: g.spec.App.Namespaces,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "namespace.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute namespace template: %v", err))
	}
	return buf.String()
}

// GenerateWorkload generates Deployment, StatefulSet, or DaemonSet
func (g *Generator) GenerateWorkload(svc *types.ServiceConfig) string {
	switch svc.Type {
	case "StatefulSet":
		return g.generateStatefulSet(svc)
	case "DaemonSet":
		return g.generateDaemonSet(svc)
	default:
		return g.generateDeployment(svc)
	}
}

// generateDeployment generates a Deployment manifest
func (g *Generator) generateDeployment(svc *types.ServiceConfig) string {
	data := g.buildWorkloadData(svc)

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "deployment.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute deployment template: %v", err))
	}
	return buf.String()
}

// generateStatefulSet generates a StatefulSet manifest
func (g *Generator) generateStatefulSet(svc *types.ServiceConfig) string {
	data := g.buildWorkloadData(svc)
	data.Storage = &storageData{
		Size: svc.Storage.Size,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "statefulset.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute statefulset template: %v", err))
	}
	return buf.String()
}

// generateDaemonSet generates a DaemonSet manifest
func (g *Generator) generateDaemonSet(svc *types.ServiceConfig) string {
	data := g.buildWorkloadData(svc)

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "daemonset.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute daemonset template: %v", err))
	}
	return buf.String()
}

// GenerateService generates a Service manifest
func (g *Generator) GenerateService(svc *types.ServiceConfig) string {
	data := serviceData{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Labels:    g.getLabels(svc),
		Ports:     g.getServicePorts(svc),
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "service.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute service template: %v", err))
	}
	return buf.String()
}

// GenerateServiceMonitor generates a ServiceMonitor for Prometheus
func (g *Generator) GenerateServiceMonitor(svc *types.ServiceConfig) string {
	data := serviceMonitorData{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Labels:    g.getLabels(svc),
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "servicemonitor.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute servicemonitor template: %v", err))
	}
	return buf.String()
}

// Helper methods

func (g *Generator) buildWorkloadData(svc *types.ServiceConfig) workloadData {
	return workloadData{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Labels:    g.getLabels(svc),
		Replicas:  svc.Replicas,
		Image:     g.image,
		Ports:     g.getPorts(svc),
		EnvVars:   g.getEnvVars(svc),
		Resources: g.getResources(svc),
		Probes:    g.getProbes(svc),
	}
}

func (g *Generator) getLabels(svc *types.ServiceConfig) map[string]string {
	labels := map[string]string{
		"app":     svc.Name,
		"version": "v1",
		"part-of": g.spec.App.Name,
	}

	// Add custom labels
	for k, v := range svc.Labels {
		labels[k] = v
	}

	return labels
}

func (g *Generator) getEnvVars(svc *types.ServiceConfig) []envVarData {
	var envVars []envVarData

	// Add downward API variables
	envVars = append(envVars, envVarData{
		Name: "NAMESPACE",
		ValueFrom: `fieldRef:
  fieldPath: metadata.namespace`,
	})

	envVars = append(envVars, envVarData{
		Name: "POD_NAME",
		ValueFrom: `fieldRef:
  fieldPath: metadata.name`,
	})

	envVars = append(envVars, envVarData{
		Name: "NODE_NAME",
		ValueFrom: `fieldRef:
  fieldPath: spec.nodeName`,
	})

	// Add OTEL endpoint
	envVars = append(envVars, envVarData{
		Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
		Value: "jaeger-collector-otlp.observability.svc.cluster.local:4317",
	})

	// Add service-specific env vars
	env := map[string]string{
		"SERVICE_NAME":    svc.Name,
		"SERVICE_VERSION": "1.0.0",
		"HTTP_PORT":       fmt.Sprintf("%d", svc.Ports.HTTP),
		"GRPC_PORT":       fmt.Sprintf("%d", svc.Ports.GRPC),
		"METRICS_PORT":    fmt.Sprintf("%d", svc.Ports.Metrics),
	}

	for k, v := range env {
		envVars = append(envVars, envVarData{
			Name:  k,
			Value: v,
		})
	}

	// Add upstreams
	if len(svc.Upstreams) > 0 {
		upstreams := g.buildUpstreamsEnv(svc)
		envVars = append(envVars, envVarData{
			Name:  "UPSTREAMS",
			Value: upstreams,
		})
	}

	// Add behavior
	if svc.Behavior.Latency != "" || svc.Behavior.ErrorRate > 0 {
		behavior := g.buildBehaviorString(svc)
		envVars = append(envVars, envVarData{
			Name:  "DEFAULT_BEHAVIOR",
			Value: behavior,
		})
	}

	return envVars
}

func (g *Generator) buildUpstreamsEnv(svc *types.ServiceConfig) string {
	// Consolidate upstreams by name to handle multiple entries with different paths
	upstreamMap := make(map[string]struct {
		url   string
		paths []string
	})

	for _, upstream := range svc.Upstreams {
		// Find the upstream service
		for _, target := range g.spec.Services {
			if target.Name == upstream.Name {
				protocol := "http"
				port := target.Ports.HTTP
				if target.HasGRPC() && !target.HasHTTP() {
					protocol = "grpc"
					port = target.Ports.GRPC
				}
				url := fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d",
					protocol, target.Name, target.Namespace, port)

				// Consolidate paths for the same upstream
				entry := upstreamMap[upstream.Name]
				entry.url = url
				entry.paths = append(entry.paths, upstream.Paths...)
				upstreamMap[upstream.Name] = entry
				break
			}
		}
	}

	// Build the environment variable string
	var parts []string
	for name, entry := range upstreamMap {
		if len(entry.paths) > 0 {
			pathsStr := strings.Join(entry.paths, ",")
			parts = append(parts, fmt.Sprintf("%s=%s:%s", name, entry.url, pathsStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", name, entry.url))
		}
	}

	// Use | as delimiter to support commas in path lists
	return strings.Join(parts, "|")
}

func (g *Generator) buildBehaviorString(svc *types.ServiceConfig) string {
	var parts []string
	if svc.Behavior.Latency != "" {
		parts = append(parts, fmt.Sprintf("latency=%s", svc.Behavior.Latency))
	}
	if svc.Behavior.ErrorRate > 0 {
		parts = append(parts, fmt.Sprintf("error=%.2f", svc.Behavior.ErrorRate))
	}
	return strings.Join(parts, ",")
}

func (g *Generator) getPorts(svc *types.ServiceConfig) []portData {
	var ports []portData

	if svc.HasHTTP() {
		ports = append(ports, portData{
			ContainerPort: svc.Ports.HTTP,
			Name:          "http",
			Protocol:      "TCP",
		})
	}

	if svc.HasGRPC() {
		ports = append(ports, portData{
			ContainerPort: svc.Ports.GRPC,
			Name:          "grpc",
			Protocol:      "TCP",
		})
	}

	ports = append(ports, portData{
		ContainerPort: svc.Ports.Metrics,
		Name:          "metrics",
		Protocol:      "TCP",
	})

	return ports
}

func (g *Generator) getServicePorts(svc *types.ServiceConfig) []servicePortData {
	var ports []servicePortData

	if svc.HasHTTP() {
		ports = append(ports, servicePortData{
			Name:       "http",
			Port:       svc.Ports.HTTP,
			TargetPort: "http",
			Protocol:   "TCP",
		})
	}

	if svc.HasGRPC() {
		ports = append(ports, servicePortData{
			Name:       "grpc",
			Port:       svc.Ports.GRPC,
			TargetPort: "grpc",
			Protocol:   "TCP",
		})
	}

	ports = append(ports, servicePortData{
		Name:       "metrics",
		Port:       svc.Ports.Metrics,
		TargetPort: "metrics",
		Protocol:   "TCP",
	})

	return ports
}

func (g *Generator) getResources(svc *types.ServiceConfig) resourcesData {
	// Default resources
	requests := resourceQuantity{
		CPU:    "100m",
		Memory: "128Mi",
	}
	limits := resourceQuantity{
		CPU:    "500m",
		Memory: "512Mi",
	}

	// Override with custom values
	if svc.Resources.Requests.CPU != "" {
		requests.CPU = svc.Resources.Requests.CPU
	}
	if svc.Resources.Requests.Memory != "" {
		requests.Memory = svc.Resources.Requests.Memory
	}
	if svc.Resources.Limits.CPU != "" {
		limits.CPU = svc.Resources.Limits.CPU
	}
	if svc.Resources.Limits.Memory != "" {
		limits.Memory = svc.Resources.Limits.Memory
	}

	return resourcesData{
		Requests: requests,
		Limits:   limits,
	}
}

func (g *Generator) getProbes(svc *types.ServiceConfig) *probesData {
	// TestService always exposes HTTP health endpoints for probes
	return &probesData{
		Liveness: probeConfig{
			Path:                "/health",
			Port:                svc.Ports.HTTP,
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
		},
		Readiness: probeConfig{
			Path:                "/ready",
			Port:                svc.Ports.HTTP,
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
	}
}
