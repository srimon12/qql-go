package server

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/srimon12/qql-go/internal/ast"
)

// --- Policy Engine Tests ---

func TestPolicyEngine_AdminFullAccess(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match: PolicyMatch{Claims: map[string]string{"role": "admin"}},
				Allow: []string{"QUERY", "INSERT", "UPDATE", "DELETE", "SHOW", "CREATE", "DROP"},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{
		Subject: "admin-user",
		Raw:     map[string]any{"role": "admin"},
	}

	policy := pe.Evaluate(claims)
	if !policy.Allowed {
		t.Fatal("admin should be allowed")
	}
	if !policy.IsOperationAllowed("DELETE") {
		t.Fatal("admin should be allowed to DELETE")
	}
	if !policy.IsOperationAllowed("QUERY") {
		t.Fatal("admin should be allowed to QUERY")
	}
}

func TestPolicyEngine_ReaderRestricted(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match: PolicyMatch{Claims: map[string]string{"role": "reader"}},
				Allow: []string{"QUERY", "SCROLL", "SELECT", "SHOW", "EXPLAIN"},
				Inject: PolicyInject{
					Where: &InjectWhere{
						Field:     "tenant_id",
						FromClaim: "org_id",
						Op:        "=",
					},
				},
				Limits: PolicyLimits{MaxLimit: 100},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{
		Subject:  "reader-user",
		TenantID: "acme-corp",
		Raw: map[string]any{
			"role":   "reader",
			"org_id": "acme-corp",
		},
	}

	policy := pe.Evaluate(claims)
	if !policy.Allowed {
		t.Fatal("reader should be allowed")
	}
	if policy.IsOperationAllowed("DELETE") {
		t.Fatal("reader should NOT be allowed to DELETE")
	}
	if !policy.IsOperationAllowed("QUERY") {
		t.Fatal("reader should be allowed to QUERY")
	}
	if policy.MaxLimit != 100 {
		t.Fatalf("expected max limit 100, got %d", policy.MaxLimit)
	}
	if policy.InjectField != "tenant_id" {
		t.Fatalf("expected inject field tenant_id, got %s", policy.InjectField)
	}
}

func TestPolicyEngine_NoMatchDenies(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match: PolicyMatch{Claims: map[string]string{"role": "admin"}},
				Allow: []string{"QUERY"},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{
		Subject: "unknown-user",
		Raw:     map[string]any{"role": "reader"},
	}

	policy := pe.Evaluate(claims)
	if policy.Allowed {
		t.Fatal("unmatched claims should be denied")
	}
}

func TestPolicyEngine_NilClaimsDenies(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match: PolicyMatch{Claims: map[string]string{"role": "reader"}},
				Allow: []string{"QUERY"},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	policy := pe.Evaluate(nil)
	if policy.Allowed {
		t.Fatal("nil claims should be denied when rules require claims")
	}
}

func TestPolicyEngine_CollectionGlob(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match:       PolicyMatch{Authenticated: boolPtr(true)},
				Allow:       []string{"QUERY"},
				Collections: []string{"public_*", "docs_*"},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{Subject: "user", Raw: map[string]any{}}
	policy := pe.Evaluate(claims)

	if !policy.IsCollectionAllowed("public_docs") {
		t.Fatal("public_docs should match public_* glob")
	}
	if !policy.IsCollectionAllowed("docs_internal") {
		t.Fatal("docs_internal should match docs_* glob")
	}
	if policy.IsCollectionAllowed("finance_reports") {
		t.Fatal("finance_reports should NOT match public_* or docs_*")
	}
}

func TestPolicyEngine_DenyOverridesAllow(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match: PolicyMatch{Authenticated: boolPtr(true)},
				Allow: []string{"QUERY", "INSERT", "DELETE"},
				Deny:  []string{"DELETE"},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{Subject: "user", Raw: map[string]any{}}
	policy := pe.Evaluate(claims)

	if policy.IsOperationAllowed("DELETE") {
		t.Fatal("DELETE should be denied even though it's in allow list")
	}
	if !policy.IsOperationAllowed("INSERT") {
		t.Fatal("INSERT should be allowed")
	}
}

