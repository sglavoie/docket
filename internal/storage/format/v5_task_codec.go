package format

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const (
	v5DateFormat = "2006-01-02"
)

var v5MetadataKeys = map[string]bool{
	"id": true, "short_id": true, "state": true, "priority": true,
	"parent": true, "tags": true, "created": true, "updated": true,
	"completed": true, "due": true, "snoozed": true, "depends": true,
	"pinned": true, "pinned_at": true, "recur": true,
	"attachments": true, "checklist": true,
}

// ValidateV5TaskRecord validates the complete target record without consulting
// any legacy task type or project tag catalog.
func ValidateV5TaskRecord(task V5TaskRecord) error {
	return validateV5TaskRecord(task, true)
}

//nolint:gocyclo // Canonical record validation intentionally centralizes every schema invariant.
func validateV5TaskRecord(task V5TaskRecord, requireTitle bool) error {
	if strings.TrimSpace(task.ID) == "" || task.ID != strings.TrimSpace(task.ID) {
		return fmt.Errorf("v5 task id is required and must be trimmed")
	}
	if strings.ContainsAny(task.ID, ",;\n\r") {
		return fmt.Errorf("v5 task id %q contains a metadata delimiter", task.ID)
	}
	if task.ShortID <= 0 {
		return fmt.Errorf("v5 task %q has invalid short id %d", task.ID, task.ShortID)
	}
	if requireTitle {
		if strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("v5 task %q has an empty title", task.ID)
		}
		if task.Title != strings.TrimSpace(task.Title) {
			return fmt.Errorf("v5 task %q title must be trimmed", task.ID)
		}
		if err := models.ValidateTaskTitle(task.Title); err != nil {
			return fmt.Errorf("v5 task %q title: %w", task.ID, err)
		}
	}
	if !isV5State(task.State) {
		return fmt.Errorf("v5 task %q has invalid state %q", task.ID, task.State)
	}
	if err := models.ValidatePriority(task.Priority, true); err != nil {
		return fmt.Errorf("v5 task %q priority: %w", task.ID, err)
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return fmt.Errorf("v5 task %q requires created and updated timestamps", task.ID)
	}
	if task.CompletedAt != nil && task.CompletedAt.IsZero() {
		return fmt.Errorf("v5 task %q has a zero completed timestamp", task.ID)
	}
	if err := validateV5Date("due", task.DueDate); err != nil {
		return fmt.Errorf("v5 task %q: %w", task.ID, err)
	}
	if err := validateV5Date("snoozed", task.SnoozedUntil); err != nil {
		return fmt.Errorf("v5 task %q: %w", task.ID, err)
	}
	if task.ParentID != "" {
		if err := validateV5Reference("parent", task.ParentID); err != nil {
			return fmt.Errorf("v5 task %q: %w", task.ID, err)
		}
		if task.ParentID == task.ID {
			return fmt.Errorf("v5 task %q cannot parent itself", task.ID)
		}
	}
	if err := validateV5StringSet("depends", task.ID, task.DependsOn); err != nil {
		return err
	}
	if err := validateCanonicalV5Tags(task.ID, task.Tags); err != nil {
		return err
	}
	if task.PinnedAt != nil && task.PinnedAt.IsZero() {
		return fmt.Errorf("v5 task %q has a zero pin timestamp", task.ID)
	}
	if task.Recurrence != nil {
		if task.Recurrence.Interval <= 0 || task.Recurrence.Interval > models.MaxRecurrenceInterval {
			return fmt.Errorf("v5 task %q recurrence interval must be between 1 and %d", task.ID, models.MaxRecurrenceInterval)
		}
		switch task.Recurrence.Unit {
		case "day", "week", "month":
		default:
			return fmt.Errorf("v5 task %q has invalid recurrence unit %q", task.ID, task.Recurrence.Unit)
		}
	}
	for i, attachment := range task.Attachments {
		if err := validateV5Attachment(attachment); err != nil {
			return fmt.Errorf("v5 task %q attachment %d: %w", task.ID, i+1, err)
		}
	}
	for i, item := range task.Checklist {
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("v5 task %q checklist item %d has empty text", task.ID, i+1)
		}
	}
	for i, comment := range task.Comments {
		if strings.TrimSpace(comment.Text) == "" {
			return fmt.Errorf("v5 task %q comment %d has empty text", task.ID, i+1)
		}
		if comment.CreatedAt.IsZero() {
			return fmt.Errorf("v5 task %q comment %d has a zero timestamp", task.ID, i+1)
		}
	}
	for i, decision := range task.Decisions {
		if strings.TrimSpace(decision.Text) == "" {
			return fmt.Errorf("v5 task %q decision %d has empty text", task.ID, i+1)
		}
		if decision.CreatedAt.IsZero() {
			return fmt.Errorf("v5 task %q decision %d has a zero timestamp", task.ID, i+1)
		}
	}
	return nil
}

