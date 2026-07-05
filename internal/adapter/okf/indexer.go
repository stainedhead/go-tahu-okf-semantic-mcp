// Package okf implements OKF document parsing, link extraction, index/log
// generation, and the NodeRepository backed by the local filesystem.
package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateIndex lists all non-reserved .md files in dirPath (non-recursive),
// reads their frontmatter for type and title, and returns a formatted markdown
// index table as a string.
func GenerateIndex(_, dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("GenerateIndex: read dir %q: %w", dirPath, err)
	}

	type conceptEntry struct {
		filename string
		typ      string
		title    string
	}

	var concepts []conceptEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "index.md" || name == "log.md" {
			continue
		}

		fp := filepath.Join(dirPath, name)
		data, err := os.ReadFile(fp)
		if err != nil {
			// Include the file but without metadata.
			concepts = append(concepts, conceptEntry{filename: name})
			continue
		}

		concept, err := ParseConcept(fp, data)
		if err != nil {
			concepts = append(concepts, conceptEntry{filename: name})
			continue
		}

		concepts = append(concepts, conceptEntry{
			filename: name,
			typ:      concept.Frontmatter.Type,
			title:    concept.Frontmatter.Title,
		})
	}

	sort.Slice(concepts, func(i, j int) bool {
		return concepts[i].filename < concepts[j].filename
	})

	var sb strings.Builder
	sb.WriteString("# Index\n\n")

	if len(concepts) == 0 {
		sb.WriteString("No concepts found.\n")
		return sb.String(), nil
	}

	sb.WriteString("| File | Type | Title |\n")
	sb.WriteString("|---|---|---|\n")
	for _, c := range concepts {
		fmt.Fprintf(&sb, "| [%s](%s) | %s | %s |\n",
			c.filename, c.filename, c.typ, c.title)
	}

	return sb.String(), nil
}

// AppendLog reads the existing log.md in dirPath (creating it if absent),
// prepends a new timestamped entry under today's date header (newest-first),
// and writes the file back.
func AppendLog(_, dirPath string, entry string, timestamp time.Time) error {
	logPath := filepath.Join(dirPath, "log.md")

	var existing string
	data, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("AppendLog: read log %q: %w", logPath, err)
	}
	if err == nil {
		existing = string(data)
	}

	dateHeader := "## " + timestamp.Format("2006-01-02")
	entryLine := "- " + entry

	if idx := strings.Index(existing, dateHeader); idx >= 0 {
		// Insert the new entry immediately after the existing date header line,
		// so newest entries appear first within the day.
		after := existing[idx+len(dateHeader):]
		newContent := existing[:idx+len(dateHeader)] + "\n" + entryLine + after
		return os.WriteFile(logPath, []byte(newContent), 0o644)
	}

	// No header for today: prepend a new section at the top of the file.
	var sb strings.Builder
	sb.WriteString(dateHeader)
	sb.WriteString("\n")
	sb.WriteString(entryLine)
	sb.WriteString("\n")
	if existing != "" {
		sb.WriteString("\n")
		sb.WriteString(existing)
	}

	return os.WriteFile(logPath, []byte(sb.String()), 0o644)
}
