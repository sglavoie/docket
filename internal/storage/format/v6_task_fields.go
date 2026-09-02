package format

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// This file owns the compound schema-6 metadata values: attachments and
// checklist items. Each pair round-trips one metadata field, and the escaping
// here is what keeps a delimiter inside a value from corrupting the record.

func encodeV6Attachments(attachments []V6TaskAttachment) string {
	entries := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		addedAt := ""
		if attachment.AddedAt != nil {
			addedAt = formatV6Time(*attachment.AddedAt)
		}
		entries = append(entries, strings.Join([]string{
			attachment.Kind,
			url.QueryEscape(attachment.Filename),
			url.QueryEscape(attachment.OriginalPath),
			strconv.Itoa(attachment.LineNumber),
			addedAt,
		}, "|"))
	}
	return strings.Join(entries, ";")
}

func decodeV6Attachments(value string) ([]V6TaskAttachment, error) {
	entries := strings.Split(value, ";")
	attachments := make([]V6TaskAttachment, 0, len(entries))
	for _, entry := range entries {
		fields := strings.Split(entry, "|")
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid v6 attachment entry %q", entry)
		}
		filename, err := url.QueryUnescape(fields[1])
		if err != nil {
			return nil, fmt.Errorf("decode v6 attachment filename: %w", err)
		}
		originalPath, err := url.QueryUnescape(fields[2])
		if err != nil {
			return nil, fmt.Errorf("decode v6 attachment original path: %w", err)
		}
		lineNumber, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("decode v6 attachment line number: %w", err)
		}
		attachment := V6TaskAttachment{
			Kind:         fields[0],
			Filename:     filename,
			OriginalPath: originalPath,
			LineNumber:   lineNumber,
		}
		if fields[4] != "" {
			addedAt, parseErr := parseV6Time(fields[4])
			if parseErr != nil {
				return nil, fmt.Errorf("decode v6 attachment timestamp: %w", parseErr)
			}
			attachment.AddedAt = &addedAt
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func encodeV6Checklist(items []V6ChecklistItem) string {
	entries := make([]string, 0, len(items))
	for _, item := range items {
		done := "0"
		if item.Done {
			done = "1"
		}
		entries = append(entries, done+"|"+url.QueryEscape(item.Text))
	}
	return strings.Join(entries, ";")
}

func decodeV6Checklist(value string) ([]V6ChecklistItem, error) {
	entries := strings.Split(value, ";")
	items := make([]V6ChecklistItem, 0, len(entries))
	for _, entry := range entries {
		done, text, ok := strings.Cut(entry, "|")
		if !ok || done != "0" && done != "1" {
			return nil, fmt.Errorf("invalid v6 checklist entry %q", entry)
		}
		decoded, err := url.QueryUnescape(text)
		if err != nil {
			return nil, fmt.Errorf("decode v6 checklist text: %w", err)
		}
		items = append(items, V6ChecklistItem{Text: decoded, Done: done == "1"})
	}
	return items, nil
}