func isV5State(state string) bool {
	switch state {
	case "inbox", "ready", "next", "in_progress", "waiting", "paused", "someday", "done", "canceled":
		return true
	default:
		return false
	}
}

func validateV5Date(field, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(v5DateFormat, value)
	if err != nil || parsed.Format(v5DateFormat) != value {
		return fmt.Errorf("%s date %q is not canonical YYYY-MM-DD", field, value)
	}
	return nil
}

func validateV5Reference(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s reference is empty or untrimmed", field)
	}
	if strings.ContainsAny(value, ",;\n\r") {
		return fmt.Errorf("%s reference %q contains a metadata delimiter", field, value)
	}
	return nil
}

func validateV5StringSet(field, taskID string, values []string) error {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if err := validateV5Reference(field, value); err != nil {
			return fmt.Errorf("v5 task %q: %w", taskID, err)
		}
		if value == taskID {
			return fmt.Errorf("v5 task %q cannot depend on itself", taskID)
		}
		if seen[value] {
			return fmt.Errorf("v5 task %q has duplicate %s reference %q", taskID, field, value)
		}
		seen[value] = true
	}
	return nil
}

func validateCanonicalV5Tags(taskID string, tags []string) error {
	normalized, err := models.NormalizeTaskTags(tags)
	if err != nil {
		return fmt.Errorf("v5 task %q tags: %w", taskID, err)
	}
	if !slices.Equal(tags, normalized) {
		return fmt.Errorf("v5 task %q tags are not canonical: got %v, want %v", taskID, tags, normalized)
	}
	return nil
}

func validateV5Attachment(attachment V5TaskAttachment) error {
	switch attachment.Kind {
	case "file", "url", "code":
	default:
		return fmt.Errorf("invalid kind %q", attachment.Kind)
	}
	if attachment.Filename == "" {
		return fmt.Errorf("filename is required")
	}
	if attachment.LineNumber < 0 {
		return fmt.Errorf("line number cannot be negative")
	}
	if attachment.Kind != "code" && attachment.LineNumber != 0 {
		return fmt.Errorf("line number is only valid for code attachments")
	}
	if attachment.AddedAt != nil && attachment.AddedAt.IsZero() {
		return fmt.Errorf("added timestamp cannot be zero")
	}
	return nil
}

