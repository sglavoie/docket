package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const legacyV4DefaultPriority = 3

// V5TaskConversionResult is a pure migration candidate. Nothing in this graph
// points at a runtime model or a filesystem path, so callers can validate the
// complete workspace candidate before publishing any writes.
type V5TaskConversionResult struct {
	ProjectName string                      `json:"project_name"`
	Tasks       []V5TaskRecord              `json:"tasks"`
	Audit       []V5TaskMigrationAuditEntry `json:"audit"`
}

// V5TaskRecord is the migration target shape used before the runtime v5 model
// and codecs take ownership. Retired v4 fields intentionally have no place in
// this type.
type V5TaskRecord struct {
	ID           string             `json:"id"`
	ShortID      int                `json:"short_id,omitempty"`
	Title        string             `json:"title"`
	Description  string             `json:"description,omitempty"`
	State        string             `json:"state"`
	Priority     int                `json:"priority"`
	ParentID     string             `json:"parent_id,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty"`
	DueDate      string             `json:"due_date,omitempty"`
	SnoozedUntil string             `json:"snoozed_until,omitempty"`
	DependsOn    []string           `json:"depends_on,omitempty"`
	Pinned       bool               `json:"pinned,omitempty"`
	PinnedAt     *time.Time         `json:"pinned_at,omitempty"`
	Recurrence   *V5TaskRecurrence  `json:"recurrence,omitempty"`
	Attachments  []V5TaskAttachment `json:"attachments,omitempty"`
	Checklist    []V5ChecklistItem  `json:"checklist,omitempty"`
	Comments     []V5TaskComment    `json:"comments,omitempty"`
	Decisions    []V5TaskDecision   `json:"decisions,omitempty"`
}

type V5TaskRecurrence struct {
	Interval int    `json:"interval"`
	Unit     string `json:"unit"`
}

type V5TaskAttachment struct {
	Kind         string     `json:"kind"`
	Filename     string     `json:"filename"`
	OriginalPath string     `json:"original_path,omitempty"`
	LineNumber   int        `json:"line_number,omitempty"`
	AddedAt      *time.Time `json:"added_at,omitempty"`
}

type V5ChecklistItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type V5TaskComment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type V5TaskDecision struct {
	Text      string    `json:"text"`
	Rationale string    `json:"rationale,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type V5TaskMigrationAuditEntry struct {
	TaskID string `json:"task_id"`
	Action string `json:"action"`
	Field  string `json:"field"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason"`
}

// ConvertLegacyV4TaskDocument converts a decoded document without mutating the
// input or touching live data. Every transformed or discarded value produces a
// deterministic audit entry.
func ConvertLegacyV4TaskDocument(document LegacyV4TaskDocument, location *time.Location) (V5TaskConversionResult, error) {
	if location == nil {
		return V5TaskConversionResult{}, fmt.Errorf("legacy v4 task location is nil")
	}
	result := V5TaskConversionResult{ProjectName: document.ProjectName}
	for _, legacy := range document.Tasks {
		task, audit, err := convertLegacyV4Task(legacy, location)
		if err != nil {
			return V5TaskConversionResult{}, fmt.Errorf("convert task %q: %w", legacy.Title, err)
		}
		result.Tasks = append(result.Tasks, task)
		result.Audit = append(result.Audit, audit...)
	}
	return result, nil
}

