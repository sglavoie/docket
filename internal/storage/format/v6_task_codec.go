package format

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const v6DateFormat = "2006-01-02"

// metadataKeyDeferredLegacy is the durable metadata key schema 8 and every
// earlier schema use for the deferral date. It is a parameter of the shared
// codec rather than a literal so schema 9 can spell the same fact differently
// without forking the one place durable key order lives.
const metadataKeyDeferredLegacy = "snoozed"

var v6MetadataKeys = map[string]bool{
	"id": true, "short_id": true, "state": true, "priority": true,
	"parent": true, "tags": true, "created": true, "updated": true,
	"completed": true, "due": true, "snoozed": true,
	"pinned": true, "pinned_at": true, "attachments": true, "checklist": true,
}

// V6TaskRecord is the complete schema-6 storage shape. It is deliberately
// independent of both the live runtime model and the migration-only v5 record:
// retired relationship and legacy narrative-record data has no representation.
// Recurrence is absent from the live model entirely and remains schema-4/5
// migration input.
type V6TaskRecord struct {
	ID           string             `json:"id"`
	ShortID      int                `json:"short_id"`
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
	Pinned       bool               `json:"pinned,omitempty"`
	PinnedAt     *time.Time         `json:"pinned_at,omitempty"`
	Attachments  []V6TaskAttachment `json:"attachments,omitempty"`
	Checklist    []V6ChecklistItem  `json:"checklist,omitempty"`
	Comments     []V6TaskComment    `json:"comments,omitempty"`
}

type V6TaskAttachment struct {
	Kind         string     `json:"kind"`
	Filename     string     `json:"filename"`
	OriginalPath string     `json:"original_path,omitempty"`
	LineNumber   int        `json:"line_number,omitempty"`
	AddedAt      *time.Time `json:"added_at,omitempty"`
}

type V6ChecklistItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type V6TaskComment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidateV6TaskRecord validates a complete schema-6 task record.
func ValidateV6TaskRecord(task V6TaskRecord) error {
	return validateV6TaskRecord(task, true, metadataKeyDeferredLegacy)
}

// deferKey names the deferral-date key in the caller's own schema, so a
// rejected date is reported under the key that schema actually writes.
//
//nolint:gocyclo // Keeping all durable field invariants together makes the schema boundary auditable.
func validateV6TaskRecord(task V6TaskRecord, requireTitle bool, deferKey string) error {
	if strings.TrimSpace(task.ID) == "" || task.ID != strings.TrimSpace(task.ID) {
		return fmt.Errorf("v6 task id is required and must be trimmed")
	}
	if strings.ContainsAny(task.ID, ",;\n\r") {
		return fmt.Errorf("v6 task id %q contains a metadata delimiter", task.ID)
	}
	if task.ShortID <= 0 {
		return fmt.Errorf("v6 task %q has invalid short id %d", task.ID, task.ShortID)
	}
	if requireTitle {
		if strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("v6 task %q has an empty title", task.ID)
		}
		if task.Title != strings.TrimSpace(task.Title) {
			return fmt.Errorf("v6 task %q title must be trimmed", task.ID)
		}
		if err := models.ValidateTaskTitle(task.Title); err != nil {
			return fmt.Errorf("v6 task %q title: %w", task.ID, err)
		}
	}
	if !isV6State(task.State) {
		return fmt.Errorf("v6 task %q has invalid state %q", task.ID, task.State)
	}
	if task.Priority < 0 || task.Priority > 3 {
		return fmt.Errorf("v6 task %q priority must be between 0 and 3", task.ID)
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return fmt.Errorf("v6 task %q requires created and updated timestamps", task.ID)
	}
	if task.CompletedAt != nil && task.CompletedAt.IsZero() {
		return fmt.Errorf("v6 task %q has a zero completed timestamp", task.ID)
	}
	if err := validateV6Date("due", task.DueDate); err != nil {
		return fmt.Errorf("v6 task %q: %w", task.ID, err)
	}
	if err := validateV6Date(deferKey, task.SnoozedUntil); err != nil {
		return fmt.Errorf("v6 task %q: %w", task.ID, err)
	}
	if task.ParentID != "" {
		if err := validateV6Reference("parent", task.ParentID); err != nil {
			return fmt.Errorf("v6 task %q: %w", task.ID, err)
		}
		if task.ParentID == task.ID {
			return fmt.Errorf("v6 task %q cannot parent itself", task.ID)
		}
	}
	if err := validateCanonicalV6Tags(task.ID, task.Tags); err != nil {
		return err
	}
	if task.PinnedAt != nil && task.PinnedAt.IsZero() {
		return fmt.Errorf("v6 task %q has a zero pin timestamp", task.ID)
	}
	for i, attachment := range task.Attachments {
		if err := validateV6Attachment(attachment); err != nil {
			return fmt.Errorf("v6 task %q attachment %d: %w", task.ID, i+1, err)
		}
	}
	for i, item := range task.Checklist {
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("v6 task %q checklist item %d has empty text", task.ID, i+1)
		}
	}
	for i, comment := range task.Comments {
		if strings.TrimSpace(comment.Text) == "" {
			return fmt.Errorf("v6 task %q comment %d has empty text", task.ID, i+1)
		}
		if comment.CreatedAt.IsZero() {
			return fmt.Errorf("v6 task %q comment %d has a zero timestamp", task.ID, i+1)
		}
	}
	return nil
}

