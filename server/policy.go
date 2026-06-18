package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// PolicyConfig is the top-level policy configuration loaded from YAML.
type PolicyConfig struct {
	Rules []PolicyRule `yaml:"rules"`
}

// PolicyRule defines a single policy rule that matches JWT claims and specifies
// what operations are allowed, what collections are accessible, and what
// filters to inject.
type PolicyRule struct {
	// Match specifies claim-based conditions. All must match (AND logic).
	Match PolicyMatch `yaml:"match"`

	// Allow lists the QQL operation types permitted for this rule.
	// Supported: QUERY, INSERT, UPDATE, DELETE, SCROLL, SELECT, SHOW,
	// CREATE, DROP, ALTER, EXPLAIN.
	// If empty, defaults to read-only (QUERY, SCROLL, SELECT, SHOW, EXPLAIN).
	Allow []string `yaml:"allow"`

	// Deny explicitly blocks operation types. Takes precedence over Allow.
	Deny []string `yaml:"deny"`

	// Collections is a glob-pattern allowlist for collection names.
	// If empty, all collections are accessible.
	Collections []string `yaml:"collections"`

	// Inject specifies automatic filter injection into queries.
	Inject PolicyInject `yaml:"inject"`

	// Limits contains resource constraints.
	Limits PolicyLimits `yaml:"limits"`
}

// PolicyMatch defines claim-based matching conditions.
type PolicyMatch struct {
	// Authenticated matches if set to true — matches any valid JWT.
	Authenticated *bool `yaml:"authenticated"`

	// Claims matches specific JWT claim values.
	// Supports string claims and string-slice claims (matches if any value matches).
	Claims map[string]string `yaml:"claims"`
}

// PolicyInject specifies automatic filter injection.
type PolicyInject struct {
	// Where defines a WHERE clause to inject into every query.
	Where *InjectWhere `yaml:"where"`
}

// InjectWhere defines a single WHERE condition to inject.
type InjectWhere struct {
	// Field is the payload field name to filter on.
	Field string `yaml:"field"`

	// FromClaim extracts the value from a JWT claim.
	// Mutually exclusive with Value.
	FromClaim string `yaml:"from_claim"`

	// Value is a static value to inject.
	// Mutually exclusive with FromClaim.
	Value string `yaml:"value"`

	// Op is the comparison operator. Defaults to "=".
	// Supported: =, !=, in, not_in
	Op string `yaml:"op"`
}

// PolicyLimits defines resource constraints.
type PolicyLimits struct {
	// MaxLimit caps the LIMIT value in queries. 0 means no cap.
	MaxLimit int `yaml:"max_limit"`
}

// EvaluatedPolicy is the result of matching a policy rule against a request.
type EvaluatedPolicy struct {
	Allowed         bool
	AllowOps        map[string]bool
	DenyOps         map[string]bool
	Collections     []string
	InjectField     string // payload field name to filter on (e.g. "tenant_id")
	InjectFromClaim string // JWT claim path to extract value from (e.g. "org_id")
	InjectValue     string // static value (mutually exclusive with InjectFromClaim)
	InjectOp        string
	MaxLimit        int
	MatchedRule     int // index of matched rule, -1 if none
}

// PolicyEngine evaluates JWT claims against policy rules.
type PolicyEngine struct {
	config PolicyConfig
	mu     sync.RWMutex
	path   string
}

// NewPolicyEngine creates a policy engine from a YAML file.
func NewPolicyEngine(path string) (*PolicyEngine, error) {
	pe := &PolicyEngine{path: path}
	if err := pe.load(); err != nil {
		return nil, err
	}
	return pe, nil
}

// NewPolicyEngineFromConfig creates a policy engine from an in-memory config.
func NewPolicyEngineFromConfig(cfg PolicyConfig) *PolicyEngine {
	return &PolicyEngine{config: cfg}
}

func (pe *PolicyEngine) load() error {
	data, err := os.ReadFile(pe.path)
	if err != nil {
		return fmt.Errorf("failed to read policy file %s: %w", pe.path, err)
	}
	var cfg PolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse policy file %s: %w", pe.path, err)
	}
	pe.config = cfg
	return nil
}

// Reload re-reads the policy file from disk.
func (pe *PolicyEngine) Reload() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	return pe.load()
}

// Evaluate matches JWT claims against policy rules and returns the effective policy.
// Rules are evaluated in order; the first matching rule wins.
func (pe *PolicyEngine) Evaluate(claims *JWTClaims) EvaluatedPolicy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	for i, rule := range pe.config.Rules {
		if pe.matchesRule(rule, claims) {
			return pe.buildPolicy(rule, i)
		}
	}

	// No rule matched — deny everything.
	return EvaluatedPolicy{
		Allowed:     false,
		MatchedRule: -1,
	}
}

