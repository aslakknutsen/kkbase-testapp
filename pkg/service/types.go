package service

import "time"

// Response represents the standard response structure
type Response struct {
	Service          *ServiceInfo   `json:"service"`
	StartTime        string         `json:"start_time"`
	EndTime          string         `json:"end_time"`
	Duration         string         `json:"duration"`
	Code             int            `json:"code"`
	Body             string         `json:"body,omitempty"`
	TraceID          string         `json:"trace_id,omitempty"`
	SpanID           string         `json:"span_id,omitempty"`
	UpstreamCalls    []UpstreamCall `json:"upstream_calls,omitempty"`
	BehaviorsApplied []string       `json:"behaviors_applied,omitempty"`
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

// UpstreamCall represents a call to an upstream service
type UpstreamCall struct {
	Name             string         `json:"name"`
	URI              string         `json:"uri"`
	Protocol         string         `json:"protocol"`
	Duration         string         `json:"duration"`
	Code             int            `json:"code"`
	Error            string         `json:"error,omitempty"`
	BehaviorsApplied []string       `json:"behaviors_applied,omitempty"`
	UpstreamCalls    []UpstreamCall `json:"upstream_calls,omitempty"`
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
		UpstreamCalls:    []UpstreamCall{},
		BehaviorsApplied: []string{},
	}
}

// Finalize completes the response with end time and duration
func (r *Response) Finalize(start time.Time) {
	end := time.Now()
	r.EndTime = end.Format(time.RFC3339Nano)
	r.Duration = end.Sub(start).String()
}