func TestPolicyEngine_RuleOrderMatters(t *testing.T) {
	cfg := PolicyConfig{
		Rules: []PolicyRule{
			{
				Match:  PolicyMatch{Claims: map[string]string{"role": "reader"}},
				Allow:  []string{"QUERY"},
				Limits: PolicyLimits{MaxLimit: 10},
			},
			{
				// This rule would also match "reader" but should never be reached.
				Match:  PolicyMatch{Claims: map[string]string{"role": "reader"}},
				Allow:  []string{"QUERY", "DELETE"},
				Limits: PolicyLimits{MaxLimit: 1000},
			},
		},
	}
	pe := NewPolicyEngineFromConfig(cfg)

	claims := &JWTClaims{
		Subject: "reader",
		Raw:     map[string]any{"role": "reader"},
	}

	policy := pe.Evaluate(claims)
	if policy.MaxLimit != 10 {
		t.Fatalf("first matching rule should win: expected max limit 10, got %d", policy.MaxLimit)
	}
	if policy.MatchedRule != 0 {
		t.Fatalf("expected rule index 0, got %d", policy.MatchedRule)
	}
}

// --- AST Injection Tests ---

func TestASTInjector_InjectTenantFilter(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:         true,
		AllowOps:        map[string]bool{"QUERY": true},
		InjectField:     "tenant_id",
		InjectFromClaim: "org_id",
		InjectOp:        "=",
	}
	claims := &JWTClaims{
		TenantID: "acme-corp",
		Raw:      map[string]any{"org_id": "acme-corp"},
	}

	injector := NewASTInjector(policy, claims)

	q := &ast.QueryStmt{
		Collection: "docs",
		Mode:       ast.QueryModeNearest,
		Limit:      10,
		QueryText:  strPtr("search query"),
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify filter was injected.
	if q.QueryFilter == nil {
		t.Fatal("expected filter to be injected")
	}

	// The filter should be an AND with the original (nil) and the injected CompareExpr.
	// Since original was nil, the injected filter replaces it directly.
	compare, ok := q.QueryFilter.(ast.CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", q.QueryFilter)
	}
	if compare.Field != "tenant_id" {
		t.Fatalf("expected field tenant_id, got %s", compare.Field)
	}
	if compare.Value != "acme-corp" {
		t.Fatalf("expected value acme-corp, got %v", compare.Value)
	}
}

func TestASTInjector_MergeWithExistingFilter(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:         true,
		AllowOps:        map[string]bool{"QUERY": true},
		InjectField:     "tenant_id",
		InjectFromClaim: "org_id",
		InjectOp:        "=",
	}
	claims := &JWTClaims{
		TenantID: "acme-corp",
		Raw:      map[string]any{"org_id": "acme-corp"},
	}

	injector := NewASTInjector(policy, claims)

	existingFilter := ast.CompareExpr{
		Field: "status",
		Op:    "=",
		Value: "active",
	}

	q := &ast.QueryStmt{
		Collection:  "docs",
		Mode:        ast.QueryModeNearest,
		Limit:       10,
		QueryText:   strPtr("search"),
		QueryFilter: existingFilter,
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be an AND of existing + injected.
	andExpr, ok := q.QueryFilter.(ast.AndExpr)
	if !ok {
		t.Fatalf("expected AndExpr, got %T", q.QueryFilter)
	}
	if len(andExpr.Operands) != 2 {
		t.Fatalf("expected 2 operands, got %d", len(andExpr.Operands))
	}
}

func TestASTInjector_LimitCap(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:  true,
		AllowOps: map[string]bool{"QUERY": true},
		MaxLimit: 50,
	}

	injector := NewASTInjector(policy, nil)

	q := &ast.QueryStmt{
		Collection: "docs",
		Mode:       ast.QueryModeNearest,
		Limit:      200,
		QueryText:  strPtr("search"),
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Limit != 50 {
		t.Fatalf("expected limit capped to 50, got %d", q.Limit)
	}
}

func TestASTInjector_LimitNotChangedWhenUnderCap(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:  true,
		AllowOps: map[string]bool{"QUERY": true},
		MaxLimit: 100,
	}

	injector := NewASTInjector(policy, nil)

	q := &ast.QueryStmt{
		Collection: "docs",
		Mode:       ast.QueryModeNearest,
		Limit:      10,
		QueryText:  strPtr("search"),
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Limit != 10 {
		t.Fatalf("limit should not change when under cap: got %d", q.Limit)
	}
}

