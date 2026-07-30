package format

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const legacyV4TaskScannerMaxBytes = 16 * 1024 * 1024
const legacyV4MaxScoreDimensionIDLength = 32

var (
	legacyV4ProjectRegex = regexp.MustCompile(`^#\s+Project:\s*(.+?)\s*$`)
	// Completed v4 archives did not repeat the active document's "# Project"
	// heading. Their durable project identity lived in this archive header
	// instead, so migration must recognize both document shapes.
	legacyV4ArchiveProjectRegex = regexp.MustCompile(`^<!--\s*project:\s*(.+?)(?:,\s*count:\s*\d+)?\s*-->\s*$`)
	legacyV4SchemaRegex         = regexp.MustCompile(`^<!--\s*tt-schema:\s*(\d+)\s*-->\s*$`)
	legacyV4HeadingRegex        = regexp.MustCompile(`^(#{2,7})\s+(Epic|Story|Task|Bug|Idea|Note|Action):\s*(.+?)\s*$`)
	legacyV4ActivityStartRegex  = regexp.MustCompile(`^<!--\s*ACTIVITY\s*-->$`)
	legacyV4ActivityEndRegex    = regexp.MustCompile(`^<!--\s*/ACTIVITY\s*-->$`)
)

// Keep this list local to the v4 decoder. The live metadata registry will shed
// retired fields as the runtime model is simplified, but migration must still
// recognize their boundaries in pre-v5 Markdown.
var legacyV4MetadataKeys = []string{
	"id", "status", "state", "priority", "parent", "tags", "section",
	"created", "updated", "completed", "due", "start", "estimated_days",
	"depends", "related", "pinned", "pinned_at", "active", "dimmed",
	"snoozed", "parked", "parked_at", "recur", "recur.skipped",
	"recur.streak", "recur.max_streak", "recur.original_day", "attachments",
	"checklist", "stage", "source", "url", "bug.reproducible", "bug.severity",
	"bug.workaround", "idea.category", "idea.effort", "idea.value",
	"idea.validated", "task.effort", "task.worktype", "story.points",
	"story.persona", "epic.scope", "epic.milestone", "epic.metrics",
	"epic.initial_children", "epic.scope_baselined",
}

// LegacyV4TaskDocument is the migration-only representation of one v4 task
// Markdown document. It deliberately contains no runtime model types: migration
// must decode every retired field before the runtime parser can normalize or
// discard it.
type LegacyV4TaskDocument struct {
	ProjectName   string         `json:"project_name"`
	SchemaVersion int            `json:"schema_version"`
	Tasks         []LegacyV4Task `json:"tasks"`
}

// LegacyV4Task captures one task heading, its raw metadata, and the rich blocks
// that were persisted by v4. Metadata remains ordered so duplicate fields can
// be rejected by the pure conversion pass instead of being silently overwritten.
type LegacyV4Task struct {
	HeadingType string                  `json:"heading_type"`
	Level       int                     `json:"level"`
	Title       string                  `json:"title"`
	Metadata    []LegacyV4MetadataField `json:"metadata"`
	Description string                  `json:"description,omitempty"`
	Activity    []string                `json:"activity,omitempty"`
	Comments    []LegacyV4Comment       `json:"comments,omitempty"`
	Decisions   []LegacyV4Decision      `json:"decisions,omitempty"`
}

type LegacyV4MetadataField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LegacyV4Comment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved,omitempty"`
}

type LegacyV4Decision struct {
	Text      string    `json:"text"`
	Rationale string    `json:"rationale,omitempty"`
	Category  string    `json:"category,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved,omitempty"`
}

