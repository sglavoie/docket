package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// cliWorkflowRunner is scoped to one command invocation. It freezes the enabled
// rule set at command start, supplies fresh checked config-artifact builders to
// every workflow-aware application call, and accumulates trigger counters across
// commands that intentionally commit one task at a time (partial-success loops).
// Trigger counts are flushed once, best-effort, after all task transactions have
// landed so an automation write failure never turns a committed task mutation
// into a command failure.
type cliWorkflowRunner struct {
	workspace *CLIWorkspace
	service   *app.Service
	rules     []workflow.WorkflowRule
	counts    map[string]int

	// baselineFresh is false when the latest committed workflow transaction could
	// not be reflected into RuntimeState. Counter persistence then fails closed:
	// writing against the stale automation baseline could overwrite a template
	// use-count artifact that was committed with the task.
	baselineFresh bool
}

func newCLIWorkflowRunner(workspace *CLIWorkspace) *cliWorkflowRunner {
	runner := &cliWorkflowRunner{
		workspace:     workspace,
		counts:        make(map[string]int),
		baselineFresh: true,
	}
	if workspace == nil {
		return runner
	}
	runner.service = newWorkspaceAppService(workspace)
	if workspace.Runtime != nil {
		runner.rules = append([]workflow.WorkflowRule(nil), workspace.Runtime.Automation().GetEnabledWorkflowRules()...)
	}
	return runner
}

// options builds the common workflow options for one command mutation. The
// task context is captured at the adapter edge to keep ledger entries readable;
// creation APIs leave it empty and calculate the minted short ID themselves.
func (r *cliWorkflowRunner) options(_ bool, undoAction string, task *models.Task) app.SingleTaskWorkflowOptions {
	undoContext := ""
	if task != nil {
		undoContext = fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	}
	return app.SingleTaskWorkflowOptions{
		Rules:                  append([]workflow.WorkflowRule(nil), r.rules...),
		UndoAction:             undoAction,
		UndoContext:            undoContext,
		ProjectStatusArtifacts: cliWorkflowProjectStatusArtifactBuilder(r.workspace),
	}
}

// optionsWithContext is used when the mutation itself determines the best undo
// context (for example an edited title or a template name plus minted task ID).
func (r *cliWorkflowRunner) optionsWithContext(contextual bool, undoAction, undoContext string) app.SingleTaskWorkflowOptions {
	opts := r.options(contextual, undoAction, nil)
	opts.UndoContext = undoContext
	return opts
}

// observe records the presentation-neutral workflow effects from one successful
// application transaction. An empty touched-project set is a guarded no-op, so
// there is no committed config state to refresh.
func (r *cliWorkflowRunner) observe(result app.BulkResult) {
	if len(result.Conflicts) > 0 {
		emitCLIWorkflowConflicts(result.Conflicts)
	}
	if len(result.TouchedProjectIDs) == 0 {
		return
	}

	if r.workspace == nil || r.workspace.Runtime == nil {
		r.baselineFresh = false
		logging.Warn("failed to refresh workflow runtime after committed transaction: runtime state is unavailable")
	} else if err := r.workspace.Runtime.RefreshAfterCommit(); err != nil {
		r.baselineFresh = false
		logging.Warn("failed to refresh workflow runtime after committed transaction: %v", err)
	} else {
		r.baselineFresh = true
	}

	for ruleID, count := range result.RuleTriggerCounts {
		if count > 0 {
			r.counts[ruleID] += count
		}
	}
}

// flush persists all trigger counts accumulated by this command in one
// automation-domain commit. It is intentionally warning-only: task/config
// mutations represented by the counts have already committed successfully.
func (r *cliWorkflowRunner) flush() {
	if len(r.counts) == 0 {
		return
	}
	if !r.baselineFresh || r.workspace == nil || r.workspace.Runtime == nil {
		logging.Warn("failed to save workflow rule trigger counts: runtime automation baseline is stale")
		return
	}
	domains, err := r.workspace.Runtime.CloneDomains()
	if err != nil {
		logging.Warn("failed to save workflow rule trigger counts: %v", err)
		return
	}
	domains.Automation.IncrementRuleTriggerCounts(r.counts)
	if err := r.workspace.Runtime.CommitAutomation(domains.Automation); err != nil {
		logging.Warn("failed to save workflow rule trigger counts: %v", err)
	}
}

func workflowResultTask(result app.BulkResult) app.TaskResult {
	if len(result.Tasks) == 0 {
		return app.TaskResult{}
	}
	return result.Tasks[0]
}

func workflowResultCommitted(result app.BulkResult) bool {
	return len(result.TouchedProjectIDs) > 0
}

// cliWorkflowProjectStatusArtifactBuilder applies project-level status rules to
// a freshly loaded catalog and returns one checked catalog artifact for the same
// grouped transaction as the task cascade.
func cliWorkflowProjectStatusArtifactBuilder(workspace *CLIWorkspace) app.WorkflowProjectStatusArtifactBuilder {
	if workspace == nil || workspace.Manager == nil {
		return nil
	}
	return func(changes []app.WorkflowProjectStatusChange) ([]storage.ArtifactChange, error) {
		coord := workspace.Manager.DomainCoordinator()
		catalog, err := coord.LoadCatalog()
		if err != nil {
			return nil, err
		}
		changed := false
		for _, change := range changes {
			found := false
			for i := range catalog.Projects {
				if catalog.Projects[i].ID != change.ProjectID {
					continue
				}
				found = true
				if catalog.Projects[i].StatusIcon != change.StatusIcon || catalog.Projects[i].StatusIconSource != change.StatusIconSource {
					catalog.Projects[i].StatusIcon = change.StatusIcon
					catalog.Projects[i].StatusIconSource = change.StatusIconSource
					changed = true
				}
				break
			}
			if !found {
				return nil, fmt.Errorf("workflow project status target %q is absent from the workspace catalog", change.ProjectID)
			}
		}
		if !changed {
			return nil, nil
		}
		artifact, err := coord.CheckedCatalogArtifact(catalog)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

// emitCLIWorkflowConflicts presents app-normalized conflicts deterministically.
// Sorting keeps multi-task partial-success commands stable regardless of map or
// rule traversal order while preserving the legacy warning-only behavior.
func emitCLIWorkflowConflicts(conflicts []app.WorkflowConflict) {
	normalized := append([]app.WorkflowConflict(nil), conflicts...)
	for i := range normalized {
		normalized[i].Rules = append([]string(nil), normalized[i].Rules...)
		sort.Strings(normalized[i].Rules)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].TaskID != normalized[j].TaskID {
			return normalized[i].TaskID < normalized[j].TaskID
		}
		if normalized[i].Field != normalized[j].Field {
			return normalized[i].Field < normalized[j].Field
		}
		return strings.Join(normalized[i].Rules, "\x00") < strings.Join(normalized[j].Rules, "\x00")
	})

	fmt.Fprintln(os.Stderr, "warning: workflow rule conflicts detected:")
	for _, conflict := range normalized {
		task := conflict.TaskID
		if conflict.TaskTitle != "" {
			task = fmt.Sprintf("%s (%s)", conflict.TaskTitle, conflict.TaskID)
		}
		fmt.Fprintf(os.Stderr, "  %s: %s <- %s\n", task, conflict.Field, strings.Join(conflict.Rules, ", "))
	}
}
