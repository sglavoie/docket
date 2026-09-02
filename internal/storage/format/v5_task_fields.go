package format

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// This file owns the compound v5 metadata values: recurrence, attachments, and
// checklist items. Each pair round-trips one metadata field, and the escaping
// here is what keeps a delimiter inside a value from corrupting the record.

func formatV5Recurrence(recurrence V5TaskRecurrence) string {
	unit := map[string]string{"day": "d", "week": "w", "month": "m"}[recurrence.Unit]
	return strconv.Itoa(recurrence.Interval) + unit
}

func parseV5Recurrence(value string) (V5TaskRecurrence, error) {
	if len(value) < 2 {
		return V5TaskRecurrence{}, fmt.Errorf("invalid v5 recurrence %q", value)
	}
	interval, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || interval <= 0 {
		return V5TaskRecurrence{}, fmt.Errorf("invalid v5 recurrence %q", value)
	}
	unit := map[byte]string{'d': "day", 'w': "week", 'm': "month"}[value[len(value)-1]]
	if unit == "" {
		return V5TaskRecurrence{}, fmt.Errorf("invalid v5 recurrence unit in %q", value)
	}
	return V5TaskRecurrence{Interval: interval, Unit: unit}, nil
}

func encodeV5Attachments(attachments []V5TaskAttachment) string {
	entries := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		addedAt := ""
		if attachment.AddedAt != nil {
			addedAt = formatV5Time(*attachment.AddedAt)
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

func decodeV5Attachments(value string) ([]V5TaskAttachment, error) {
	entries := strings.Split(value, ";")
	attachments := make([]V5TaskAttachment, 0, len(entries))
	for _, entry := range entries {
		fields := strings.Split(entry, "|")
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid v5 attachment entry %q", entry)
		}
		filename, err := url.QueryUnescape(fields[1])
		if err != nil {
			return nil, fmt.Errorf("decode v5 attachment filename: %w", err)
		}
		originalPath, err := url.QueryUnescape(fields[2])
		if err != nil {
			return nil, fmt.Errorf("decode v5 attachment original path: %w", err)
		}
		lineNumber, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("decode v5 attachment line number: %w", err)
		}
		attachment := V5TaskAttachment{
			Kind:         fields[0],
			Filename:     filename,
			OriginalPath: originalPath,
			LineNumber:   lineNumber,
		}
		if fields[4] != "" {
			addedAt, parseErr := parseV5Time(fields[4])
			if parseErr != nil {
				return nil, fmt.Errorf("decode v5 attachment timestamp: %w", parseErr)
			}
			attachment.AddedAt = &addedAt
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func encodeV5Checklist(items []V5ChecklistItem) string {
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

func decodeV5Checklist(value string) ([]V5ChecklistItem, error) {
	entries := strings.Split(value, ";")
	items := make([]V5ChecklistItem, 0, len(entries))
	for _, entry := range entries {
		done, text, ok := strings.Cut(entry, "|")
		if !ok || done != "0" && done != "1" {
			return nil, fmt.Errorf("invalid v5 checklist entry %q", entry)
		}
		decoded, err := url.QueryUnescape(text)
		if err != nil {
			return nil, fmt.Errorf("decode v5 checklist text: %w", err)
		}
		items = append(items, V5ChecklistItem{Text: decoded, Done: done == "1"})
	}
	return items, nil
}
