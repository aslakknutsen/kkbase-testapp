# CLI Reference

Complete reference for the `testgen` command-line tool.

## Overview

TestGen is a CLI tool that generates Kubernetes and Gateway API manifests from YAML DSL files.

```bash
testgen [command] [flags]
```

## Commands

### generate

Generate Kubernetes manifests from a DSL file.

**Usage:**
```bash
testgen generate <dsl-file> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--output-dir` | `-o` | string | "./output" | Output directory for generated manifests |
| `--image` | `-i` | string | "testservice:latest" | TestService container image |
| `--validate-only` | | bool | false | Only validate, don't generate |

**Examples:**

Basic generation:
```bash
testgen generate examples/simple-web/app.yaml
```

Custom output directory:
```bash
testgen generate examples/simple-web/app.yaml -o /tmp/manifests
```

Custom image:
```bash
testgen generate examples/simple-web/app.yaml --image myregistry/testservice:v2.0
```

Validation only:
```bash
testgen generate examples/simple-web/app.yaml --validate-only
```

**Output Structure:**

```
output/<app-name>/
├── 00-namespaces.yaml
├── 10-services/
│   ├── <service>-deployment.yaml
│   ├── <service>-service.yaml
│   ├── <service>-servicemonitor.yaml
│   ├── <service>-statefulset.yaml (for StatefulSets)
│   └── ...
├── 20-gateway/
│   ├── gateway.yaml
│   ├── certificates.yaml
│   ├── <service>-httproute.yaml
│   ├── <service>-grpcroute.yaml
│   └── referencegrants.yaml
└── README.md
```

### validate

Validate a DSL file without generating manifests.

**Usage:**
```bash
testgen validate <dsl-file>
```

**Examples:**

```bash
testgen validate examples/simple-web/app.yaml
```

**Validations Performed:**
- YAML syntax
- Required fields present
- Service name uniqueness
- Upstream references exist
- Circular dependency detection
- Protocol compatibility
- StatefulSet requirements
- Namespace declarations

**Exit Codes:**
- `0` - Validation successful
- `1` - Validation failed

### init

Create a new application template.

**Usage:**
```bash
testgen init <app-name> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--output` | `-o` | string | "./<app-name>.yaml" | Output file path |

**Examples:**

```bash
testgen init my-app
```

Creates `my-app.yaml` with a basic template.

Custom output location:
```bash
testgen init my-app -o config/apps/my-app.yaml
```

**Generated Template:**

```yaml
app:
  name: my-app
  namespaces:
    - default

services:
  - name: frontend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
    upstreams: [backend]
    ingress:
      enabled: true
      host: myapp.local
      tls: true

  - name: backend
    namespace: default
    type: Deployment
    replicas: 2
    protocols: [http]
    upstreams: [database]

  - name: database
    namespace: default
    type: StatefulSet
    replicas: 1
    protocols: [http]
    storage:
      size: 1Gi
```

### examples

List available example applications.

**Usage:**
```bash
testgen examples
```

**Output:**

```
Available examples:

  simple-web
    3-tier application: frontend → api → database
    Location: examples/simple-web/app.yaml

  ecommerce
    8 services across 4 namespaces with mixed HTTP/gRPC protocols
    Location: examples/ecommerce/app.yaml

  microservices
    15+ services with complex topology
    Location: examples/microservices/app.yaml
```

## Global Flags

These flags apply to all commands.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--help` | `-h` | bool | false | Show help for command |
| `--version` | `-v` | bool | false | Show version information |

**Examples:**

```bash
testgen --version
testgen generate --help
```

## Environment Variables

### TESTSERVICE_IMAGE

Override default TestService image.

```bash
export TESTSERVICE_IMAGE=myregistry/testservice:v2
testgen generate examples/simple-web/app.yaml
```

Equivalent to:
```bash
testgen generate examples/simple-web/app.yaml --image myregistry/testservice:v2
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (validation failed, file not found, etc.) |
| 2 | Invalid command-line arguments |

## Common Workflows

### Create and Deploy New Application

```bash
# 1. Initialize
testgen init my-app

# 2. Edit DSL
vim my-app.yaml

# 3. Validate
testgen validate my-app.yaml

# 4. Generate manifests
testgen generate my-app.yaml -o output

# 5. Deploy
kubectl apply -f output/my-app/
```

### Update Existing Application

```bash
# 1. Edit DSL
vim my-app.yaml

# 2. Validate changes
testgen validate my-app.yaml

# 3. Regenerate manifests
testgen generate my-app.yaml -o output

# 4. Apply changes
kubectl apply -f output/my-app/
```

### Test with Custom Image

```bash
# Build new image
docker build -t testservice:test .

# Generate with test image
testgen generate my-app.yaml --image testservice:test

# Deploy
kubectl apply -f output/my-app/
```

### Validate Multiple Files

```bash
#!/bin/bash
for file in examples/**/app.yaml; do
    echo "Validating $file..."
    testgen validate "$file" || exit 1
done
echo "All files valid"
```

## Configuration Files

TestGen looks for configuration in these locations:

1. `.testgen.yaml` in current directory
2. `~/.testgen.yaml` in home directory

**Example `.testgen.yaml`:**

```yaml
default-image: myregistry/testservice:latest
output-dir: ./manifests
```

**Note:** Configuration file support is planned but not yet implemented.

## Tips

**Use validate before generate:**
```bash
testgen validate app.yaml && testgen generate app.yaml
```

**Generate to temporary directory for review:**
```bash
testgen generate app.yaml -o /tmp/review
ls -R /tmp/review
```

**Use version control for DSL files:**
```bash
git add my-app.yaml
git commit -m "Add order service"
```

**Keep generated manifests out of version control:**
```bash
echo "output/" >> .gitignore
```

## Troubleshooting

### Command not found

Ensure testgen is in PATH:
```bash
export PATH=$PATH:$(pwd)
./testgen --version
```

### Validation errors

Read error message carefully:
```bash
testgen validate app.yaml
# Error: circular dependency detected: api -> database -> api
```

Fix the DSL and revalidate.

### Permission denied

Check file permissions:
```bash
chmod +x testgen
```

### Invalid YAML

Use a YAML validator:
```bash
yamllint app.yaml
```

## See Also

- [DSL Reference](dsl-spec.md) - YAML DSL specification
- [Quick Start](../getting-started/quickstart.md) - Getting started guide
- [Examples](../../examples/) - Example applications

