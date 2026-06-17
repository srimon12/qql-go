package server

import (
	"time"

	"github.com/spf13/cobra"
)

// NewServeCmd returns the cobra command for `qql-go serve`.
func NewServeCmd() *cobra.Command {
	var (
		listenAddr string
		qdrantURL  string
		qdrantKey  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the QQL Connect RPC gateway",
		Long: `Start the QQL gateway server. The gateway accepts QQL queries over
Connect RPC (HTTP POST with protobuf or JSON) and executes them against Qdrant.

Any language can send QQL queries:
  - curl:       curl -X POST http://localhost:50051/qql.QQL/Exec -d '{"query":"..."}'
  - Python:     pip install qql
  - TypeScript: npm install @qql/client
  - Go:         gen/qqlpbconnect.NewQQLClient(...)`,
		Example: `  qql-go serve --qdrant-url http://localhost:6334
  qql-go serve --qdrant-url http://qdrant:6334 --listen :8080
  qql-go serve --qdrant-url https://cloud.qdrant.io:6334 --api-key YOUR_KEY`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if qdrantURL == "" {
				qdrantURL = "http://localhost:6334"
			}
			if listenAddr == "" {
				listenAddr = ":50051"
			}

			return Run(Config{
				ListenAddr:      listenAddr,
				QdrantURL:       qdrantURL,
				QdrantAPIKey:    qdrantKey,
				ShutdownTimeout: 10 * time.Second,
			})
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", ":50051", "address to listen on")
	cmd.Flags().StringVar(&qdrantURL, "qdrant-url", "http://localhost:6334", "Qdrant endpoint URL")
	cmd.Flags().StringVar(&qdrantKey, "api-key", "", "Qdrant API key")

	return cmd
}
