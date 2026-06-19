package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"
)

// loggingInterceptor logs every RPC call with duration and status.
// Used as a fallback when no gateway config is provided.
func loggingInterceptor() connect.Interceptor {
	return &loggingInt{}
}

type loggingInt struct{}

func (l *loggingInt) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		procedure := req.Spec().Procedure
		resp, err := next(ctx, req)
		duration := time.Since(start)

		status := "ok"
		if err != nil {
			status = connect.CodeOf(err).String()
		}
		fmt.Fprintf(os.Stderr, "%s %s %v\n", procedure, status, duration)

		return resp, err
	}
}

func (l *loggingInt) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (l *loggingInt) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// policyContextKey stores the evaluated policy in the request context.
type policyContextKey string

const evalPolicyKey policyContextKey = "evaluated_policy"

// chainInterceptors builds the full interceptor chain:
// JWT validation → Policy evaluation → Audit logging → Handler execution.
func chainInterceptors(cfg *GatewayConfig) connect.Interceptor {
	return &chainInt{cfg: cfg}
}

type chainInt struct {
	cfg *GatewayConfig
}

func (c *chainInt) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		procedure := req.Spec().Procedure

		// Skip auth for health checks, convert, and explain (utility endpoints).
		if procedure == "/qql.QQL/Health" || procedure == "/qql.QQL/Convert" || procedure == "/qql.QQL/Explain" {
			resp, err := next(ctx, req)
			c.cfg.Audit.Log(AuditEntry{
				Timestamp: time.Now(),
				Operation: operationFromProcedure(procedure),
				Allowed:   true,
				LatencyMs: time.Since(start).Milliseconds(),
				Status:    "ok",
			})
			return resp, err
		}

		// Step 1: JWT validation (if JWKS is configured).
		var claims *JWTClaims
		if c.cfg.JWTValidator != nil {
			token := extractBearerToken(req.Header().Get("Authorization"))
			if token == "" {
				if c.cfg.PolicyEngine != nil {
					// No token but policy engine exists — evaluate with nil claims.
					policy := c.cfg.PolicyEngine.Evaluate(nil)
					if !policy.Allowed {
						c.cfg.Audit.LogDenied(nil, operationFromProcedure(procedure), "", "no token provided", time.Since(start))
						return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
					}
					ctx = injectEvaluatedPolicy(ctx, policy)
				} else {
					c.cfg.Audit.LogDenied(nil, operationFromProcedure(procedure), "", "no token provided", time.Since(start))
					return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
				}
			} else {
				var err error
				claims, err = c.cfg.JWTValidator.Validate(ctx, token)
				if err != nil {
					c.cfg.Audit.LogDenied(nil, operationFromProcedure(procedure), "", fmt.Sprintf("jwt validation failed: %v", err), time.Since(start))
					return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
				}
				ctx = injectClaimsIntoContext(ctx, claims)
			}
		}

		// Step 2: Policy evaluation (if policy engine is configured).
		if c.cfg.PolicyEngine != nil || c.cfg.PolicyReloader != nil {
			engine := c.cfg.PolicyEngine
			if c.cfg.PolicyReloader != nil {
				engine = c.cfg.PolicyReloader.Engine()
			}
			if engine != nil {
				policy := engine.Evaluate(claims)
				if !policy.Allowed {
					c.cfg.Audit.LogDenied(claims, operationFromProcedure(procedure), "", "no matching policy rule", time.Since(start))
					return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("access denied: no matching policy"))
				}
				ctx = injectEvaluatedPolicy(ctx, policy)
			}
		}

		// Step 3: Execute handler with AuditMeta context.
		meta := &AuditMeta{}
		ctx = ContextWithAuditMeta(ctx, meta)
		resp, err := next(ctx, req)

		// Step 4: Audit log.
		latency := time.Since(start)

		entry := AuditEntry{
			Timestamp: time.Now(),
			Operation: operationFromProcedure(procedure),
			LatencyMs: latency.Milliseconds(),
		}

		if meta.Collection != "" {
			entry.Collection = meta.Collection
		}
		if meta.Query != "" {
			entry.Query = truncate(meta.Query, 500)
		}
		if meta.FiltersInjected != nil {
			entry.FiltersInjected = meta.FiltersInjected
		}
		if meta.LimitCapped != nil {
			entry.LimitCapped = meta.LimitCapped
		}
		entry.RuleIndex = meta.RuleIndex

		if err != nil {
			entry.Status = "error"
			entry.Error = err.Error()
		} else {
			entry.Status = "ok"
			entry.Allowed = true
		}

		// Fill claims
		if claims != nil {
			entry.Subject = claims.Subject
			entry.Email = claims.Email
			entry.TenantID = claims.TenantID
			entry.Roles = claims.Roles
		}

		c.cfg.Audit.Log(entry)

		return resp, err
	}
}

func (c *chainInt) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (c *chainInt) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// injectEvaluatedPolicy stores the evaluated policy in the context.
func injectEvaluatedPolicy(ctx context.Context, policy EvaluatedPolicy) context.Context {
	return context.WithValue(ctx, evalPolicyKey, policy)
}

// ExtractEvaluatedPolicy retrieves the evaluated policy from context.
func ExtractEvaluatedPolicy(ctx context.Context) *EvaluatedPolicy {
	p, ok := ctx.Value(evalPolicyKey).(EvaluatedPolicy)
	if !ok {
		return nil
	}
	return &p
}

// operationFromProcedure maps a Connect RPC procedure to an operation name.
func operationFromProcedure(procedure string) string {
	switch procedure {
	case "/qql.QQL/Exec":
		return "EXEC"
	case "/qql.QQL/ExecBatch":
		return "EXEC_BATCH"
	case "/qql.QQL/Explain":
		return "EXPLAIN"
	case "/qql.QQL/Convert":
		return "CONVERT"
	case "/qql.QQL/Health":
		return "HEALTH"
	default:
		return "UNKNOWN"
	}
}
