package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

const CapabilityHarness Capability = "surface:test-harness"

const (
	operationHarnessNewProject      OperationID = "tui.harness.new-project"
	operationHarnessOutliner        OperationID = "tui.harness.outliner"
	operationHarnessOutlinerRoot    OperationID = "tui.harness.outliner-root"
	operationHarnessReference       OperationID = "tui.harness.reference"
	operationHarnessScope           OperationID = "tui.harness.scope"
	operationHarnessInbox           OperationID = "tui.harness.inbox"
	operationHarnessProjectInbox    OperationID = "tui.harness.project-inbox"
	operationHarnessRenameProject   OperationID = "tui.harness.rename-project"
	operationHarnessTaskAttachments OperationID = "tui.harness.task-attachments"
	operationHarnessTaskComments    OperationID = "tui.harness.task-comments"
	operationHarnessProjectNotes    OperationID = "tui.harness.project-notes"
	operationHarnessOutlineErrors   OperationID = "tui.harness.outline-errors"
	operationHarnessOutlineDelete   OperationID = "tui.harness.outline-delete-confirm"
	operationHarnessEditorSigils    OperationID = "tui.harness.external-editor-sigils"
	operationHarnessEditorRefusal   OperationID = "tui.harness.external-editor-refusal"
	operationHarnessDanglingLens    OperationID = "tui.harness.dangling-active-lens"
	operationHarnessRecordConflict  OperationID = "tui.harness.record-conflict"
)

// The canned conflict bundle. A conflict is recorded only when a project changes
// underneath a mutation, which no scenario action can stage, so the harness
// writes one directly. Its project identity is fixed rather than read off the
// fixture: the browser is read-only and never resolves the project, so the
// record is self-describing, and fixed values are what let a scenario assert on
// the row it draws.
const (
	harnessConflictProjectName = "Rich Project"
	harnessConflictProjectSlug = "rich-project"
	harnessConflictAction      = "update task"
	harnessConflictTaskTitle   = "Ship the release notes"
)

// harnessDanglingLensID is a canonical UUID that names no saved lens. It stands
// for the record another machine deleted while this one still points at it: the
// pointer is shape-valid and unresolvable, which is the one state no public key
// on this machine can produce, because deleting a lens here clears the pointer
// in the same commit.
const harnessDanglingLensID = "3f4c2a91-7d58-4b0e-9c16-2ae5d81f6b34"

// Canned external-editor titles. A scenario command carries no arguments, so
// each title the harness can return needs its own operation.
const (
	harnessEditorSigilTitle   = "Harness edited title !1 #urgent $next"
	harnessEditorRefusedTitle = "Harness edited title $invalid"
)

// harnessOutlineScenarioPath is the buffer path the outline scenarios name. No
// file is written: the scenarios only assert that a refusal points back at the
// buffer the user still has.
const harnessOutlineScenarioPath = "/tmp/tt-outline-scenario.md"

type harnessOperationSpec struct {
	id       OperationID
	label    string
	category OperationCategory
	message  func() tea.Msg
}