// WriteV5TaskMetadata writes only target v5 fields in one deterministic
// metadata comment. Compound values are query-escaped so delimiters cannot
// corrupt the record boundary.
func WriteV5TaskMetadata(task V5TaskRecord) (string, error) {
	if err := validateV5TaskRecord(task, false); err != nil {
		return "", err
	}
	parts := []string{
		"id: " + task.ID,
		"short_id: " + strconv.Itoa(task.ShortID),
		"state: " + task.State,
		"priority: " + strconv.Itoa(task.Priority),
	}
	if task.ParentID != "" {
		parts = append(parts, "parent: "+task.ParentID)
	}
	if len(task.Tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(task.Tags, ";"))
	}
	parts = append(parts,
		"created: "+formatV5Time(task.CreatedAt),
		"updated: "+formatV5Time(task.UpdatedAt),
	)
	if task.CompletedAt != nil {
		parts = append(parts, "completed: "+formatV5Time(*task.CompletedAt))
	}
	if task.DueDate != "" {
		parts = append(parts, "due: "+task.DueDate)
	}
	if task.SnoozedUntil != "" {
		parts = append(parts, "snoozed: "+task.SnoozedUntil)
	}
	if len(task.DependsOn) > 0 {
		parts = append(parts, "depends: "+strings.Join(task.DependsOn, ";"))
	}
	if task.Pinned {
		parts = append(parts, "pinned: true")
	}
	if task.PinnedAt != nil {
		parts = append(parts, "pinned_at: "+formatV5Time(*task.PinnedAt))
	}
	if task.Recurrence != nil {
		parts = append(parts, "recur: "+formatV5Recurrence(*task.Recurrence))
	}
	if len(task.Attachments) > 0 {
		parts = append(parts, "attachments: "+encodeV5Attachments(task.Attachments))
	}
	if len(task.Checklist) > 0 {
		parts = append(parts, "checklist: "+encodeV5Checklist(task.Checklist))
	}
	return "<!-- " + strings.Join(parts, ", ") + " -->", nil
}

