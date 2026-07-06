// Command tahu is the OKF knowledge-management daemon. It exposes an MCP
// server over stdio or HTTP/SSE that gives AI agents tools for reading,
// writing, and semantically searching OKF knowledge bases.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/embedder"
	mcpadapter "github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/mcp"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/vectorstore"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/config"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/registry"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/transport"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

func main() {
	root := buildRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tahu:", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Root command
// ---------------------------------------------------------------------------

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "tahu",
		Version: version,
		Short:   "OKF knowledge-management daemon with MCP tools",
		Long: `tahu manages OKF (Open Knowledge Format) bundle registries and
exposes 14 MCP tools for reading, writing, and semantically searching
knowledge bases over stdio or HTTP/SSE.`,
		SilenceUsage: true,
	}

	root.AddCommand(buildServe())
	root.AddCommand(buildBundle())
	root.AddCommand(buildSearch())
	root.AddCommand(buildConcept())
	root.AddCommand(buildVersionCmd())

	return root
}

// ---------------------------------------------------------------------------
// serve
// ---------------------------------------------------------------------------

func buildServe() *cobra.Command {
	var (
		transportFlag string
		port          int
		bind          string
		configPath    string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start the tahu MCP server. Use --transport stdio (default) for CLI
agents or --transport http for orchestration pipelines.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return fmt.Errorf("serve: load config: %w", err)
			}
			// Flag overrides (non-zero values win over config file).
			if cmd.Flags().Changed("transport") {
				cfg.Transport = transportFlag
			}
			if cmd.Flags().Changed("port") {
				cfg.Port = port
			}
			if cmd.Flags().Changed("bind") {
				cfg.BindAddr = bind
			}

			// Structured JSON logger (FR-021).
			initLogger(cfg.LogLevel)

			svc, store, err := buildServices(cfg)
			if err != nil {
				return fmt.Errorf("serve: build services: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Persist the vector index on clean shutdown.
			defer func() {
				if perr := store.Persist(context.Background()); perr != nil {
					slog.Error("persist vector index", slog.String("error", perr.Error()))
				}
			}()

			srv := transport.NewMCPServer(svc, version)

			switch cfg.Transport {
			case "http":
				addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
				slog.Info("starting HTTP/SSE MCP server", slog.String("addr", addr))
				return transport.ServeHTTP(ctx, srv, addr)
			default: // stdio
				slog.Info("starting stdio MCP server")
				return transport.ServeStdio(ctx, srv)
			}
		},
	}

	cmd.Flags().StringVar(&transportFlag, "transport", "stdio", "MCP transport: stdio|http")
	cmd.Flags().IntVar(&port, "port", 0, "TCP port for HTTP transport (default from config: 3000)")
	cmd.Flags().StringVar(&bind, "bind", "", "Bind address for HTTP transport (default from config: 127.0.0.1)")
	cmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: ~/.tahu/config.yaml)")

	return cmd
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func buildBundle() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Manage OKF bundle registrations",
	}
	cmd.AddCommand(buildBundleList())
	cmd.AddCommand(buildBundleAdd())
	cmd.AddCommand(buildBundleReindex())
	return cmd
}

func buildBundleList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered OKF bundles",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			bundleRepo := registry.NewYAMLBundleRepository(cfg.BundleRegistry)
			bundles, err := bundleRepo.List(context.Background())
			if err != nil {
				return err
			}
			if len(bundles) == 0 {
				fmt.Println("No bundles registered.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ALIAS\tROOT_PATH\tCONCEPT_COUNT\tLAST_INDEXED_AT")
			for _, b := range bundles {
				indexedAt := "-"
				if !b.LastIndexedAt.IsZero() {
					indexedAt = b.LastIndexedAt.Format("2006-01-02T15:04:05Z")
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
					b.Alias, b.RootPath, b.ConceptCount, indexedAt)
			}
			return w.Flush()
		},
	}
}

func buildBundleAdd() *cobra.Command {
	var alias string

	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register an OKF bundle by filesystem path",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			bundleRepo := registry.NewYAMLBundleRepository(cfg.BundleRegistry)
			nodeRepo := okf.NewFileNodeRepository(nil)
			svc := &usecase.BundleService{
				BundleRepository: bundleRepo,
				NodeRepository:   nodeRepo,
			}
			entry, err := svc.AddBundle(context.Background(), alias, args[0], "", nil)
			if err != nil {
				return err
			}
			fmt.Printf("Bundle registered: alias=%s path=%s\n", entry.Alias, entry.RootPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "Unique alias for the bundle (required)")
	_ = cmd.MarkFlagRequired("alias")
	return cmd
}

func buildBundleReindex() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex <alias>",
		Short: "Force a full re-embed and reindex of a bundle",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			svc, store, err := buildServices(cfg)
			if err != nil {
				return err
			}
			alias := args[0]
			if err := svc.Bundle.ReindexBundle(context.Background(), alias, svc.Embedder, store); err != nil {
				return err
			}
			if err := store.Persist(context.Background()); err != nil {
				slog.Warn("persist vector index after reindex", slog.String("error", err.Error()))
			}
			fmt.Printf("Bundle reindexed: %s\n", alias)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// search
// ---------------------------------------------------------------------------

