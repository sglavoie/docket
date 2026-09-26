package cmd

import "fmt"

// doctorSection is one reported bucket. The one table below drives both the
// text report and the JSON report, so the two cannot name different buckets.
type doctorSection struct {
	title  string
	key    string
	info   bool
	issues []doctorIssue
}

func doctorSections(results *doctorResults) []doctorSection {
	return []doctorSection{
		{"Workspace status", "workspace_status", true, results.workspaceNotes},
		{"Workspace integrity issues", "workspace_integrity", false, results.workspaceIssues},
		{"Lens issues", "lenses", false, results.lensIssues},
		{"Lens status", "lens_status", true, results.lensNotes},
		{"Transaction issues", "transactions", false, results.transactionIssues},
		{"Transaction status", "transaction_status", true, results.transactionNotes},
		{"Conflict issues", "conflicts", false, results.conflictIssues},
		{"Outstanding conflicts", "outstanding_conflicts", true, results.conflictNotes},
		{"Lock file issues", "lock_files", false, results.lockIssues},
		{"Lock files", "lock_file_status", true, results.lockNotes},
		{"Planning membership issues", "planning_membership", false, results.planningIssues},
		{"Unavailable planning references", "unavailable_planning_references", true, results.unavailablePlanningRefs},
		{"Attachment issues", "attachments", false, results.attachmentIssues},
		{"Orphaned attachments", "orphaned_attachments", true, results.attachmentNotes},
		{"Checklist and comment issues", "checklist_and_comments", false, results.structureIssues},
		{"State clock issues", "state_clocks", false, results.stateClockIssues},
		{"Clock skew", "clock_skew", true, results.clockSkewNotes},
		{"Project load failures", "project_load_failures", false, results.loadFailures},
		{"Missing task files", "missing_task_files", false, results.missingTaskFiles},
		{"Duplicate task IDs", "duplicate_task_ids", false, results.duplicateIDs},
		{"Duplicate short IDs", "duplicate_short_ids", false, results.duplicateShortIDs},
		{"Broken parent refs", "broken_parent_refs", false, results.brokenParentRefs},
		{"Invalid completion timestamps", "invalid_completion_timestamps", false, results.invalidCompletionTimestamps},
		{"Broken task references", "broken_task_references", false, results.brokenTaskRefs},
		{"Unavailable task references", "unavailable_task_references", true, results.unavailableTaskRefs},
		{"Task reference cycles", "task_reference_cycles", true, results.dependencyCycles},
		{"Empty projects", "empty_projects", true, results.emptyProjects},
	}
}

// printResults writes the text report. An empty section says nothing a reader
// needs, so it is skipped; the verdict line runDoctor prints afterwards is what
// states a clean run. The JSON report still carries every section. It reports
// whether it printed anything, so the verdict knows whether to set itself off.
func printResults(results *doctorResults) bool {
	printed := false
	for _, section := range doctorSections(results) {
		if len(section.issues) == 0 {
			continue
		}
		printed = true
		if section.info {
			printInfoSection(section.title, section.issues)
		} else {
			printSection(section.title, section.issues)
		}
	}
	return printed
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

// doctorReportSchema versions the --json shape, so a consumer can refuse a
// report it does not understand.
const doctorReportSchema = 1

// doctorReport is the --json document. Buckets hold issues and Notes the
// informational sections; every section is present, empty ones as [].
type doctorReport struct {
	Schema      int                          `json:"schema"`
	Buckets     map[string][]doctorIssueJSON `json:"buckets"`
	Notes       map[string][]doctorIssueJSON `json:"notes"`
	Fixed       []string                     `json:"fixed"`
	TotalIssues int                          `json:"total_issues"`
}

type doctorIssueJSON struct {
	Project string `json:"project"`
	Detail  string `json:"detail"`
}

func newDoctorReport(results *doctorResults, fixed []string) doctorReport {
	report := doctorReport{
		Schema:      doctorReportSchema,
		Buckets:     map[string][]doctorIssueJSON{},
		Notes:       map[string][]doctorIssueJSON{},
		Fixed:       append([]string{}, fixed...),
		TotalIssues: results.totalIssues(),
	}
	for _, section := range doctorSections(results) {
		issues := make([]doctorIssueJSON, 0, len(section.issues))
		for _, issue := range section.issues {
			issues = append(issues, doctorIssueJSON{Project: issue.project, Detail: issue.detail})
		}
		if section.info {
			report.Notes[section.key] = issues
		} else {
			report.Buckets[section.key] = issues
		}
	}
	return report
}
