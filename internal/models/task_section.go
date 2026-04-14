package models

// SetSection assigns a section to the task, routing to the correct field.
// Built-in GTD sections (Inbox, Next, Waiting, Someday, Review, Standby,
// Completed) and the empty string (Active view) are stored in Section;
// all other names are stored in CustomSection.
//
// When assigning a GTD section, CustomSection is cleared (and vice versa)
// to prevent the task from appearing in two sections simultaneously.
func (t *Task) SetSection(name string) {
	if IsGTDSection(name) || name == "" {
		t.Section = name
		t.CustomSection = ""
	} else {
		t.CustomSection = name
		t.Section = ""
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
