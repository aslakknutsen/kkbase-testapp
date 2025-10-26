package k8s

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
)

// Generator generates Kubernetes manifests
type Generator struct {
	spec  *types.AppSpec
	image string // TestService container image
}

// NewGenerator creates a new Kubernetes manifest generator
func NewGenerator(spec *types.AppSpec, image string) *Generator {
	if image == "" {
		image = "testservice:latest"
	}
	return &Generator{
		spec:  spec,
		image: image,
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

		// PVC for StatefulSet (embedded in workload spec)
	}

	return manifests, nil
}

// GenerateNamespaces generates namespace manifests
func (g *Generator) GenerateNamespaces() string {
	var b strings.Builder
	for i, ns := range g.spec.App.Namespaces {
		if i > 0 {
			b.WriteString("---\n")
		}
		b.WriteString(fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app: %s
`, ns, g.spec.App.Name))
	}
	return b.String()
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
	metadataLabels := g.getLabels(svc, 4)
	podLabels := g.getLabels(svc, 8)
	envVars := g.getEnvVars(svc)
	ports := g.getPorts(svc)
	resources := g.getResources(svc)
	probes := g.getProbes(svc)

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
%s
    spec:
      containers:
      - name: testservice
        image: %s
        imagePullPolicy: Always
        ports:
%s
        env:
%s
        resources:
%s%s
`,
		svc.Name,
		svc.Namespace,
		metadataLabels,
		svc.Replicas,
		svc.Name,
		podLabels,
		g.image,
		ports,
		envVars,
		resources,
		probes,
	)
}

// generateStatefulSet generates a StatefulSet manifest
func (g *Generator) generateStatefulSet(svc *types.ServiceConfig) string {
	metadataLabels := g.getLabels(svc, 4)
	podLabels := g.getLabels(svc, 8)
	envVars := g.getEnvVars(svc)
	ports := g.getPorts(svc)
	resources := g.getResources(svc)
	probes := g.getProbes(svc)

	return fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  serviceName: %s
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
%s
    spec:
      containers:
      - name: testservice
        image: %s
        imagePullPolicy: Always
        ports:
%s
        env:
%s
        resources:
%s%s
        volumeMounts:
        - name: data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: %s
`,
		svc.Name,
		svc.Namespace,
		metadataLabels,
		svc.Name,
		svc.Replicas,
		svc.Name,
		podLabels,
		g.image,
		ports,
		envVars,
		resources,
		probes,
		svc.Storage.Size,
	)
}

// generateDaemonSet generates a DaemonSet manifest
func (g *Generator) generateDaemonSet(svc *types.ServiceConfig) string {
	metadataLabels := g.getLabels(svc, 4)
	podLabels := g.getLabels(svc, 8)
	envVars := g.getEnvVars(svc)
	ports := g.getPorts(svc)
	resources := g.getResources(svc)
	probes := g.getProbes(svc)

	return fmt.Sprintf(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
%s
    spec:
      containers:
      - name: testservice
        image: %s
        imagePullPolicy: Always
        ports:
%s
        env:
%s
        resources:
%s%s
`,
		svc.Name,
		svc.Namespace,
		metadataLabels,
		svc.Name,
		podLabels,
		g.image,
		ports,
		envVars,
		resources,
		probes,
	)
}

