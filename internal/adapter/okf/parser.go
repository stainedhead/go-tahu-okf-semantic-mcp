package okf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"gopkg.in/yaml.v3"
)

// knownFMKeys are the standard OKF frontmatter keys in canonical write order.
var knownFMKeys = []string{"type", "title", "description", "resource", "tags", "timestamp"}

// standardFrontmatter is a yaml-tagged struct for decoding the known frontmatter fields.
type standardFrontmatter struct {
	Type        string    `yaml:"type"`
	Title       string    `yaml:"title"`
	Description string    `yaml:"description"`
	Resource    string    `yaml:"resource"`
	Tags        []string  `yaml:"tags"`
	Timestamp   time.Time `yaml:"timestamp"`
}

// ParseConcept parses an OKF markdown document from data.
// path is used only in error messages.
func ParseConcept(path string, data []byte) (*domain.OKFConcept, error) {
	fmStr, body := splitFrontmatter(string(data))

	fm, err := parseFrontmatter(fmStr)
	if err != nil {
		return nil, fmt.Errorf("ParseConcept %s: %w", path, err)
	}

	return &domain.OKFConcept{
		Frontmatter: fm,
		Body:        body,
	}, nil
}

// splitFrontmatter splits an OKF document into YAML frontmatter and markdown body.
// The frontmatter is the content between the first two "---" delimiter lines.
// Returns empty frontmatter and the full input as body if no valid block is found.
func splitFrontmatter(s string) (frontmatter, body string) {
	lines := strings.Split(s, "\n")
	if len(lines) < 2 || trimCR(lines[0]) != "---" {
		return "", s
	}

	for i := 1; i < len(lines); i++ {
		if trimCR(lines[i]) == "---" {
			fm := strings.Join(lines[1:i], "\n")
			// Drop the single leading newline from body, if present.
			body := strings.Join(lines[i+1:], "\n")
			body = strings.TrimPrefix(body, "\n")
			return fm, body
		}
	}

	// No closing ---: treat whole doc as body.
	return "", s
}

func trimCR(s string) string {
	return strings.TrimRight(s, "\r")
}

// parseFrontmatter decodes a YAML string into domain.OKFFrontmatter.
// Unknown keys are captured in Extra.
func parseFrontmatter(yamlStr string) (domain.OKFFrontmatter, error) {
	if strings.TrimSpace(yamlStr) == "" {
		return domain.OKFFrontmatter{}, nil
	}

	raw := []byte(yamlStr)

	var std standardFrontmatter
	if err := yaml.Unmarshal(raw, &std); err != nil {
		return domain.OKFFrontmatter{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	var rawMap map[string]any
	if err := yaml.Unmarshal(raw, &rawMap); err != nil {
		return domain.OKFFrontmatter{}, fmt.Errorf("parse frontmatter raw map: %w", err)
	}

	knownSet := make(map[string]bool, len(knownFMKeys))
	for _, k := range knownFMKeys {
		knownSet[k] = true
	}

	var extra map[string]any
	for k, v := range rawMap {
		if !knownSet[k] {
			if extra == nil {
				extra = make(map[string]any)
			}
			extra[k] = v
		}
	}

	return domain.OKFFrontmatter{
		Type:        std.Type,
		Title:       std.Title,
		Description: std.Description,
		Resource:    std.Resource,
		Tags:        std.Tags,
		Timestamp:   std.Timestamp,
		Extra:       extra,
	}, nil
}

// SerializeConcept serializes an OKFConcept to markdown bytes with normalized
// frontmatter. Key order: type, title, description, resource, tags, timestamp,
// then Extra keys. Zero-value optional fields are omitted; type is always emitted.
func SerializeConcept(concept *domain.OKFConcept) ([]byte, error) {
	fm := concept.Frontmatter

	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	addScalar := func(key, val string) {
		if val == "" {
			return
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: val},
		)
	}

	addSeq := func(key string, vals []string) {
		if len(vals) == 0 {
			return
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, v := range vals {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			seq,
		)
	}

	// type is always emitted, even when empty (callers check ValidateFrontmatter first).
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: fm.Type},
	)

	addScalar("title", fm.Title)
	addScalar("description", fm.Description)
	addScalar("resource", fm.Resource)
	addSeq("tags", fm.Tags)

	if !fm.Timestamp.IsZero() {
		valNode, err := valueToNode(fm.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("SerializeConcept: encode timestamp: %w", err)
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "timestamp"},
			valNode,
		)
	}

	for k, v := range fm.Extra {
		valNode, err := valueToNode(v)
		if err != nil {
			return nil, fmt.Errorf("SerializeConcept: encode extra key %q: %w", k, err)
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			valNode,
		)
	}

	docNode := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(docNode); err != nil {
		return nil, fmt.Errorf("SerializeConcept: encode YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("SerializeConcept: close encoder: %w", err)
	}

	yamlStr := strings.TrimRight(buf.String(), "\n")

	var out bytes.Buffer
	out.WriteString("---\n")
	out.WriteString(yamlStr)
	out.WriteString("\n---\n")
	if concept.Body != "" {
		out.WriteString(concept.Body)
		if !strings.HasSuffix(concept.Body, "\n") {
			out.WriteString("\n")
		}
	}

	return out.Bytes(), nil
}

// valueToNode converts a Go value to a *yaml.Node by round-tripping through
// yaml.Marshal / yaml.Unmarshal.
func valueToNode(v any) (*yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Value: "null", Tag: "!!null"}, nil
}
