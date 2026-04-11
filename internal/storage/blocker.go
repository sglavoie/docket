package storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const blockersFileName = "blockers.md"

var (
	// Matches blocker headings like "## Waiting for API access"
	blockerHeadingRegex = regexp.MustCompile(`^##\s+(.+)$`)
	// Matches metadata comments like "<!-- id: uuid, created: 2026-01-15T10:30:00 -->"
	blockerMetadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->$`)
)

// blockersPath returns the path to the blockers.md file for a project.
func blockersPath(project *models.Project) string {
	return filepath.Join(project.Path, blockersFileName)
}

// LoadBlockers reads the blockers from a project's blockers.md file.
// Returns an empty slice if the file doesn't exist.
func LoadBlockers(project *models.Project) ([]*models.Blocker, error) {
	path := blockersPath(project)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []*models.Blocker{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parseBlockersMarkdown(string(content))
}

// SaveBlockers writes blockers to a project's blockers.md file.
func SaveBlockers(project *models.Project, blockers []*models.Blocker) error {
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return err
	}

	content := renderBlockersMarkdown(blockers)
	path := blockersPath(project)
	return util.AtomicWriteFile(path, []byte(content), FilePermissions)
}

// GetActiveBlockerCount returns the count of unresolved blockers for a project.
func GetActiveBlockerCount(project *models.Project) int {
	blockers, err := LoadBlockers(project)
	if err != nil {
		return 0
	}

	count := 0
	for _, b := range blockers {
		if !b.IsResolved() {
			count++
		}
	}
	return count
}

// parseBlockersMarkdown parses blockers.md content into Blocker structs.
func parseBlockersMarkdown(content string) ([]*models.Blocker, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Collect all lines for lookahead (needed to distinguish blocker headings
	// from ## headings inside descriptions).
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var blockers []*models.Blocker
	var currentBlocker *models.Blocker
	var descBuilder strings.Builder

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Skip only the exact file header "# Blockers" (not arbitrary H1 lines
		// that might appear in descriptions).
		if strings.TrimSpace(line) == "# Blockers" {
			continue
		}

		// A line matching "## <title>" is a real blocker heading only when the
		// next non-empty line is a metadata comment (<!-- ... -->). Without this
		// lookahead, ## headings inside a blocker's description would be
		// misinterpreted as new blockers, corrupting data on roundtrip.
		if matches := blockerHeadingRegex.FindStringSubmatch(line); matches != nil {
			nextLine := ""
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "" {
					nextLine = strings.TrimSpace(lines[j])
					break
				}
			}
			if blockerMetadataRegex.MatchString(nextLine) {
				// Save previous blocker's description
				if currentBlocker != nil {
					currentBlocker.Description = strings.TrimSpace(descBuilder.String())
					blockers = append(blockers, currentBlocker)
				}

				currentBlocker = &models.Blocker{
					Title: strings.TrimSpace(matches[1]),
				}
				descBuilder.Reset()
				continue
			}
			// No metadata comment follows — treat as description content, fall through
		}

		// Check for metadata comment (only if we have a current blocker without ID)
		if currentBlocker != nil && currentBlocker.ID == "" {
			if matches := blockerMetadataRegex.FindStringSubmatch(line); matches != nil {
				parseBlockerMetadata(currentBlocker, matches[1])
				continue
			}
		}

		// Accumulate description content
		if currentBlocker != nil {
			descBuilder.WriteString(line)
			descBuilder.WriteString("\n")
		}
	}

	// Don't forget the last blocker
	if currentBlocker != nil {
		currentBlocker.Description = strings.TrimSpace(descBuilder.String())
		blockers = append(blockers, currentBlocker)
	}

	return blockers, nil
}

// parseBlockerMetadata parses metadata string and sets blocker fields.
// Format: "id: uuid, created: 2026-01-15T10:30:00, resolved: 2026-01-16T14:30:00"
func parseBlockerMetadata(blocker *models.Blocker, metaStr string) {
	pairs := strings.SplitSeq(metaStr, ",")
	for pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "id":
			blocker.ID = value
		case "created":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				blocker.CreatedAt = t
			}
		case "resolved":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				blocker.ResolvedAt = &t
			}
		}
	}
}

// renderBlockersMarkdown renders blockers to markdown format.
func renderBlockersMarkdown(blockers []*models.Blocker) string {
	var b strings.Builder

	b.WriteString("# Blockers\n\n")

	for _, blocker := range blockers {
		// Title as heading
		fmt.Fprintf(&b, "## %s\n", blocker.Title)

		// Metadata comment
		meta := fmt.Sprintf("id: %s, created: %s", blocker.ID, blocker.CreatedAt.Format(time.RFC3339))
		if blocker.ResolvedAt != nil {
			meta += fmt.Sprintf(", resolved: %s", blocker.ResolvedAt.Format(time.RFC3339))
		}
		fmt.Fprintf(&b, "<!-- %s -->\n", meta)

		// Description (if any)
		if blocker.Description != "" {
			b.WriteString("\n")
			b.WriteString(blocker.Description)
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	return b.String()
}

// FindBlockerByPrefix finds a blocker by ID prefix (for CLI convenience).
func FindBlockerByPrefix(blockers []*models.Blocker, prefix string) *models.Blocker {
	prefix = strings.ToLower(prefix)
	for _, b := range blockers {
		if strings.HasPrefix(strings.ToLower(b.ID), prefix) {
			return b
		}
	}
	return nil
}
