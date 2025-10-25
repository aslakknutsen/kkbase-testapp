package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/kagenti/kkbase/testapp/pkg/dsl/types"
)

// Generator generates Gateway API manifests
type Generator struct {
	spec *types.AppSpec
}

// NewGenerator creates a new Gateway API manifest generator
func NewGenerator(spec *types.AppSpec) *Generator {
	return &Generator{spec: spec}
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

	var listeners strings.Builder

	if needsHTTP {
		listeners.WriteString(`  - name: http
    protocol: HTTP
    port: 80
`)
	}

	if needsHTTPS {
		listeners.WriteString(`  - name: https
    protocol: HTTPS
    port: 443
    tls:
      mode: Terminate
      certificateRefs:
      - name: gateway-tls-cert
`)
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s-gateway
  namespace: default
spec:
  gatewayClassName: openshift-default
  listeners:
%s
`,
		g.spec.App.Name,
		listeners.String(),
	)
}

// GenerateHTTPRoute generates an HTTPRoute manifest
func (g *Generator) GenerateHTTPRoute(svc *types.ServiceConfig) string {
	paths := svc.Ingress.Paths
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var rules strings.Builder
	for _, path := range paths {
		rules.WriteString(fmt.Sprintf(`  - matches:
    - path:
        type: PathPrefix
        value: %s
    backendRefs:
    - name: %s
      namespace: %s
      port: %d
`,
			path,
			svc.Name,
			svc.Namespace,
			svc.Ports.HTTP,
		))
	}

	hostnames := ""
	if svc.Ingress.Host != "" {
		hostnames = fmt.Sprintf(`  hostnames:
  - %s
`, svc.Ingress.Host)
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %s
  namespace: %s
spec:
  parentRefs:
  - name: %s-gateway
    namespace: default
%s  rules:
%s
`,
		svc.Name,
		svc.Namespace,
		g.spec.App.Name,
		hostnames,
		rules.String(),
	)
}

// GenerateGRPCRoute generates a GRPCRoute manifest
func (g *Generator) GenerateGRPCRoute(svc *types.ServiceConfig) string {
	hostnames := ""
	if svc.Ingress.Host != "" {
		hostnames = fmt.Sprintf(`  hostnames:
  - %s
`, svc.Ingress.Host)
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1alpha2
kind: GRPCRoute
metadata:
  name: %s
  namespace: %s
spec:
  parentRefs:
  - name: %s-gateway
    namespace: default
%s  rules:
  - backendRefs:
    - name: %s
      port: %d
`,
		svc.Name,
		svc.Namespace,
		g.spec.App.Name,
		hostnames,
		svc.Name,
		svc.Ports.GRPC,
	)
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

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: gateway-tls-cert
  namespace: default
type: kubernetes.io/tls
data:
  tls.crt: %s
  tls.key: %s
`,
		certBase64,
		keyBase64,
	), nil
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

	var b strings.Builder
	first := true
	for ns := range namespaces {
		if !first {
			b.WriteString("---\n")
		}
		first = false

		b.WriteString(fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: %s-to-%s
  namespace: %s
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: default
  - group: gateway.networking.k8s.io
    kind: GRPCRoute
    namespace: default
  to:
  - group: ""
    kind: Service
`,
			g.spec.App.Name,
			ns,
			ns,
		))
	}

	return b.String()
}
