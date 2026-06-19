package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is unexported to prevent collisions.
type contextKey string

const claimsContextKey contextKey = "jwt_claims"

// JWTClaims holds the extracted claims from a validated JWT.
type JWTClaims struct {
	Subject  string
	Issuer   string
	Roles    []string
	TenantID string
	Email    string
	Raw      map[string]any
}

// JWKSConfig configures the JWT/JWKS interceptor.
type JWKSConfig struct {
	JWKSURL     string
	Issuer      string
	Audience    string
	CacheTTL    time.Duration
	RoleClaim   string // JWT claim path for roles, e.g. "role", "https://myapp.com/roles"
	TenantClaim string // JWT claim path for tenant, e.g. "org_id", "tenant"
}

// jwksCache holds a cached JWKS key set with expiry.
type jwksCache struct {
	mu      sync.RWMutex
	keys    map[string][]byte // kid → PEM-encoded public key
	rawKeys []jwksKey
	fetched time.Time
	ttl     time.Duration
	url     string
	client  *http.Client
}

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

func newJWKSCache(url string, ttl time.Duration) *jwksCache {
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	return &jwksCache{
		url:    url,
		ttl:    ttl,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// keyFunc returns a jwt.Keyfunc that validates against the cached JWKS.
func (c *jwksCache) keyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)

		keys, err := c.getKeys()
		if err != nil {
			return nil, fmt.Errorf("jwks fetch failed: %w", err)
		}

		if len(keys) == 0 {
			return nil, fmt.Errorf("jwks: no keys found at %s", c.url)
		}

		// If kid is specified, find the matching key.
		if kid != "" {
			for _, k := range keys {
				if k.Kid == kid {
					return jwtKeyFromJWKS(k)
				}
			}
			return nil, fmt.Errorf("jwks: kid %q not found", kid)
		}

		// No kid — try the first key.
		return jwtKeyFromJWKS(keys[0])
	}
}

func (c *jwksCache) getKeys() ([]jwksKey, error) {
	c.mu.RLock()
	if c.rawKeys != nil && time.Since(c.fetched) < c.ttl {
		keys := c.rawKeys
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	return c.fetch()
}

func (c *jwksCache) fetch() ([]jwksKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if c.rawKeys != nil && time.Since(c.fetched) < c.ttl {
		return c.rawKeys, nil
	}

	resp, err := c.client.Get(c.url)
	if err != nil {
		return nil, fmt.Errorf("jwks request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("jwks read failed: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("jwks parse failed: %w", err)
	}

	c.rawKeys = jwks.Keys
	c.fetched = time.Now()
	return c.rawKeys, nil
}

// jwtKeyFromJWKS converts a JWKS key to a crypto public key.
func jwtKeyFromJWKS(k jwksKey) (any, error) {
	switch k.Kty {
	case "RSA":
		return rsaPublicKeyFromJWKS(k)
	case "EC":
		return ecPublicKeyFromJWKS(k)
	case "OKP":
		return ed25519PublicKeyFromJWKS(k)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", k.Kty)
	}
}

// JWTValidator validates JWTs and extracts claims.
type JWTValidator struct {
	config JWKSConfig
	cache  *jwksCache
}

// NewJWTValidator creates a new JWT validator with JWKS caching.
func NewJWTValidator(cfg JWKSConfig) *JWTValidator {
	return &JWTValidator{
		config: cfg,
		cache:  newJWKSCache(cfg.JWKSURL, cfg.CacheTTL),
	}
}

// Validate parses and validates the JWT token string, returning extracted claims.
func (v *JWTValidator) Validate(ctx context.Context, tokenStr string) (*JWTClaims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA", "PS256", "PS384", "PS512"}),
	}
	if v.config.Issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.config.Issuer))
	}
	if v.config.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.config.Audience))
	}

	token, err := jwt.Parse(tokenStr, v.cache.keyFunc(), parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("jwt validation failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("jwt token is invalid")
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("jwt: unexpected claims type")
	}

	claims := &JWTClaims{
		Raw: mapClaims,
	}

	if sub, err := mapClaims.GetSubject(); err == nil {
		claims.Subject = sub
	}
	if iss, err := mapClaims.GetIssuer(); err == nil {
		claims.Issuer = iss
	}
	if email, ok := mapClaims["email"].(string); ok {
		claims.Email = email
	}

	// Extract tenant from configurable claim path.
	claims.TenantID = extractStringClaim(mapClaims, v.config.TenantClaim)

	// Extract roles from configurable claim path.
	claims.Roles = extractStringSliceClaim(mapClaims, v.config.RoleClaim)

	return claims, nil
}

// ExtractClaimsFromContext retrieves JWT claims from the context.
func ExtractClaimsFromContext(ctx context.Context) *JWTClaims {
	claims, _ := ctx.Value(claimsContextKey).(*JWTClaims)
	return claims
}

// injectClaimsIntoContext stores JWT claims in the context.
func injectClaimsIntoContext(ctx context.Context, claims *JWTClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// extractStringClaim extracts a string value from a claims map, supporting
// nested paths separated by "#" (e.g. "https://myapp.com#tenant").
func extractStringClaim(claims jwt.MapClaims, path string) string {
	if path == "" {
		return ""
	}
	val := walkClaimPath(claims, path)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// extractStringSliceClaim extracts a string or []string from claims.
func extractStringSliceClaim(claims jwt.MapClaims, path string) []string {
	if path == "" {
		return nil
	}
	val := walkClaimPath(claims, path)
	switch v := val.(type) {
	case string:
		return []string{v}
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// walkClaimPath navigates nested claim maps using "#" separated paths.
// Example: "https://myapp.com#tenant" → claims["https://myapp.com"]["tenant"]
//
// Use "#" because "." conflicts with domain names and "/" with URL paths.
func walkClaimPath(claims jwt.MapClaims, path string) any {
	parts := strings.Split(path, "#")
	var current any = claims
	for _, part := range parts {
		// Try jwt.MapClaims first (named type won't match map[string]any).
		if m, ok := current.(jwt.MapClaims); ok {
			current = m[part]
			continue
		}
		// Try map[string]any (covers nested plain maps from JSON deserialization).
		if m, ok := current.(map[string]any); ok {
			current = m[part]
			continue
		}
		return nil
	}
	return current
}

// extractBearerToken extracts the token from "Bearer <token>" header.
func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return ""
}