func (pe *PolicyEngine) matchesRule(rule PolicyRule, claims *JWTClaims) bool {
	match := rule.Match

	// Check authenticated flag.
	if match.Authenticated != nil {
		if *match.Authenticated && claims == nil {
			return false
		}
		if !*match.Authenticated && claims != nil {
			return false
		}
	}

	// Check claim matches.
	for key, want := range match.Claims {
		if claims == nil {
			return false
		}
		got := extractStringClaimFromJWT(claims, key)
		if !claimValueMatches(got, want) {
			return false
		}
	}

	return true
}

func (pe *PolicyEngine) buildPolicy(rule PolicyRule, index int) EvaluatedPolicy {
	p := EvaluatedPolicy{
		Allowed:     true,
		MatchedRule: index,
	}

	// Build allow/deny sets.
	if len(rule.Allow) > 0 {
		p.AllowOps = make(map[string]bool, len(rule.Allow))
		for _, op := range rule.Allow {
			p.AllowOps[strings.ToUpper(op)] = true
		}
	}
	if len(rule.Deny) > 0 {
		p.DenyOps = make(map[string]bool, len(rule.Deny))
		for _, op := range rule.Deny {
			p.DenyOps[strings.ToUpper(op)] = true
		}
	}

	p.Collections = rule.Collections

	// Injection config.
	if rule.Inject.Where != nil {
		p.InjectField = rule.Inject.Where.Field
		p.InjectFromClaim = rule.Inject.Where.FromClaim
		p.InjectValue = rule.Inject.Where.Value
		p.InjectOp = rule.Inject.Where.Op
		if p.InjectOp == "" {
			p.InjectOp = "="
		}
	}

	p.MaxLimit = rule.Limits.MaxLimit

	return p
}

// IsOperationAllowed checks if a specific operation is permitted by the policy.
func (p *EvaluatedPolicy) IsOperationAllowed(op string) bool {
	op = strings.ToUpper(op)

	// Deny takes precedence.
	if p.DenyOps != nil && p.DenyOps[op] {
		return false
	}

	// If allow list is defined, operation must be in it.
	if p.AllowOps != nil {
		return p.AllowOps[op]
	}

	// No allow list defined — default to read-only.
	readOps := map[string]bool{
		"QUERY": true, "SCROLL": true, "SELECT": true,
		"SHOW": true, "EXPLAIN": true,
	}
	return readOps[op]
}

// IsCollectionAllowed checks if a collection name is permitted by the policy.
func (p *EvaluatedPolicy) IsCollectionAllowed(collection string) bool {
	if len(p.Collections) == 0 {
		return true
	}
	for _, pattern := range p.Collections {
		if matchGlob(pattern, collection) {
			return true
		}
	}
	return false
}

// ResolveInjectValue resolves the injection value from JWT claims.
func (p *EvaluatedPolicy) ResolveInjectValue(claims *JWTClaims) string {
	if p.InjectValue != "" {
		return p.InjectValue
	}
	if p.InjectFromClaim != "" && claims != nil {
		return extractStringClaimFromJWT(claims, p.InjectFromClaim)
	}
	return ""
}

// extractStringClaimFromJWT extracts a claim value from JWTClaims.
func extractStringClaimFromJWT(claims *JWTClaims, path string) string {
	if claims == nil || claims.Raw == nil {
		return ""
	}
	val := walkClaimPathFromMap(claims.Raw, path)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func walkClaimPathFromMap(m map[string]any, path string) any {
	parts := strings.Split(path, "#")
	var current any = m
	for _, part := range parts {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = cm[part]
	}
	return current
}

// claimValueMatches checks if a claim value matches the expected value.
// Supports exact match and "in" syntax (comma-separated).
func claimValueMatches(got, want string) bool {
	if got == want {
		return true
	}
	// Check if want is a comma-separated list (implicit "in").
	for item := range strings.SplitSeq(want, ",") {
		if strings.TrimSpace(item) == got {
			return true
		}
	}
	return false
}

// matchGlob does simple glob matching with * wildcard.
func matchGlob(pattern, name string) bool {
	if pattern == name {
		return true
	}
	if before, ok := strings.CutSuffix(pattern, "*"); ok {
		prefix := before
		return strings.HasPrefix(name, prefix)
	}
	if after, ok := strings.CutPrefix(pattern, "*"); ok {
		suffix := after
		return strings.HasSuffix(name, suffix)
	}
	// Check filepath.Match for more complex patterns.
	matched, _ := filepath.Match(pattern, name)
	return matched
}
