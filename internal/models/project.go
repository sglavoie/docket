package models

import (
	"regexp"
	"strings"
	"time"
)

// Project represents a collection of tasks.
type Project struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	StatusIcon       string `json:"status_icon,omitempty"`
	StatusIconSource string `json:"status_icon_source,omitempty"` // "user" (manually set), "auto" (workflow-inferred), "" (unset)
	Context     string     `json:"context"`
	Path        string     `json:"path"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	Sandbox     bool       `json:"sandbox,omitempty"`
	Internal    bool       `json:"internal,omitempty"`

	// Computed fields (not persisted in config, stored in tasks.md)
	Tasks         []*Task           `json:"-"`
	NextShortID   int               `json:"-"`
	Sections      []string          `json:"-"` // Project-level section definitions
	SectionColors map[string]string `json:"-"` // Section name -> color name mapping
}

const (
	// SectionNext is the default section for active tasks.
	SectionNext = "Next"
	// SectionInbox is the default section for newly created recurring instances.
	SectionInbox = "Inbox"
)

// NewProject creates a new project with default values.
func NewProject(id, name, path string) *Project {
	now := time.Now()
	return &Project{
		ID:            id,
		Slug:          Slugify(name),
		Name:          name,
		Context:       "personal",
		Path:          path,
		CreatedAt:     now,
		UpdatedAt:     now,
		Archived:      false,
		Tasks:         []*Task{},
		NextShortID:   1,
		Sections:      []string{SectionNext},
		SectionColors: make(map[string]string),
	}
}

// Slugify converts a name to a URL-safe slug.
func Slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	reg := regexp.MustCompile("[^a-z0-9-]")
	slug = reg.ReplaceAllString(slug, "")
	reg = regexp.MustCompile("-+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// Archive marks the project as archived.
func (p *Project) Archive() {
	now := time.Now()
	p.Archived = true
	p.ArchivedAt = &now
	p.UpdatedAt = now
}

// Unarchive removes the archived status.
func (p *Project) Unarchive() {
	p.Archived = false
	p.ArchivedAt = nil
	p.UpdatedAt = time.Now()
}

// Pin marks the project as pinned to appear at the top of lists.
func (p *Project) Pin() {
	now := time.Now()
	p.Pinned = true
	p.PinnedAt = &now
	p.UpdatedAt = now
}

// Unpin removes the pinned status.
func (p *Project) Unpin() {
	p.Pinned = false
	p.PinnedAt = nil
	p.UpdatedAt = time.Now()
}

// MutateTask looks up a task by ID, calls fn to mutate it, and sets UpdatedAt.
// Returns false if the task was not found.
func (p *Project) MutateTask(taskID string, fn func(*Task)) bool {
	for _, t := range p.Tasks {
		if t.ID == taskID {
			fn(t)
			t.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// StatusIconEmoji returns the emoji for the project's status icon, or empty string if not set.
func (p *Project) StatusIconEmoji() string {
	if p.StatusIcon == "" {
		return ""
	}
	if icon := ProjectStatusIconByKey(p.StatusIcon); icon != nil {
		return icon.Emoji
	}
	return ""
}

// TogglePinned toggles the pinned status.
func (p *Project) TogglePinned() {
	if p.Pinned {
		p.Unpin()
	} else {
		p.Pin()
	}
}
