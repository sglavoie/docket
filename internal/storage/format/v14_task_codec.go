package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

// V14TaskDependency is one persisted soft dependency: a stable, project-
// qualified locator for the task this record depends on. It mirrors
// models.TaskReference as a storage shape, the way V6TaskAttachment mirrors
// models.Attachment, so the codec owns its own record type and the domain owns
// its own value.
type V14TaskDependency struct {
	ProjectID string
	TaskID    string
}

// V14TaskRecord is the complete schema-14 storage shape: the schema-13 fields
// plus an ordered list of soft dependencies. The list is optional, so a record
// with no dependencies is byte-identical to its schema-13 predecessor and the
// additive key leaves every older validator unchanged, mirroring how schema 13
// added due_time.
type V14TaskRecord struct {
	V13TaskRecord
	Dependencies []V14TaskDependency
}

// v14DependencyEntrySeparator splits one dependency locator from the next.
const v14DependencyEntrySeparator = ";"

// v14DependencyComponentSeparator splits a locator's project id from its task
// id. It matches models.TaskReferenceSeparator, so a persisted locator and an
// in-memory one are the same string.
const v14DependencyComponentSeparator = models.TaskReferenceSeparator

var v14MetadataKeys = v14TaskMetadataKeys()

func v14TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v13MetadataKeys)+1)
	for key := range v13MetadataKeys {
		keys[key] = true
	}
	keys["depends"] = true
	return keys
}

// Dependencies are optional, so schema 14 has the same required metadata as
// schema 13.
var v14RequiredMetadataKeys = v13RequiredMetadataKeys

// ValidateV14TaskRecord validates a complete schema-14 task record.
func ValidateV14TaskRecord(task V14TaskRecord) error {
	return validateV14TaskRecord(task, true)
}

func validateV14TaskRecord(task V14TaskRecord, requireTitle bool) error {
	if err := validateV13TaskRecord(task.V13TaskRecord, requireTitle); err != nil {
		return err
	}
	if err := validateV14Dependencies(task.Dependencies); err != nil {
		return fmt.Errorf("v14 task %q: %w", task.ID, err)
	}
	return nil
}

// validateV14Dependencies enforces the durable list contract: every component
// is a nonempty identity carrying no reserved delimiter, and no locator repeats.
// Self-reference and target existence stay with the workspace-aware application
// layer, because a project document does not carry its own project id and so
// cannot tell whether a locator names the record beside it.
func validateV14Dependencies(dependencies []V14TaskDependency) error {
	seen := make(map[V14TaskDependency]bool, len(dependencies))
	for index, dependency := range dependencies {
		if err := models.ValidateTaskReference(dependency.ProjectID, dependency.TaskID); err != nil {
			return fmt.Errorf("depends entry %d: %w", index, err)
		}
		if seen[dependency] {
			return fmt.Errorf("depends entry %d: duplicate dependency %q", index, encodeV14Dependency(dependency))
		}
		seen[dependency] = true
	}
	return nil
}

func encodeV14Dependency(dependency V14TaskDependency) string {
	return dependency.ProjectID + v14DependencyComponentSeparator + dependency.TaskID
}

func encodeV14Dependencies(dependencies []V14TaskDependency) string {
	entries := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		entries = append(entries, encodeV14Dependency(dependency))
	}
	return strings.Join(entries, v14DependencyEntrySeparator)
}

// decodeV14Dependencies reads the canonical "project:task;project:task" value.
// It refuses an empty entry and a locator with a missing or extra separator so a
// malformed value can never be silently reinterpreted as a different task.
func decodeV14Dependencies(value string) ([]V14TaskDependency, error) {
	entries := strings.Split(value, v14DependencyEntrySeparator)
	dependencies := make([]V14TaskDependency, 0, len(entries))
	for _, entry := range entries {
		projectID, taskID, found := strings.Cut(entry, v14DependencyComponentSeparator)
		if !found {
			return nil, fmt.Errorf("invalid v14 depends entry %q: missing %q separator", entry, v14DependencyComponentSeparator)
		}
		dependencies = append(dependencies, V14TaskDependency{ProjectID: projectID, TaskID: taskID})
	}
	if err := validateV14Dependencies(dependencies); err != nil {
		return nil, fmt.Errorf("parse v14 depends: %w", err)
	}
	return dependencies, nil
}