// ParseV5TaskMetadata strictly parses a canonical v5 metadata comment. It does
// not recognize legacy fields, task types, or catalog metadata.
func ParseV5TaskMetadata(comment string) (V5TaskRecord, error) {
	match := MetadataRegex().FindStringSubmatch(strings.TrimSpace(comment))
	if match == nil {
		return V5TaskRecord{}, fmt.Errorf("invalid v5 task metadata comment")
	}
	values := make(map[string]string)
	for _, pair := range strings.Split(match[1], ", ") {
		key, value, ok := strings.Cut(pair, ":")
		if !ok {
			return V5TaskRecord{}, fmt.Errorf("invalid v5 metadata pair %q", pair)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !v5MetadataKeys[key] {
			return V5TaskRecord{}, fmt.Errorf("unknown v5 metadata field %q", key)
		}
		if _, duplicate := values[key]; duplicate {
			return V5TaskRecord{}, fmt.Errorf("duplicate v5 metadata field %q", key)
		}
		values[key] = value
	}
	for _, required := range []string{"id", "short_id", "state", "priority", "created", "updated"} {
		if values[required] == "" {
			return V5TaskRecord{}, fmt.Errorf("missing required v5 metadata field %q", required)
		}
	}

	shortID, err := strconv.Atoi(values["short_id"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 short_id: %w", err)
	}
	priority, err := strconv.Atoi(values["priority"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 priority: %w", err)
	}
	created, err := parseV5Time(values["created"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 created: %w", err)
	}
	updated, err := parseV5Time(values["updated"])
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 updated: %w", err)
	}

	task := V5TaskRecord{
		ID:           values["id"],
		ShortID:      shortID,
		State:        values["state"],
		Priority:     priority,
		ParentID:     values["parent"],
		Tags:         splitV5Set(values["tags"]),
		CreatedAt:    created,
		UpdatedAt:    updated,
		DueDate:      values["due"],
		SnoozedUntil: values["snoozed"],
		DependsOn:    splitV5Set(values["depends"]),
	}
	task.Tags, err = models.NormalizeTaskTags(task.Tags)
	if err != nil {
		return V5TaskRecord{}, fmt.Errorf("parse v5 tags: %w", err)
	}
	if values["completed"] != "" {
		parsed, parseErr := parseV5Time(values["completed"])
		if parseErr != nil {
			return V5TaskRecord{}, fmt.Errorf("parse v5 completed: %w", parseErr)
		}
		task.CompletedAt = &parsed
	}
	if values["pinned"] != "" {
		if values["pinned"] != "true" {
			return V5TaskRecord{}, fmt.Errorf("v5 pinned must be true when present")
		}
		task.Pinned = true
	}
	if values["pinned_at"] != "" {
		parsed, parseErr := parseV5Time(values["pinned_at"])
		if parseErr != nil {
			return V5TaskRecord{}, fmt.Errorf("parse v5 pinned_at: %w", parseErr)
		}
		task.PinnedAt = &parsed
	}
	if values["recur"] != "" {
		recurrence, parseErr := parseV5Recurrence(values["recur"])
		if parseErr != nil {
			return V5TaskRecord{}, parseErr
		}
		task.Recurrence = &recurrence
	}
	if values["attachments"] != "" {
		task.Attachments, err = decodeV5Attachments(values["attachments"])
		if err != nil {
			return V5TaskRecord{}, err
		}
	}
	if values["checklist"] != "" {
		task.Checklist, err = decodeV5Checklist(values["checklist"])
		if err != nil {
			return V5TaskRecord{}, err
		}
	}
	if err := validateV5TaskRecord(task, false); err != nil {
		return V5TaskRecord{}, err
	}
	return task, nil
}

func formatV5Time(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func parseV5Time(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	if formatV5Time(parsed) != value {
		return time.Time{}, fmt.Errorf("timestamp %q is not canonical RFC3339Nano", value)
	}
	return parsed, nil
}

func splitV5Set(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ";")
}

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

// DerivedV5TaskTags returns the sorted vocabulary derived exclusively from
// task usage. Project tag catalogs and pinned-tag metadata are intentionally
// absent from this API.
func DerivedV5TaskTags(tasks []V5TaskRecord) []string {
	var tags []string
	for _, task := range tasks {
		tags = append(tags, task.Tags...)
	}
	tags = models.DedupeTags(tags)
	sort.Strings(tags)
	return tags
}

// EqualV5TaskRecord compares target records using instant equality for
// timestamps, which avoids treating equivalent RFC3339 offsets as different
// records in codec tests and migration validation.
func EqualV5TaskRecord(left, right V5TaskRecord) bool {
	if left.ID != right.ID || left.ShortID != right.ShortID || left.Title != right.Title ||
		left.Description != right.Description || left.State != right.State ||
		left.Priority != right.Priority || left.ParentID != right.ParentID ||
		left.DueDate != right.DueDate || left.SnoozedUntil != right.SnoozedUntil ||
		left.Pinned != right.Pinned || !left.CreatedAt.Equal(right.CreatedAt) ||
		!left.UpdatedAt.Equal(right.UpdatedAt) || !equalV5TimePtr(left.CompletedAt, right.CompletedAt) ||
		!equalV5TimePtr(left.PinnedAt, right.PinnedAt) ||
		!slices.Equal(left.Tags, right.Tags) || !slices.Equal(left.DependsOn, right.DependsOn) ||
		!equalV5Recurrence(left.Recurrence, right.Recurrence) ||
		!equalV5Attachments(left.Attachments, right.Attachments) ||
		!slices.Equal(left.Checklist, right.Checklist) ||
		!equalV5Comments(left.Comments, right.Comments) ||
		!equalV5Decisions(left.Decisions, right.Decisions) {
		return false
	}
	return true
}

func equalV5TimePtr(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func equalV5Recurrence(left, right *V5TaskRecurrence) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func equalV5Attachments(left, right []V5TaskAttachment) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Kind != right[i].Kind || left[i].Filename != right[i].Filename ||
			left[i].OriginalPath != right[i].OriginalPath || left[i].LineNumber != right[i].LineNumber ||
			!equalV5TimePtr(left[i].AddedAt, right[i].AddedAt) {
			return false
		}
	}
	return true
}

func equalV5Comments(left, right []V5TaskComment) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Text != right[i].Text || !left[i].CreatedAt.Equal(right[i].CreatedAt) {
			return false
		}
	}
	return true
}

func equalV5Decisions(left, right []V5TaskDecision) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Text != right[i].Text || left[i].Rationale != right[i].Rationale ||
			!left[i].CreatedAt.Equal(right[i].CreatedAt) {
			return false
		}
	}
	return true
}
