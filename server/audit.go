package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEntry represents a single structured audit log entry.
type AuditEntry struct {
	Timestamp       time.Time `json:"ts"`
	Subject         string    `json:"subject,omitempty"`
	Email           string    `json:"email,omitempty"`
	TenantID        string    `json:"tenant_id,omitempty"`
	Roles           []string  `json:"roles,omitempty"`
	Operation       string    `json:"operation"`
	Collection      string    `json:"collection,omitempty"`
	Query           string    `json:"query,omitempty"`
	FiltersInjected []string  `json:"filters_injected,omitempty"`
	LimitCapped     *int      `json:"limit_capped,omitempty"`
	Allowed         bool      `json:"allowed"`
	Denied          bool      `json:"denied"`
	DeniedReason    string    `json:"denied_reason,omitempty"`
	LatencyMs       int64     `json:"latency_ms"`
	Status          string    `json:"status"`
	Error           string    `json:"error,omitempty"`
	RuleIndex       int       `json:"rule_index,omitempty"`
}

// AuditLogger writes structured JSON audit entries to an output writer.
type AuditLogger struct {
	mu     sync.Mutex
	writer io.Writer
	enable bool
}

// NewAuditLogger creates an audit logger that writes to the given writer.
// If writer is nil, writes to stderr.
func NewAuditLogger(w io.Writer, enable bool) *AuditLogger {
	if w == nil {
		w = os.Stderr
	}
	return &AuditLogger{writer: w, enable: enable}
}

// Log writes a single audit entry as a JSON line.
func (al *AuditLogger) Log(entry AuditEntry) {
	if !al.enable {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error":"audit marshal failed: %s"}`+"\n", err)
		return
	}

	al.writer.Write(data)
	al.writer.Write([]byte("\n"))
}

// LogDenied logs a denied request (operation or collection not permitted).
func (al *AuditLogger) LogDenied(claims *JWTClaims, operation, collection, reason string, latency time.Duration) {
	entry := AuditEntry{
		Timestamp:    time.Now(),
		Operation:    operation,
		Collection:   collection,
		Allowed:      false,
		Denied:       true,
		DeniedReason: reason,
		LatencyMs:    latency.Milliseconds(),
		Status:       "denied",
	}
	if claims != nil {
		entry.Subject = claims.Subject
		entry.Email = claims.Email
		entry.TenantID = claims.TenantID
		entry.Roles = claims.Roles
	}
	al.Log(entry)
}

type auditMetaKey struct{}

// AuditMeta is a mutable object stored in the request context.
// Handlers populate this with AST data (e.g. injected filters, specific collection)
// so the interceptor can log it accurately.
type AuditMeta struct {
	Collection      string
	Query           string
	FiltersInjected []string
	RuleIndex       int
	LimitCapped     *int
}

// ContextWithAuditMeta injects the mutable AuditMeta into the context.
func ContextWithAuditMeta(ctx context.Context, meta *AuditMeta) context.Context {
	return context.WithValue(ctx, auditMetaKey{}, meta)
}

// ExtractAuditMeta retrieves the AuditMeta from the context.
func ExtractAuditMeta(ctx context.Context) *AuditMeta {
	m, ok := ctx.Value(auditMetaKey{}).(*AuditMeta)
	if !ok {
		return nil
	}
	return m
}
