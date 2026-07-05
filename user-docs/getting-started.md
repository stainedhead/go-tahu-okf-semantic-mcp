# Getting Started with tahu

This guide walks you from zero to a working MCP server connected to Claude Code, with your first knowledge bundle indexed and searchable.

---

## 1. Install prerequisites

| Tool | Version | How to install |
|---|---|---|
| Go | ≥ 1.26 | Download from [go.dev/dl](https://go.dev/dl) |
| golangci-lint | latest | `brew install golangci-lint` |
| goimports | latest | `go install golang.org/x/tools/cmd/goimports@latest` |

Verify Go:

```bash
go version
# go version go1.26.x ...
```

---

## 2. Build the binary

Clone the repository and build:

```bash
git clone https://github.com/stainedhead/go-tahu-okf-semantic-mcp.git
cd go-tahu-okf-semantic-mcp
make build
```

This produces `bin/tahu`. You can copy it anywhere on your `$PATH`:

```bash
sudo cp bin/tahu /usr/local/bin/tahu
```

---

## 3. Register your first bundle

An OKF bundle is any directory containing markdown files with YAML frontmatter. If you already have one, register it:

```bash
tahu bundle add /path/to/your/knowledge-base --alias my-kb
```

Expected output:

```
Bundle registered: alias=my-kb path=/path/to/your/knowledge-base
```

To create a minimal bundle from scratch:

```bash
mkdir -p ~/my-kb/notes
cat > ~/my-kb/notes/hello.md << 'EOF'
---
type: note
title: Hello World
description: My first OKF concept
---

# Hello World

This is my first knowledge concept.
EOF

tahu bundle add ~/my-kb --alias my-kb
```

List registered bundles to confirm:

```bash
tahu bundle list
# ALIAS  ROOT_PATH             CONCEPT_COUNT  LAST_INDEXED_AT
# my-kb  /Users/you/my-kb      1              -
```

---

## 4. Run the MCP server in stdio mode

For use with Claude Code (and other CLI agents), run the server in stdio mode:

```bash
tahu serve --transport stdio
```

The server listens on stdin/stdout, logging to stderr. Keep this terminal open, or configure Claude Code to launch it automatically (step 5).

---

## 5. Connect Claude Code

Add tahu to your Claude Code MCP configuration. The config file is typically at `~/.claude/claude_desktop_config.json` (or wherever Claude Code's MCP config lives):

```json
{
  "mcpServers": {
    "tahu": {
      "command": "/usr/local/bin/tahu",
      "args": ["serve", "--transport", "stdio"]
    }
  }
}
```

Restart Claude Code. tahu's 14 tools should now appear in the available tools list.

---

## 6. Run your first semantic search

From the Claude Code conversation (or any MCP client), call:

```json
{
  "tool": "search_rag",
  "query": "hello world",
  "scope": "global",
  "top_k": 5
}
```

From the CLI:

```bash
tahu search "hello world"
```

If the bundle has not been indexed yet (no vector data), re-index it first:

```bash
tahu bundle reindex my-kb
```

Then search again. You will receive a JSON array of `ScoredChunk` results.

---

## 7. Write a concept

Use the `concept_write` MCP tool (from an agent):

```json
{
  "tool": "concept_write",
  "ref": "my-kb:notes/deployment.md",
  "type": "runbook",
  "title": "Deployment Runbook",
  "description": "Steps to deploy the production service",
  "body": "# Deployment Runbook\n\n1. Run smoke tests.\n2. Push to production.\n3. Monitor for 10 minutes."
}
```

The daemon writes the file, regenerates `index.md` in the `notes/` directory, and appends to `log.md`.

---

## 8. Read a concept and its links

Read the concept back (MCP tool or CLI):

```bash
tahu concept read my-kb:notes/deployment.md
```

To retrieve outbound links from the concept body, use the `concept_links` MCP tool:

```json
{
  "tool": "concept_links",
  "ref": "my-kb:notes/deployment.md"
}
```

This returns a list of link targets with `Target`, `Text`, and `Broken` fields. Broken links (targets that don't exist on disk) are included with `Broken: true`.

---

## Next steps

- See [`configuration.md`](configuration.md) to switch to HTTP/SSE mode or change embedding settings.
- See [`mcp-tools.md`](mcp-tools.md) for the full parameter reference for all 14 tools.
- See [`cli-reference.md`](cli-reference.md) for all CLI subcommands and flags.
