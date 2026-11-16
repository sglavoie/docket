package tui

import _ "embed"

//go:embed tutor.txt
var tutorText string

// tutor uses the import reader and builder, independent of the current place.
func (m *Model) tutor() {
	if m.pasteUnder("", "", false, readPaste(tutorText), false, false) == 0 {
		return
	}
	roots := m.tree.Children("")
	id := roots[len(roots)-1]
	m.sideFocus, m.sideFrom = false, nil
	m.open(entry{kind: kindProject, id: id, label: "Welcome"})
	m.msg = "Welcome · work through these rows; :help is the reference"
}
