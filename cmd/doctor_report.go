package cmd

import "fmt"

func printResults(results *doctorResults) {
	printInfoSection("Workspace status", results.workspaceNotes)
	printSection("Workspace integrity issues", results.workspaceIssues)
	printSection("Lens issues", results.lensIssues)
	printInfoSection("Lens status", results.lensNotes)
	printSection("Transaction issues", results.transactionIssues)
	printInfoSection("Transaction status", results.transactionNotes)
	printSection("Conflict issues", results.conflictIssues)
	printInfoSection("Outstanding conflicts", results.conflictNotes)
	printSection("Lock file issues", results.lockIssues)
	printInfoSection("Lock files", results.lockNotes)
	printSection("Planning membership issues", results.planningIssues)
	printInfoSection("Unavailable planning references", results.unavailablePlanningRefs)
	printSection("Attachment issues", results.attachmentIssues)
	printInfoSection("Orphaned attachments", results.attachmentNotes)
	printSection("Checklist and comment issues", results.structureIssues)
	printSection("State clock issues", results.stateClockIssues)
	printInfoSection("Clock skew", results.clockSkewNotes)
	printSection("Project load failures", results.loadFailures)
	printSection("Missing task files", results.missingTaskFiles)
	printSection("Duplicate task IDs", results.duplicateIDs)
	printSection("Duplicate short IDs", results.duplicateShortIDs)
	printSection("Broken parent refs", results.brokenParentRefs)
	printSection("Invalid completion timestamps", results.invalidCompletionTimestamps)
	printSection("Broken task references", results.brokenTaskRefs)
	printInfoSection("Unavailable task references", results.unavailableTaskRefs)
	printInfoSection("Task reference cycles", results.dependencyCycles)
	printInfoSection("Empty projects", results.emptyProjects)
}

func printSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s: %d\n", name, len(issues))
	for _, issue := range issues {
		fmt.Printf("  [%s] %s\n", issue.project, issue.detail)
	}
}

func printInfoSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s (info): %d\n", name, len(issues))
	for _, issue := range issues {
		if issue.detail == "" {
			fmt.Printf("  %s\n", issue.project)
			continue
		}
		fmt.Printf("  [%s] %s\n", issue.project, issue.detail)
	}
}
