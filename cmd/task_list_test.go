package cmd

import "testing"

// resetFilterVars resets all filter package variables to their zero values.
func resetFilterVars() {
	listTaskType = ""
	listTaskStatus = ""
	listState = ""
	listBlocked = false
	listFilterProject = nil
	listPriority = 0
	listTag = ""
	listShowAll = false
	listTreeView = false
	listSnoozed = false
	listParked = false
	listSort = ""
	listJSON = false
}

func TestTaskListCommandRegistersParkedFlag(t *testing.T) {
	if flag := taskListCmd.Flags().Lookup("parked"); flag == nil {
		t.Fatal("expected task list command to register --parked flag")
	}
}
