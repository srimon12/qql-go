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

	if err := a.enforceFilterComplexity(q.QueryFilter); err != nil {
		return err
	}
	if err := a.enforceCTEComplexity(q.CTEs); err != nil {
		return err
	}

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

	if err := a.enforceFilterComplexity(s.QueryFilter); err != nil {
		return err
	}

	return nil
}

// TransformDelete applies policy transformations to a DeleteStmt.
// Injects the tenant filter into the WHERE clause to prevent cross-tenant deletion.
func (a *ASTInjector) TransformDelete(d *ast.DeleteStmt) error {
	if err := a.EnforceCollection(d.Collection); err != nil {
		return err
	}
	a.injectDeleteFilter(d)
	if err := a.enforceFilterComplexity(d.QueryFilter); err != nil {
		return err
	}
	return nil
}

// TransformInsert applies policy transformations to an InsertStmt.
func (a *ASTInjector) TransformInsert(i *ast.InsertStmt) error {
	return a.EnforceCollection(i.Collection)
}

// TransformUpdatePayload applies policy transformations to an UpdatePayloadStmt.
// Injects the tenant filter into the WHERE clause to prevent cross-tenant updates.
func (a *ASTInjector) TransformUpdatePayload(u *ast.UpdatePayloadStmt) error {
	if err := a.EnforceCollection(u.Collection); err != nil {
		return err
	}
	a.injectUpdatePayloadFilter(u)
	if err := a.enforceFilterComplexity(u.QueryFilter); err != nil {
		return err
	}
	return nil
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

// injectDeleteFilter injects the policy's WHERE clause into a DeleteStmt.
func (a *ASTInjector) injectDeleteFilter(d *ast.DeleteStmt) {
	if a.policy.InjectField != "" {
		resolved := a.resolveValue()
		if resolved != "" {
			injected := a.buildCompareExpr(resolved)
			d.QueryFilter = mergeFilters(d.QueryFilter, injected)
		}
	}

	for _, f := range a.policy.InjectFilters {
		value := f.Value
		if f.FromClaim != "" && a.claims != nil {
			value = extractStringClaimFromJWT(a.claims, f.FromClaim)
		}
		if value == "" {
			continue
		}
		compare := a.buildFilterExpr(f.Field, f.Op, value)
		d.QueryFilter = mergeFilters(d.QueryFilter, compare)
	}
}

// injectUpdatePayloadFilter injects the policy's WHERE clause into an UpdatePayloadStmt.
func (a *ASTInjector) injectUpdatePayloadFilter(u *ast.UpdatePayloadStmt) {
	if a.policy.InjectField != "" {
		resolved := a.resolveValue()
		if resolved != "" {
			injected := a.buildCompareExpr(resolved)
			u.QueryFilter = mergeFilters(u.QueryFilter, injected)
		}
	}

	for _, f := range a.policy.InjectFilters {
		value := f.Value
		if f.FromClaim != "" && a.claims != nil {
			value = extractStringClaimFromJWT(a.claims, f.FromClaim)
		}
		if value == "" {
			continue
		}
		compare := a.buildFilterExpr(f.Field, f.Op, value)
		u.QueryFilter = mergeFilters(u.QueryFilter, compare)
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

// enforceFilterComplexity checks the filter tree for resource-exhaustion risks.
func (a *ASTInjector) enforceFilterComplexity(filter ast.FilterExpr) error {
	if a.policy.MaxFilterDepth > 0 {
		depth := measureFilterDepth(filter, 0)
		if depth > a.policy.MaxFilterDepth {
			return fmt.Errorf("filter nesting depth %d exceeds policy maximum %d", depth, a.policy.MaxFilterDepth)
		}
	}
	if a.policy.MaxOrOperands > 0 {
		orCount := countOrOperands(filter)
		for _, cnt := range orCount {
			if cnt > a.policy.MaxOrOperands {
				return fmt.Errorf("OR operand count %d exceeds policy maximum %d", cnt, a.policy.MaxOrOperands)
			}
		}
	}
	return nil
}

// enforceCTEComplexity checks the number of nested CTE/prefetch stages.
func (a *ASTInjector) enforceCTEComplexity(ctes []ast.CTE) error {
	if a.policy.MaxPrefetchDepth > 0 && len(ctes) > a.policy.MaxPrefetchDepth {
		return fmt.Errorf("CTE count %d exceeds policy maximum %d", len(ctes), a.policy.MaxPrefetchDepth)
	}
	return nil
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

// measureFilterDepth returns the maximum nesting depth of a filter tree.
func measureFilterDepth(expr ast.FilterExpr, depth int) int {
	if expr == nil {
		return depth
	}
	switch e := expr.(type) {
	case ast.AndExpr:
		maxChild := depth
		for _, op := range e.Operands {
			d := measureFilterDepth(op, depth+1)
			if d > maxChild {
				maxChild = d
			}
		}
		return maxChild
	case ast.OrExpr:
		maxChild := depth
		for _, op := range e.Operands {
			d := measureFilterDepth(op, depth+1)
			if d > maxChild {
				maxChild = d
			}
		}
		return maxChild
	case ast.NotExpr:
		return measureFilterDepth(e.Operand, depth+1)
	default:
		return depth
	}
}

// countOrOperands returns the maximum OR operand count at each level of the
// filter tree. Returns a slice so we can check each level independently.
func countOrOperands(expr ast.FilterExpr) []int {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case ast.OrExpr:
		counts := []int{len(e.Operands)}
		for _, op := range e.Operands {
			counts = append(counts, countOrOperands(op)...)
		}
		return counts
	case ast.AndExpr:
		var counts []int
		for _, op := range e.Operands {
			counts = append(counts, countOrOperands(op)...)
		}
		return counts
	case ast.NotExpr:
		return countOrOperands(e.Operand)
	default:
		return nil
	}
}

// VerifyFilterInjection checks that the policy filter is present in the AST
// after injection. This is a defense-in-depth check that catches AST compiler
// bugs where the injected filter might be silently dropped during compilation.
// Returns nil if verification is skipped (no injection expected) or if the
// filter is correctly present. Returns an error if injection was expected
// but the filter tree does not contain it.
func (a *ASTInjector) VerifyFilterInjection(node ast.ASTNode) error {
	if a.policy.InjectField == "" && len(a.policy.InjectFilters) == 0 {
		return nil
	}

	filter := filterFromNode(node)
	if filter == nil {
		return fmt.Errorf("policy requires filter injection but AST has no WHERE clause")
	}

	// Build the set of expected filter fields.
	expectedFields := make(map[string]bool)
	if a.policy.InjectField != "" {
		expectedFields[a.policy.InjectField] = true
	}
	for _, f := range a.policy.InjectFilters {
		expectedFields[f.Field] = true
	}

	// Walk the filter tree and collect all field names.
	foundFields := make(map[string]bool)
	collectFilterFields(filter, foundFields)

	for field := range expectedFields {
		if !foundFields[field] {
			return fmt.Errorf("policy requires filter on field %q but it was not found in the WHERE clause", field)
		}
	}
	return nil
}

// filterFromNode extracts the FilterExpr from an AST node.
func filterFromNode(node ast.ASTNode) ast.FilterExpr {
	switch n := node.(type) {
	case *ast.QueryStmt:
		return n.QueryFilter
	case *ast.ScrollStmt:
		return n.QueryFilter
	case *ast.DeleteStmt:
		return n.QueryFilter
	case *ast.UpdatePayloadStmt:
		return n.QueryFilter
	default:
		return nil
	}
}

// collectFilterFields recursively walks a FilterExpr tree and collects
// all field names that appear in comparisons.
func collectFilterFields(expr ast.FilterExpr, fields map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case ast.CompareExpr:
		fields[e.Field] = true
	case ast.InExpr:
		fields[e.Field] = true
	case ast.NotInExpr:
		fields[e.Field] = true
	case ast.BetweenExpr:
		fields[e.Field] = true
	case ast.IsNullExpr:
		fields[e.Field] = true
	case ast.IsNotNullExpr:
		fields[e.Field] = true
	case ast.IsEmptyExpr:
		fields[e.Field] = true
	case ast.IsNotEmptyExpr:
		fields[e.Field] = true
	case ast.MatchTextExpr:
		fields[e.Field] = true
	case ast.MatchAnyExpr:
		fields[e.Field] = true
	case ast.MatchPhraseExpr:
		fields[e.Field] = true
	case ast.AndExpr:
		for _, op := range e.Operands {
			collectFilterFields(op, fields)
		}
	case ast.OrExpr:
		for _, op := range e.Operands {
			collectFilterFields(op, fields)
		}
	case ast.NotExpr:
		collectFilterFields(e.Operand, fields)
	}
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
