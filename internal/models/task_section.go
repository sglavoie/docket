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
