package parser

import (
	"fmt"
	"os"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
	"gopkg.in/yaml.v3"
)

// Parse parses a DSL file and returns an AppSpec
func Parse(filename string) (*types.AppSpec, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseBytes(data)
}

// ParseBytes parses DSL from bytes
func ParseBytes(data []byte) (*types.AppSpec, error) {
	var spec types.AppSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults
	for i := range spec.Services {
		spec.Services[i].Defaults()
	}

	// Validate
	if err := Validate(&spec); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &spec, nil
}

// Validate validates the AppSpec
func Validate(spec *types.AppSpec) error {
	if spec.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	if len(spec.Services) == 0 {
		return fmt.Errorf("at least one service is required")
	}

	// Build service name map for validation
	serviceNames := make(map[string]bool)
	for _, svc := range spec.Services {
		if svc.Name == "" {
			return fmt.Errorf("service name is required")
		}

		// Check for duplicate names
		key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
		if serviceNames[key] {
			return fmt.Errorf("duplicate service name: %s in namespace %s", svc.Name, svc.Namespace)
		}
		serviceNames[key] = true

		// Validate type
		switch svc.Type {
		case "Deployment", "StatefulSet", "DaemonSet":
			// Valid
		default:
			return fmt.Errorf("invalid service type: %s (must be Deployment, StatefulSet, or DaemonSet)", svc.Type)
		}

		// Validate protocols
		for _, proto := range svc.Protocols {
			if proto != "http" && proto != "grpc" {
				return fmt.Errorf("invalid protocol %s for service %s (must be http or grpc)", proto, svc.Name)
			}
		}

		// Validate StatefulSet requirements
		if svc.Type == "StatefulSet" && svc.Storage.Size == "" {
			return fmt.Errorf("StatefulSet %s requires storage.size", svc.Name)
		}

		// Validate replicas for DaemonSet
		if svc.Type == "DaemonSet" && svc.Replicas > 1 {
			return fmt.Errorf("DaemonSet %s cannot specify replicas (managed by DaemonSet controller)", svc.Name)
		}
	}

	// Validate upstream references
	for _, svc := range spec.Services {
		for _, upstream := range svc.Upstreams {
			found := false
			for _, target := range spec.Services {
				if target.Name == upstream.Name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("service %s references unknown upstream: %s", svc.Name, upstream.Name)
			}
		}
	}

	// Validate traffic targets
	for _, traffic := range spec.Traffic {
		found := false
		for _, svc := range spec.Services {
			if svc.Name == traffic.Target {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("traffic %s targets unknown service: %s", traffic.Name, traffic.Target)
		}
	}

	// Check for circular dependencies
	if err := checkCircularDeps(spec); err != nil {
		return err
	}

	return nil
}

// checkCircularDeps checks for circular dependencies in upstream calls
func checkCircularDeps(spec *types.AppSpec) error {
	// Build adjacency list
	graph := make(map[string][]string)
	for _, svc := range spec.Services {
		upstreamNames := make([]string, 0, len(svc.Upstreams))
		for _, upstream := range svc.Upstreams {
			upstreamNames = append(upstreamNames, upstream.Name)
		}
		graph[svc.Name] = upstreamNames
	}

	// Check each service for circular deps using DFS
	for _, svc := range spec.Services {
		visited := make(map[string]bool)
		if hasCycle(svc.Name, graph, visited, make(map[string]bool)) {
			return fmt.Errorf("circular dependency detected involving service: %s", svc.Name)
		}
	}

	return nil
}

// hasCycle performs DFS to detect cycles
func hasCycle(node string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if hasCycle(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}