// WriteV14TaskMetadata writes the canonical schema-14 metadata comment. An empty
// dependency list omits the key entirely, which is what keeps a migrated record
// byte-equal to the schema-13 record it came from.
func WriteV14TaskMetadata(task V14TaskRecord) (string, error) {
	if err := validateV14TaskRecord(task, false); err != nil {
		return "", err
	}
	extra := []string{"state_changed: " + formatV6Time(task.StateChangedAt)}
	parts := taskMetadataParts(task.V6TaskRecord, deferInstant, extra)
	if task.Context != "" {
		parts = append(parts, "context: "+task.Context)
	}
	if task.DeferCount > 0 {
		parts = append(parts, "defer_count: "+strconv.Itoa(task.DeferCount))
	}
	if task.DueTime != "" {
		parts = append(parts, "due_time: "+task.DueTime)
	}
	if len(task.Dependencies) > 0 {
		parts = append(parts, "depends: "+encodeV14Dependencies(task.Dependencies))
	}
	return wrapTaskMetadata(parts), nil
}

// ParseV14TaskMetadata strictly parses one schema-14 metadata comment.
func ParseV14TaskMetadata(comment string) (V14TaskRecord, error) {
	values, err := parseTaskMetadataFields("v14", comment, v14MetadataKeys, v14RequiredMetadataKeys)
	if err != nil {
		return V14TaskRecord{}, err
	}
	base, err := v6RecordFromMetadataValues(values, deferInstant)
	if err != nil {
		return V14TaskRecord{}, err
	}
	stateChangedAt, err := parseV6Time(values["state_changed"])
	if err != nil {
		return V14TaskRecord{}, fmt.Errorf("parse v14 state_changed: %w", err)
	}
	deferCount := 0
	if value, present := values["defer_count"]; present {
		deferCount, err = strconv.Atoi(value)
		if err != nil {
			return V14TaskRecord{}, fmt.Errorf("parse v14 defer_count: %w", err)
		}
		if deferCount == 0 {
			return V14TaskRecord{}, fmt.Errorf("parse v14 defer_count: value must be positive when present")
		}
	}
	task := V14TaskRecord{
		V13TaskRecord: V13TaskRecord{
			V12TaskRecord: V12TaskRecord{
				V11TaskRecord: V11TaskRecord{
					V10TaskRecord: V10TaskRecord{V6TaskRecord: base, StateChangedAt: stateChangedAt},
					Context:       values["context"],
				},
				DeferCount: deferCount,
			},
			DueTime: values["due_time"],
		},
	}
	if raw, present := values["depends"]; present {
		task.Dependencies, err = decodeV14Dependencies(raw)
		if err != nil {
			return V14TaskRecord{}, err
		}
	}
	if err := validateV14TaskRecord(task, false); err != nil {
		return V14TaskRecord{}, err
	}
	return task, nil
}

// WriteV14TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV14TaskBody(task V14TaskRecord) (string, error) {
	if err := ValidateV14TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV14TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV14TaskBody parses the body below one generic Task heading.
func ParseV14TaskBody(title, body string) (V14TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V14TaskRecord{}, fmt.Errorf("v14 task %q is missing metadata", title)
	}
	task, err := ParseV14TaskMetadata(lines[0])
	if err != nil {
		return V14TaskRecord{}, fmt.Errorf("v14 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v14", title, lines)
	if err != nil {
		return V14TaskRecord{}, err
	}
	if err := ValidateV14TaskRecord(task); err != nil {
		return V14TaskRecord{}, err
	}
	return task, nil
}
