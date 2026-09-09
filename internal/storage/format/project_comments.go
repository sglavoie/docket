package format

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// This file owns the PROJECT-COMMENTS block: the one document-level block in
// live task Markdown. It sits between the schema marker and the first task
// heading, because it belongs to the project rather than to any task, and it is
// written by exactly one schema — a historical reader must never learn a block
// that did not exist when its documents were written.
//
// What it reuses is the layer below the block set: escapeBlockText and
// unescapeBlockText from comments.go, which is how prose is kept out of the
// grammar. That escaping is what makes the delimiters below unambiguous: a
// comment body can never spell "<!--" or "-->", so no stored text can close the
// block or open an entry.

// ProjectCommentsStartLine and ProjectCommentsEndLine are the exact delimiters
// the canonical writer emits. They are compared verbatim rather than by a
// tolerant pattern: every accepted shape is one this writer produces, so a
// near-miss is a malformed document rather than a spelling to normalize.
const (
	ProjectCommentsStartLine = "<!-- PROJECT-COMMENTS -->"
	ProjectCommentsEndLine   = "<!-- /PROJECT-COMMENTS -->"
)

var projectCommentHeaderRegex = regexp.MustCompile(`^<!-- project-comment: (\S+) (\S+) -->$`)

// ProjectCommentRecord is one stored project comment. It is the record shape,
// not the domain value: the timestamp is still text here because the durable
// spelling is what a codec owns.
type ProjectCommentRecord struct {
	ID        string
	Text      string
	CreatedAt time.Time
}

// WriteProjectCommentsBlock renders the canonical block, or the empty string
// when there is nothing to write. An empty collection is written as no block at
// all, so a project that never carried a comment keeps byte-identical bytes.
//
// The records are sorted into canonical order on a copy: creation instant
// first, identity to break ties. Two machines holding the same set therefore
// render the same document regardless of the order they accumulated it in.
func WriteProjectCommentsBlock(records []ProjectCommentRecord) (string, error) {
	if len(records) == 0 {
		return "", nil
	}
	if err := ValidateProjectCommentRecords(records); err != nil {
		return "", err
	}
	ordered := make([]ProjectCommentRecord, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})

	var builder strings.Builder
	builder.WriteString(ProjectCommentsStartLine)
	builder.WriteString("\n")
	for _, record := range ordered {
		fmt.Fprintf(&builder, "<!-- project-comment: %s %s -->\n", record.ID, formatProjectCommentTime(record.CreatedAt))
		builder.WriteString(escapeBlockText(record.Text))
		builder.WriteString("\n")
	}
	builder.WriteString(ProjectCommentsEndLine)
	builder.WriteString("\n")
	return builder.String(), nil
}

// ParseProjectCommentsBlock decodes the lines between the two delimiters, which
// the caller has already consumed. A block whose first line is not an entry
// header, whose identity or timestamp is unreadable, or whose text is blank is
// refused rather than repaired: every shape this accepts is one
// WriteProjectCommentsBlock can produce.
//
// An empty block is one of those refusals. WriteProjectCommentsBlock writes no
// delimiters at all for an empty collection, so a document carrying a pair with
// nothing between them was written by something else, and reading it as "no
// comments" would silently accept bytes this codec cannot reproduce.
func ParseProjectCommentsBlock(lines []string) ([]ProjectCommentRecord, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty PROJECT-COMMENTS block")
	}
	var records []ProjectCommentRecord
	for i := 0; i < len(lines); {
		match := projectCommentHeaderRegex.FindStringSubmatch(lines[i])
		if match == nil {
			return nil, fmt.Errorf("invalid project comment header %q", lines[i])
		}
		createdAt, err := parseProjectCommentTime(match[2])
		if err != nil {
			return nil, fmt.Errorf("parse project comment %s timestamp: %w", match[1], err)
		}
		i++
		start := i
		for i < len(lines) && projectCommentHeaderRegex.FindStringSubmatch(lines[i]) == nil {
			i++
		}
		records = append(records, ProjectCommentRecord{
			ID:        match[1],
			Text:      unescapeBlockText(strings.Join(lines[start:i], "\n")),
			CreatedAt: createdAt,
		})
	}
	if err := ValidateProjectCommentRecords(records); err != nil {
		return nil, err
	}
	return records, nil
}

// ValidateProjectCommentRecords holds the stored records to the domain's own
// invariants, so a document can never carry a collection a loaded project would
// be refused for.
func ValidateProjectCommentRecords(records []ProjectCommentRecord) error {
	seen := make(map[string]bool, len(records))
	for _, record := range records {
		if err := models.ValidateProjectComment(record.ID, record.Text, record.CreatedAt); err != nil {
			return err
		}
		if seen[record.ID] {
			return fmt.Errorf("duplicate project comment id %q", record.ID)
		}
		seen[record.ID] = true
	}
	return nil
}

// formatProjectCommentTime writes the instant in UTC. Normalizing the zone is
// what keeps the canonical rendering — and therefore the document revision —
// identical on two machines that recorded the same moment in different places.
func formatProjectCommentTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

// parseProjectCommentTime reads a stored instant back and normalizes it to UTC,
// so the value a document round-trips through is the value it was written with.
func parseProjectCommentTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
