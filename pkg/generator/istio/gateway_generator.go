package istio

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
)

// GatewayGenerator generates Istio Gateway manifests for ingress
type GatewayGenerator struct {
	spec      *types.AppSpec
	templates *template.Template
}

// NewGatewayGenerator creates a new Istio gateway generator
func NewGatewayGenerator(spec *types.AppSpec) *GatewayGenerator {
	tmpl := template.Must(template.New("istio-gateway").Funcs(funcMap()).ParseFS(templatesFS, "templates/*.tmpl"))
	return &GatewayGenerator{
		spec:      spec,
		templates: tmpl,
	}
}

// Name returns the generator name
func (g *GatewayGenerator) Name() string {
	return "istio-gateway"
}

// Generate generates Istio Gateway manifests for ingress
func (g *GatewayGenerator) Generate() (map[string]string, error) {
	manifests := make(map[string]string)

	// Find services that need ingress
	ingressServices := []types.ServiceConfig{}
	for _, svc := range g.spec.Services {
		if svc.NeedsIngress() {
			ingressServices = append(ingressServices, svc)
		}
	}

	if len(ingressServices) == 0 {
		return manifests, nil
	}

	// Generate Istio Gateway
	gateway, err := g.generateGateway(ingressServices)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Istio Gateway: %w", err)
	}
	manifests["20-gateway/istio-gateway.yaml"] = gateway

	// Generate VirtualService for each ingress-enabled service
	for _, svc := range ingressServices {
		vs, err := g.generateIngressVirtualService(svc)
		if err != nil {
			return nil, fmt.Errorf("failed to generate VirtualService for %s: %w", svc.Name, err)
		}
		manifests[fmt.Sprintf("20-gateway/%s-ingress-virtualservice.yaml", svc.Name)] = vs
	}

	return manifests, nil
}

// istioGatewayData holds data for Istio Gateway template
type istioGatewayData struct {
	Name      string
	Namespace string
	AppName   string
	Servers   []gatewayServer
}

type gatewayServer struct {
	Port     int
	Protocol string
	Hosts    []string
	TLS      *gatewayTLS
}

type gatewayTLS struct {
	Mode           string
	CredentialName string
}

func (g *GatewayGenerator) generateGateway(services []types.ServiceConfig) (string, error) {
	// Collect all hosts
	hostsHTTP := []string{}
	hostsHTTPS := []string{}

	for _, svc := range services {
		host := svc.Ingress.Host
		if host == "" {
			host = "*"
		}

		if svc.Ingress.TLS {
			hostsHTTPS = append(hostsHTTPS, host)
		} else {
			hostsHTTP = append(hostsHTTP, host)
		}
	}

	var servers []gatewayServer

	// HTTP server
	if len(hostsHTTP) > 0 {
		servers = append(servers, gatewayServer{
			Port:     80,
			Protocol: "HTTP",
			Hosts:    hostsHTTP,
		})
	}

	// HTTPS server
	if len(hostsHTTPS) > 0 {
		servers = append(servers, gatewayServer{
			Port:     443,
			Protocol: "HTTPS",
			Hosts:    hostsHTTPS,
			TLS: &gatewayTLS{
				Mode:           "SIMPLE",
				CredentialName: fmt.Sprintf("%s-tls", g.spec.App.Name),
			},
		})
	}

	// Use default namespace for gateway
	namespace := "default"
	if len(g.spec.App.Namespaces) > 0 {
		namespace = g.spec.App.Namespaces[0]
	}

	data := istioGatewayData{
		Name:      g.spec.App.Name,
		Namespace: namespace,
		AppName:   g.spec.App.Name,
		Servers:   servers,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "gateway.yaml.tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ingressVirtualServiceData holds data for ingress VirtualService
type ingressVirtualServiceData struct {
	Name       string
	Namespace  string
	AppName    string
	Hosts      []string
	Gateways   []string
	HTTPRoutes []ingressHTTPRoute
}

type ingressHTTPRoute struct {
	Match       []matchCondition
	Destination destination
}

func (g *GatewayGenerator) generateIngressVirtualService(svc types.ServiceConfig) (string, error) {
	host := svc.Ingress.Host
	if host == "" {
		host = "*"
	}

	paths := svc.Ingress.Paths
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var routes []ingressHTTPRoute
	for _, path := range paths {
		routes = append(routes, ingressHTTPRoute{
			Match: []matchCondition{
				{URIPrefix: path},
			},
			Destination: destination{
				Host:      svc.Name,
				Namespace: svc.Namespace,
				Port:      svc.Ports.HTTP,
			},
		})
	}

	data := ingressVirtualServiceData{
		Name:       fmt.Sprintf("%s-ingress", svc.Name),
		Namespace:  svc.Namespace,
		AppName:    g.spec.App.Name,
		Hosts:      []string{host},
		Gateways:   []string{fmt.Sprintf("%s/%s", g.spec.App.Namespaces[0], g.spec.App.Name)},
		HTTPRoutes: routes,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "ingress-virtualservice.yaml.tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
