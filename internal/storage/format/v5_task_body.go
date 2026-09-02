package format

import (
	"fmt"
	"strings"
)

// This file owns the v5 task body: the rich-text blocks written beneath the
// metadata comment, and the reader that recovers them. The metadata line itself
// belongs to v5_task_codec.go.

// WriteV5TaskBody renders one task's metadata and retained rich text blocks.
// The caller owns the generic Task heading because its level is document
// hierarchy state rather than task-record state.
func WriteV5TaskBody(task V5TaskRecord) (string, error) {
	if err := ValidateV5TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV5TaskMetadata(task)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(metadata)
	builder.WriteString("\n")
	if task.Description != "" {
		builder.WriteString("<!-- DESCRIPTION -->\n")
		builder.WriteString(escapeBlockText(task.Description))
		builder.WriteString("\n<!-- /DESCRIPTION -->\n")
	}
	if len(task.Comments) > 0 {
		builder.WriteString("<!-- COMMENTS -->\n")
		for _, comment := range task.Comments {
			fmt.Fprintf(&builder, "<!-- comment: %s -->\n", formatV5Time(comment.CreatedAt))
			builder.WriteString(escapeBlockText(comment.Text))
			builder.WriteString("\n")
		}
		builder.WriteString("<!-- /COMMENTS -->\n")
	}
	if len(task.Decisions) > 0 {
		builder.WriteString("<!-- DECISIONS -->\n")
		for _, decision := range task.Decisions {
			fmt.Fprintf(&builder, "<!-- decision: %s -->\n", formatV5Time(decision.CreatedAt))
			builder.WriteString(escapeBlockText(decision.Text))
			builder.WriteString("\n<!-- rationale -->\n")
			builder.WriteString(escapeBlockText(decision.Rationale))
			builder.WriteString("\n")
		}
		builder.WriteString("<!-- /DECISIONS -->\n")
	}
	return builder.String(), nil
}

// ParseV5TaskBody parses one task body beneath a generic Task heading.
func ParseV5TaskBody(title, body string) (V5TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V5TaskRecord{}, fmt.Errorf("v5 task %q is missing metadata", title)
	}
	task, err := ParseV5TaskMetadata(lines[0])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
	}
	task.Title = title

	var descriptionSeen, commentsSeen, decisionsSeen bool
	for i := 1; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		switch {
		case descriptionStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if descriptionSeen {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q has duplicate DESCRIPTION blocks", title)
			}
			var block []string
			block, i, err = readV5Block(lines, i, descriptionEndRegex, "DESCRIPTION")
			if err != nil {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
			}
			task.Description = unescapeBlockText(strings.Join(block, "\n"))
			descriptionSeen = true
		case commentsStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if commentsSeen {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q has duplicate COMMENTS blocks", title)
			}
			var block []string
			block, i, err = readV5Block(lines, i, commentsEndRegex, "COMMENTS")
			if err != nil {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
			}
			task.Comments, err = parseV5Comments(block)
			if err != nil {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
			}
			commentsSeen = true
		case decisionsStartRegex.MatchString(strings.TrimSpace(lines[i])):
			if decisionsSeen {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q has duplicate DECISIONS blocks", title)
			}
			var block []string
			block, i, err = readV5Block(lines, i, decisionsEndRegex, "DECISIONS")
			if err != nil {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
			}
			task.Decisions, err = parseV5Decisions(block)
			if err != nil {
				return V5TaskRecord{}, fmt.Errorf("v5 task %q: %w", title, err)
			}
			decisionsSeen = true
		default:
			return V5TaskRecord{}, fmt.Errorf("v5 task %q has unexpected body line %q", title, lines[i])
		}
	}
	if err := ValidateV5TaskRecord(task); err != nil {
		return V5TaskRecord{}, err
	}
	return task, nil
}

func readV5Block(lines []string, start int, end interface{ MatchString(string) bool }, name string) ([]string, int, error) {
	var block []string
	for i := start + 1; i < len(lines); i++ {
		if end.MatchString(strings.TrimSpace(lines[i])) {
			return block, i + 1, nil
		}
		block = append(block, lines[i])
	}
	return nil, len(lines), fmt.Errorf("unterminated %s block", name)
}

func parseV5Comments(lines []string) ([]V5TaskComment, error) {
	var comments []V5TaskComment
	for i := 0; i < len(lines); {
		match := commentHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if match == nil || strings.Contains(match[1], ",") {
			return nil, fmt.Errorf("invalid v5 comment header %q", lines[i])
		}
		createdAt, err := parseV5Time(match[1])
		if err != nil {
			return nil, fmt.Errorf("parse v5 comment timestamp: %w", err)
		}
		i++
		start := i
		for i < len(lines) && commentHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i])) == nil {
			i++
		}
		comments = append(comments, V5TaskComment{
			Text:      unescapeBlockText(strings.Join(lines[start:i], "\n")),
			CreatedAt: createdAt,
		})
	}
	return comments, nil
}

func parseV5Decisions(lines []string) ([]V5TaskDecision, error) {
	var decisions []V5TaskDecision
	for i := 0; i < len(lines); {
		match := decisionHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if match == nil || strings.Contains(match[1], ",") {
			return nil, fmt.Errorf("invalid v5 decision header %q", lines[i])
		}
		createdAt, err := parseV5Time(match[1])
		if err != nil {
			return nil, fmt.Errorf("parse v5 decision timestamp: %w", err)
		}
		i++
		textStart := i
		for i < len(lines) && !rationaleMarkerRegex.MatchString(strings.TrimSpace(lines[i])) {
			if decisionHeaderRegex.MatchString(strings.TrimSpace(lines[i])) {
				return nil, fmt.Errorf("v5 decision is missing a rationale marker")
			}
			i++
		}
		if i == len(lines) {
			return nil, fmt.Errorf("v5 decision is missing a rationale marker")
		}
		text := unescapeBlockText(strings.Join(lines[textStart:i], "\n"))
		i++
		rationaleStart := i
		for i < len(lines) && decisionHeaderRegex.FindStringSubmatch(strings.TrimSpace(lines[i])) == nil {
			i++
		}
		decisions = append(decisions, V5TaskDecision{
			Text:      text,
			Rationale: unescapeBlockText(strings.Join(lines[rationaleStart:i], "\n")),
			CreatedAt: createdAt,
		})
	}
	return decisions, nil
}