// registerHarnessOperations keeps layout and semantic scenario setup on the
// same OperationRegistry/OperationExecutor foundation as production actions.
// CapabilityHarness is added only by UITestDriver.runCommand, so these deep
// links never appear in help, footers, or available-command output.
func (m *AppModel) registerHarnessOperations() error {
	specs := []harnessOperationSpec{
		{operationHarnessNewProject, "new project", OperationCategoryProject, func() tea.Msg { return CreateProjectMsg{} }},
		{operationHarnessOutliner, "outliner", OperationCategoryNavigation, func() tea.Msg { return m.harnessOpenOutliner(OutlinerOpenOptions{}) }},
		{operationHarnessOutlinerRoot, "outliner root", OperationCategoryNavigation, func() tea.Msg { return m.harnessOpenOutliner(OutlinerOpenOptions{}) }},
		{operationHarnessReference, "reference", OperationCategorySystem, func() tea.Msg {
			return ShowHelpMsg{
				ViewName: "Outliner",
				Items:    m.coreInputHelpItems(),
			}
		}},
		{operationHarnessScope, "choose scope", OperationCategoryScope, func() tea.Msg { return ShowProjectHubMsg{} }},
		{operationHarnessInbox, "global inbox mode", OperationCategoryMode, func() tea.Msg {
			return m.harnessOpenOutliner(OutlinerOpenOptions{SetMode: true, Mode: ModeInbox})
		}},
		{operationHarnessProjectInbox, "project inbox mode", OperationCategoryMode, func() tea.Msg {
			return m.harnessOpenOutliner(OutlinerOpenOptions{SetMode: true, Mode: ModeInbox})
		}},
		{operationHarnessRenameProject, "rename project", OperationCategoryProject, func() tea.Msg {
			_, project := m.outlinerTarget()
			if project == nil {
				return nil
			}
			return EditProjectMsg{Project: project}
		}},
		{operationHarnessTaskAttachments, "task attachments", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskAttachmentsMsg{Task: task, Project: project}
		}},
		{operationHarnessTaskComments, "task comments", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskCommentsMsg{Task: task, Project: project}
		}},
		{operationHarnessProjectNotes, "show notes", OperationCategoryNote, func() tea.Msg {
			_, project := m.outlinerTarget()
			if project != nil {
				return ShowNotesManagerMsg{Scope: ProjectNotesScope(project.ID)}
			}
			return ShowNotesManagerMsg{Scope: AllNotesScope()}
		}},
		{operationHarnessOutlineErrors, "outline errors", OperationCategorySystem, func() tea.Msg {
			// The outline refusal surface is only reachable through an external
			// editor round trip, which no harness action can replay: editors are
			// disabled and the `command` action carries no buffer payload. Scenarios
			// therefore install it directly with a canned problem list.
			m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(
				"Rich Project",
				harnessOutlineScenarioPath,
				harnessOutlineProblems(),
			))
			return nil
		}},
		{operationHarnessOutlineDelete, "outline delete confirm", OperationCategorySystem, func() tea.Msg {
			// Same reason as the refusal surface: the round trip cannot be replayed,
			// so the harness dumps the cursor's project, drops the buffer's last line,
			// and hands the result to the real handler. Everything the prompt shows is
			// therefore produced by the production path.
			return m.harnessOutlineDeleteConfirm()
		}},
		{operationHarnessEditorSigils, "external editor sigils", OperationCategorySystem, func() tea.Msg {
			return m.harnessTaskContentEdited(harnessEditorSigilTitle, false)
		}},
		{operationHarnessEditorRefusal, "external editor refusal", OperationCategorySystem, func() tea.Msg {
			return m.harnessTaskContentEdited(harnessEditorRefusedTitle, true)
		}},
		{operationHarnessDanglingLens, "dangling active lens", OperationCategorySystem, func() tea.Msg {
			// The saved definitions are left alone: what is being staged is a
			// pointer that outlived its record, which is what the next launch has
			// to fail closed on.
			if err := m.commitNamedLensMutation(func(machine *config.MachineState) error {
				machine.ActiveLensID = harnessDanglingLensID
				return nil
			}); err != nil {
				return ErrorMsg{Err: err}
			}
			return nil
		}},
		{operationHarnessRecordConflict, "record merge conflict", OperationCategorySystem, func() tea.Msg {
			if err := m.harnessRecordConflict(); err != nil {
				return ErrorMsg{Err: err}
			}
			// The strip counts the same directory this just wrote to, so the count
			// is refreshed here rather than waiting for the 60s tick a scenario
			// never reaches.
			m.refreshOutstandingConflicts()
			return nil
		}},
	}

	for _, spec := range specs {
		spec := spec
		descriptor := OperationDescriptor{
			ID:                   spec.id,
			Label:                spec.label,
			Category:             spec.category,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityHarness),
			Risk:                 OperationRiskReadOnly,
		}
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilityHarness),
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
			Handler: func(OperationInvocation) OperationHandlerResult {
				return OperationHandlerResult{Message: spec.message()}
			},
		}); err != nil {
			return fmt.Errorf("%s: %w", spec.id, err)
		}
	}
	return nil
}

