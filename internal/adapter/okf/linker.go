package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ExtractLinks parses the markdown body and returns all outbound hyperlink
// targets found in the document. Each link is resolved relative to conceptDir
// and checked for existence via os.Stat; missing local targets are marked
// Broken:true. External links (http://, https://, mailto:, //) are never
// checked against the filesystem.
func ExtractLinks(body string, bundleRoot, conceptDir string) ([]domain.ConceptLink, error) {
	src := []byte(body)
	reader := text.NewReader(src)
	gm := goldmark.New()
	doc := gm.Parser().Parse(reader)

	var links []domain.ConceptLink

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		target := string(link.Destination)
		linkText := extractLinkText(link, src)

		cl := domain.ConceptLink{
			Target: target,
			Text:   linkText,
		}

		// Skip existence check for external / non-file links.
		if isExternalLink(target) {
			links = append(links, cl)
			return ast.WalkContinue, nil
		}

		// Resolve relative path from the concept's directory.
		var absTarget string
		if filepath.IsAbs(target) {
			absTarget = filepath.Join(bundleRoot, target)
		} else {
			absTarget = filepath.Join(conceptDir, target)
		}
		absTarget = filepath.Clean(absTarget)

		if _, err := os.Stat(absTarget); err != nil {
			cl.Broken = true
		}

		links = append(links, cl)
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, fmt.Errorf("ExtractLinks: walk AST: %w", err)
	}

	return links, nil
}

// isExternalLink reports whether target is a non-filesystem URI.
func isExternalLink(target string) bool {
	return strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "//") ||
		strings.HasPrefix(target, "mailto:")
}

// extractLinkText returns the plain text content of a link node by collecting
// all direct *ast.Text children.
func extractLinkText(link *ast.Link, src []byte) string {
	var sb strings.Builder
	for child := link.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
			if t.SoftLineBreak() {
				sb.WriteString(" ")
			}
		}
	}
	return sb.String()
}
