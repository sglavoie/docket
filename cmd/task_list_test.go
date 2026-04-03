package cmd

// resetFilterVars resets all filter package variables to their zero values.
func resetFilterVars() {
	listTaskType = ""
	listTaskStatus = ""
	listPriority = 0
	listTag = ""
	listShowAll = false
	listTreeView = false
	listSnoozed = false
	listSort = ""
	listJSON = false
}
