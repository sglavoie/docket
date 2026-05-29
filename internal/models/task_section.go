package models

import "time"

// SetSection assigns a section to the task, routing to the correct field.
// Built-in GTD sections (Inbox, Next, Waiting, Someday, Review, Standby,
// Completed) and the empty string (Active view) are stored in Section;
// all other names are stored in CustomSection.
//
// When assigning a GTD section, CustomSection is cleared (and vice versa)
// to prevent the task from appearing in two sections simultaneously.
func (t *Task) SetSection(name string) {
	oldSection := t.Section
	oldCustomSection := t.CustomSection
	if IsGTDSection(name) || name == "" {
		t.Section = name
		t.CustomSection = ""
		if t.Section != oldSection || t.CustomSection != oldCustomSection {
			t.UpdatedAt = time.Now()
		}
		return
	}
	// Reject custom section names that fail ValidateSectionName (whitespace,
	// semicolons, colons, over-length). Without this guard, malformed names
	// land in CustomSection and corrupt the markdown round-trip because ";"
	// and ":" delimit section-color metadata and whitespace separates the
	// /Section sigil from quick-capture text. No-op behavior mirrors
	// EnsureSection; callers needing a validation error should call
	// ValidateSectionName explicitly first.
	if err := ValidateSectionName(name); err != nil {
		return
	}
	// Reject case-variant collisions with GTD section names (e.g. "inbox"
	// when the canonical "Inbox" already exists). Otherwise CustomSection
	// would hold a name that looks identical to a GTD section to the user
	// but stays distinct in storage, scattering the task across two buckets.
	if IsGTDSectionNameCI(name) {
		return
	}
	t.CustomSection = name
	t.Section = ""
	if t.Section != oldSection || t.CustomSection != oldCustomSection {
		t.UpdatedAt = time.Now()
	}
}

// SetCustomSection assigns or clears a custom section.
//
// Non-empty values clear the GTD Section field to keep section membership
// exclusive. Empty string only clears CustomSection, preserving any GTD section.
func (t *Task) SetCustomSection(name string) {
	oldSection := t.Section
	oldCustomSection := t.CustomSection
	if name == "" {
		t.CustomSection = ""
		if t.CustomSection != oldCustomSection {
			t.UpdatedAt = time.Now()
		}
		return
	}
	if IsGTDSectionNameCI(name) {
		return
	}
	if err := ValidateSectionName(name); err != nil {
		return
	}
	t.CustomSection = name
	t.Section = ""
	if t.Section != oldSection || t.CustomSection != oldCustomSection {
		t.UpdatedAt = time.Now()
	}
}

// InSection reports whether the task belongs to the given section.
// It checks both the GTD Section field and the CustomSection field,
// so callers do not need to know which field stores the section name.
//
// When CustomSection is set, the task belongs to that custom section
// (not the Active view, even though Section is "").
func (t *Task) InSection(name string) bool {
	if t.CustomSection != "" {
		return t.CustomSection == name
	}
	return t.Section == name
}
