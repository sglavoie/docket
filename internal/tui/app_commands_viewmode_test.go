package tui

// advanceCursorToTask moves the outliner cursor to the first row that has a task (skipping section headers).
func advanceCursorToTask(o *OutlinerModel) {
	for i, row := range o.rows {
		if row.task != nil {
			o.cursor = i
			return
		}
	}
}

// findCommand searches a command slice by name and returns the matching command.
func findCommand(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}
