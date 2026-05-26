package template

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TaskTemplate defines a reusable template for creating tasks.
type TaskTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Source      string          `json:"source,omitempty"`   // "default" for built-in
	Category    string          `json:"category,omitempty"` // bug, feature, process
	Type        models.TaskType `json:"type"`
	Priority    int             `json:"priority,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Section     string          `json:"section,omitempty"`
	Description string          `json:"description,omitempty"`
	Children    []TemplateChild `json:"children,omitempty"`  // Subtasks for Epic/Story templates
	Checklist   []string        `json:"checklist,omitempty"` // Checklist items to create
	UseCount    int64           `json:"use_count,omitempty"`
}

// TemplateChild defines a child task to create when using a template.
// Children inherit priority and tags from the parent task (not template-defined).
type TemplateChild struct {
	Title    string          `json:"title"`
	Type     models.TaskType `json:"type,omitempty"` // default: Task
	Optional bool            `json:"optional"`
}

// NewTemplate creates a new task template with a generated ID.
func NewTemplate(name string, taskType models.TaskType) *TaskTemplate {
	return &TaskTemplate{
		ID:       uuid.New().String()[:8],
		Name:     name,
		Enabled:  true,
		Type:     taskType,
		Priority: 0, // 0 means use default (3)
		Tags:     []string{},
		Children: []TemplateChild{},
	}
}

// Validate checks if the template is properly configured.
func (t *TaskTemplate) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("template name is required")
	}

	if !isValidTaskType(t.Type) {
		return fmt.Errorf("invalid task type: %s", t.Type)
	}

	if t.Priority < 0 || t.Priority > models.PriorityMax {
		return fmt.Errorf("priority must be 0-%d (0 for default)", models.PriorityMax)
	}

	if len(t.Children) > 0 && !t.Type.CanHaveChildren() {
		return fmt.Errorf("task type %s cannot have children", t.Type)
	}

	seen := make(map[string]int, len(t.Children))
	for i, child := range t.Children {
		if strings.TrimSpace(child.Title) == "" {
			return fmt.Errorf("item %d: title is required", i+1)
		}
		if child.Type != "" && !isValidTaskType(child.Type) {
			return fmt.Errorf("item %d: invalid task type: %s", i+1, child.Type)
		}
		childType := child.GetChildType()
		if !childType.CanBeChildOf(t.Type) {
			return fmt.Errorf("item %d: %s cannot be a child of %s", i+1, childType, t.Type)
		}
		if prev, dup := seen[child.Title]; dup {
			return fmt.Errorf("item %d: duplicate child title %q (first seen at item %d)", i+1, child.Title, prev)
		}
		seen[child.Title] = i + 1
	}

	return nil
}

// isValidTaskType checks if a task type is valid.
func isValidTaskType(t models.TaskType) bool {
	switch t {
	case models.TaskTypeEpic, models.TaskTypeStory, models.TaskTypeTask,
		models.TaskTypeBug, models.TaskTypeIdea:
		return true
	}
	return false
}

// Summary returns a short description of the template.
func (t *TaskTemplate) Summary() string {
	var parts []string

	parts = append(parts, string(t.Type))

	if t.Priority > 0 {
		parts = append(parts, fmt.Sprintf("P%d", t.Priority))
	}

	if len(t.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(t.Tags, " #"))
	}

	if t.Section != "" {
		parts = append(parts, "→"+t.Section)
	}

	// Show checklist count (prefer checklist over children)
	if len(t.Checklist) > 0 {
		if len(t.Checklist) == 1 {
			parts = append(parts, "1 checklist item")
		} else {
			parts = append(parts, fmt.Sprintf("%d checklist items", len(t.Checklist)))
		}
	} else if len(t.Children) > 0 {
		if len(t.Children) == 1 {
			parts = append(parts, "1 item")
		} else {
			parts = append(parts, fmt.Sprintf("%d items", len(t.Children)))
		}
	}

	return strings.Join(parts, ", ")
}

// GetEffectivePriority returns the priority to use, defaulting to 3 if not set.
func (t *TaskTemplate) GetEffectivePriority() int {
	if t.Priority == 0 {
		return models.PriorityDefault
	}
	return t.Priority
}

// GetChildType returns the effective type for a child, defaulting to Task.
func (c *TemplateChild) GetChildType() models.TaskType {
	if c.Type == "" {
		return models.TaskTypeTask
	}
	return c.Type
}

// ContentEqual reports whether two templates have the same user-visible content.
// All slice fields (Tags, Checklist, Children) are compared as ORDERED lists
// because users perceive their order in the rendered Summary (e.g.
// "#backend #api" vs "#api #backend") and in checklist/child task creation
// order. Treating them as sets would silently hide reorderings.
//
// Metadata fields ID, Source, Enabled, and UseCount are excluded — they are
// instance-level state rather than template content. Description and Section
// are content and are compared. The receiver-nil case treats two nil templates
// as equal and any (nil, non-nil) pair as unequal.
func (t *TaskTemplate) ContentEqual(other *TaskTemplate) bool {
	if t == nil || other == nil {
		return t == other
	}
	if t.Name != other.Name {
		return false
	}
	if t.Category != other.Category {
		return false
	}
	if t.Type != other.Type {
		return false
	}
	if t.Priority != other.Priority {
		return false
	}
	if t.Section != other.Section {
		return false
	}
	if t.Description != other.Description {
		return false
	}

	if len(t.Tags) != len(other.Tags) {
		return false
	}
	for i := range t.Tags {
		if t.Tags[i] != other.Tags[i] {
			return false
		}
	}

	if len(t.Checklist) != len(other.Checklist) {
		return false
	}
	for i := range t.Checklist {
		if t.Checklist[i] != other.Checklist[i] {
			return false
		}
	}

	if len(t.Children) != len(other.Children) {
		return false
	}
	for i := range t.Children {
		if t.Children[i].Title != other.Children[i].Title ||
			t.Children[i].Type != other.Children[i].Type ||
			t.Children[i].Optional != other.Children[i].Optional {
			return false
		}
	}

	return true
}
