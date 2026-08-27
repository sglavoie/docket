package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/util"
)

// Pre-compiled regexps for Slugify to avoid recompilation on each call.
var (
	slugNonAlnumRe  = regexp.MustCompile("[^a-z0-9-]")
	slugMultiDashRe = regexp.MustCompile("-+")
)

// Project represents a collection of tasks.
type Project struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Path        string     `json:"path"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot,omitempty"`
	Tags        []string   `json:"tags,omitempty"`

	// Computed fields (not persisted in config, stored in tasks.md)
	Tasks       []*Task `json:"-"`
	NextShortID int     `json:"-"`
}

const (
	// MaxProjectSlugLength bounds the portable directory component persisted in
	// the workspace catalog. Slugify can emit at most one ASCII byte per rune in
	// a valid project name, so this matches MaxProjectNameLength.
	MaxProjectSlugLength = MaxProjectNameLength
)

// IsLoaded reports whether the project's task slice has been populated from
// storage. A catalog project is rehydrated with a nil Tasks slice, so counting
// its tasks yields a misleading zero; callers that display counts must treat an
// unloaded project as unknown rather than empty.
func (p *Project) IsLoaded() bool {
	return p != nil && p.Tasks != nil
}

var reservedProjectSlugs = map[string]bool{
	".tt":      true,
	"projects": true,
	"new":      true,
	"list":     true,
	"delete":   true,
	"switch":   true,
	"archive":  true,
}

// ValidateProjectSlug reports whether slug is the exact portable directory
// component accepted in a workspace catalog. Ordinary slugs must already be in
// Slugify's canonical lowercase ASCII form; validation never normalizes persisted
// input.
func ValidateProjectSlug(slug string) error {
	if err := util.ValidateChildName(slug); err != nil {
		return fmt.Errorf("project slug %q is not a safe path component: %w", slug, err)
	}
	if len(slug) > MaxProjectSlugLength {
		return fmt.Errorf("project slug too long (%d chars, max %d)", len(slug), MaxProjectSlugLength)
	}
	if slug == "_sandbox" {
		return fmt.Errorf("project slug %q is reserved", slug)
	}
	if reservedProjectSlugs[slug] {
		return fmt.Errorf("project slug %q is reserved", slug)
	}
	if canonical := Slugify(slug); canonical != slug {
		return fmt.Errorf("project slug %q is not canonical (canonical form %q)", slug, canonical)
	}
	return nil
}

// NewProject creates a new project with default values.
// Returns an error if id or name is empty.
func NewProject(id, name, path string) (*Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if n := utf8.RuneCountInString(name); n > MaxProjectNameLength {
		return nil, fmt.Errorf("project name too long (%d chars, max %d)", n, MaxProjectNameLength)
	}
	slug := Slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("project name %q produces empty slug after sanitization", name)
	}
	if err := ValidateProjectSlug(slug); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Project{
		ID:          id,
		Slug:        slug,
		Name:        name,
		Path:        path,
		CreatedAt:   now,
		UpdatedAt:   now,
		Archived:    false,
		Tasks:       []*Task{},
		NextShortID: 1,
	}, nil
}

// Slugify converts a name to a URL-safe slug.
func Slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return '-'
		}
		return r
	}, slug)
	slug = slugNonAlnumRe.ReplaceAllString(slug, "")
	slug = slugMultiDashRe.ReplaceAllString(slug, "-")
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
// Returns false if the task was not found. An empty taskID always returns
// false so a caller passing an unset identifier cannot accidentally mutate a
// task whose own ID became empty after a malformed parse (mirrors the guard
// in GetTaskByID).
func (p *Project) MutateTask(taskID string, fn func(*Task)) bool {
	if taskID == "" || fn == nil {
		return false
	}
	for _, t := range p.Tasks {
		if t.ID == taskID {
			fn(t)
			t.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// TogglePinned toggles the pinned status.
func (p *Project) TogglePinned() {
	if p.Pinned {
		p.Unpin()
	} else {
		p.Pin()
	}
}
