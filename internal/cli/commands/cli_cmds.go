package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/dump"
	"github.com/srimon12/qql-go/internal/embedding"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/srimon12/qql-go/internal/repl"
	"github.com/srimon12/qql-go/internal/script"
)

func addOutputFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Emit structured JSON output")
	cmd.Flags().Bool("quiet", false, "Reduce decoration; with --json, emit compact JSON")
}

func readOutputMode(cmd *cobra.Command) commandOutputMode {
	jsonOut, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	return commandOutputMode{
		json:  jsonOut,
		quiet: quiet,
	}
}

func NewConnectCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a Qdrant instance",
		Long:  `Connect to a Qdrant instance and save the configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			url, _ := cmd.Flags().GetString("url")
			secret, _ := cmd.Flags().GetString("secret")
			inferenceMode, _ := cmd.Flags().GetString("inference-mode")
			embeddingEndpoint, _ := cmd.Flags().GetString("embedding-endpoint")
			embeddingKey, _ := cmd.Flags().GetString("embedding-key")
			embeddingModel, _ := cmd.Flags().GetString("embedding-model")
			embeddingDimension, _ := cmd.Flags().GetInt("embedding-dimension")
			noVerify, _ := cmd.Flags().GetBool("no-verify")
			caCert, _ := cmd.Flags().GetString("ca-cert")

			if url == "" {
				return commandError(out, mode, "connect", "", fmt.Errorf("--url is required"))
			}
			if inferenceMode == "" {
				inferenceMode = defaultInferenceMode
			}
			if (inferenceMode == "local" || inferenceMode == "external") && (embeddingEndpoint == "" || embeddingModel == "") {
				return commandError(out, mode, "connect", "", fmt.Errorf("--embedding-endpoint and --embedding-model are required for %s mode", inferenceMode))
			}

			// Auto-probe embedding dimension if not provided
			if (inferenceMode == "local" || inferenceMode == "external") && embeddingDimension <= 0 && embeddingEndpoint != "" && embeddingModel != "" {
				if !mode.json && !mode.quiet {
					out.Print("Probing embedding dimension from endpoint...")
				}
				probeClient, probeErr := embedding.NewClient(embedding.Config{
					Endpoint:  embeddingEndpoint,
					Model:     embeddingModel,
					APIKey:    embeddingKey,
					Dimension: 1,
				})
				if probeErr == nil {
					dim, probeErr := probeClient.ProbeDimension(context.Background(), "probe")
					if probeErr == nil {
						embeddingDimension = dim
						if !mode.json && !mode.quiet {
							out.Print(fmt.Sprintf("Auto-detected embedding dimension: %d", dim))
						}
					}
				}
				if probeErr != nil && !mode.json && !mode.quiet {
					out.Print(fmt.Sprintf("Warning: could not probe embedding dimension: %v", probeErr))
				}
			}
			if (inferenceMode == "local" || inferenceMode == "external") && embeddingDimension <= 0 {
				return commandError(out, mode, "connect", "", fmt.Errorf("--embedding-dimension is required (or endpoint must be reachable for auto-probe) for %s mode", inferenceMode))
			}

			if !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Connecting to %s...", url))
			}

			client, err := newClientFromURL(url, secret, noVerify, caCert)
			if err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("connection failed: %w", err))
			}

			collections, err := client.ListCollections(context.Background())
			if err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("connection failed: %w", err))
			}

			cfg := &config.Config{
				URL:                url,
				Secret:             secret,
				InferenceMode:      inferenceMode,
				EmbeddingEndpoint:  embeddingEndpoint,
				EmbeddingAPIKey:    embeddingKey,
				EmbeddingModel:     embeddingModel,
				EmbeddingDimension: embeddingDimension,
				NoVerify:           noVerify,
				CACert:             caCert,
			}

			// Validate embedding endpoint is reachable in local/external mode
			if (inferenceMode == "local" || inferenceMode == "external") && embeddingEndpoint != "" {
				testClient, testErr := embedding.NewClient(embedding.Config{
					Endpoint:  embeddingEndpoint,
					Model:     embeddingModel,
					APIKey:    embeddingKey,
					Dimension: embeddingDimension,
				})
				if testErr == nil {
					_, testErr = testClient.Embed(context.Background(), "test")
				}
				if testErr != nil && !mode.json && !mode.quiet {
					out.Print(fmt.Sprintf("Warning: embedding endpoint test failed: %v", testErr))
				}
			}

			if err := config.SaveConfig(cfg); err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("failed to save config: %w", err))
			}

			message := savedConfigMessage()
			if mode.json {
				return writeJSON(out, &ConnectResponse{
					OK:          true,
					Command:     "connect",
					URL:         url,
					Connected:   true,
					Collections: len(collections),
					Message:     message,
				}, mode.quiet)
			}

			if mode.quiet {
				out.Print(message)
				return nil
			}

			out.PrintSuccess(message)

			repl := repl.NewREPL(cfg, NewExecutor(client, cfg))
			return repl.Run()
		},
	}

	cmd.Flags().String("url", "", "Qdrant instance URL (for text INSERT/SEARCH use your Qdrant Cloud URL)")
	cmd.Flags().String("secret", "", "API key / secret (optional)")
	cmd.Flags().String("inference-mode", defaultInferenceMode, "Inference mode: cloud, external, or local")
	cmd.Flags().String("embedding-endpoint", "", "OpenAI-compatible embeddings endpoint for local/external modes")
	cmd.Flags().String("embedding-key", "", "API key for the embeddings endpoint")
	cmd.Flags().String("embedding-model", "", "Embedding model name for local/external modes")
	cmd.Flags().Int("embedding-dimension", 0, "Embedding dimension for local/external modes")
	addOutputFlags(cmd)
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

func NewDisconnectCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Remove saved connection config",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			if err := config.DeleteConfig(); err != nil {
				return commandError(out, mode, "disconnect", "", fmt.Errorf("failed to delete config: %w", err))
			}
			message := "Disconnected. Config removed."
			if mode.json {
				return writeJSON(out, &CommandResponse{
					OK:      true,
					Command: "disconnect",
					Message: message,
				}, mode.quiet)
			}
			if mode.quiet {
				out.Print(message)
				return nil
			}
			out.PrintSuccess(message)
			return nil
		},
	}
	addOutputFlags(cmd)
	return cmd
}

func NewREPLCmd(out *output.Outputter) *cobra.Command {
	return &cobra.Command{
		Use:   "repl",
		Short: "Launch interactive shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfiguredREPL()
		},
	}
}

func NewExecCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a single query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "exec", args[0], err)
			}

			executor := NewExecutor(client, cfg)
			if mode.json {
				result, err := executor.ExecuteResult(args[0])
				if err != nil {
					return commandError(out, mode, "exec", args[0], err)
				}
				return writeJSON(out, result, mode.quiet)
			}

			result, err := executor.Execute(args[0])
			if err != nil {
				return commandError(out, mode, "exec", args[0], err)
			}

			out.Print(result)
			return nil
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func NewExecuteCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute a .qql script file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			stopOnError, _ := cmd.Flags().GetBool("stop-on-error")
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "execute", args[0], err)
			}
			executor := NewExecutor(client, cfg)
			okCount, failCount, err := script.RunFile(args[0], executor, stopOnError)
			if err != nil {
				return commandError(out, mode, "execute", args[0], err)
			}
			message := fmt.Sprintf("Executed script %s (%d succeeded, %d failed)", args[0], okCount, failCount)
			if mode.json {
				return writeJSON(out, &ScriptResponse{
					OK:        true,
					Command:   "execute",
					Path:      args[0],
					Succeeded: okCount,
					Failed:    failCount,
					Message:   message,
				}, mode.quiet)
			}
			out.Print(message)
			return nil
		},
	}
	cmd.Flags().Bool("stop-on-error", false, "Stop after the first failing statement")
	addOutputFlags(cmd)
	return cmd
}

func NewExplainCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Show query plan without executing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			executor := NewExecutor(nil, nil)
			if mode.json {
				result, err := executor.ExplainResult(args[0])
				if err != nil {
					return commandError(out, mode, "explain", args[0], err)
				}
				return writeJSON(out, result, mode.quiet)
			}

			plan, err := executor.Explain(args[0])
			if err != nil {
				return commandError(out, mode, "explain", args[0], err)
			}

			if mode.quiet {
				out.Print(plan)
				return nil
			}
			out.PrintExplain(plan)
			return nil
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func NewDoctorCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check connection health",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "doctor", "", err)
			}

			if !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Checking connection to %s...", cfg.URL))
			}

			collections, err := client.ListCollections(context.Background())
			if err != nil {
				return commandError(out, mode, "doctor", "", fmt.Errorf("failed to connect: %w", err))
			}

			message := "Connection is healthy."
			if mode.json {
				return writeJSON(out, &DoctorResponse{
					OK:          true,
					Command:     "doctor",
					URL:         cfg.URL,
					Healthy:     true,
					Collections: len(collections),
					Message:     message,
				}, mode.quiet)
			}

			if mode.quiet {
				out.Print(fmt.Sprintf("healthy url=%s collections=%d", cfg.URL, len(collections)))
				return nil
			}

			out.PrintConnectionStatus(cfg.URL, true)
			out.Print(fmt.Sprintf("\nCollections: %d", len(collections)))
			out.Print("\n" + message)

			return nil
		},
	}

	addOutputFlags(cmd)

	return cmd
}

func NewDumpCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump a collection to a .qql script file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			if batchSize <= 0 {
				return commandError(out, mode, "dump", strings.Join(args, " "), fmt.Errorf("--batch-size must be greater than 0"))
			}
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "dump", strings.Join(args, " "), err)
			}
			_ = cfg
			written, skipped, err := dump.Collection(context.Background(), client, args[0], args[1], batchSize)
			if err != nil {
				return commandError(out, mode, "dump", strings.Join(args, " "), err)
			}
			message := fmt.Sprintf("Dumped collection '%s' to %s (%d written, %d skipped)", args[0], args[1], written, skipped)
			if mode.json {
				return writeJSON(out, &DumpResponse{
					OK:         true,
					Command:    "dump",
					Collection: args[0],
					Path:       args[1],
					Written:    written,
					Skipped:    skipped,
					Message:    message,
				}, mode.quiet)
			}
			out.Print(message)
			return nil
		},
	}
	cmd.Flags().Int("batch-size", 50, "Number of points per INSERT BULK batch in dump output")
	addOutputFlags(cmd)
	return cmd
}

func NewVersionCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			version := displayVersion()
			message := versionMessage()
			if mode.json {
				return writeJSON(out, &VersionResponse{
					OK:      true,
					Command: "version",
					Version: version,
					Message: message,
				}, mode.quiet)
			}
			if mode.quiet {
				out.Print(version)
				return nil
			}
			out.PrintSection("qql-go Version", version)
			return nil
		},
	}
	addOutputFlags(cmd)
	return cmd
}
