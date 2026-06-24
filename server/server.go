package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/srimon12/qql-go/gen/qqlpbconnect"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/pkg/qql"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Version is set at build time via ldflags.
var Version = "dev"

// Config holds server configuration.
type Config struct {
	// ListenAddr is the address to listen on, e.g. ":50051".
	ListenAddr string
	// QdrantURL is the Qdrant endpoint, e.g. "http://localhost:6334".
	QdrantURL string
	// QdrantAPIKey is the API key for Qdrant authentication.
	QdrantAPIKey string
	// AllowedOrigins is a list of permitted CORS origins.
	// If empty, only the Origin request header is echoed back (nil values default to the
	// request's Origin when the request is same-origin, but the caller must be careful).
	// For development, set to ["*"] to allow all.
	AllowedOrigins []string
	// ShutdownTimeout is the grace period for in-flight requests.
	ShutdownTimeout time.Duration
	// QQLConfig is the QQL executor config (embedding, inference, BM25, etc.).
	QQLConfig *config.Config

	// Gateway configuration (optional — if set, enables auth + policy + audit).
	Gateway *GatewayConfig
}

// GatewayConfig holds the full gateway policy enforcement configuration.
type GatewayConfig struct {
	// JWTValidator validates JWT tokens from the Authorization header.
	// If nil, JWT validation is skipped (no auth).
	JWTValidator *JWTValidator

	// PolicyEngine evaluates JWT claims against policy rules.
	// If nil, no policy enforcement is applied.
	PolicyEngine *PolicyEngine

	// PolicyReloader watches the policy file for changes and atomically swaps.
	// If set, PolicyEngine is derived from it.
	PolicyReloader *PolicyReloader

	// RateLimiter enforces per-tenant request rate limits.
	// If nil, no rate limiting is applied.
	RateLimiter *RateLimiter

	// AnonymousRateLimiter enforces rate limits on unauthenticated requests
	// keyed by client IP. Prevents resource exhaustion from invalid-token floods.
	// If nil, no anonymous rate limiting is applied.
	AnonymousRateLimiter *RateLimiter

	// Templates is the query template engine for agent-safe operations.
	// If nil, template execution is disabled.
	Templates *TemplateEngine

	// Audit is the structured audit logger.
	Audit *AuditLogger
}

// Run starts the QQL Connect RPC server. It blocks until the server shuts down.
func Run(cfg Config) error {
	// Create Qdrant client
	qdrantClient, err := qql.NewQdrantClient(qql.ClientConfig{
		URL:    cfg.QdrantURL,
		Secret: cfg.QdrantAPIKey,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant at %s: %w", cfg.QdrantURL, err)
	}

	// Verify Qdrant connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = qdrantClient.ListCollections(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("cannot reach Qdrant at %s: %w", cfg.QdrantURL, err)
	}

	// Build Connect handler
	handler := NewHandlerWithConfig(qdrantClient, cfg.QQLConfig)

	// Build interceptor chain.
	var interceptors []connect.Interceptor
	if cfg.Gateway != nil && cfg.Gateway.Audit != nil {
		// Rate limiter goes first (cheapest check).
		if cfg.Gateway.RateLimiter != nil {
			interceptors = append(interceptors, rateLimitInterceptor(cfg.Gateway.RateLimiter))
		}
		interceptors = append(interceptors, chainInterceptors(cfg.Gateway))
	} else {
		interceptors = append(interceptors, loggingInterceptor())
	}

	path, svcHandler := qqlpbconnect.NewQQLHandler(
		handler,
		connect.WithInterceptors(interceptors...),
	)

	// Build HTTP mux
	mux := http.NewServeMux()
	mux.Handle(path, svcHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"version":"%s"}`, Version)
	})

	// Wrap with h2c for HTTP/2 without TLS (Connect clients prefer HTTP/2)
	h2cHandler := h2c.NewHandler(corsMiddleware(mux, cfg.AllowedOrigins), &http2.Server{})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h2cHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	errCh := make(chan error, 1)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\nreceived %v, shutting down...\n", sig)

		shutdownTimeout := cfg.ShutdownTimeout
		if shutdownTimeout == 0 {
			shutdownTimeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		errCh <- srv.Shutdown(ctx)
	}()

	fmt.Fprintf(os.Stderr, "qql-gateway %s listening on %s (qdrant: %s)\n", Version, cfg.ListenAddr, cfg.QdrantURL)
	if cfg.Gateway != nil {
		if cfg.Gateway.JWTValidator != nil {
			fmt.Fprintf(os.Stderr, "  auth: JWKS enabled\n")
		}
		if cfg.Gateway.PolicyEngine != nil {
			fmt.Fprintf(os.Stderr, "  policy: enabled\n")
		}
		if cfg.Gateway.Audit != nil {
			fmt.Fprintf(os.Stderr, "  audit: enabled\n")
		}
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	fmt.Fprintln(os.Stderr, "server stopped")
	return nil
}

// corsMiddleware adds CORS headers for browser clients using the server Config.
func corsMiddleware(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := origin
		if len(origins) == 0 {
			if origin == "" {
				allowed = "*"
			}
		} else {
			allowed = ""
			for _, o := range origins {
				if o == "*" {
					allowed = "*"
					break
				}
				if o == origin {
					allowed = origin
					break
				}
			}
		}
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
