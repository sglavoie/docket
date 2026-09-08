package format

import (
	"fmt"
	"strings"
)

// This file owns the schema-6 task body: the DESCRIPTION and COMMENTS blocks
// written beneath the metadata comment, and the reader that recovers them. The
// metadata line itself belongs to v6_task_codec.go. Both halves are shared with
// the later schemas that changed nothing about the body.
//
// writeTaskBody and parseTaskBodyBlocks are frozen. Every schema from 6 to the
// current one reads and writes its body through them, so widening either widens
// all of them at once: a historical reader would start accepting a block that
// did not exist when its documents were written, and the archives a migration
// validates against would stop being a fixed target. A schema that admits a new
// block writes its own body file and calls a helper of its own, exactly as
// v15_task_body.go would if schema 15 had added one.

// WriteV6TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV6TaskBody(task V6TaskRecord) (string, error) {
	if err := ValidateV6TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV6TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

func writeTaskBody(metadata, description string, comments []V6TaskComment) string {
	var builder strings.Builder
	builder.WriteString(metadata)
	builder.WriteString("\n")
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

// ParseV6TaskBody parses the body below one generic Task heading.
func ParseV6TaskBody(title, body string) (V6TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V6TaskRecord{}, fmt.Errorf("v6 task %q is missing metadata", title)
	}
	task, err := ParseV6TaskMetadata(lines[0])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("v6 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v6", title, lines)
	if err != nil {
		return V6TaskRecord{}, err
	}
	if err := ValidateV6TaskRecord(task); err != nil {
		return V6TaskRecord{}, err
	}
	return task, nil
}

// parseTaskBodyBlocks reads the optional DESCRIPTION and COMMENTS blocks that
// follow the metadata line. lines includes the metadata line at index 0.
func parseTaskBodyBlocks(schema, title string, lines []string) (string, []V6TaskComment, error) {
	var (
		description                   string
		comments                      []V6TaskComment
		descriptionSeen, commentsSeen bool
		err                           error
	)
	for i := 1; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		switch {
		case descriptionStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if descriptionSeen {
				return "", nil, fmt.Errorf("%s task %q has duplicate DESCRIPTION blocks", schema, title)
			}
			var block []string
			block, i, err = readV6Block(lines, i, descriptionEndRegex, "DESCRIPTION")
			if err != nil {
				return "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			description = unescapeBlockText(strings.Join(block, "\n"))
			descriptionSeen = true
		case commentsStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if commentsSeen {
				return "", nil, fmt.Errorf("%s task %q has duplicate COMMENTS blocks", schema, title)
			}
			var block []string
			block, i, err = readV6Block(lines, i, commentsEndRegex, "COMMENTS")
			if err != nil {
				return "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			comments, err = parseV6Comments(block)
			if err != nil {
				return "", nil, fmt.Errorf("%s task %q: %w", schema, title, err)
			}
			commentsSeen = true
		default:
			return "", nil, fmt.Errorf("%s task %q has unexpected body line %q", schema, title, lines[i])
		}
	}
	return description, comments, nil
}

func readV6Block(lines []string, start int, end interface{ MatchString(string) bool }, name string) ([]string, int, error) {
	var block []string
	for i := start + 1; i < len(lines); i++ {
		if end.MatchString(strings.TrimSpace(lines[i])) {
			return block, i + 1, nil
		}
		block = append(block, lines[i])
	}
	return nil, len(lines), fmt.Errorf("unterminated %s block", name)
}

func parseV6Comments(lines []string) ([]V6TaskComment, error) {
	var comments []V6TaskComment
	for i := 0; i < len(lines); {
		match := commentHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if match == nil || strings.Contains(match[1], ",") {
			return nil, fmt.Errorf("invalid v6 comment header %q", lines[i])
		}
		createdAt, err := parseV6Time(match[1])
		if err != nil {
			return nil, fmt.Errorf("parse v6 comment timestamp: %w", err)
		}
		i++
		start := i
		for i < len(lines) && commentHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i])) == nil {
			i++
		}
		comments = append(comments, V6TaskComment{
			Text:      unescapeBlockText(strings.Join(lines[start:i], "\n")),
			CreatedAt: createdAt,
		})
	}
	return comments, nil
}