func buildSearch() *cobra.Command {
	var (
		scope string
		topK  int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Run a RAG semantic search over registered bundles",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			query := args[0]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			svc, _, err := buildServices(cfg)
			if err != nil {
				return err
			}
			sc, err := domain.ParseScope(scope)
			if err != nil {
				return fmt.Errorf("search: scope: %w", err)
			}
			chunks, err := svc.Search.RAGSearch(context.Background(), query, sc, topK, 0.0)
			if err != nil {
				return err
			}
			if len(chunks) == 0 {
				fmt.Println("No results.")
				return nil
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(chunks)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "Search scope: global|bundle:<alias>|path:<alias>:<subpath>")
	cmd.Flags().IntVar(&topK, "top-k", 5, "Maximum number of results")
	return cmd
}

// ---------------------------------------------------------------------------
// concept
// ---------------------------------------------------------------------------

func buildConcept() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "concept",
		Short: "Read or write OKF concepts",
	}
	cmd.AddCommand(buildConceptRead())
	return cmd
}

func buildConceptRead() *cobra.Command {
	return &cobra.Command{
		Use:   "read <alias:relative/path.md>",
		Short: "Read and print an OKF concept",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			svc, _, err := buildServices(cfg)
			if err != nil {
				return err
			}
			ref, err := parseConceptRef(args[0])
			if err != nil {
				return err
			}
			concept, err := svc.Concept.ReadConcept(context.Background(), ref)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(concept)
		},
	}
}

// ---------------------------------------------------------------------------
// DI wiring helpers
// ---------------------------------------------------------------------------

// buildServices constructs the full dependency graph from cfg. It:
//  1. Creates the YAML bundle registry.
//  2. Lists registered bundles to build the NodeRepository roots map.
//  3. Creates the BM25 embedder.
//  4. Creates the HNSW vector store (dims inferred from existing index or BM25 maxDims).
//  5. Loads the existing vector index from disk (cold-start rebuild is a no-op).
//  6. Wires all use cases.
//
// The returned domain.VectorStore should be Persist()ed on shutdown.
func buildServices(cfg *config.Config) (mcpadapter.Services, domain.VectorStore, error) {
	ctx := context.Background()

	// 1. Bundle registry.
	bundleRepo := registry.NewYAMLBundleRepository(cfg.BundleRegistry)

	// 2. Resolve registered bundle roots for the NodeRepository.
	bundles, err := bundleRepo.List(ctx)
	if err != nil {
		return mcpadapter.Services{}, nil, fmt.Errorf("buildServices: list bundles: %w", err)
	}
	roots := make(map[string]string, len(bundles))
	for _, b := range bundles {
		roots[b.Alias] = b.RootPath
	}
	nodeRepo := okf.NewFileNodeRepository(roots)

	// 3. Embedder — only "bm25" is supported; return a clear error for anything else (FR-015).
	if cfg.EmbeddingModel != "bm25" {
		return mcpadapter.Services{}, nil, fmt.Errorf(
			"buildServices: embedding_model %q is not supported (only \"bm25\" is available)",
			cfg.EmbeddingModel,
		)
	}
	bm25Embedder := embedder.New() // maxDims=4096; Dims() always returns 4096

	// 4. Vector store — derive path from registry location.
	hnswPath := filepath.Join(filepath.Dir(cfg.BundleRegistry), "hnsw.index")
	dims := bm25Embedder.Dims() // always 4096 after our BM25 fix
	store, err := vectorstore.New(hnswPath, dims, cfg.HNSWEfConstruction, cfg.HNSWM)
	if err != nil {
		return mcpadapter.Services{}, nil, fmt.Errorf("buildServices: create vector store: %w", err)
	}

	// 5. Restore existing index from disk; cold-start (no file) is a no-op.
	if err := store.Load(ctx); err != nil {
		return mcpadapter.Services{}, nil, fmt.Errorf("buildServices: load vector index: %w", err)
	}

	// 6. Wire use cases.
	bundleSvc := &usecase.BundleService{
		BundleRepository: bundleRepo,
		NodeRepository:   nodeRepo,
	}
	conceptSvc := &usecase.ConceptService{
		NodeRepository:   nodeRepo,
		BundleRepository: bundleRepo,
	}
	searchSvc := &usecase.SearchService{
		Embedder:    bm25Embedder,
		VectorStore: store,
	}

	svc := mcpadapter.Services{
		Bundle:      bundleSvc,
		Concept:     conceptSvc,
		Search:      searchSvc,
		Embedder:    bm25Embedder,
		VectorStore: store,
	}
	return svc, store, nil
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

// initLogger configures the global slog logger with a JSON handler to stderr.
func initLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(handler))
}

// ---------------------------------------------------------------------------
// version
// ---------------------------------------------------------------------------

func buildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("tahu %s\n", version)
		},
	}
}

// parseConceptRef parses "alias:relative/path.md" into a domain.ConceptRef.
func parseConceptRef(s string) (domain.ConceptRef, error) {
	for i, c := range s {
		if c == ':' && i > 0 {
			return domain.ConceptRef{
				BundleAlias:  s[:i],
				RelativePath: s[i+1:],
			}, nil
		}
	}
	return domain.ConceptRef{}, fmt.Errorf("invalid concept ref %q: expected alias:relative/path.md", s)
}
