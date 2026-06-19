package server

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/srimon12/qql-go/internal/config"
)

// NewServeCmd returns the cobra command for `qql-go serve`.
func NewServeCmd() *cobra.Command {
	var (
		listenAddr string
		qdrantURL  string
		qdrantKey  string

		// Gateway flags.
		jwksURL      string
		jwksIssuer   string
		jwksAudience string
		jwksCacheTTL time.Duration
		roleClaim    string
		tenantClaim  string

		policyFile   string
		policyReload bool

		auditEnable bool
		auditFile   string

		// Rate limiting.
		rateLimit         float64
		rateLimitCapacity int

		// Templates.
		templateFile string

		// Embedding flags.
		inferenceMode      string
		embeddingEndpoint  string
		embeddingModel     string
		embeddingDimension int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the QQL Connect RPC gateway",
		Long: `Start the QQL gateway server. The gateway accepts QQL queries over
Connect RPC (HTTP POST with protobuf or JSON) and executes them against Qdrant.

When --jwks-url is set, the gateway validates JWT tokens from the Authorization
header against the IdP's JWKS endpoint. Use --policy-file to enforce per-tenant
access control, collection scoping, and automatic filter injection.

Any language can send QQL queries:
  - curl:       curl -X POST http://localhost:50051/qql.QQL/Exec -d '{"query":"..."}'
  - Python:     pip install qql
  - TypeScript: npm install @qql/client
  - Go:         gen/qqlpbconnect.NewQQLClient(...)`,
		Example: `  qql-go serve --qdrant-url http://localhost:6334
  qql-go serve --qdrant-url http://qdrant:6334 --listen :8080
  qql-go serve --qdrant-url https://cloud.qdrant.io:6334 --api-key YOUR_KEY

  # With JWT auth
  qql-go serve --qdrant-url http://localhost:6334 \
    --jwks-url https://idp.example.com/.well-known/jwks.json \
    --tenant-claim org_id \
    --role-claim role

  # With policy enforcement + hot-reload + rate limiting + templates
  qql-go serve --qdrant-url http://localhost:6334 \
    --jwks-url https://idp.example.com/.well-known/jwks.json \
    --policy-file policies.yaml \
    --policy-reload \
    --rate-limit 10 \
    --templates templates.yaml \
    --audit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if qdrantURL == "" {
				qdrantURL = "http://localhost:6334"
			}
			if listenAddr == "" {
				listenAddr = ":50051"
			}

			cfg := Config{
				ListenAddr:      listenAddr,
				QdrantURL:       qdrantURL,
				QdrantAPIKey:    qdrantKey,
				ShutdownTimeout: 10 * time.Second,
				QQLConfig: &config.Config{
					InferenceMode:      inferenceMode,
					EmbeddingEndpoint:  embeddingEndpoint,
					EmbeddingModel:     embeddingModel,
					EmbeddingDimension: embeddingDimension,
				},
			}

			// Build gateway config if any gateway flag is set.
			if jwksURL != "" || policyFile != "" || auditEnable || rateLimit > 0 || templateFile != "" {
				gw := &GatewayConfig{}

				// Audit logger.
				if auditEnable {
					var auditWriter *os.File
					if auditFile != "" {
						f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if err != nil {
							return fmt.Errorf("failed to open audit file %s: %w", auditFile, err)
						}
						auditWriter = f
					}
					gw.Audit = NewAuditLogger(auditWriter, true)
				}

				// JWT validator.
				if jwksURL != "" {
					gw.JWTValidator = NewJWTValidator(JWKSConfig{
						JWKSURL:     jwksURL,
						Issuer:      jwksIssuer,
						Audience:    jwksAudience,
						CacheTTL:    jwksCacheTTL,
						RoleClaim:   roleClaim,
						TenantClaim: tenantClaim,
					})
				}

				// Policy engine (with optional hot-reload).
				if policyFile != "" {
					if policyReload {
						reloader, err := NewPolicyReloader(policyFile)
						if err != nil {
							return fmt.Errorf("failed to start policy reloader: %w", err)
						}
						gw.PolicyReloader = reloader
						gw.PolicyEngine = reloader.Engine()
					} else {
						pe, err := NewPolicyEngine(policyFile)
						if err != nil {
							return fmt.Errorf("failed to load policy file: %w", err)
						}
						gw.PolicyEngine = pe
					}
				}

				// Rate limiter.
				if rateLimit > 0 {
					gw.RateLimiter = NewRateLimiter(RateLimitConfig{
						Rate:     rateLimit,
						Capacity: rateLimitCapacity,
						Enabled:  true,
					})
				}

				// Template engine.
				if templateFile != "" {
					te, err := NewTemplateEngine(templateFile)
					if err != nil {
						return fmt.Errorf("failed to load template file: %w", err)
					}
					gw.Templates = te
				}

				cfg.Gateway = gw
			}

			return Run(cfg)
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", ":50051", "address to listen on")
	cmd.Flags().StringVar(&qdrantURL, "qdrant-url", "http://localhost:6334", "Qdrant endpoint URL")
	cmd.Flags().StringVar(&qdrantKey, "api-key", "", "Qdrant API key")

	// Gateway: JWKS / Auth flags.
	cmd.Flags().StringVar(&jwksURL, "jwks-url", "", "JWKS endpoint URL for JWT validation (e.g. https://idp.example.com/.well-known/jwks.json)")
	cmd.Flags().StringVar(&jwksIssuer, "jwt-issuer", "", "Expected JWT issuer claim (optional)")
	cmd.Flags().StringVar(&jwksAudience, "jwt-audience", "", "Expected JWT audience claim (optional)")
	cmd.Flags().DurationVar(&jwksCacheTTL, "jwks-cache-ttl", 10*time.Minute, "JWKS key cache TTL")
	cmd.Flags().StringVar(&roleClaim, "role-claim", "role", "JWT claim path for role extraction")
	cmd.Flags().StringVar(&tenantClaim, "tenant-claim", "tenant_id", "JWT claim path for tenant extraction")

	// Gateway: Policy flags.
	cmd.Flags().StringVar(&policyFile, "policy-file", "", "path to YAML policy file for access control")
	cmd.Flags().BoolVar(&policyReload, "policy-reload", false, "watch policy file for changes and reload automatically")

	// Gateway: Audit flags.
	cmd.Flags().BoolVar(&auditEnable, "audit", false, "enable structured JSON audit logging")
	cmd.Flags().StringVar(&auditFile, "audit-file", "", "path to audit log file (default: stderr)")

	// Gateway: Rate limiting flags.
	cmd.Flags().Float64Var(&rateLimit, "rate-limit", 0, "max requests per second per user (0 = unlimited)")
	cmd.Flags().IntVar(&rateLimitCapacity, "rate-limit-capacity", 20, "max burst size per user for rate limiting")

	// Gateway: Template flags.
	cmd.Flags().StringVar(&templateFile, "templates", "", "path to YAML query template file for agent-safe operations")

	// Embedding flags (same as CLI connect).
	cmd.Flags().StringVar(&inferenceMode, "inference-mode", "local", "Inference mode: cloud, external, or local")
	cmd.Flags().StringVar(&embeddingEndpoint, "embedding-endpoint", "", "Embedding server endpoint (required for local/external mode)")
	cmd.Flags().StringVar(&embeddingModel, "embedding-model", "sentence-transformers/all-minilm-l6-v2", "Embedding model name")
	cmd.Flags().IntVar(&embeddingDimension, "embedding-dimension", 384, "Embedding vector dimension")

	return cmd
}
