package storage

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/storage/format"
)

var (
	legacyV4BlockerHeading  = regexp.MustCompile(`^##\s+(.+)$`)
	legacyV4BlockerMetadata = regexp.MustCompile(`^<!--\s*(.+?)\s*-->$`)
)

// LegacyV4Blocker is the migration-only representation of one standalone
// project blocker. Resolved blockers remain in the rollback snapshot only;
// unresolved blockers become ordinary Inbox tasks tagged blocker.
type LegacyV4Blocker struct {
	ID          string
	Title       string
	Description string
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}

// DecodeLegacyV4Blockers strictly decodes the retired blockers.md format
// without constructing a runtime blocker model.
func DecodeLegacyV4Blockers(content string) ([]LegacyV4Blocker, error) {
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(content, "\uFEFF")))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, strings.TrimSuffix(scanner.Text(), "\r"))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan legacy v4 blockers: %w", err)
	}

	var blockers []LegacyV4Blocker
	var current *LegacyV4Blocker
	var description strings.Builder
	flush := func() error {
		if current == nil {
			return nil
		}
		current.Description = strings.TrimSpace(description.String())
		if strings.TrimSpace(current.ID) == "" || strings.TrimSpace(current.Title) == "" {
			return fmt.Errorf("legacy v4 blocker requires an id and title")
		}
		if current.CreatedAt.IsZero() {
			return fmt.Errorf("legacy v4 blocker %q requires a created timestamp", current.ID)
		}
		blockers = append(blockers, *current)
		current = nil
		description.Reset()
		return nil
	}

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if current == nil && strings.TrimSpace(line) == "# Blockers" {
			continue
		}
		if match := legacyV4BlockerHeading.FindStringSubmatch(line); match != nil {
			next := ""
			for lookahead := index + 1; lookahead < len(lines); lookahead++ {
				if strings.TrimSpace(lines[lookahead]) != "" {
					next = strings.TrimSpace(lines[lookahead])
					break
				}
			}
			if legacyV4BlockerMetadata.MatchString(next) {
				if err := flush(); err != nil {
					return nil, err
				}
				current = &LegacyV4Blocker{Title: strings.TrimSpace(match[1])}
				continue
			}
		}
		if current != nil && current.ID == "" {
			if match := legacyV4BlockerMetadata.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				if err := decodeLegacyV4BlockerMetadata(current, match[1]); err != nil {
					return nil, err
				}
				continue
			}
		}
		if current != nil {
			description.WriteString(line)
			description.WriteByte('\n')
			continue
		}
		if strings.TrimSpace(line) != "" {
			return nil, fmt.Errorf("unexpected legacy v4 blockers content %q", line)
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return blockers, nil
}

func decodeLegacyV4BlockerMetadata(blocker *LegacyV4Blocker, raw string) error {
	seen := make(map[string]bool)
	for pair := range strings.SplitSeq(raw, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(pair), ":")
		if !ok {
			return fmt.Errorf("invalid legacy v4 blocker metadata pair %q", pair)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if seen[key] {
			return fmt.Errorf("duplicate legacy v4 blocker metadata field %q", key)
		}
		seen[key] = true
		switch key {
		case "id":
			blocker.ID = value
		case "created":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return fmt.Errorf("parse legacy v4 blocker created timestamp: %w", err)
			}
			blocker.CreatedAt = parsed
		case "resolved":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return fmt.Errorf("parse legacy v4 blocker resolved timestamp: %w", err)
			}
			blocker.ResolvedAt = &parsed
		default:
			return fmt.Errorf("unknown legacy v4 blocker metadata field %q", key)
		}
	}
	return nil
}

// ConvertLegacyV4Blockers produces the unresolved blockers that participate in
// the workspace-wide task reintegration pass. That pass assigns final ShortIDs
// and validates stable-ID uniqueness and references.
func ConvertLegacyV4Blockers(blockers []LegacyV4Blocker) []format.V5TaskRecord {
	var tasks []format.V5TaskRecord
	for _, blocker := range blockers {
		if blocker.ResolvedAt != nil {
			continue
		}
		tasks = append(tasks, format.V5TaskRecord{
			ID:          blocker.ID,
			Title:       blocker.Title,
			Description: blocker.Description,
			State:       "inbox",
			Priority:    3,
			Tags:        []string{"blocker"},
			CreatedAt:   blocker.CreatedAt,
			UpdatedAt:   blocker.CreatedAt,
		})
	}
	return tasks
}