// DecodeLegacyV4TaskDocument decodes v4 task Markdown without constructing or
// mutating runtime tasks. The caller supplies the workspace location so legacy
// wall-clock timestamps are interpreted consistently with date migration.
func DecodeLegacyV4TaskDocument(content string, location *time.Location) (LegacyV4TaskDocument, error) {
	if location == nil {
		return LegacyV4TaskDocument{}, fmt.Errorf("legacy v4 task location is nil")
	}

	content = strings.TrimPrefix(content, "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), legacyV4TaskScannerMaxBytes)

	document := LegacyV4TaskDocument{SchemaVersion: 1}
	var current *legacyV4TaskDraft

	flush := func() error {
		if current == nil {
			return nil
		}
		task, err := current.finish(location)
		if err != nil {
			return fmt.Errorf("decode task %q: %w", current.title, err)
		}
		document.Tasks = append(document.Tasks, task)
		current = nil
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if match := legacyV4HeadingRegex.FindStringSubmatch(line); match != nil {
			if err := flush(); err != nil {
				return LegacyV4TaskDocument{}, err
			}
			current = &legacyV4TaskDraft{
				headingType: match[2],
				level:       len(match[1]) - 2,
				title:       strings.TrimSpace(match[3]),
			}
			continue
		}

		if current != nil {
			current.lines = append(current.lines, line)
			continue
		}
		if match := legacyV4ProjectRegex.FindStringSubmatch(line); match != nil {
			document.ProjectName = strings.TrimSpace(match[1])
			continue
		}
		if document.ProjectName == "" {
			if match := legacyV4ArchiveProjectRegex.FindStringSubmatch(line); match != nil {
				document.ProjectName = strings.TrimSpace(match[1])
				continue
			}
		}
		if match := legacyV4SchemaRegex.FindStringSubmatch(line); match != nil {
			version, err := strconv.Atoi(match[1])
			if err != nil {
				return LegacyV4TaskDocument{}, fmt.Errorf("invalid task schema %q", match[1])
			}
			document.SchemaVersion = version
		}
	}
	if err := scanner.Err(); err != nil {
		return LegacyV4TaskDocument{}, fmt.Errorf("scan legacy v4 task document: %w", err)
	}
	if err := flush(); err != nil {
		return LegacyV4TaskDocument{}, err
	}
	if document.ProjectName == "" {
		return LegacyV4TaskDocument{}, fmt.Errorf("legacy v4 task document is missing project name")
	}
	if document.SchemaVersion < 1 || document.SchemaVersion > 2 {
		return LegacyV4TaskDocument{}, fmt.Errorf("unsupported task schema %d", document.SchemaVersion)
	}
	return document, nil
}

type legacyV4TaskDraft struct {
	headingType string
	level       int
	title       string
	lines       []string
}

func (draft legacyV4TaskDraft) finish(location *time.Location) (LegacyV4Task, error) {
	task := LegacyV4Task{
		HeadingType: draft.headingType,
		Level:       draft.level,
		Title:       draft.title,
	}

	body := make([]string, 0, len(draft.lines))
	metadataFound := false
	for _, line := range draft.lines {
		match := MetadataRegex().FindStringSubmatch(strings.TrimSpace(line))
		if !metadataFound && match != nil && strings.HasPrefix(strings.TrimSpace(match[1]), "id:") {
			fields, err := decodeLegacyV4Metadata(match[1])
			if err != nil {
				return LegacyV4Task{}, err
			}
			task.Metadata = fields
			metadataFound = true
			continue
		}
		body = append(body, line)
	}
	if !metadataFound {
		return LegacyV4Task{}, fmt.Errorf("missing task metadata")
	}

	rawBody := strings.TrimSpace(strings.Join(body, "\n"))
	activityLines, clean := ExtractBlock(rawBody, legacyV4ActivityStartRegex, legacyV4ActivityEndRegex)
	task.Activity = legacyV4OriginalLines(activityLines)

	commentLines, clean := ExtractBlock(clean, commentsStartRegex, commentsEndRegex)
	comments, err := decodeLegacyV4Comments(commentLines, location)
	if err != nil {
		return LegacyV4Task{}, err
	}
	task.Comments = comments

	decisionLines, clean := ExtractBlock(clean, decisionsStartRegex, decisionsEndRegex)
	decisions, err := decodeLegacyV4Decisions(decisionLines, location)
	if err != nil {
		return LegacyV4Task{}, err
	}
	task.Decisions = decisions

	descriptionLines, outside := ExtractBlock(clean, descriptionStartRegex, descriptionEndRegex)
	if len(descriptionLines) == 0 {
		task.Description = strings.TrimSpace(outside)
	} else {
		task.Description = strings.TrimSpace(strings.Join(legacyV4OriginalLines(descriptionLines), "\n"))
		if orphaned := strings.TrimSpace(outside); orphaned != "" {
			if task.Description != "" {
				task.Description += "\n"
			}
			task.Description += orphaned
		}
	}
	task.Description = unescapeBlockText(task.Description)
	return task, nil
}

func decodeLegacyV4Metadata(payload string) ([]LegacyV4MetadataField, error) {
	pairs := splitLegacyV4MetadataPairs(payload)
	fields := make([]LegacyV4MetadataField, 0, len(pairs))
	for _, pair := range pairs {
		key, value, ok := strings.Cut(strings.TrimSpace(pair), ":")
		if !ok {
			return nil, fmt.Errorf("malformed metadata pair %q", pair)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("metadata pair %q has empty key", pair)
		}
		fields = append(fields, LegacyV4MetadataField{
			Key:   key,
			Value: strings.TrimSpace(value),
		})
	}
	return fields, nil
}

func splitLegacyV4MetadataPairs(payload string) []string {
	var pairs []string
	remaining := payload
	seen := make(map[string]bool)
	for remaining != "" {
		key, _, _ := strings.Cut(strings.TrimSpace(remaining), ":")
		seen[strings.TrimSpace(key)] = true

		index := legacyV4NextMetadataDelimiter(remaining, seen)
		if index < 0 {
			pairs = append(pairs, strings.TrimSpace(remaining))
			break
		}
		pairs = append(pairs, strings.TrimSpace(remaining[:index]))
		remaining = remaining[index+2:]
	}
	return pairs
}

func legacyV4NextMetadataDelimiter(value string, seen map[string]bool) int {
	next := -1
	for _, key := range legacyV4MetadataKeys {
		if seen[key] {
			continue
		}
		if index := legacyV4UnescapedDelimiterIndex(value, ", "+key+":"); index >= 0 && (next < 0 || index < next) {
			next = index
		}
	}
	if index := legacyV4ScoreDelimiterIndex(value, seen); index >= 0 && (next < 0 || index < next) {
		next = index
	}
	return next
}

func legacyV4UnescapedDelimiterIndex(value, delimiter string) int {
	searchFrom := 0
	for {
		index := strings.Index(value[searchFrom:], delimiter)
		if index < 0 {
			return -1
		}
		index += searchFrom
		backslashes := 0
		for i := index - 1; i >= 0 && value[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return index
		}
		searchFrom = index + 1
	}
}

func legacyV4ScoreDelimiterIndex(value string, seen map[string]bool) int {
	searchFrom := 0
	for {
		index := legacyV4UnescapedDelimiterIndex(value[searchFrom:], ", score.")
		if index < 0 {
			return -1
		}
		index += searchFrom
		rest := value[index+len(", score."):]
		colon := strings.IndexByte(rest, ':')
		if colon > 0 {
			key := "score." + rest[:colon]
			if !seen[key] && legacyV4ValidScoreKey(rest[:colon]) {
				return index
			}
		}
		searchFrom = index + len(", ")
		if searchFrom >= len(value) {
			return -1
		}
	}
}

func legacyV4ValidScoreKey(value string) bool {
	if value == "" || len(value) > legacyV4MaxScoreDimensionIDLength {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '_' {
			return false
		}
	}
	return true
}

func legacyV4OriginalLines(lines []BlockLine) []string {
	if len(lines) == 0 {
		return nil
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, line.Original)
	}
	return result
}