func isV6State(state string) bool {
	switch state {
	case "inbox", "backlog", "doing", "waiting", "paused", "someday", "done", "canceled":
		return true
	default:
		return false
	}
}

func validateV6Date(field, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(v6DateFormat, value)
	if err != nil || parsed.Format(v6DateFormat) != value {
		return fmt.Errorf("%s date %q is not canonical YYYY-MM-DD", field, value)
	}
	return nil
}

func validateV6Reference(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s reference is empty or untrimmed", field)
	}
	if strings.ContainsAny(value, ",;\n\r") {
		return fmt.Errorf("%s reference %q contains a metadata delimiter", field, value)
	}
	return nil
}

func validateCanonicalV6Tags(taskID string, tags []string) error {
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		// Schema 6 predates the lowercase-kebab contract. Keep this reader
		// frozen so schema-8 activation can ingest and audit legacy spellings.
		if tag == "" || strings.HasPrefix(tag, "#") || tag != strings.TrimSpace(tag) || len(tag) > models.MaxTagLength ||
			strings.ContainsAny(tag, ",;\n\r") {
			return fmt.Errorf("v6 task %q has invalid legacy tag %q", taskID, tag)
		}
		if seen[tag] {
			return fmt.Errorf("v6 task %q has duplicate tag %q", taskID, tag)
		}
		seen[tag] = true
	}
	return nil
}

// ValidateV7TaskRecord applies the schema-7 lowercase-kebab tag contract to
// the otherwise unchanged schema-6 task record shape.
func ValidateV7TaskRecord(task V6TaskRecord) error {
	return validateV7TaskRecord(task, metadataKeyDeferredLegacy)
}

func validateV7TaskRecord(task V6TaskRecord, deferKey string) error {
	if err := validateV6TaskRecord(task, true, deferKey); err != nil {
		return err
	}
	for _, tag := range task.Tags {
		if err := models.ValidateTag(tag); err != nil {
			return fmt.Errorf("v7 task %q tags: %w", task.ID, err)
		}
	}
	return nil
}

func validateV6Attachment(attachment V6TaskAttachment) error {
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

// WriteV6TaskMetadata writes the canonical schema-6 metadata comment.
func WriteV6TaskMetadata(task V6TaskRecord) (string, error) {
	if err := validateV6TaskRecord(task, false, metadataKeyDeferredLegacy); err != nil {
		return "", err
	}
	return wrapTaskMetadata(v6MetadataParts(task)), nil
}

func wrapTaskMetadata(parts []string) string {
	return "<!-- " + strings.Join(parts, ", ") + " -->"
}

func v6MetadataParts(task V6TaskRecord) []string {
	return taskMetadataParts(task, metadataKeyDeferredLegacy, nil)
}

// taskMetadataParts renders the shared metadata fields. deferKey names the
// deferral-date key in the caller's own schema. afterUpdated carries fields a
// later schema inserts next to the other timestamps rather than at the end of
// the comment.
func taskMetadataParts(task V6TaskRecord, deferKey string, afterUpdated []string) []string {
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
		"created: "+formatV6Time(task.CreatedAt),
		"updated: "+formatV6Time(task.UpdatedAt),
	)
	parts = append(parts, afterUpdated...)
	if task.CompletedAt != nil {
		parts = append(parts, "completed: "+formatV6Time(*task.CompletedAt))
	}
	if task.DueDate != "" {
		parts = append(parts, "due: "+task.DueDate)
	}
	if task.SnoozedUntil != "" {
		parts = append(parts, deferKey+": "+task.SnoozedUntil)
	}
	if task.Pinned {
		parts = append(parts, "pinned: true")
	}
	if task.PinnedAt != nil {
		parts = append(parts, "pinned_at: "+formatV6Time(*task.PinnedAt))
	}
	if len(task.Attachments) > 0 {
		parts = append(parts, "attachments: "+encodeV6Attachments(task.Attachments))
	}
	if len(task.Checklist) > 0 {
		parts = append(parts, "checklist: "+encodeV6Checklist(task.Checklist))
	}
	return parts
}

