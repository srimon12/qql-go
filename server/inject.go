package server

import (
	"fmt"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
)

// ASTInjector applies policy-based transformations to parsed AST nodes
// before execution. This is the core enforcement mechanism — the gateway
// rewrites queries at the AST level, not the result level.
type ASTInjector struct {
	policy EvaluatedPolicy
	claims *JWTClaims
}

// NewASTInjector creates an injector for the given policy and claims.
func NewASTInjector(policy EvaluatedPolicy, claims *JWTClaims) *ASTInjector {
	return &ASTInjector{policy: policy, claims: claims}
}

// EnforceOperation checks if the AST node's operation type is allowed by policy.
// Returns an error if the operation is denied.
func (a *ASTInjector) EnforceOperation(node ast.ASTNode) error {
	op := operationFromNode(node)
	if !a.policy.IsOperationAllowed(op) {
		return fmt.Errorf("operation %s not permitted for current token", op)
	}
	return nil
}

// EnforceCollection checks if the node's target collection is allowed by policy.
func (a *ASTInjector) EnforceCollection(collection string) error {
	if !a.policy.IsCollectionAllowed(collection) {
		return fmt.Errorf("access to collection %q not permitted", collection)
	}
	return nil
}

// TransformQuery applies all policy-based transformations to a QueryStmt.
// This mutates the AST in-place:
//   - Injects tenant filter into WHERE clause
//   - Enforces LIMIT cap
//   - Validates collection access
func (a *ASTInjector) TransformQuery(q *ast.QueryStmt) error {
	if err := a.EnforceCollection(q.Collection); err != nil {
		return err
	}

	a.injectFilter(q)
	a.enforceLimit(&q.Limit)

	for i := range q.CTEs {
		if q.CTEs[i].Stmt != nil {
			a.enforceLimit(&q.CTEs[i].Stmt.Limit)
		}
	}

	return nil
}

// TransformScroll applies policy transformations to a ScrollStmt.
func (a *ASTInjector) TransformScroll(s *ast.ScrollStmt) error {
	if err := a.EnforceCollection(s.Collection); err != nil {
		return err
	}

	a.injectScrollFilter(s)
	a.enforceLimit(&s.Limit)

	return nil
}

// TransformDelete applies policy transformations to a DeleteStmt.
func (a *ASTInjector) TransformDelete(d *ast.DeleteStmt) error {
	return a.EnforceCollection(d.Collection)
}

// TransformInsert applies policy transformations to an InsertStmt.
func (a *ASTInjector) TransformInsert(i *ast.InsertStmt) error {
	return a.EnforceCollection(i.Collection)
}

// TransformUpdatePayload applies policy transformations to an UpdatePayloadStmt.
func (a *ASTInjector) TransformUpdatePayload(u *ast.UpdatePayloadStmt) error {
	return a.EnforceCollection(u.Collection)
}

// TransformUpdateVector applies policy transformations to an UpdateVectorStmt.
func (a *ASTInjector) TransformUpdateVector(u *ast.UpdateVectorStmt) error {
	return a.EnforceCollection(u.Collection)
}

// TransformCreateCollection applies policy transformations to a CreateCollectionStmt.
func (a *ASTInjector) TransformCreateCollection(c *ast.CreateCollectionStmt) error {
	return a.EnforceCollection(c.Collection)
}

// TransformDropCollection applies policy transformations to a DropCollectionStmt.
func (a *ASTInjector) TransformDropCollection(d *ast.DropCollectionStmt) error {
	return a.EnforceCollection(d.Collection)
}

// TransformAlterCollection applies policy transformations to an AlterCollectionStmt.
func (a *ASTInjector) TransformAlterCollection(al *ast.AlterCollectionStmt) error {
	return a.EnforceCollection(al.Collection)
}

// TransformCreateIndex applies policy transformations to a CreateIndexStmt.
func (a *ASTInjector) TransformCreateIndex(ci *ast.CreateIndexStmt) error {
	return a.EnforceCollection(ci.Collection)
}

// GetEffectiveLimit returns the effective limit after policy cap.
func (a *ASTInjector) GetEffectiveLimit(requested int) int {
	if a.policy.MaxLimit > 0 && requested > a.policy.MaxLimit {
		return a.policy.MaxLimit
	}
	return requested
}

