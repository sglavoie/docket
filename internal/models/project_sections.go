package models

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

// ErrSectionAlreadyExists is returned when a section rename targets an existing name.
var ErrSectionAlreadyExists = fmt.Errorf("section already exists")

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
const MaxSectionNameLength = 256

// EnsureSection adds a section if it does not already exist.
// Silently rejects names that fail validation — this is intentional because
// 26+ callers (including TUI handlers) rely on the no-error signature.
// Invalid names are a no-op; callers that need validation should call
// ValidateSectionName explicitly before this method.
func (p *Project) EnsureSection(name string) {
	if err := ValidateSectionName(name); err != nil {
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
	if name == SectionNext {
		return false // Cannot remove Next
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
	if oldName == SectionNext {
		return fmt.Errorf("cannot rename the Next section")
	}
	if newName == "" {
		return fmt.Errorf("new section name cannot be empty")
	}
	if err := ValidateSectionName(newName); err != nil {
		return err
	}
	if p.HasSection(newName) {
		return fmt.Errorf("cannot rename section: target name %q already exists", newName)
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
func (p *Project) GetTasksBySection(section string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.Section == section {
			result = append(result, task)
		}
	}
	return result
}

// CountTasksBySection returns the number of tasks in each section.
// Tasks with empty section are counted separately (representing Active view).
func (p *Project) CountTasksBySection() map[string]int {
	counts := make(map[string]int)
	for _, task := range p.Tasks {
		counts[task.Section]++
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
		return "gray"
	}
	if color, ok := p.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// SetSectionColor sets the color for a section.
func (p *Project) SetSectionColor(section, color string) {
	if p.SectionColors == nil {
		p.SectionColors = make(map[string]string)
	}
	p.SectionColors[section] = color
	p.UpdatedAt = time.Now()
}