func decodeLegacyV4Comments(lines []BlockLine, location *time.Location) ([]LegacyV4Comment, error) {
	var comments []LegacyV4Comment
	var current *LegacyV4Comment
	var text strings.Builder

	flush := func() {
		if current == nil {
			return
		}
		current.Text = unescapeBlockText(text.String())
		if strings.TrimSpace(current.Text) != "" {
			comments = append(comments, *current)
		}
		current = nil
		text.Reset()
	}

	for _, line := range lines {
		match := commentHeaderRegex.FindStringSubmatch(line.Trimmed)
		if match != nil {
			flush()
			header := strings.SplitN(match[1], ",", 2)
			createdAt, err := parseLegacyV4Timestamp(strings.TrimSpace(header[0]), location)
			if err != nil {
				return nil, fmt.Errorf("comment timestamp: %w", err)
			}
			current = &LegacyV4Comment{CreatedAt: createdAt}
			if len(header) == 2 {
				current.Resolved = strings.TrimSpace(header[1]) == "resolved"
			}
			continue
		}
		if current != nil {
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(line.Original)
		}
	}
	flush()
	return comments, nil
}

func decodeLegacyV4Decisions(lines []BlockLine, location *time.Location) ([]LegacyV4Decision, error) {
	var decisions []LegacyV4Decision
	var current *LegacyV4Decision
	var text strings.Builder
	var rationale strings.Builder
	collectingRationale := false

	flush := func() {
		if current == nil {
			return
		}
		current.Text = unescapeBlockText(text.String())
		current.Rationale = unescapeBlockText(rationale.String())
		if strings.TrimSpace(current.Text) != "" {
			decisions = append(decisions, *current)
		}
		current = nil
		text.Reset()
		rationale.Reset()
		collectingRationale = false
	}

	for _, line := range lines {
		match := decisionHeaderRegex.FindStringSubmatch(line.Trimmed)
		if match != nil {
			flush()
			parts := strings.Split(match[1], ",")
			createdAt, err := parseLegacyV4Timestamp(strings.TrimSpace(parts[0]), location)
			if err != nil {
				return nil, fmt.Errorf("decision timestamp: %w", err)
			}
			current = &LegacyV4Decision{CreatedAt: createdAt}
			for _, part := range parts[1:] {
				part = strings.TrimSpace(part)
				switch {
				case strings.HasPrefix(part, "category:"):
					current.Category = strings.TrimSpace(strings.TrimPrefix(part, "category:"))
				case part == "resolved":
					current.Resolved = true
				}
			}
			continue
		}
		if rationaleMarkerRegex.MatchString(line.Trimmed) {
			collectingRationale = true
			continue
		}
		if current == nil {
			continue
		}
		target := &text
		if collectingRationale {
			target = &rationale
		}
		if target.Len() > 0 {
			target.WriteByte('\n')
		}
		target.WriteString(line.Original)
	}
	flush()
	return decisions, nil
}

func parseLegacyV4Timestamp(value string, location *time.Location) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}