func TestASTInjector_CollectionDenied(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:     true,
		AllowOps:    map[string]bool{"QUERY": true},
		Collections: []string{"public_*"},
	}

	injector := NewASTInjector(policy, nil)

	q := &ast.QueryStmt{
		Collection: "finance_reports",
		Mode:       ast.QueryModeNearest,
		Limit:      10,
		QueryText:  strPtr("search"),
	}

	err := injector.TransformQuery(q)
	if err == nil {
		t.Fatal("expected error for denied collection")
	}
}

func TestASTInjector_CollectionAllowed(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:     true,
		AllowOps:    map[string]bool{"QUERY": true},
		Collections: []string{"public_*", "docs_*"},
	}

	injector := NewASTInjector(policy, nil)

	q := &ast.QueryStmt{
		Collection: "public_docs",
		Mode:       ast.QueryModeNearest,
		Limit:      10,
		QueryText:  strPtr("search"),
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestASTInjector_OperationDenied(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:  true,
		AllowOps: map[string]bool{"QUERY": true},
	}

	injector := NewASTInjector(policy, nil)

	node := &ast.DeleteStmt{
		Collection: "docs",
		PointID:    "some-id",
	}

	err := injector.EnforceOperation(node)
	if err == nil {
		t.Fatal("expected error for denied DELETE operation")
	}
}

func TestASTInjector_StaticValueInjection(t *testing.T) {
	policy := EvaluatedPolicy{
		Allowed:     true,
		AllowOps:    map[string]bool{"QUERY": true},
		InjectField: "access_level",
		InjectValue: "public",
		InjectOp:    "=",
	}

	injector := NewASTInjector(policy, nil)

	q := &ast.QueryStmt{
		Collection: "docs",
		Mode:       ast.QueryModeNearest,
		Limit:      10,
		QueryText:  strPtr("search"),
	}

	err := injector.TransformQuery(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	compare, ok := q.QueryFilter.(ast.CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", q.QueryFilter)
	}
	if compare.Value != "public" {
		t.Fatalf("expected value public, got %v", compare.Value)
	}
}

// --- Claim Extraction Tests ---

func TestExtractStringClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "user-123",
		"https://myapp.com": map[string]any{
			"tenant": "acme",
		},
	}

	val := extractStringClaim(claims, "sub")
	if val != "user-123" {
		t.Fatalf("expected user-123, got %s", val)
	}

	val = extractStringClaim(claims, "https://myapp.com#tenant")
	t.Logf("nested result: %q", val)
	if val != "acme" {
		t.Fatalf("expected acme, got %s", val)
	}
}

func TestExtractStringSliceClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"roles": []any{"admin", "reader"},
	}

	val := extractStringSliceClaim(claims, "roles")
	if len(val) != 2 || val[0] != "admin" || val[1] != "reader" {
		t.Fatalf("expected [admin, reader], got %v", val)
	}
}

// --- Operation From Node Tests ---

func TestOperationFromNode(t *testing.T) {
	tests := []struct {
		node ast.ASTNode
		want string
	}{
		{&ast.QueryStmt{}, "QUERY"},
		{&ast.InsertStmt{}, "INSERT"},
		{&ast.DeleteStmt{}, "DELETE"},
		{&ast.UpdatePayloadStmt{}, "UPDATE"},
		{&ast.UpdateVectorStmt{}, "UPDATE"},
		{&ast.ScrollStmt{}, "SCROLL"},
		{&ast.SelectStmt{}, "SELECT"},
		{&ast.ShowCollectionsStmt{}, "SHOW"},
		{&ast.ShowCollectionStmt{}, "SHOW"},
		{&ast.CreateCollectionStmt{}, "CREATE"},
		{&ast.DropCollectionStmt{}, "DROP"},
		{&ast.AlterCollectionStmt{}, "ALTER"},
		{&ast.CreateIndexStmt{}, "CREATE_INDEX"},
	}

	for _, tt := range tests {
		got := operationFromNode(tt.node)
		if got != tt.want {
			t.Errorf("operationFromNode(%T) = %s, want %s", tt.node, got, tt.want)
		}
	}
}

// --- Glob Matching Tests ---

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"public_*", "public_docs", true},
		{"public_*", "public_reports", true},
		{"public_*", "finance_reports", false},
		{"*_reports", "finance_reports", true},
		{"*_reports", "public_docs", false},
		{"exact", "exact", true},
		{"exact", "other", false},
	}

	for _, tt := range tests {
		got := matchGlob(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

// --- Helper Functions ---

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