// ParseV6TaskMetadata strictly parses one schema-6 metadata comment. Unknown
// fields, including every field retired from schema 5, are rejected.
func ParseV6TaskMetadata(comment string) (V6TaskRecord, error) {
	values, err := parseTaskMetadataFields("v6", comment, v6MetadataKeys, v6RequiredMetadataKeys)
	if err != nil {
		return V6TaskRecord{}, err
	}
	task, err := v6RecordFromMetadataValues(values, metadataKeyDeferredLegacy)
	if err != nil {
		return V6TaskRecord{}, err
	}
	if err := validateV6TaskRecord(task, false, metadataKeyDeferredLegacy); err != nil {
		return V6TaskRecord{}, err
	}
	return task, nil
}

var v6RequiredMetadataKeys = []string{"id", "short_id", "state", "priority", "created", "updated"}

// parseTaskMetadataFields strictly splits one metadata comment into its
// key/value pairs. schema names the schema in every error so a later schema
// reusing this kernel reports its own version.
func parseTaskMetadataFields(schema, comment string, allowed map[string]bool, required []string) (map[string]string, error) {
	match := MetadataRegex().FindStringSubmatch(strings.TrimSpace(comment))
	if match == nil {
		return nil, fmt.Errorf("invalid %s task metadata comment", schema)
	}
	values := make(map[string]string)
	for _, pair := range strings.Split(match[1], ", ") {
		key, value, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("invalid %s metadata pair %q", schema, pair)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !allowed[key] {
			return nil, fmt.Errorf("unknown %s metadata field %q", schema, key)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate %s metadata field %q", schema, key)
		}
		values[key] = value
	}
	for _, key := range required {
		if values[key] == "" {
			return nil, fmt.Errorf("missing required %s metadata field %q", schema, key)
		}
	}
	return values, nil
}

// v6RecordFromMetadataValues decodes the shared metadata fields. deferKey names
// the deferral-date key in the caller's own schema.
func v6RecordFromMetadataValues(values map[string]string, deferKey string) (V6TaskRecord, error) {
	shortID, err := strconv.Atoi(values["short_id"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 short_id: %w", err)
	}
	priority, err := strconv.Atoi(values["priority"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 priority: %w", err)
	}
	created, err := parseV6Time(values["created"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 created: %w", err)
	}
	updated, err := parseV6Time(values["updated"])
	if err != nil {
		return V6TaskRecord{}, fmt.Errorf("parse v6 updated: %w", err)
	}

	task := V6TaskRecord{
		ID:           values["id"],
		ShortID:      shortID,
		State:        values["state"],
		Priority:     priority,
		ParentID:     values["parent"],
		Tags:         splitV6Set(values["tags"]),
		CreatedAt:    created,
		UpdatedAt:    updated,
		DueDate:      values["due"],
		SnoozedUntil: values[deferKey],
	}
	if values["completed"] != "" {
		parsed, parseErr := parseV6Time(values["completed"])
		if parseErr != nil {
			return V6TaskRecord{}, fmt.Errorf("parse v6 completed: %w", parseErr)
		}
		task.CompletedAt = &parsed
	}
	if values["pinned"] != "" {
		if values["pinned"] != "true" {
			return V6TaskRecord{}, fmt.Errorf("v6 pinned must be true when present")
		}
		task.Pinned = true
	}
	if values["pinned_at"] != "" {
		parsed, parseErr := parseV6Time(values["pinned_at"])
		if parseErr != nil {
			return V6TaskRecord{}, fmt.Errorf("parse v6 pinned_at: %w", parseErr)
		}
		task.PinnedAt = &parsed
	}
	if values["attachments"] != "" {
		task.Attachments, err = decodeV6Attachments(values["attachments"])
		if err != nil {
			return V6TaskRecord{}, err
		}
	}
	if values["checklist"] != "" {
		task.Checklist, err = decodeV6Checklist(values["checklist"])
		if err != nil {
			return V6TaskRecord{}, err
		}
	}
	return task, nil
}

func formatV6Time(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func parseV6Time(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	if formatV6Time(parsed) != value {
		return time.Time{}, fmt.Errorf("timestamp %q is not canonical RFC3339Nano", value)
	}
	return parsed, nil
}

func splitV6Set(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ";")
}

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
