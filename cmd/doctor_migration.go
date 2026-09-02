package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
)

func checkMigrationSnapshots(dataDir string, schema int, results *doctorResults) {
	v5Inspection, err := config.InspectPreSimplificationV5Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  err.Error(),
		})
		return
	}
	for _, issue := range v5Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  issue,
		})
	}
	if v5Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v5Inspection.Pairs), "pre-v5 snapshot pair", "pre-v5 snapshot pairs")),
		})
	}
	v6Inspection, err := config.InspectPreConsolidationV6Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: err.Error()})
		return
	}
	for _, issue := range v6Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: issue})
	}
	if v6Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v6Inspection.Pairs), "pre-v6 snapshot pair", "pre-v6 snapshot pairs")),
		})
	} else if schema >= config.V6ConfigSchemaVersion {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{project: "Migration", detail: "clean schema-6 workspace; no pre-v6 snapshot recorded"})
	}
	v7Inspection, err := config.InspectPreScopeV7Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: err.Error()})
		return
	}
	for _, issue := range v7Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: issue})
	}
	if v7Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v7Inspection.Pairs), "pre-v7 snapshot pair", "pre-v7 snapshot pairs")),
		})
	} else if schema >= config.V7ConfigSchemaVersion {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{project: "Migration", detail: "clean schema-7 workspace; no pre-v7 snapshot recorded"})
	}
	checkLaterMigrationSnapshots(dataDir, results)
}

// checkLaterMigrationSnapshots covers every boundary from schema 8 on. The v5,
// v6, and v7 checks above each validate a structural conversion's own artifacts,
// but from v8 the contract is uniform — an archive and its audit report, published
// as a pair — so one generic scan covers all ten and advances with the chain
// instead of freezing at the schema it was written for.
//
// A boundary that published nothing is silent rather than noted: a workspace
// created at the current schema crossed none of them, and ten "no snapshot"
// lines would bury the findings that matter.
func checkLaterMigrationSnapshots(dataDir string, results *doctorResults) {
	inspections, err := config.InspectMigrationSnapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: err.Error()})
		return
	}
	for _, inspection := range inspections {
		for _, issue := range inspection.Issues {
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: issue})
		}
		if !inspection.HasArtifacts {
			continue
		}
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail: fmt.Sprintf(
				"%s recorded",
				Pluralize(
					len(inspection.Pairs),
					fmt.Sprintf("pre-v%d snapshot pair", inspection.Schema),
					fmt.Sprintf("pre-v%d snapshot pairs", inspection.Schema),
				),
			),
		})
	}
}
