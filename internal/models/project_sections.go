package models

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// ErrSectionAlreadyExists is returned when a section rename targets an existing name.
var ErrSectionAlreadyExists = errors.New("section already exists")

var defaultGTDSections = []string{SectionNext, SectionInbox, SectionWaiting, SectionSomeday, SectionReview, SectionStandby, SectionCompleted}

// FilterGTDSections removes stale non-GTD section names and deduplicates the
// remaining built-in sections while preserving their first-seen order.
func FilterGTDSections(sections []string) []string {
	seen := make(map[string]bool, len(sections))
	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		if !IsGTDSection(section) || seen[section] {
			continue
		}
		seen[section] = true
		filtered = append(filtered, section)
	}
	return filtered
}

// EnsureDefaultSections guarantees that all built-in sections used by lifecycle
// methods (Complete, Block, Reopen, etc.) exist in the project's section list.
// Call this during project load so that status transitions never assign a task
// to a section the project doesn't know about.
func (p *Project) EnsureDefaultSections() {
	p.Sections = FilterGTDSections(p.Sections)
	for _, s := range defaultGTDSections {
		if !slices.Contains(p.Sections, s) {
			p.Sections = append(p.Sections, s)
		}
	}

	if p.SectionColors != nil {
		for section := range p.SectionColors {
			if !IsGTDSection(section) {
				delete(p.SectionColors, section)
			}
		}
	}
}

// GetSections returns the project's sections, ensuring Next is always first.
func (p *Project) GetSections() []string {
	sections := FilterGTDSections(p.Sections)
	if len(sections) == 0 {
		return []string{SectionNext}
	}
	// Ensure Next is first
	hasNext := slices.Contains(sections, SectionNext)
	if !hasNext {
		return append([]string{SectionNext}, sections...)
	}
	// If Next exists but isn't first, reorder
	if sections[0] != SectionNext {
		ordered := make([]string, 0, len(sections))
		ordered = append(ordered, SectionNext)
		for _, s := range sections {
			if s != SectionNext {
				ordered = append(ordered, s)
			}
		}
		return ordered
	}
	return sections
}

// HasSection returns true if the section exists in the project.
func (p *Project) HasSection(name string) bool {
	return IsGTDSection(name) && slices.Contains(p.GetSections(), name)
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
	if !IsGTDSection(name) {
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
	if !IsGTDSection(newName) {
		return fmt.Errorf("cannot rename section to %q: flexible grouping now uses tags", newName)
	}
	if p.HasSection(newName) {
		return fmt.Errorf("cannot rename section to %q: %w", newName, ErrSectionAlreadyExists)
	}
	for i, s := range p.Sections {
		if s == oldName {
			p.Sections[i] = newName
			// Update tasks with this section.
			for _, task := range p.Tasks {
				if task.Section == oldName {
					task.Section = newName
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
func (p *Project) CountTasksBySection() map[string]int {
	counts := make(map[string]int)
	for _, task := range p.Tasks {
		counts[task.Section]++
	}
	return counts
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
