package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func legacyV4MetadataMap(fields []LegacyV4MetadataField) (map[string]string, error) {
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		if _, exists := result[field.Key]; exists {
			return nil, fmt.Errorf("duplicate metadata field %q", field.Key)
		}
		result[field.Key] = field.Value
	}
	return result, nil
}

func requiredLegacyV4Timestamp(metadata map[string]string, key string, location *time.Location) (time.Time, error) {
	raw := strings.TrimSpace(metadata[key])
	if raw == "" {
		return time.Time{}, fmt.Errorf("missing %s", key)
	}
	value, err := parseLegacyV4Timestamp(raw, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func normalizeLegacyV4Date(raw string, location *time.Location) (string, error) {
	if parsed, err := time.ParseInLocation("2006-01-02", raw, location); err == nil {
		return parsed.Format("2006-01-02"), nil
	}
	parsed, err := parseLegacyV4Timestamp(raw, location)
	if err != nil {
		return "", err
	}
	return parsed.In(location).Format("2006-01-02"), nil
}

func normalizeLegacyV4Tag(raw string) (string, error) {
	tag := strings.TrimSpace(raw)
	tag = strings.TrimPrefix(tag, "#")
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("empty tag")
	}
	if len(tag) > 100 {
		return "", fmt.Errorf("tag %q exceeds 100 bytes", tag)
	}
	if strings.ContainsAny(tag, ",;\n\r") {
		return "", fmt.Errorf("tag %q contains an invalid delimiter", tag)
	}
	switch strings.ToLower(tag) {
	case "epic", "type:epic":
		return "epic", nil
	case "story", "type:story":
		return "story", nil
	case "bug", "type:bug":
		return "bug", nil
	case "idea", "type:idea":
		return "idea", nil
	}
	return tag, nil
}

func addV5TaskTag(tags *[]string, tag string) {
	for _, existing := range *tags {
		if existing == tag {
			return
		}
	}
	*tags = append(*tags, tag)
}

func splitLegacyV4Set(raw string) []string {
	var result []string
	seen := make(map[string]bool)
	for value := range strings.SplitSeq(raw, ";") {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func unescapeLegacyV4MetadataScalar(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var result strings.Builder
	result.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			switch value[index+1] {
			case '\\':
				result.WriteByte('\\')
				index++
				continue
			case ',':
				result.WriteByte(',')
				index++
				continue
			}
		}
		result.WriteByte(value[index])
	}
	return result.String()
}

func decodeLegacyV4Attachments(raw string) ([]V5TaskAttachment, error) {
	var attachments []V5TaskAttachment
	for _, entry := range splitAttachmentEntries(raw) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		var addedAt *time.Time
		if index := strings.LastIndex(entry, "@"); index > 0 {
			epoch, err := strconv.ParseInt(entry[index+1:], 10, 64)
			if err != nil || epoch < 0 {
				return nil, fmt.Errorf("invalid attachment timestamp in %q", entry)
			}
			value := time.Unix(epoch, 0).UTC()
			addedAt = &value
			entry = entry[:index]
		}

		originalPath := ""
		if index := strings.Index(entry, "||"); index > 0 {
			originalPath = entry[index+2:]
			entry = entry[:index]
		}

		attachment := V5TaskAttachment{Kind: "file", AddedAt: addedAt}
		switch {
		case strings.HasPrefix(entry, "url:"):
			attachment.Kind = "url"
			attachment.Filename = unescapeAttachmentField(strings.TrimPrefix(entry, "url:"))
		case strings.HasPrefix(entry, "code:"):
			attachment.Kind = "code"
			attachment.Filename = unescapeAttachmentField(strings.TrimPrefix(entry, "code:"))
			if index := strings.LastIndex(originalPath, ":"); index >= 0 {
				prefix, suffix := originalPath[:index], originalPath[index+1:]
				if isDecimalDigits(suffix) && looksLikeCodeRefPath(prefix) {
					line, _ := strconv.Atoi(suffix)
					if line > 0 {
						attachment.LineNumber = line
						originalPath = prefix
					}
				}
			}
		default:
			attachment.Filename = strings.TrimPrefix(entry, "file:")
			attachment.Filename = unescapeAttachmentField(attachment.Filename)
		}
		attachment.OriginalPath = unescapeAttachmentField(originalPath)
		if attachment.Kind == "url" && attachment.OriginalPath == "" {
			attachment.OriginalPath = attachment.Filename
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func decodeLegacyV4Checklist(raw string) []V5ChecklistItem {
	var checklist []V5ChecklistItem
	for _, entry := range splitChecklistItems(raw) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		item := V5ChecklistItem{}
		switch {
		case strings.HasSuffix(entry, "[x]"):
			item.Done = true
			entry = strings.TrimSuffix(entry, "[x]")
		case strings.HasSuffix(entry, "[ ]"):
			entry = strings.TrimSuffix(entry, "[ ]")
		}
		item.Text = unescapeChecklistText(strings.TrimSpace(entry))
		if item.Text != "" {
			checklist = append(checklist, item)
		}
	}
	return checklist
}

func convertLegacyV4Recurrence(
	raw string,
	dueDate string,
	addAudit func(action, field, from, to, reason string),
) (*V5TaskRecurrence, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	expression := raw
	if strings.HasSuffix(expression, "~") {
		expression = strings.TrimSuffix(expression, "~")
		addAudit("discarded", "recurrence.mode", "from_completion", "", "v5 recurrence always advances from the prior due date")
	}

	weekdaySpec := ""
	if base, weekdays, ok := strings.Cut(expression, ":"); ok {
		expression = base
		weekdaySpec = weekdays
		if dueDate == "" {
			return nil, fmt.Errorf("weekday recurrence %q has no due date to anchor it", raw)
		}
	}

	recurrence := &V5TaskRecurrence{Interval: 1}
	switch expression {
	case "daily":
		recurrence.Unit = "day"
	case "weekly":
		recurrence.Unit = "week"
	case "biweekly":
		recurrence.Interval = 2
		recurrence.Unit = "week"
	case "monthly":
		recurrence.Unit = "month"
	case "yearly":
		recurrence.Interval = 12
		recurrence.Unit = "month"
		addAudit("converted", "recurrence.unit", "year", "12 months", "v5 recurrence has no yearly unit")
	default:
		if len(expression) < 2 {
			return nil, fmt.Errorf("invalid recurrence %q", raw)
		}
		interval, err := strconv.Atoi(expression[:len(expression)-1])
		if err != nil || interval <= 0 {
			return nil, fmt.Errorf("invalid recurrence %q", raw)
		}
		recurrence.Interval = interval
		switch expression[len(expression)-1] {
		case 'd':
			recurrence.Unit = "day"
		case 'w':
			recurrence.Unit = "week"
		case 'm':
			recurrence.Unit = "month"
		case 'y':
			recurrence.Unit = "month"
			recurrence.Interval *= 12
			addAudit("converted", "recurrence.unit", expression, strconv.Itoa(recurrence.Interval)+" months", "v5 recurrence has no yearly unit")
		default:
			return nil, fmt.Errorf("invalid recurrence %q", raw)
		}
	}

	if weekdaySpec != "" {
		if recurrence.Unit != "week" {
			return nil, fmt.Errorf("weekday recurrence %q does not use a weekly interval", raw)
		}
		if err := validateLegacyV4Weekdays(weekdaySpec); err != nil {
			return nil, fmt.Errorf("invalid recurrence %q: %w", raw, err)
		}
		addAudit("discarded", "recurrence.weekdays", weekdaySpec, "", "v5 keeps the weekly interval anchored by the existing due date")
	}
	return recurrence, nil
}

func validateLegacyV4Weekdays(value string) error {
	valid := map[string]bool{
		"sun": true, "mon": true, "tue": true, "wed": true,
		"thu": true, "fri": true, "sat": true,
	}
	seen := make(map[string]bool)
	for day := range strings.SplitSeq(value, ",") {
		day = strings.ToLower(strings.TrimSpace(day))
		if !valid[day] {
			return fmt.Errorf("invalid weekday %q", day)
		}
		if seen[day] {
			return fmt.Errorf("duplicate weekday %q", day)
		}
		seen[day] = true
	}
	if len(seen) == 0 {
		return fmt.Errorf("empty weekday list")
	}
	return nil
}

func legacyV4DiscardReason(key string) string {
	switch {
	case key == "related":
		return "related links remain snapshot-only"
	case key == "dimmed":
		return "dim state remains snapshot-only"
	case key == "parked" || key == "parked_at":
		return "parking data remains snapshot-only"
	case key == "start" || key == "estimated_days":
		return "retired scheduling metadata remains snapshot-only"
	case strings.HasPrefix(key, "score."):
		return "scores remain snapshot-only"
	case strings.HasPrefix(key, "bug."), strings.HasPrefix(key, "idea."),
		strings.HasPrefix(key, "task."), strings.HasPrefix(key, "story."),
		strings.HasPrefix(key, "epic."):
		return "type-specific metadata remains snapshot-only"
	default:
		return "unsupported legacy metadata remains snapshot-only"
	}
}
