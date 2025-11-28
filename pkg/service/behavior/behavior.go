package behavior

import (
	"context"
	"fmt"
	"strings"
)

// Behavior represents parsed behavior directives
type Behavior struct {
	Latency         *LatencyBehavior
	Error           *ErrorBehavior
	CPU             *CPUBehavior
	Memory          *MemoryBehavior
	Panic           *PanicBehavior
	CrashIfFile     *CrashIfFileBehavior
	ErrorIfFile     *ErrorIfFileBehavior
	Disk            *DiskBehavior
	UpstreamWeights *UpstreamWeightsBehavior // Weights for grouped upstreams (ID -> weight)
}

// ServiceBehavior represents a behavior targeted at a specific service
type ServiceBehavior struct {
	Service  string    // Target service name (empty = applies to all)
	Behavior *Behavior // The actual behavior
}

// BehaviorChain represents multiple behaviors that can target different services
type BehaviorChain struct {
	Behaviors []ServiceBehavior
}

// ForService returns the behavior applicable to the given service name
func (bc *BehaviorChain) ForService(serviceName string) *Behavior {
	var specificBehavior *Behavior
	var globalBehavior *Behavior

	for _, sb := range bc.Behaviors {
		if sb.Service == serviceName {
			// Found behavior specifically for this service
			if specificBehavior == nil {
				specificBehavior = sb.Behavior
			} else {
				// Merge multiple behaviors for same service
				specificBehavior = mergeBehaviors(specificBehavior, sb.Behavior)
			}
		} else if sb.Service == "" {
			// Global behavior (no service prefix)
			if globalBehavior == nil {
				globalBehavior = sb.Behavior
			} else {
				globalBehavior = mergeBehaviors(globalBehavior, sb.Behavior)
			}
		}
	}

	// Specific behavior takes precedence over global
	if specificBehavior != nil {
		return specificBehavior
	}
	return globalBehavior
}

// String returns the behavior chain as a string for propagation
func (bc *BehaviorChain) String() string {
	if len(bc.Behaviors) == 0 {
		return ""
	}

	var parts []string
	for _, sb := range bc.Behaviors {
		behaviorStr := sb.Behavior.String()
		if behaviorStr == "" {
			continue
		}

		if sb.Service != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", sb.Service, behaviorStr))
		} else {
			parts = append(parts, behaviorStr)
		}
	}

	return strings.Join(parts, ",")
}

// String returns the behavior as a string
func (b *Behavior) String() string {
	var parts []string

	if b.Latency != nil {
		parts = append(parts, b.Latency.String())
	}

	if b.Error != nil && b.Error.Prob > 0 {
		parts = append(parts, b.Error.String())
	}

	if b.Panic != nil && b.Panic.Prob > 0 {
		parts = append(parts, b.Panic.String())
	}

	if b.CrashIfFile != nil {
		parts = append(parts, b.CrashIfFile.String())
	}

	if b.ErrorIfFile != nil {
		parts = append(parts, b.ErrorIfFile.String())
	}

	if b.CPU != nil {
		parts = append(parts, b.CPU.String())
	}

	if b.Memory != nil {
		parts = append(parts, b.Memory.String())
	}

	if b.Disk != nil {
		parts = append(parts, b.Disk.String())
	}

	if b.UpstreamWeights != nil {
		parts = append(parts, b.UpstreamWeights.String())
	}

	return strings.Join(parts, ",")
}

// mergeBehaviors combines two behaviors (b2 takes precedence over b1)
func mergeBehaviors(b1, b2 *Behavior) *Behavior {
	return &Behavior{
		Latency:         mergeField(b1.Latency, b2.Latency),
		Error:           mergeField(b1.Error, b2.Error),
		CPU:             mergeField(b1.CPU, b2.CPU),
		Memory:          mergeField(b1.Memory, b2.Memory),
		Panic:           mergeField(b1.Panic, b2.Panic),
		CrashIfFile:     mergeField(b1.CrashIfFile, b2.CrashIfFile),
		ErrorIfFile:     mergeField(b1.ErrorIfFile, b2.ErrorIfFile),
		Disk:            mergeField(b1.Disk, b2.Disk),
		UpstreamWeights: mergeField(b1.UpstreamWeights, b2.UpstreamWeights),
	}
}

// Parse parses a behavior string into a Behavior struct using the registry
// Format: "latency=100ms,error=503:0.1,cpu=spike:5s,memory=leak-slow:10m"
func Parse(behaviorStr string) (*Behavior, error) {
	if behaviorStr == "" {
		return &Behavior{}, nil
	}

	b := &Behavior{}

	parts := strings.Split(behaviorStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		// Look up parser in registry
		if parser, ok := parsers[key]; ok {
			if err := parser(b, value); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("unknown behavior key: %s", key)
		}
	}

	return b, nil
}

// ParseChain parses a behavior chain that can target specific services
// Syntax: "service1:latency=100ms,service2:error=0.5,latency=50ms"
// - "service1:latency=100ms" - applies only to service1
// - "latency=50ms" - applies to all services (no prefix)
func ParseChain(behaviorStr string) (*BehaviorChain, error) {
	if behaviorStr == "" {
		return &BehaviorChain{Behaviors: []ServiceBehavior{}}, nil
	}

	chain := &BehaviorChain{
		Behaviors: []ServiceBehavior{},
	}

	// Split by comma, but need to handle service:key=value format
	// Strategy: Look for patterns like "service:" or "key="
	var currentService string
	var currentBehaviorParts []string

	parts := strings.Split(behaviorStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part has a service prefix (contains : before =)
		colonPos := strings.Index(part, ":")
		equalsPos := strings.Index(part, "=")

		if colonPos > 0 && (equalsPos < 0 || colonPos < equalsPos) {
			// This is a service prefix: "service:latency=100ms"
			// Save previous behavior if any
			if len(currentBehaviorParts) > 0 {
				b, err := Parse(strings.Join(currentBehaviorParts, ","))
				if err != nil {
					return nil, err
				}
				chain.Behaviors = append(chain.Behaviors, ServiceBehavior{
					Service:  currentService,
					Behavior: b,
				})
				currentBehaviorParts = nil
			}

			// Extract service name and behavior
			serviceName := strings.TrimSpace(part[:colonPos])
			behaviorPart := strings.TrimSpace(part[colonPos+1:])

			currentService = serviceName
			if behaviorPart != "" {
				currentBehaviorParts = append(currentBehaviorParts, behaviorPart)
			}
		} else {
			// This is a regular behavior part
			currentBehaviorParts = append(currentBehaviorParts, part)
		}
	}

	// Don't forget the last behavior
	if len(currentBehaviorParts) > 0 {
		b, err := Parse(strings.Join(currentBehaviorParts, ","))
		if err != nil {
			return nil, err
		}
		chain.Behaviors = append(chain.Behaviors, ServiceBehavior{
			Service:  currentService,
			Behavior: b,
		})
	}

	return chain, nil
}

// Apply applies the behavior to the current request
func (b *Behavior) Apply(ctx context.Context) error {
	if b.Latency != nil {
		if err := b.applyLatency(ctx); err != nil {
			return err
		}
	}

	if b.CPU != nil {
		b.applyCPU(ctx)
	}

	if b.Memory != nil {
		b.applyMemory(ctx)
	}

	return nil
}