// GenerateService generates a Service manifest
func (g *Generator) GenerateService(svc *types.ServiceConfig) string {
	labels := g.getLabels(svc, 4)
	ports := g.getServicePorts(svc)

	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  type: ClusterIP
  selector:
    app: %s
  ports:
%s
`,
		svc.Name,
		svc.Namespace,
		labels,
		svc.Name,
		ports,
	)
}

// GenerateServiceMonitor generates a ServiceMonitor for Prometheus
func (g *Generator) GenerateServiceMonitor(svc *types.ServiceConfig) string {
	labels := g.getLabels(svc, 4)

	return fmt.Sprintf(`apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  selector:
    matchLabels:
      app: %s
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
`,
		svc.Name,
		svc.Namespace,
		labels,
		svc.Name,
	)
}

// Helper methods

func (g *Generator) getLabels(svc *types.ServiceConfig, indent int) string {
	var b strings.Builder
	labels := map[string]string{
		"app":     svc.Name,
		"version": "v1",
		"part-of": g.spec.App.Name,
	}

	// Add custom labels
	for k, v := range svc.Labels {
		labels[k] = v
	}

	indentStr := strings.Repeat(" ", indent)
	for k, v := range labels {
		b.WriteString(fmt.Sprintf("%s%s: %s\n", indentStr, k, v))
	}
	return b.String()
}

func (g *Generator) getEnvVars(svc *types.ServiceConfig) string {
	var b strings.Builder

	env := map[string]string{
		"SERVICE_NAME":    svc.Name,
		"SERVICE_VERSION": "1.0.0",
		"HTTP_PORT":       fmt.Sprintf("%d", svc.Ports.HTTP),
		"GRPC_PORT":       fmt.Sprintf("%d", svc.Ports.GRPC),
		"METRICS_PORT":    fmt.Sprintf("%d", svc.Ports.Metrics),
	}

	// Add namespace from downward API
	b.WriteString("        - name: NAMESPACE\n")
	b.WriteString("          valueFrom:\n")
	b.WriteString("            fieldRef:\n")
	b.WriteString("              fieldPath: metadata.namespace\n")

	b.WriteString("        - name: POD_NAME\n")
	b.WriteString("          valueFrom:\n")
	b.WriteString("            fieldRef:\n")
	b.WriteString("              fieldPath: metadata.name\n")

	b.WriteString("        - name: NODE_NAME\n")
	b.WriteString("          valueFrom:\n")
	b.WriteString("            fieldRef:\n")
	b.WriteString("              fieldPath: spec.nodeName\n")

	// Add regular env vars
	for k, v := range env {
		b.WriteString(fmt.Sprintf("        - name: %s\n", k))
		b.WriteString(fmt.Sprintf("          value: \"%s\"\n", v))
	}

	// Add upstreams
	if len(svc.Upstreams) > 0 {
		upstreams := g.buildUpstreamsEnv(svc)
		b.WriteString(fmt.Sprintf("        - name: UPSTREAMS\n"))
		b.WriteString(fmt.Sprintf("          value: \"%s\"\n", upstreams))
	}

	// Add behavior
	if svc.Behavior.Latency != "" || svc.Behavior.ErrorRate > 0 {
		behavior := g.buildBehaviorString(svc)
		b.WriteString(fmt.Sprintf("        - name: DEFAULT_BEHAVIOR\n"))
		b.WriteString(fmt.Sprintf("          value: \"%s\"\n", behavior))
	}

	return b.String()
}

func (g *Generator) buildUpstreamsEnv(svc *types.ServiceConfig) string {
	var parts []string
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

				// Add paths if configured
				if len(upstream.Paths) > 0 {
					pathsStr := strings.Join(upstream.Paths, ",")
					parts = append(parts, fmt.Sprintf("%s:%s:%s", upstream.Name, url, pathsStr))
				} else {
					parts = append(parts, fmt.Sprintf("%s:%s", upstream.Name, url))
				}
				break
			}
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

func (g *Generator) getPorts(svc *types.ServiceConfig) string {
	var b strings.Builder

	if svc.HasHTTP() {
		b.WriteString(fmt.Sprintf("        - containerPort: %d\n", svc.Ports.HTTP))
		b.WriteString("          name: http\n")
		b.WriteString("          protocol: TCP\n")
	}

	if svc.HasGRPC() {
		b.WriteString(fmt.Sprintf("        - containerPort: %d\n", svc.Ports.GRPC))
		b.WriteString("          name: grpc\n")
		b.WriteString("          protocol: TCP\n")
	}

	b.WriteString(fmt.Sprintf("        - containerPort: %d\n", svc.Ports.Metrics))
	b.WriteString("          name: metrics\n")
	b.WriteString("          protocol: TCP\n")

	return b.String()
}

func (g *Generator) getServicePorts(svc *types.ServiceConfig) string {
	var b strings.Builder

	if svc.HasHTTP() {
		b.WriteString(fmt.Sprintf("  - name: http\n"))
		b.WriteString(fmt.Sprintf("    port: %d\n", svc.Ports.HTTP))
		b.WriteString(fmt.Sprintf("    targetPort: http\n"))
		b.WriteString(fmt.Sprintf("    protocol: TCP\n"))
	}

	if svc.HasGRPC() {
		b.WriteString("  - name: grpc\n")
		b.WriteString("    port: ")
		b.WriteString(strconv.Itoa(svc.Ports.GRPC))
		b.WriteString("\n")
		b.WriteString("    targetPort: grpc\n")
		b.WriteString("    protocol: TCP\n")
	}

	b.WriteString(fmt.Sprintf("  - name: metrics\n"))
	b.WriteString(fmt.Sprintf("    port: %d\n", svc.Ports.Metrics))
	b.WriteString(fmt.Sprintf("    targetPort: metrics\n"))
	b.WriteString(fmt.Sprintf("    protocol: TCP\n"))

	return b.String()
}

func (g *Generator) getResources(svc *types.ServiceConfig) string {
	// Default resources
	requests := map[string]string{
		"cpu":    "100m",
		"memory": "128Mi",
	}
	limits := map[string]string{
		"cpu":    "500m",
		"memory": "512Mi",
	}

	// Override with custom values
	if svc.Resources.Requests.CPU != "" {
		requests["cpu"] = svc.Resources.Requests.CPU
	}
	if svc.Resources.Requests.Memory != "" {
		requests["memory"] = svc.Resources.Requests.Memory
	}
	if svc.Resources.Limits.CPU != "" {
		limits["cpu"] = svc.Resources.Limits.CPU
	}
	if svc.Resources.Limits.Memory != "" {
		limits["memory"] = svc.Resources.Limits.Memory
	}

	return fmt.Sprintf(`          requests:
            cpu: %s
            memory: %s
          limits:
            cpu: %s
            memory: %s`,
		requests["cpu"],
		requests["memory"],
		limits["cpu"],
		limits["memory"],
	)
}

func (g *Generator) getProbes(svc *types.ServiceConfig) string {
	// TestService always exposes HTTP health endpoints for probes
	// even if the main application protocol is gRPC
	// Liveness: /health - checks if process is alive
	// Readiness: /ready - checks if ready to accept traffic
	return fmt.Sprintf(`
        livenessProbe:
          httpGet:
            path: /health
            port: %d
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: %d
          initialDelaySeconds: 5
          periodSeconds: 5`,
		svc.Ports.HTTP,
		svc.Ports.HTTP,
	)
}
