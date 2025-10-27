package service

import "time"

// Response represents the standard response structure
// Also used to represent upstream calls (where it captures another service's response)
type Response struct {
	Service          *ServiceInfo `json:"service,omitempty"` // Service info (nil for upstream call stubs)
	URL              string       `json:"url,omitempty"`     // URL that was called (may include path for HTTP)
	StartTime        string       `json:"start_time,omitempty"`
	EndTime          string       `json:"end_time,omitempty"`
	Duration         string       `json:"duration"`
	Code             int          `json:"code"`
	Body             string       `json:"body,omitempty"`
	Error            string       `json:"error,omitempty"` // Error message (for failed calls)
	TraceID          string       `json:"trace_id,omitempty"`
	SpanID           string       `json:"span_id,omitempty"`
	UpstreamCalls    []Response   `json:"upstream_calls,omitempty"` // Recursive: upstream responses
	BehaviorsApplied []string     `json:"behaviors_applied,omitempty"`
}

// ServiceInfo describes the service
type ServiceInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Node      string `json:"node,omitempty"`
	Protocol  string `json:"protocol"`
}

// NewResponse creates a new response with service info
func NewResponse(cfg *Config, protocol string) *Response {
	now := time.Now()
	return &Response{
		Service: &ServiceInfo{
			Name:      cfg.Name,
			Version:   cfg.Version,
			Namespace: cfg.Namespace,
			Pod:       cfg.PodName,
			Node:      cfg.NodeName,
			Protocol:  protocol,
		},
		StartTime:        now.Format(time.RFC3339Nano),
		UpstreamCalls:    []Response{},
		BehaviorsApplied: []string{},
	}
}

// Finalize completes the response with end time and duration
func (r *Response) Finalize(start time.Time) {
	end := time.Now()
	r.EndTime = end.Format(time.RFC3339Nano)
	r.Duration = end.Sub(start).String()
}
