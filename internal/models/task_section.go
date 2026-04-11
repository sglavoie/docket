package models

// SetSection assigns a section to the task, routing to the correct field.
// Built-in GTD sections (Inbox, Next, Waiting, Someday, Review, Standby,
// Completed) and the empty string (Active view) are stored in Section;
// all other names are stored in CustomSection.
func (t *Task) SetSection(name string) {
	if IsGTDSection(name) || name == "" {
		t.Section = name
	} else {
		t.CustomSection = name
	}
}
