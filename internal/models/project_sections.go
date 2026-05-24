package models

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

// ErrSectionAlreadyExists is returned when a section rename targets an existing name.
var ErrSectionAlreadyExists = errors.New("section already exists")

// EnsureDefaultSections guarantees that all built-in sections used by lifecycle
// methods (Complete, Block, Reopen, etc.) exist in the project's section list.
// Call this during project load so that status transitions never assign a task
// to a section the project doesn't know about.
func (p *Project) EnsureDefaultSections() {
	defaults := []string{SectionNext, SectionInbox, SectionWaiting, SectionSomeday, SectionReview, SectionStandby, SectionCompleted}
	for _, s := range defaults {
		if !slices.Contains(p.Sections, s) {
			p.Sections = append(p.Sections, s)
		}
	}
	// Deduplicate while preserving order (keep first occurrence).
	seen := make(map[string]bool, len(p.Sections))
	deduped := p.Sections[:0]
	for _, s := range p.Sections {
		if !seen[s] {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}
	p.Sections = deduped
}

// GetSections returns the project's sections, ensuring Next is always first.
func (p *Project) GetSections() []string {
	if len(p.Sections) == 0 {
		return []string{SectionNext}
	}
	// Ensure Next is first
	hasNext := slices.Contains(p.Sections, SectionNext)
	if !hasNext {
		return append([]string{SectionNext}, p.Sections...)
	}
	// If Next exists but isn't first, reorder
	if p.Sections[0] != SectionNext {
		sections := make([]string, 0, len(p.Sections))
		sections = append(sections, SectionNext)
		for _, s := range p.Sections {
			if s != SectionNext {
				sections = append(sections, s)
			}
		}
		return sections
	}
	return p.Sections
}

// HasSection returns true if the section exists in the project.
func (p *Project) HasSection(name string) bool {
	return slices.Contains(p.Sections, name)
}

// MaxSectionNameLength is the maximum allowed length for a section name.
const MaxSectionNameLength = 50

// EnsureSection adds a section if it does not already exist.
// Silently rejects names that fail validation — this is intentional because
// 26+ callers (including TUI handlers) rely on the no-error signature.
// Invalid names are a no-op; callers that need validation should call
// ValidateSectionName explicitly before this method.
//
// Case-variant collisions with built-in GTD section names (e.g. "inbox" when
// the canonical "Inbox" already exists) are rejected to prevent duplicate
// sections that look identical to the user but are distinct in storage.
func (p *Project) EnsureSection(name string) {
	if err := ValidateSectionName(name); err != nil {
		return
	}
	if IsGTDSectionNameCI(name) && !IsGTDSection(name) {
		return
	}
	if !p.HasSection(name) {
		p.Sections = append(p.Sections, name)
		p.UpdatedAt = time.Now()
	}
}

// RemoveSection removes a section from the project.
// Returns false if trying to remove the Next section or if section doesn't exist.
// Tasks in the removed section will have their section cleared.
func (p *Project) RemoveSection(name string) bool {
	if IsGTDSection(name) {
		return false // Cannot remove built-in sections
	}
	for i, s := range p.Sections {
		if s == name {
			p.Sections = append(p.Sections[:i], p.Sections[i+1:]...)
			// Clear section from tasks that had this section
			for _, task := range p.Tasks {
				if task.Section == name {
					task.Section = ""
					task.UpdatedAt = time.Now()
				}
				if task.CustomSection == name {
					task.CustomSection = ""
					task.UpdatedAt = time.Now()
				}
			}
			// Remove color mapping
			if p.SectionColors != nil {
				delete(p.SectionColors, name)
			}
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// RenameSection renames a section and updates all tasks with that section.
// Returns an error if the rename cannot be performed.
func (p *Project) RenameSection(oldName, newName string) error {
	if IsGTDSection(oldName) {
		return fmt.Errorf("cannot rename built-in section '%s'", oldName)
	}
	if newName == "" {
		return fmt.Errorf("new section name cannot be empty")
	}
	if err := ValidateSectionName(newName); err != nil {
		return err
	}
	if IsGTDSectionNameCI(newName) && !IsGTDSection(newName) {
		return fmt.Errorf("cannot rename section to %q: collides with built-in GTD section name", newName)
	}
	if p.HasSection(newName) {
		return fmt.Errorf("cannot rename section to %q: %w", newName, ErrSectionAlreadyExists)
	}
	for i, s := range p.Sections {
		if s == oldName {
			p.Sections[i] = newName
			// Update tasks with this section (both GTD and custom section references)
			for _, task := range p.Tasks {
				if task.Section == oldName {
					task.Section = newName
					task.UpdatedAt = time.Now()
				}
				if task.CustomSection == oldName {
					task.CustomSection = newName
					task.UpdatedAt = time.Now()
				}
			}
			// Rename color mapping
			if p.SectionColors != nil {
				if color, exists := p.SectionColors[oldName]; exists {
					delete(p.SectionColors, oldName)
					p.SectionColors[newName] = color
				}
			}
			p.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("section %q not found", oldName)
}

// GetTasksBySection returns tasks filtered by section.
// Uses InSection() to correctly handle both GTD sections and custom sections.
// When section is "" (Active view), only tasks with both Section="" AND
// CustomSection="" are returned — tasks with a CustomSection belong to that
// custom section, not the Active view.
func (p *Project) GetTasksBySection(section string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.InSection(section) {
			result = append(result, task)
		}
	}
	return result
}

// CountTasksBySection returns the number of tasks in each section.
// Tasks with empty section AND empty custom section are counted under ""
// (representing Active view). Tasks with a CustomSection are counted only
// under the custom section name, not under the empty key.
func (p *Project) CountTasksBySection() map[string]int {
	counts := make(map[string]int)
	for _, task := range p.Tasks {
		if task.CustomSection != "" {
			counts[task.CustomSection]++
		} else {
			counts[task.Section]++
		}
	}
	return counts
}

// GetCustomSections returns sections that are not built-in (Inbox, Next, Waiting, Someday, Completed), sorted alphabetically.
func (p *Project) GetCustomSections() []string {
	builtIn := map[string]bool{
		SectionInbox:     true,
		SectionNext:      true,
		SectionWaiting:   true,
		SectionSomeday:   true,
		SectionReview:    true,
		SectionStandby:   true,
		SectionCompleted: true,
	}
	var custom []string
	for _, s := range p.Sections {
		if !builtIn[s] {
			custom = append(custom, s)
		}
	}
	sort.Strings(custom)
	return custom
}

// GetSectionColor returns the color for a section, defaulting to "gray".
func (p *Project) GetSectionColor(section string) string {
	if p.SectionColors == nil {
		p.SectionColors = make(map[string]string)
	}
	if color, ok := p.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// ValidSectionColors is the set of color names supported by the section color palette.
// This must match the entries in tui/styles.go SectionColorPalette.
var ValidSectionColors = map[string]bool{
	"gray": true, "red": true, "orange": true, "yellow": true, "green": true,
	"cyan": true, "blue": true, "purple": true, "magenta": true, "white": true,
}

// IsValidSectionColor reports whether the given color name is a known section color.
func IsValidSectionColor(color string) bool {
	return ValidSectionColors[color]
}

// SetSectionColor sets the color for a section.
// Silently rejects colors not in ValidSectionColors — callers needing validation
// should call IsValidSectionColor first.
func (p *Project) SetSectionColor(section, color string) {
	if !IsValidSectionColor(color) {
		return
	}
	if p.SectionColors == nil {
		p.SectionColors = make(map[string]string)
	}
	p.SectionColors[section] = color
	p.UpdatedAt = time.Now()
}
