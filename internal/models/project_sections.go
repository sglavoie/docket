package models

import (
	"slices"
	"time"
)

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

// CountTasksByState returns task counts keyed by exact canonical workflow
// state. Project sections and display buckets do not participate.
func (p *Project) CountTasksByState() map[WorkflowState]int {
	counts := make(map[WorkflowState]int)
	for _, task := range p.Tasks {
		counts[task.State()]++
	}
	return counts
}

// GetTasksBySection is a legacy presentation adapter. New runtime consumers
// should query exact state with GetTasksByState.
func (p *Project) GetTasksBySection(section string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.InSection(section) {
			result = append(result, task)
		}
	}
	return result
}

// CountTasksBySection is a legacy presentation adapter over canonical state
// counts. New runtime consumers should use CountTasksByState.
func (p *Project) CountTasksBySection() map[string]int {
	counts := make(map[string]int)
	for state, count := range p.CountTasksByState() {
		counts[SectionForState(state)] += count
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