//nolint:gocyclo // Conversion keeps field decisions together so the audit exactly matches the emitted task.
func convertLegacyV4Task(legacy LegacyV4Task, location *time.Location) (V5TaskRecord, []V5TaskMigrationAuditEntry, error) {
	metadata, err := legacyV4MetadataMap(legacy.Metadata)
	if err != nil {
		return V5TaskRecord{}, nil, err
	}
	id := strings.TrimSpace(metadata["id"])
	if id == "" {
		return V5TaskRecord{}, nil, fmt.Errorf("missing id")
	}
	if strings.TrimSpace(legacy.Title) == "" {
		return V5TaskRecord{}, nil, fmt.Errorf("empty title")
	}

	task := V5TaskRecord{
		ID:          id,
		Title:       legacy.Title,
		Description: legacy.Description,
		Priority:    legacyV4DefaultPriority,
	}
	var audit []V5TaskMigrationAuditEntry
	reviewState := false
	addAudit := func(action, field, from, to, reason string) {
		audit = append(audit, V5TaskMigrationAuditEntry{
			TaskID: id,
			Action: action,
			Field:  field,
			From:   from,
			To:     to,
			Reason: reason,
		})
	}

	task.State = strings.TrimSpace(metadata["state"])
	switch task.State {
	case "review":
		task.State = "ready"
		reviewState = true
		addAudit("converted", "state", "review", "ready + tag:review", "review is not a v5 workflow state")
	case "inbox", "ready", "next", "in_progress", "waiting", "paused", "someday", "done", "canceled":
	case "":
		return V5TaskRecord{}, nil, fmt.Errorf("missing state")
	default:
		return V5TaskRecord{}, nil, fmt.Errorf("invalid state %q", task.State)
	}

	if raw := metadata["priority"]; raw != "" {
		priority, parseErr := strconv.Atoi(raw)
		if parseErr != nil || priority < 0 || priority > 5 {
			return V5TaskRecord{}, nil, fmt.Errorf("invalid priority %q", raw)
		}
		task.Priority = priority
	}
	task.ParentID = strings.TrimSpace(metadata["parent"])
	task.DependsOn = splitLegacyV4Set(metadata["depends"])

	for _, raw := range splitLegacyV4Set(metadata["tags"]) {
		tag, tagErr := normalizeLegacyV4Tag(raw)
		if tagErr != nil {
			return V5TaskRecord{}, nil, tagErr
		}
		addV5TaskTag(&task.Tags, tag)
	}
	if reviewState {
		addV5TaskTag(&task.Tags, "review")
	}

	if legacy.HeadingType != "Task" {
		switch legacy.HeadingType {
		case "Epic", "Story", "Bug", "Idea":
			tag := strings.ToLower(legacy.HeadingType)
			addV5TaskTag(&task.Tags, tag)
			addAudit("converted", "type", legacy.HeadingType, tag, "v5 writes every task with the neutral Task heading")
		case "Action", "Note":
			tag := "type:" + strings.ToLower(legacy.HeadingType)
			addV5TaskTag(&task.Tags, tag)
			addAudit("converted", "type", legacy.HeadingType, tag, "v5 writes every task with the neutral Task heading")
		default:
			return V5TaskRecord{}, nil, fmt.Errorf("unsupported legacy heading type %q", legacy.HeadingType)
		}
	}
	if source := strings.TrimSpace(unescapeLegacyV4MetadataScalar(metadata["source"])); source != "" {
		tag, tagErr := normalizeLegacyV4Tag("source:" + source)
		if tagErr != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("source tag: %w", tagErr)
		}
		addV5TaskTag(&task.Tags, tag)
		addAudit("converted", "source", source, tag, "v5 represents source as a namespaced tag")
	}

	task.CreatedAt, err = requiredLegacyV4Timestamp(metadata, "created", location)
	if err != nil {
		return V5TaskRecord{}, nil, err
	}
	if updated := strings.TrimSpace(metadata["updated"]); updated != "" {
		task.UpdatedAt, err = parseLegacyV4Timestamp(updated, location)
		if err != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("updated: %w", err)
		}
	} else {
		task.UpdatedAt = task.CreatedAt
	}
	if completed := strings.TrimSpace(metadata["completed"]); completed != "" {
		value, parseErr := parseLegacyV4Timestamp(completed, location)
		if parseErr != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("completed: %w", parseErr)
		}
		task.CompletedAt = &value
	}
	if due := strings.TrimSpace(metadata["due"]); due != "" {
		task.DueDate, err = normalizeLegacyV4Date(due, location)
		if err != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("due: %w", err)
		}
		if due != task.DueDate {
			addAudit("converted", "due", due, task.DueDate, "v5 due values are local calendar dates")
		}
	}
	if snoozed := strings.TrimSpace(metadata["snoozed"]); snoozed != "" {
		task.SnoozedUntil, err = normalizeLegacyV4Date(snoozed, location)
		if err != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("snoozed: %w", err)
		}
		if snoozed != task.SnoozedUntil {
			addAudit("converted", "snoozed", snoozed, task.SnoozedUntil, "v5 snooze values are local calendar dates")
		}
	}

	if pinned := strings.TrimSpace(metadata["pinned"]); pinned != "" {
		task.Pinned, err = strconv.ParseBool(pinned)
		if err != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("invalid pinned value %q", pinned)
		}
	}
	if pinnedAt := strings.TrimSpace(metadata["pinned_at"]); pinnedAt != "" {
		value, parseErr := parseLegacyV4Timestamp(pinnedAt, location)
		if parseErr != nil {
			return V5TaskRecord{}, nil, fmt.Errorf("pinned_at: %w", parseErr)
		}
		task.PinnedAt = &value
	}

	task.Attachments, err = decodeLegacyV4Attachments(metadata["attachments"])
	if err != nil {
		return V5TaskRecord{}, nil, err
	}
	if legacyURL := strings.TrimSpace(unescapeLegacyV4MetadataScalar(metadata["url"])); legacyURL != "" {
		task.Attachments = append(task.Attachments, V5TaskAttachment{
			Kind:         "url",
			Filename:     legacyURL,
			OriginalPath: legacyURL,
		})
		addAudit("converted", "url", legacyURL, "url attachment", "v5 has no standalone URL field")
	}
	task.Checklist = decodeLegacyV4Checklist(metadata["checklist"])

	task.Recurrence, err = convertLegacyV4Recurrence(metadata["recur"], task.DueDate, addAudit)
	if err != nil {
		return V5TaskRecord{}, nil, err
	}
	for _, field := range []string{"recur.skipped", "recur.streak", "recur.max_streak", "recur.original_day"} {
		if value := strings.TrimSpace(metadata[field]); value != "" {
			addAudit("discarded", field, value, "", "rich recurrence state remains snapshot-only")
		}
	}

	for index, comment := range legacy.Comments {
		task.Comments = append(task.Comments, V5TaskComment{Text: comment.Text, CreatedAt: comment.CreatedAt})
		if comment.Resolved {
			addAudit("discarded", fmt.Sprintf("comments[%d].resolved", index), "true", "", "v5 comments have no resolved state")
		}
	}
	for index, decision := range legacy.Decisions {
		task.Decisions = append(task.Decisions, V5TaskDecision{
			Text:      decision.Text,
			Rationale: decision.Rationale,
			CreatedAt: decision.CreatedAt,
		})
		if decision.Category != "" {
			addAudit("discarded", fmt.Sprintf("decisions[%d].category", index), decision.Category, "", "v5 decisions have no category")
		}
		if decision.Resolved {
			addAudit("discarded", fmt.Sprintf("decisions[%d].resolved", index), "true", "", "v5 decisions have no resolved state")
		}
	}
	if len(legacy.Activity) > 0 {
		addAudit("discarded", "activity", strconv.Itoa(len(legacy.Activity))+" entries", "", "activity history remains snapshot-only")
	}

	retained := map[string]bool{
		"id": true, "state": true, "priority": true, "parent": true, "tags": true,
		"created": true, "updated": true, "completed": true, "due": true,
		"depends": true, "pinned": true, "pinned_at": true, "snoozed": true,
		"recur": true, "recur.skipped": true, "recur.streak": true,
		"recur.max_streak": true, "recur.original_day": true,
		"attachments": true, "checklist": true, "source": true, "url": true,
	}
	for _, field := range legacy.Metadata {
		if retained[field.Key] {
			continue
		}
		addAudit("discarded", field.Key, field.Value, "", legacyV4DiscardReason(field.Key))
	}

	return task, audit, nil
}

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
