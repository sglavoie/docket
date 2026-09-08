package format

import (
	"fmt"
	"strings"
)

// This file owns the schema-16 task body: the optional RESUME block, then the
// DESCRIPTION and COMMENTS blocks every schema from 6 on already carried. The
// metadata line is schema 15's unchanged, because schema 16 adds no metadata
// key; only the block set moved.
//
// Schema 16 is the first schema to admit a block writeTaskBody and
// parseTaskBodyBlocks do not know about, so it writes its own pair here instead
// of widening theirs. Widening the shared helpers would teach every historical
// reader a block that did not exist when its documents were written, and the
// archives a migration validates against would stop being a fixed target.
//
// What is reused is the layer below the block set: readV6Block, parseV6Comments,
// and the escaping in comments.go. Those are how a block is read and how prose
// is kept out of the grammar, not which blocks exist, and copying them would let
// two escapings drift apart while both claimed to be canonical.

// WriteV16TaskBody renders metadata plus the retained resume note, description,
// and comment blocks. The project document owns the generic Task heading.
func WriteV16TaskBody(task V16TaskRecord) (string, error) {
	if err := ValidateV16TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV15TaskMetadata(task.V15TaskRecord)
	if err != nil {
		return "", err
	}
	return writeTaskBodyV16(metadata, task.ResumeNote, task.Description, task.Comments), nil
}

func writeTaskBodyV16(metadata, resumeNote, description string, comments []V6TaskComment) string {
	var builder strings.Builder
	builder.WriteString(metadata)
	builder.WriteString("\n")
	// The note leads the body because it is the shortest and the most likely to
	// be read first by a human opening the file; the reader does not depend on
	// the order, but the canonical writer has to pick one.
	if resumeNote != "" {
		builder.WriteString("<!-- RESUME -->\n")
		builder.WriteString(escapeBlockText(resumeNote))
		builder.WriteString("\n<!-- /RESUME -->\n")
	}
	if description != "" {
		builder.WriteString("<!-- DESCRIPTION -->\n")
		builder.WriteString(escapeBlockText(description))
		builder.WriteString("\n<!-- /DESCRIPTION -->\n")
	}
	if len(comments) > 0 {
		builder.WriteString("<!-- COMMENTS -->\n")
		for _, comment := range comments {
			fmt.Fprintf(&builder, "<!-- comment: %s -->\n", formatV6Time(comment.CreatedAt))
			builder.WriteString(escapeBlockText(comment.Text))
			builder.WriteString("\n")
		}
		builder.WriteString("<!-- /COMMENTS -->\n")
	}
	return builder.String()
}

// ParseV16TaskBody parses the body below one generic Task heading.
func ParseV16TaskBody(title, body string) (V16TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V16TaskRecord{}, fmt.Errorf("v16 task %q is missing metadata", title)
	}
	metadata, err := ParseV15TaskMetadata(lines[0])
	if err != nil {
		return V16TaskRecord{}, fmt.Errorf("v16 task %q: %w", title, err)
	}
	task := V16TaskRecord{V15TaskRecord: metadata}
	task.Title = title

	task.ResumeNote, task.Description, task.Comments, err = parseTaskBodyBlocksV16("v16", title, lines)
	if err != nil {
		return V16TaskRecord{}, err
	}
	if err := ValidateV16TaskRecord(task); err != nil {
		return V16TaskRecord{}, err
	}
	return task, nil
}

// parseTaskBodyBlocksV16 reads the optional RESUME, DESCRIPTION, and COMMENTS
// blocks that follow the metadata line. lines includes the metadata line at
// index 0. A missing block is an empty value; a duplicate, unterminated, or
// unknown one is an error rather than a best guess, because every accepted shape
// is one the canonical writer can produce.
func parseTaskBodyBlocksV16(schema, title string, lines []string) (string, string, []V6TaskComment, error) {
	var (
		resumeNote, description                   string
		comments                                  []V6TaskComment
		resumeSeen, descriptionSeen, commentsSeen bool
		err                                       error
	)
	for i := 1; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		switch {
		case resumeStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if resumeSeen {
				return "", "", nil, fmt.Errorf("%s task %q has duplicate RESUME blocks", schema, title)
			}
			var block []string
			block, i, err = readV6Block(lines, i, resumeEndRegex, "RESUME")
			if err != nil {
				return "", "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			resumeNote, err = resumeNoteFromBlock(schema, title, block)
			if err != nil {
				return "", "", nil, err
			}
			resumeSeen = true
		case descriptionStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if descriptionSeen {
				return "", "", nil, fmt.Errorf("%s task %q has duplicate DESCRIPTION blocks", schema, title)
			}
			var block []string
			block, i, err = readV6Block(lines, i, descriptionEndRegex, "DESCRIPTION")
			if err != nil {
				return "", "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			description = unescapeBlockText(strings.Join(block, "\n"))
			descriptionSeen = true
		case commentsStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if commentsSeen {
				return "", "", nil, fmt.Errorf("%s task %q has duplicate COMMENTS blocks", schema, title)
			}
			var block []string
			block, i, err = readV6Block(lines, i, commentsEndRegex, "COMMENTS")
			if err != nil {
				return "", "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			comments, err = parseV6Comments(block)
			if err != nil {
				return "", "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			commentsSeen = true
		default:
			return "", "", nil, fmt.Errorf("%s task %q has unexpected body line %q", schema, title, lines[i])
		}
	}
	return resumeNote, description, comments, nil
}

// resumeNoteFromBlock reads the one line a RESUME block holds. A block with any
// other shape is refused rather than joined or ignored: the note is single-line
// by contract, and an empty block is written as no block at all, so both shapes
// are files something other than this writer produced.
func resumeNoteFromBlock(schema, title string, block []string) (string, error) {
	if len(block) != 1 {
		return "", fmt.Errorf(
			"%s task %q has a RESUME block of %d lines; a resume note is exactly one line",
			schema, title, len(block),
		)
	}
	note := unescapeBlockText(block[0])
	if note == "" {
		return "", fmt.Errorf("%s task %q has an empty RESUME block; an absent note is written as no block", schema, title)
	}
	return note, nil
}
