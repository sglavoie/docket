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
	Children    []TemplateChild `json:"children,omitempty"`  // Deprecated: subtasks, use Checklist instead
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

	for i, child := range t.Children {
		if strings.TrimSpace(child.Title) == "" {
			return fmt.Errorf("item %d: title is required", i+1)
		}
		if child.Type != "" && !isValidTaskType(child.Type) {
			return fmt.Errorf("item %d: invalid task type: %s", i+1, child.Type)
		}
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
		parts = append(parts, fmt.Sprintf("%d checklist items", len(t.Checklist)))
	} else if len(t.Children) > 0 {
		parts = append(parts, fmt.Sprintf("%d items", len(t.Children)))
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

// ValidCategories returns the list of valid template categories.
func ValidCategories() []string {
	return []string{"bug", "feature", "process"}
}

// ValidTaskTypes returns all valid task types for templates.
func ValidTaskTypes() []models.TaskType {
	return []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}
}
