package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aslakknutsen/kkbase/testapp/pkg/dsl/parser"
	"github.com/aslakknutsen/kkbase/testapp/pkg/dsl/types"
	"github.com/aslakknutsen/kkbase/testapp/pkg/generator/gateway"
	"github.com/aslakknutsen/kkbase/testapp/pkg/generator/istio"
	"github.com/aslakknutsen/kkbase/testapp/pkg/generator/k8s"
	"github.com/aslakknutsen/kkbase/testapp/pkg/generator/traffic"
	"github.com/spf13/cobra"
)

var (
	outputDir      string
	validateOnly   bool
	image          string
	applyManifests bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "testgen",
		Short: "TestApp manifest generator",
		Long:  "Generate Kubernetes and Gateway API manifests from TestApp DSL",
	}

	generateCmd := &cobra.Command{
		Use:   "generate <dsl-file>",
		Short: "Generate manifests from DSL",
		Args:  cobra.ExactArgs(1),
		RunE:  runGenerate,
	}
	generateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "./output", "Output directory for manifests")
	generateCmd.Flags().BoolVar(&validateOnly, "validate-only", false, "Only validate, don't generate")
	generateCmd.Flags().StringVarP(&image, "image", "i", "testservice:latest", "TestService container image")

	validateCmd := &cobra.Command{
		Use:   "validate <dsl-file>",
		Short: "Validate DSL without generating",
		Args:  cobra.ExactArgs(1),
		RunE:  runValidate,
	}

	applyCmd := &cobra.Command{
		Use:   "apply <dsl-file>",
		Short: "Generate and apply manifests",
		Args:  cobra.ExactArgs(1),
		RunE:  runApply,
	}
	applyCmd.Flags().StringVarP(&image, "image", "i", "testservice:latest", "TestService container image")

	deleteCmd := &cobra.Command{
		Use:   "delete <dsl-file>",
		Short: "Generate and delete manifests",
		Args:  cobra.ExactArgs(1),
		RunE:  runDelete,
	}

	examplesCmd := &cobra.Command{
		Use:   "examples",
		Short: "List available example DSLs",
		RunE:  runExamples,
	}

	initCmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a starter DSL template",
		Args:  cobra.ExactArgs(1),
		RunE:  runInit,
	}

	rootCmd.AddCommand(generateCmd, validateCmd, applyCmd, deleteCmd, examplesCmd, initCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Generator interface for all manifest generators
type Generator interface {
	Name() string
	Generate() (map[string]string, error)
}

// getGenerators returns the list of enabled generators based on the spec
func getGenerators(spec *types.AppSpec) []Generator {
	generators := []Generator{}

	// K8s generator - always runs (core resources)
	generators = append(generators, &k8sGeneratorAdapter{gen: k8s.NewGenerator(spec, image)})

	// Ingress provider (if any service has ingress.enabled)
	if hasIngress(spec) {
		ingressProvider := spec.App.Providers.Ingress
		if ingressProvider == "" {
			ingressProvider = "gateway-api" // default
		}

		switch ingressProvider {
		case "gateway-api":
			generators = append(generators, &gatewayGeneratorAdapter{gen: gateway.NewGenerator(spec)})
		case "istio-gateway":
			generators = append(generators, istio.NewGatewayGenerator(spec))
		case "none":
			// skip
		}
	}

	// Mesh provider (if enabled and not "none")
	meshProvider := spec.App.Providers.Mesh
	if meshProvider != "" && meshProvider != "none" {
		switch meshProvider {
		case "istio":
			generators = append(generators, istio.NewMeshGenerator(spec))
			// Future: linkerd, gateway-api-mesh
		}
	}

	// Traffic generator (if any traffic configs exist)
	if len(spec.Traffic) > 0 {
		generators = append(generators, &trafficGeneratorAdapter{gen: traffic.NewGenerator(spec)})
	}

	return generators
}

// hasIngress checks if any service has ingress enabled
func hasIngress(spec *types.AppSpec) bool {
	for _, svc := range spec.Services {
		if svc.Ingress.Enabled {
			return true
		}
	}
	return false
}

// Adapter types to make existing generators compatible with the Generator interface
type k8sGeneratorAdapter struct {
	gen *k8s.Generator
}

func (a *k8sGeneratorAdapter) Name() string {
	return "k8s"
}

func (a *k8sGeneratorAdapter) Generate() (map[string]string, error) {
	return a.gen.GenerateAll()
}

type gatewayGeneratorAdapter struct {
	gen *gateway.Generator
}

func (a *gatewayGeneratorAdapter) Name() string {
	return "gateway-api"
}

func (a *gatewayGeneratorAdapter) Generate() (map[string]string, error) {
	return a.gen.GenerateAll()
}

type trafficGeneratorAdapter struct {
	gen *traffic.Generator
}

func (a *trafficGeneratorAdapter) Name() string {
	return "traffic"
}

func (a *trafficGeneratorAdapter) Generate() (map[string]string, error) {
	return a.gen.GenerateAll()
}

func runGenerate(cmd *cobra.Command, args []string) error {
	dslFile := args[0]

	// Parse DSL
	fmt.Printf("Parsing DSL file: %s\n", dslFile)
	spec, err := parser.Parse(dslFile)
	if err != nil {
		return fmt.Errorf("failed to parse DSL: %w", err)
	}

	fmt.Printf("✓ DSL validated successfully\n")
	fmt.Printf("  App: %s\n", spec.App.Name)
	fmt.Printf("  Services: %d\n", len(spec.Services))
	fmt.Printf("  Traffic generators: %d\n", len(spec.Traffic))

	if validateOnly {
		fmt.Println("✓ Validation complete (no manifests generated)")
		return nil
	}

	// Generate manifests
	fmt.Println("\nGenerating manifests...")

	// Get enabled generators
	generators := getGenerators(spec)

	// Generate manifests from all enabled generators
	allManifests := make(map[string]string)
	for _, gen := range generators {
		manifests, err := gen.Generate()
		if err != nil {
			return fmt.Errorf("generator %s failed: %w", gen.Name(), err)
		}

		// Merge manifests
		for k, v := range manifests {
			allManifests[k] = v
		}

		if len(manifests) > 0 {
			fmt.Printf("  ✓ %s: %d manifests\n", gen.Name(), len(manifests))
		}
	}

	// Write manifests to disk
	fmt.Println("\nWriting manifests...")
	appOutputDir := filepath.Join(outputDir, spec.App.Name)
	if err := os.MkdirAll(appOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for filename, content := range allManifests {
		fullPath := filepath.Join(appOutputDir, filename)

		// Create subdirectories if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fullPath, err)
		}
	}

	// Generate README
	readme := generateReadme(spec)
	readmePath := filepath.Join(appOutputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}
	fmt.Printf("  ✓ README.md\n")

	fmt.Printf("\n✓ Generated %d manifests in %s\n", len(allManifests)+1, appOutputDir)
	fmt.Printf("\nTo apply:\n")
	fmt.Printf("  kubectl apply -f %s/\n", appOutputDir)

	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	validateOnly = true
	return runGenerate(cmd, args)
}

func runApply(cmd *cobra.Command, args []string) error {
	// First generate
	outputDir = "/tmp/testgen-" + filepath.Base(args[0])
	if err := runGenerate(cmd, args); err != nil {
		return err
	}

	// Parse to get app name
	spec, err := parser.Parse(args[0])
	if err != nil {
		return err
	}

	appOutputDir := filepath.Join(outputDir, spec.App.Name)

	// Apply with kubectl
	fmt.Println("\nApplying manifests...")
	// Note: In a real implementation, use os/exec to run kubectl
	fmt.Printf("  kubectl apply -f %s/\n", appOutputDir)
	fmt.Println("\n✓ To actually apply, run: kubectl apply -f " + appOutputDir + "/")

	return nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	spec, err := parser.Parse(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("To delete the application, run:\n")
	fmt.Printf("  kubectl delete namespace ")
	for i, ns := range spec.App.Namespaces {
		if i > 0 {
			fmt.Printf(" ")
		}
		fmt.Printf("%s", ns)
	}
	fmt.Println()

	return nil
}

func runExamples(cmd *cobra.Command, args []string) error {
	fmt.Println("Available examples:")
	fmt.Println()
	fmt.Println("  simple-web/       - Basic 3-tier web application")
	fmt.Println("  ecommerce/        - Complex multi-namespace e-commerce app")
	fmt.Println("  microservices/    - Large microservices mesh")
	fmt.Println()
	fmt.Println("Examples are located in the examples/ directory")
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	name := args[0]
	filename := name + ".yaml"

	template := fmt.Sprintf(`app:
  name: %s
  namespaces:
    - default

services:
  - name: frontend
    replicas: 2
    protocols: [http]
    upstreams: [backend]
    ingress:
      enabled: true
      host: %s.local
      tls: true

  - name: backend
    replicas: 2
    protocols: [http]
    behavior:
      latency: "10-50ms"
      errorRate: 0.01

traffic:
  - name: load-gen
    target: frontend
    rate: "10/s"
    pattern: steady
`, name, name)

	if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	fmt.Printf("✓ Created %s\n", filename)
	fmt.Printf("\nEdit the file and then generate manifests with:\n")
	fmt.Printf("  testgen generate %s\n", filename)

	return nil
}

func generateReadme(spec *types.AppSpec) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", spec.App.Name))
	b.WriteString("Generated by TestApp Generator\n\n")

	b.WriteString("## Overview\n\n")
	b.WriteString(fmt.Sprintf("- **Services**: %d\n", len(spec.Services)))
	b.WriteString(fmt.Sprintf("- **Namespaces**: %d\n", len(spec.App.Namespaces)))
	b.WriteString(fmt.Sprintf("- **Traffic Generators**: %d\n\n", len(spec.Traffic)))

	b.WriteString("## Services\n\n")
	for _, svc := range spec.Services {
		b.WriteString(fmt.Sprintf("### %s\n\n", svc.Name))
		b.WriteString(fmt.Sprintf("- **Type**: %s\n", svc.Type))
		b.WriteString(fmt.Sprintf("- **Namespace**: %s\n", svc.Namespace))
		b.WriteString(fmt.Sprintf("- **Replicas**: %d\n", svc.Replicas))
		b.WriteString(fmt.Sprintf("- **Protocols**: %v\n", svc.Protocols))
		if len(svc.Upstreams) > 0 {
			b.WriteString(fmt.Sprintf("- **Upstreams**: %v\n", svc.Upstreams))
		}
		if svc.NeedsIngress() {
			b.WriteString("- **Ingress**: Enabled\n")
			if svc.Ingress.Host != "" {
				b.WriteString(fmt.Sprintf("  - Host: %s\n", svc.Ingress.Host))
			}
			if svc.Ingress.TLS {
				b.WriteString("  - TLS: Enabled\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Deployment\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Apply all manifests\n")
	b.WriteString("kubectl apply -f .\n\n")
	b.WriteString("# Check status\n")
	for _, ns := range spec.App.Namespaces {
		b.WriteString(fmt.Sprintf("kubectl get pods -n %s\n", ns))
	}
	b.WriteString("```\n\n")

	b.WriteString("## Testing\n\n")
	for _, svc := range spec.Services {
		if svc.NeedsIngress() {
			b.WriteString(fmt.Sprintf("### %s\n\n", svc.Name))
			b.WriteString("```bash\n")
			if svc.Ingress.Host != "" {
				proto := "http"
				if svc.Ingress.TLS {
					proto = "https"
				}
				b.WriteString(fmt.Sprintf("curl %s://%s/\n", proto, svc.Ingress.Host))
			}
			b.WriteString("```\n\n")
		}
	}

	b.WriteString("## Cleanup\n\n")
	b.WriteString("```bash\n")
	b.WriteString("kubectl delete -f .\n")
	b.WriteString("```\n")

	return b.String()
}
