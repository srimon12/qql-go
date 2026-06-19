package server

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// TemplateEngine manages named query templates that agents and applications
// can invoke instead of writing raw QQL. Templates are defined in a YAML file
// and support variable substitution from JWT claims and caller-provided params.
type TemplateEngine struct {
	mu        sync.RWMutex
	templates map[string]Template
	path      string
}

// Template is a single named query template.
type Template struct {
	// Description explains what this template does.
	Description string `yaml:"description"`
	// Query is the QQL template string with {variable} placeholders.
	Query string `yaml:"query"`
	// RequireClaims lists JWT claims that must be present to use this template.
	RequireClaims []string `yaml:"require_claims"`
	// MaxLimit overrides the template's LIMIT if the policy has a lower cap.
	MaxLimit int `yaml:"max_limit"`
}

// TemplateConfig is the top-level YAML structure.
type TemplateConfig struct {
	Templates map[string]Template `yaml:"templates"`
}

// TemplateRequest is what a client sends to invoke a template.
type TemplateRequest struct {
	// Name is the template identifier.
	Name string `json:"name"`
	// Params are the caller-provided variables.
	Params map[string]string `json:"params"`
}

// NewTemplateEngine loads templates from a YAML file.
func NewTemplateEngine(path string) (*TemplateEngine, error) {
	te := &TemplateEngine{path: path}
	if err := te.load(); err != nil {
		return nil, err
	}
	return te, nil
}

func (te *TemplateEngine) load() error {
	data, err := os.ReadFile(te.path)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", te.path, err)
	}
	var cfg TemplateConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse template file %s: %w", te.path, err)
	}
	if cfg.Templates == nil {
		cfg.Templates = make(map[string]Template)
	}
	te.templates = cfg.Templates
	return nil
}

// Reload re-reads the template file from disk.
func (te *TemplateEngine) Reload() error {
	te.mu.Lock()
	defer te.mu.Unlock()
	return te.load()
}

// Get returns a template by name.
func (te *TemplateEngine) Get(name string) (Template, bool) {
	te.mu.RLock()
	defer te.mu.RUnlock()
	t, ok := te.templates[name]
	return t, ok
}

// List returns all template names.
func (te *TemplateEngine) List() []string {
	te.mu.RLock()
	defer te.mu.RUnlock()
	names := make([]string, 0, len(te.templates))
	for name := range te.templates {
		names = append(names, name)
	}
	return names
}

// Resolve expands a template with the given params and JWT claims.
// Variables use {name} syntax. Sources (in priority order):
//  1. Caller-provided params
//  2. JWT claims (prefixed with "claims." in the template, e.g. {claims.org_id})
//  3. Static defaults from the template itself
//
// Returns the resolved QQL query string.
func (te *TemplateEngine) Resolve(name string, params map[string]string, claims *JWTClaims) (string, error) {
	te.mu.RLock()
	tmpl, ok := te.templates[name]
	te.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	// Check required claims.
	if claims != nil {
		for _, required := range tmpl.RequireClaims {
			val := extractStringClaimFromJWT(claims, required)
			if val == "" {
				return "", fmt.Errorf("template %q requires claim %q which is not present", name, required)
			}
		}
	}

	// Build variable map: params override claims.
	vars := make(map[string]string)
	if claims != nil {
		vars["claims.sub"] = claims.Subject
		vars["claims.email"] = claims.Email
		vars["claims.tenant_id"] = claims.TenantID
		// Copy all raw claims.
		for k, v := range claims.Raw {
			if s, ok := v.(string); ok {
				vars["claims."+k] = s
			}
		}
	}
	maps.Copy(vars, params)

	// Resolve {variables}.
	result := varPattern.ReplaceAllStringFunc(tmpl.Query, func(match string) string {
		key := match[1 : len(match)-1] // strip { and }
		if val, ok := vars[key]; ok {
			return val
		}
		// Return original if not found — don't silently drop.
		return match
	})

	return result, nil
}

var varPattern = regexp.MustCompile(`\{[^}]+\}`)

// TemplateExecRequest is the wire format for the template Exec RPC extension.
type TemplateExecRequest struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params"`
}

// TemplateListEntry is a single template in the list response.
type TemplateListEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Query       string   `json:"query"`
	Params      []string `json:"params"`
}

// ExtractParams returns the {variable} names from a template's query.
func (t Template) ExtractParams() []string {
	matches := varPattern.FindAllString(t.Query, -1)
	seen := make(map[string]bool)
	var params []string
	for _, m := range matches {
		key := m[1 : len(m)-1]
		if !strings.HasPrefix(key, "claims.") && !seen[key] {
			seen[key] = true
			params = append(params, key)
		}
	}
	return params
}