// harnessRecordConflict writes one conflict bundle into the workspace's store.
// The three documents carry the project name and no tasks: the browser reads the
// record alone, but Load parses all three, so a bundle written with nil documents
// would be a shape the merge path never produces.
func (m *AppModel) harnessRecordConflict() error {
	store := m.conflictStore()
	if store == nil {
		return fmt.Errorf("recording a conflict needs a workspace")
	}
	side := func() *storage.ProjectDocument {
		return &storage.ProjectDocument{
			ProjectName:   harnessConflictProjectName,
			SchemaVersion: storage.SchemaVersionV14,
		}
	}
	retryRequired := true
	_, err := store.Save(storage.ConflictRecord{
		ProjectName:            harnessConflictProjectName,
		ProjectSlug:            harnessConflictProjectSlug,
		Action:                 harnessConflictAction,
		BaseRevision:           "b0b0b0b0b0b0b0b0",
		LocalRevision:          "1oca11oca11oca11",
		RemoteRevision:         "5e3e5e3e5e3e5e3e",
		RequiresOperationRetry: &retryRequired,
		Conflicts: []storage.MergeConflict{
			{
				Kind:      storage.MergeConflictFieldEdit,
				TaskTitle: harnessConflictTaskTitle,
				Field:     "title",
				Base:      "Draft the release notes",
				Local:     "Ship the release notes",
				Current:   "Publish the release notes",
			},
			{
				Kind:      storage.MergeConflictHierarchy,
				TaskTitle: harnessConflictTaskTitle,
				Detail:    "both sides reparented this task",
			},
		},
	}, side(), side(), side())
	return err
}

// harnessOutlineProblems is one rejected buffer's worth of problems, holding
// both validation phases and a message long enough to need wrapping at narrow
// widths.
func harnessOutlineProblems() []lineError {
	return []lineError{
		{line: 4, message: "indent of 3 spaces is not a multiple of 2"},
		{line: 6, message: "marker [[tt:deadbeef]] does not name a task in this project"},
		{line: 9, message: "indent jumps from level 0 to level 2; indent one level at a time"},
		{line: 11, message: "task title cannot be empty"},
		{line: 12, message: "indentation sets parentage here; remove the >parent sigil"},
		{line: 14, message: "indentation sets parentage here; remove the <child sigil"},
		{line: 15, message: "the outline buffer cannot defer; remove the @@defer sigil"},
	}
}

// harnessOutlineDeleteConfirm replays the returned half of the round trip. The
// dropped line is the buffer's last, which depth-first order guarantees is a
// leaf, so the remaining buffer is still a valid tree.
func (m *AppModel) harnessOutlineDeleteConfirm() tea.Msg {
	target, label, refusal := m.outlineEditorRequest()
	if target == nil || refusal != "" {
		return nil
	}
	project, revision, buffer, scopeTaskIDs, err := m.outlineDumpSource(target.ID, label)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(buffer, "\n"), "\n")
	m.handleOutlineEdited(OutlineEditedMsg{
		Project:      project,
		Revision:     revision,
		ScopeTaskIDs: scopeTaskIDs,
		Path:         harnessOutlineScenarioPath,
		Buffer:       strings.Join(lines[:len(lines)-1], "\n"),
	})
	return nil
}

// harnessTaskContentEdited replays the returned half of the Ctrl+E round trip.
// The editor half cannot run under a scenario, so a canned buffer result is
// injected and the production handler does the rest. retry marks the message as
// a reopened buffer the user saved without fixing, which is the one refusal
// outcome a scenario can observe: the reopen itself would launch an editor.
func (m *AppModel) harnessTaskContentEdited(title string, retry bool) tea.Msg {
	task, project := m.outlinerTarget()
	if task == nil || project == nil {
		return nil
	}
	msg := TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		NewTitle:       title,
		NewDescription: "Body written in the external editor.",
	}
	if retry {
		_, err := parseQuickEdit(title, task, project, m.availableProjects())
		if err == nil {
			return nil
		}
		msg.RejectedTitle = title
		msg.Notice = err.Error()
	}
	return msg
}

func (m *AppModel) harnessOpenOutliner(options OutlinerOpenOptions) tea.Msg {
	presentationCmd := m.prepareCrossProjectOutliner()
	m.crossOutliner.ApplyOpenOptions(options)
	route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err != nil {
		return ErrorMsg{Err: err}
	}
	return OperationCommandResultMsg{Cmd: tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)}
}