// injectFilter injects the policy's WHERE clause into a QueryStmt and its CTEs.
func (a *ASTInjector) injectFilter(q *ast.QueryStmt) {
	// Legacy single filter.
	if a.policy.InjectField != "" {
		resolved := a.resolveValue()
		if resolved != "" {
			injected := a.buildCompareExpr(resolved)
			q.QueryFilter = mergeFilters(q.QueryFilter, injected)
		}
	}

	// Multiple filters.
	for _, f := range a.policy.InjectFilters {
		value := f.Value
		if f.FromClaim != "" && a.claims != nil {
			value = extractStringClaimFromJWT(a.claims, f.FromClaim)
		}
		if value == "" {
			continue
		}
		compare := a.buildFilterExpr(f.Field, f.Op, value)
		q.QueryFilter = mergeFilters(q.QueryFilter, compare)
	}

	// Recursively apply to CTEs
	for i := range q.CTEs {
		if q.CTEs[i].Stmt != nil {
			a.injectFilter(q.CTEs[i].Stmt)
		}
	}
}

// injectScrollFilter injects the policy's WHERE clause into a ScrollStmt.
func (a *ASTInjector) injectScrollFilter(s *ast.ScrollStmt) {
	// Legacy single filter.
	if a.policy.InjectField != "" {
		resolved := a.resolveValue()
		if resolved != "" {
			injected := a.buildCompareExpr(resolved)
			s.QueryFilter = mergeFilters(s.QueryFilter, injected)
		}
	}

	// Multiple filters.
	for _, f := range a.policy.InjectFilters {
		value := f.Value
		if f.FromClaim != "" && a.claims != nil {
			value = extractStringClaimFromJWT(a.claims, f.FromClaim)
		}
		if value == "" {
			continue
		}
		compare := a.buildFilterExpr(f.Field, f.Op, value)
		s.QueryFilter = mergeFilters(s.QueryFilter, compare)
	}
}

// resolveValue determines the injection value from policy config or JWT claims.
func (a *ASTInjector) resolveValue() string {
	if a.policy.InjectValue != "" {
		return a.policy.InjectValue
	}
	if a.policy.InjectFromClaim != "" && a.claims != nil {
		return extractStringClaimFromJWT(a.claims, a.policy.InjectFromClaim)
	}
	return ""
}

// buildCompareExpr creates a CompareExpr for the injected filter (legacy single filter).
func (a *ASTInjector) buildCompareExpr(value string) ast.FilterExpr {
	op := a.policy.InjectOp
	if op == "" {
		op = "="
	}
	return a.buildFilterExpr(a.policy.InjectField, op, value)
}

// buildFilterExpr creates a filter expression for a given field, operator, and value.
func (a *ASTInjector) buildFilterExpr(field, op, value string) ast.FilterExpr {
	switch strings.ToLower(op) {
	case "in":
		parts := strings.Split(value, ",")
		vals := make([]any, len(parts))
		for i, p := range parts {
			vals[i] = strings.TrimSpace(p)
		}
		return ast.InExpr{Field: field, Values: vals}
	case "not_in":
		parts := strings.Split(value, ",")
		vals := make([]any, len(parts))
		for i, p := range parts {
			vals[i] = strings.TrimSpace(p)
		}
		return ast.NotInExpr{Field: field, Values: vals}
	case "!=":
		return ast.CompareExpr{Field: field, Op: "!=", Value: value}
	default:
		return ast.CompareExpr{Field: field, Op: op, Value: value}
	}
}

// enforceLimit caps the limit value if the policy specifies a max.
func (a *ASTInjector) enforceLimit(limit *int) {
	if a.policy.MaxLimit > 0 && *limit > a.policy.MaxLimit {
		*limit = a.policy.MaxLimit
	}
}

// mergeFilters combines an existing filter with an injected one using AND.
func mergeFilters(existing ast.FilterExpr, injected ast.FilterExpr) ast.FilterExpr {
	if existing == nil {
		return injected
	}
	if injected == nil {
		return existing
	}
	return ast.AndExpr{Operands: []ast.FilterExpr{existing, injected}}
}

// operationFromNode extracts the operation type string from an AST node.
func operationFromNode(node ast.ASTNode) string {
	switch node.(type) {
	case *ast.QueryStmt:
		return "QUERY"
	case *ast.InsertStmt:
		return "INSERT"
	case *ast.UpdatePayloadStmt, *ast.UpdateVectorStmt:
		return "UPDATE"
	case *ast.DeleteStmt:
		return "DELETE"
	case *ast.ScrollStmt:
		return "SCROLL"
	case *ast.SelectStmt:
		return "SELECT"
	case *ast.ShowCollectionsStmt, *ast.ShowCollectionStmt:
		return "SHOW"
	case *ast.CreateCollectionStmt:
		return "CREATE"
	case *ast.DropCollectionStmt:
		return "DROP"
	case *ast.AlterCollectionStmt:
		return "ALTER"
	case *ast.CreateIndexStmt:
		return "CREATE_INDEX"
	default:
		return "UNKNOWN"
	}
}
