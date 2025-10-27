package gateway

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"text/template"
	"time"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Generator generates Gateway API manifests
type Generator struct {
	spec      *types.AppSpec
	templates *template.Template
}

// Template data structures
type gatewayData struct {
	Name       string
	NeedsHTTP  bool
	NeedsHTTPS bool
}

type httpRouteData struct {
	Name        string
	Namespace   string
	GatewayName string
	Hostname    string
	Rules       []httpRouteRule
}

type httpRouteRule struct {
	Path             string
	BackendName      string
	BackendNamespace string
	BackendPort      int
}

type grpcRouteData struct {
	Name        string
	Namespace   string
	GatewayName string
	Hostname    string
	BackendName string
	BackendPort int
}

type tlsSecretData struct {
	CertBase64 string
	KeyBase64  string
}

type referenceGrantsData struct {
	Grants []referenceGrant
}

type referenceGrant struct {
	Name      string
	Namespace string
}

// NewGenerator creates a new Gateway API manifest generator
func NewGenerator(spec *types.AppSpec) *Generator {
	// Parse templates
	tmpl := template.Must(template.New("gateway").ParseFS(templatesFS, "templates/*.tmpl"))

	return &Generator{
		spec:      spec,
		templates: tmpl,
	}
}

// GenerateAll generates all Gateway API manifests
func (g *Generator) GenerateAll() (map[string]string, error) {
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

	// Generate Gateway
	gateway := g.GenerateGateway()
	manifests["20-gateway/gateway.yaml"] = gateway

	// Generate TLS certificates if needed
	needsTLS := false
	for _, svc := range ingressServices {
		if svc.Ingress.TLS {
			needsTLS = true
			break
		}
	}

	if needsTLS {
		certs, err := g.GenerateTLSSecrets(ingressServices)
		if err != nil {
			return nil, fmt.Errorf("failed to generate TLS secrets: %w", err)
		}
		manifests["20-gateway/certificates.yaml"] = certs
	}

	// Generate HTTPRoute or GRPCRoute for each service
	for _, svc := range ingressServices {
		if svc.HasGRPC() && !svc.HasHTTP() {
			route := g.GenerateGRPCRoute(&svc)
			manifests[fmt.Sprintf("20-gateway/%s-grpcroute.yaml", svc.Name)] = route
		} else {
			route := g.GenerateHTTPRoute(&svc)
			manifests[fmt.Sprintf("20-gateway/%s-httproute.yaml", svc.Name)] = route
		}
	}

	// Generate ReferenceGrants for cross-namespace access
	grants := g.GenerateReferenceGrants(ingressServices)
	if grants != "" {
		manifests["20-gateway/referencegrants.yaml"] = grants
	}

	return manifests, nil
}

// GenerateGateway generates a Gateway manifest
func (g *Generator) GenerateGateway() string {
	// Determine if we need HTTP and/or HTTPS listeners
	needsHTTP := false
	needsHTTPS := false

	for _, svc := range g.spec.Services {
		if svc.NeedsIngress() {
			needsHTTP = true
			if svc.Ingress.TLS {
				needsHTTPS = true
			}
		}
	}

	data := gatewayData{
		Name:       g.spec.App.Name,
		NeedsHTTP:  needsHTTP,
		NeedsHTTPS: needsHTTPS,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "gateway.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute gateway template: %v", err))
	}
	return buf.String()
}

// GenerateHTTPRoute generates an HTTPRoute manifest
func (g *Generator) GenerateHTTPRoute(svc *types.ServiceConfig) string {
	paths := svc.Ingress.Paths
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var rules []httpRouteRule
	for _, path := range paths {
		rules = append(rules, httpRouteRule{
			Path:             path,
			BackendName:      svc.Name,
			BackendNamespace: svc.Namespace,
			BackendPort:      svc.Ports.HTTP,
		})
	}

	data := httpRouteData{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		GatewayName: g.spec.App.Name,
		Hostname:    svc.Ingress.Host,
		Rules:       rules,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "httproute.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute httproute template: %v", err))
	}
	return buf.String()
}

// GenerateGRPCRoute generates a GRPCRoute manifest
func (g *Generator) GenerateGRPCRoute(svc *types.ServiceConfig) string {
	data := grpcRouteData{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		GatewayName: g.spec.App.Name,
		Hostname:    svc.Ingress.Host,
		BackendName: svc.Name,
		BackendPort: svc.Ports.GRPC,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "grpcroute.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute grpcroute template: %v", err))
	}
	return buf.String()
}

// GenerateTLSSecrets generates self-signed TLS certificates
func (g *Generator) GenerateTLSSecrets(services []types.ServiceConfig) (string, error) {
	// Collect all unique hosts
	hosts := make(map[string]bool)
	for _, svc := range services {
		if svc.Ingress.TLS && svc.Ingress.Host != "" {
			hosts[svc.Ingress.Host] = true
		}
	}

	if len(hosts) == 0 {
		hosts["*.local"] = true
	}

	// Generate self-signed certificate
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"TestApp"},
			CommonName:   g.spec.App.Name,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for host := range hosts {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", err
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Base64 encode for Secret
	certBase64 := base64.StdEncoding.EncodeToString(certPEM)
	keyBase64 := base64.StdEncoding.EncodeToString(keyPEM)

	data := tlsSecretData{
		CertBase64: certBase64,
		KeyBase64:  keyBase64,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "secret-tls.yaml.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute tls secret template: %w", err)
	}
	return buf.String(), nil
}

// GenerateReferenceGrants generates ReferenceGrant manifests for cross-namespace access
func (g *Generator) GenerateReferenceGrants(services []types.ServiceConfig) string {
	// Find services in namespaces other than default
	namespaces := make(map[string]bool)
	for _, svc := range services {
		if svc.Namespace != "" && svc.Namespace != "default" {
			namespaces[svc.Namespace] = true
		}
	}

	if len(namespaces) == 0 {
		return ""
	}

	var grants []referenceGrant
	for ns := range namespaces {
		grants = append(grants, referenceGrant{
			Name:      fmt.Sprintf("%s-to-%s", g.spec.App.Name, ns),
			Namespace: ns,
		})
	}

	data := referenceGrantsData{
		Grants: grants,
	}

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "referencegrant.yaml.tmpl", data); err != nil {
		panic(fmt.Sprintf("failed to execute referencegrant template: %v", err))
	}
	return buf.String()
}
