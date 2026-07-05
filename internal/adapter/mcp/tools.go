package mcpadapter

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers all 14 MCP tools with the given server. Each tool
// delegates its business logic to the appropriate use-case method via svc.
//
// Tool naming follows the OKF MCP convention: snake_case, verb_noun pairs.
// Input schemas use JSON Schema "object" with required fields listed
// explicitly so schema-aware clients can validate before sending.
func RegisterTools(s *server.MCPServer, svc Services) {
	// -----------------------------------------------------------------------
	// Bundle management (FR-001 through FR-004)
	// -----------------------------------------------------------------------

	s.AddTool(
		mcp.NewTool("bundle_list",
			mcp.WithDescription("List all registered OKF bundles with alias, root_path, concept_count, and last_indexed_at."),
		),
		svc.HandleBundleList,
	)

	s.AddTool(
		mcp.NewTool("bundle_add",
			mcp.WithDescription("Register an OKF bundle by filesystem path and alias. "+
				"Fails if the path does not exist, contains no .md files, or the alias/path is already registered."),
			mcp.WithString("alias",
				mcp.Required(),
				mcp.Description("Unique identifier for the bundle (e.g. \"my-kb\")."),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute filesystem path to the OKF bundle root directory."),
			),
			mcp.WithString("description",
				mcp.Description("Human-readable description of the bundle."),
			),
			mcp.WithArray("tags",
				mcp.Description("Optional tags for the bundle."),
				mcp.WithStringItems(),
			),
		),
		svc.HandleBundleAdd,
	)

	s.AddTool(
		mcp.NewTool("bundle_remove",
			mcp.WithDescription("Unregister a bundle by alias. Files are not deleted."),
			mcp.WithString("alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle to remove."),
			),
		),
		svc.HandleBundleRemove,
	)

	s.AddTool(
		mcp.NewTool("bundle_reindex",
			mcp.WithDescription("Force a full re-embed and reindex of a bundle. Updates last_indexed_at."),
			mcp.WithString("alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle to reindex."),
			),
		),
		svc.HandleBundleReindex,
	)

	// -----------------------------------------------------------------------
	// OKF read / navigate (FR-005 through FR-010)
	// -----------------------------------------------------------------------

	s.AddTool(
		mcp.NewTool("concept_read",
			mcp.WithDescription("Return parsed frontmatter and markdown body for an OKF concept. "+
				"Returns a structured not-found error when the path is absent."),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Concept reference in \"alias:relative/path.md\" format."),
			),
		),
		svc.HandleConceptRead,
	)

	s.AddTool(
		mcp.NewTool("concept_write",
			mcp.WithDescription("Create or update an OKF concept document. "+
				"Requires a non-empty frontmatter type field. "+
				"Regenerates index.md and appends log.md in the containing directory on success. "+
				"Body must not exceed 1 MB."),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Concept reference in \"alias:relative/path.md\" format. "+
					"Target filename must not be index.md or log.md."),
			),
			mcp.WithString("type",
				mcp.Required(),
				mcp.Description("OKF frontmatter type field (required by OKF v0.1)."),
			),
			mcp.WithString("title",
				mcp.Description("Optional frontmatter title."),
			),
			mcp.WithString("description",
				mcp.Description("Optional frontmatter description."),
			),
			mcp.WithString("resource",
				mcp.Description("Optional frontmatter resource URI."),
			),
			mcp.WithArray("tags",
				mcp.Description("Optional frontmatter tags."),
				mcp.WithStringItems(),
			),
			mcp.WithString("body",
				mcp.Description("Markdown body content (max 1 MB)."),
			),
		),
		svc.HandleConceptWrite,
	)

	s.AddTool(
		mcp.NewTool("concept_list",
			mcp.WithDescription("List all non-reserved .md files at a given directory level within a bundle. "+
				"Returns an empty list (not an error) for non-existent directories."),
			mcp.WithString("bundle_alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle to list concepts from."),
			),
			mcp.WithString("sub_path",
				mcp.Description("Sub-directory path within the bundle. Empty string lists the bundle root."),
			),
		),
		svc.HandleConceptList,
	)

	s.AddTool(
		mcp.NewTool("concept_links",
			mcp.WithDescription("Return all outbound markdown hyperlink targets from a concept body. "+
				"Broken links are included with broken=true."),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Concept reference in \"alias:relative/path.md\" format."),
			),
		),
		svc.HandleConceptLinks,
	)

	s.AddTool(
		mcp.NewTool("index_read",
			mcp.WithDescription("Return the raw content of index.md at the given directory level. "+
				"Returns a structured not-found error when the file is absent."),
			mcp.WithString("bundle_alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle."),
			),
			mcp.WithString("dir_path",
				mcp.Description("Directory path within the bundle. Empty string addresses the bundle root."),
			),
		),
		svc.HandleIndexRead,
	)

	s.AddTool(
		mcp.NewTool("log_read",
			mcp.WithDescription("Return the raw content of log.md at the given directory level. "+
				"Returns a structured not-found error when the file is absent."),
			mcp.WithString("bundle_alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle."),
			),
			mcp.WithString("dir_path",
				mcp.Description("Directory path within the bundle. Empty string addresses the bundle root."),
			),
		),
		svc.HandleLogRead,
	)

	s.AddTool(
		mcp.NewTool("concept_type_list",
			mcp.WithDescription("Return all distinct frontmatter type values present in a bundle."),
			mcp.WithString("bundle_alias",
				mcp.Required(),
				mcp.Description("Alias of the bundle."),
			),
		),
		svc.HandleConceptTypeList,
	)

	// -----------------------------------------------------------------------
	// Search (FR-012 through FR-014)
	// -----------------------------------------------------------------------

	s.AddTool(
		mcp.NewTool("search_semantic",
			mcp.WithDescription("Return a ranked list of chunks via vector similarity search. "+
				"Scope controls which bundles are searched. No network call at query time."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Natural-language search query (max 4 KB)."),
			),
			mcp.WithString("scope",
				mcp.Description("Search scope: \"global\", \"bundle:<alias>\", or \"path:<alias>:<subpath>\". "+
					"Defaults to \"global\"."),
			),
			mcp.WithInteger("top_k",
				mcp.Description("Maximum number of results to return. Default 10."),
				mcp.Min(1),
			),
		),
		svc.HandleSearchSemantic,
	)

	s.AddTool(
		mcp.NewTool("search_keyword",
			mcp.WithDescription("Return a ranked list of chunks via Okapi BM25 keyword search. "+
				"Same scope semantics and response shape as search_semantic."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Keyword search query (max 4 KB)."),
			),
			mcp.WithString("scope",
				mcp.Description("Search scope: \"global\", \"bundle:<alias>\", or \"path:<alias>:<subpath>\". "+
					"Defaults to \"global\"."),
			),
			mcp.WithInteger("top_k",
				mcp.Description("Maximum number of results to return. Default 10."),
				mcp.Min(1),
			),
		),
		svc.HandleSearchKeyword,
	)

	s.AddTool(
		mcp.NewTool("search_rag",
			mcp.WithDescription("Semantic search filtered by minimum score. "+
				"Returns up to top_k chunks with score >= min_score in descending order. "+
				"Returns an empty list (not an error) when no chunks meet the threshold."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Natural-language search query (max 4 KB)."),
			),
			mcp.WithString("scope",
				mcp.Description("Search scope: \"global\", \"bundle:<alias>\", or \"path:<alias>:<subpath>\". "+
					"Defaults to \"global\"."),
			),
			mcp.WithInteger("top_k",
				mcp.Description("Maximum number of chunks to return (1–20, default 5)."),
				mcp.Min(1),
				mcp.Max(20),
			),
			mcp.WithNumber("min_score",
				mcp.Description("Minimum similarity score threshold (0.0–1.0, default 0.0)."),
				mcp.Min(0.0),
				mcp.Max(1.0),
			),
		),
		svc.HandleSearchRAG,
	)
}
