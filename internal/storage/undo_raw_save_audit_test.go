package storage

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	rawSaveCLIHelper     = "CLI save helper implementation; undo-aware callers use SaveOptions metadata"
	rawSaveCLIMigration  = "legacy CLI mutation path explicitly allowed while undo coverage is audited; new callsites must use undo metadata"
	rawSaveRepair        = "repair/side-effect save path outside direct user mutation; excluded from user undo history"
	rawSaveTUIHelper     = "TUI save helper implementation; undo-aware callers use ProjectSaveOptions metadata"
	rawSaveTUIMigration  = "legacy TUI mutation path explicitly allowed while undo coverage is audited; new callsites must use undo-aware save options"
	rawSaveTUIRecorded   = "TUI path records undo before saving through a noncanonical helper"
	rawSaveRestoreResult = "undo/redo restore result save; not a new user mutation"
)

var rawProjectSaveAllowlist = map[string]string{
	"cmd/board_add.go:runBoardAdd:storage.SaveProjectLocked(targetProject)":                                                              rawSaveCLIMigration,
	"cmd/board_delete.go:runBoardDelete:SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr})":                             rawSaveCLIMigration,
	"cmd/doctor.go:fixAutoRepaired:storage.SaveProjectLocked(project)":                                                                   rawSaveRepair,
	"cmd/doctor.go:fixIssues:storage.SaveProjectLocked(project)":                                                                         rawSaveRepair,
	"cmd/doctor.go:fixMissingDescriptionMarkers:storage.SaveProjectLocked(project)":                                                      rawSaveRepair,
	"cmd/project_delete.go:cleanCrossProjectRefs:storage.SaveProjectLocked(p)":                                                           rawSaveRepair,
	"cmd/project_edit.go:runProjectEdit:storage.SaveProjectLocked(project)":                                                              rawSaveCLIMigration,
	"cmd/project_edit.go:updateCrossProjectRefs:storage.SaveProjectLocked(p)":                                                            rawSaveRepair,
	"cmd/project_ops.go:SaveProject:storage.SaveProject(project)":                                                                        rawSaveCLIHelper,
	"cmd/project_ops.go:processDeferredMovesWithUndo:storage.SaveProject(destProject)":                                                   rawSaveCLIMigration,
	"cmd/sandbox_reset.go:runSandboxReset:SaveProject(sandbox, &SaveOptions{Config: cfg, ConfigManager: configManager})":                 rawSaveCLIMigration,
	"cmd/task_delete.go:cleanDeletedTaskCrossProjectRefs:storage.SaveProjectLocked(p)":                                                   rawSaveRepair,
	"cmd/task_delete.go:runTaskDelete:SaveProject(project, &SaveOptions{ Config: cfg, ConfigManager: configManager, })":                  rawSaveCLIMigration,
	"cmd/task_move_to.go:runTaskMoveTo:SaveProject(destProject, &SaveOptions{Config: cfg, ConfigManager: configManager})":                rawSaveCLIMigration,
	"cmd/task_move_to.go:runTaskMoveTo:SaveProject(sourceProject, &SaveOptions{Config: cfg, ConfigManager: configManager})":              rawSaveCLIMigration,
	"cmd/undo.go:reapplySideEffectsAndSave:storage.SaveProject(project)":                                                                 rawSaveRestoreResult,
	"internal/tui/app_handlers.go:(*AppModel).applyTagsToTask:saveProjectSync(project)":                                                  rawSaveTUIMigration,
	"internal/tui/app_pinned_tasks_update.go:(*AppModel).reorderPinnedTask:m.persister.SaveWithBackupSync(a.Project)":                    rawSaveTUIMigration,
	"internal/tui/app_pinned_tasks_update.go:(*AppModel).reorderPinnedTask:m.persister.SaveWithBackupSync(b.Project)":                    rawSaveTUIMigration,
	"internal/tui/app_pinned_tasks_update.go:(*AppModel).repinFromPopup:m.persister.SaveWithBackupSync(entry.Project)":                   rawSaveTUIMigration,
	"internal/tui/app_pinned_tasks_update.go:(*AppModel).unpinFromPopup:m.persister.SaveWithBackupSync(entry.Project)":                   rawSaveTUIMigration,
	"internal/tui/app_run.go:RunApp:storage.SaveProjectLocked(current)":                                                                  rawSaveRepair,
	"internal/tui/app_run.go:evaluateWorkflowAtStartup:storage.SaveProjectLocked(project)":                                               rawSaveRepair,
	"internal/tui/async_save.go:SaveMultipleProjectsAsync:projectService.SaveMultiple(projects, successMsg)":                             rawSaveTUIHelper,
	"internal/tui/bulk_capture.go:(*BulkCaptureModel).createTasks:saveProjectSync(m.project)":                                            rawSaveTUIMigration,
	"internal/tui/cross_project_linker_update.go:(*CrossProjectLinkerModel).createLink:storage.SaveProjectLocked(srcProj)":               rawSaveTUIMigration,
	"internal/tui/cross_project_linker_update.go:(*CrossProjectLinkerModel).createLink:storage.SaveProjectLocked(tgtProj)":               rawSaveTUIMigration,
	"internal/tui/cross_project_outliner_mutations_actions.go:(*OutlinerModel).crossSendTaskToSection:svc.Save(proj)":                    rawSaveTUIRecorded,
	"internal/tui/cross_project_outliner_mutations_common.go:(*OutlinerModel).crossPersistFn:svc.Save(project)":                          rawSaveTUIRecorded,
	"internal/tui/cross_project_outliner_mutations_common.go:(*OutlinerModel).crossSaveProject:svc.Save(proj)":                           rawSaveTUIMigration,
	"internal/tui/cross_project_outliner_mutations_fields.go:(*OutlinerModel).crossHandleDeferRecurrence:m.projectService.Save(proj)":    rawSaveTUIRecorded,
	"internal/tui/cross_project_outliner_mutations_fields.go:(*OutlinerModel).crossHandleSkipRecurrence:m.projectService.Save(proj)":     rawSaveTUIRecorded,
	"internal/tui/cross_project_outliner_mutations_fields.go:(*OutlinerModel).crossHandleSnoozeMutation:m.projectService.Save(proj)":     rawSaveTUIRecorded,
	"internal/tui/cross_project_outliner_mutations_status.go:(*OutlinerModel).crossExecutePendingBulkStatus:svc.Save(group.proj)":        rawSaveTUIRecorded,
	"internal/tui/date_input.go:(*DateInputModel).saveAndReturn:saveProjectSync(project)":                                                rawSaveTUIMigration,
	"internal/tui/focus_board_input.go:(*FocusBoardModel).createItemFromInput:storage.SaveProjectLocked(targetProject)":                  rawSaveTUIMigration,
	"internal/tui/global_inbox_processor.go:(*GlobalInboxProcessorModel).saveCmd:saveProjectSync(p)":                                     rawSaveTUIMigration,
	"internal/tui/handler_project.go:(*AppModel).executeSandboxReset:storage.SaveProjectLocked(sandbox)":                                 rawSaveTUIRecorded,
	"internal/tui/handler_task_move.go:(*AppModel).saveBothProjectsAsync:persister.Save(dest)":                                           rawSaveTUIMigration,
	"internal/tui/handler_task_move.go:(*AppModel).saveBothProjectsAsync:persister.Save(source)":                                         rawSaveTUIMigration,
	"internal/tui/inbox_processor.go:(*InboxProcessorModel).saveCmd:saveProjectSync(project)":                                            rawSaveTUIMigration,
	"internal/tui/linked_task_creator.go:(*LinkedTaskCreatorModel).createTask:saveProjectSync(m.project)":                                rawSaveTUIMigration,
	"internal/tui/linked_task_selector_update.go:(*LinkedTaskSelectorModel).unlinkTask:storage.SaveProjectLocked(project)":               rawSaveTUIMigration,
	"internal/tui/outliner_sidebar.go:(*OutlinerModel).UnlinkSelectedSidebarTask:storage.SaveProjectLocked(targetProject)":               rawSaveTUIMigration,
	"internal/tui/project_service.go:(*ProjectService).Save:storage.SaveProjectLocked(project)":                                          rawSaveTUIHelper,
	"internal/tui/project_service.go:(*ProjectService).SaveMultipleOptions:saveProjectCore(project, true, opts)":                         rawSaveTUIHelper,
	"internal/tui/project_service.go:(*ProjectService).SaveMultipleSyncOptions:saveProjectCore(project, true, opts)":                     rawSaveTUIHelper,
	"internal/tui/project_service.go:(*ProjectService).SaveWithBackupOptions:saveProjectCore(project, true, opts)":                       rawSaveTUIHelper,
	"internal/tui/project_service.go:(*ProjectService).SaveWithBackupSyncOptions:saveProjectCore(project, true, opts)":                   rawSaveTUIHelper,
	"internal/tui/project_service.go:saveProjectCore:storage.SaveProject(project)":                                                       rawSaveTUIHelper,
	"internal/tui/project_service.go:saveProjectNoReload:saveProjectCore(project, false, ProjectSaveOptions{})":                          rawSaveTUIHelper,
	"internal/tui/project_service.go:saveProjectSync:saveProjectCore(project, true, ProjectSaveOptions{})":                               rawSaveTUIHelper,
	"internal/tui/project_tree_sidebar_host.go:(*ProjectTreeModel).UnlinkSelectedSidebarTask:ps.Save(project)":                           rawSaveTUIMigration,
	"internal/tui/project_tree_sidebar_host.go:(*ProjectTreeModel).UnlinkSelectedSidebarTask:storage.SaveProjectLocked(targetProject)":   rawSaveTUIMigration,
	"internal/tui/quick_capture.go:(*QuickCaptureModel).createTask:saveProjectSync(targetProject)":                                       rawSaveTUIMigration,
	"internal/tui/quick_note_overlay.go:(*QuickNoteOverlay).handleKey:saveProjectNoReload(q.project)":                                    rawSaveTUIMigration,
	"internal/tui/score_editor.go:(*ScoreEditorModel).save:saveProjectNoReload(project)":                                                 rawSaveTUIMigration,
	"internal/tui/section_manager_actions.go:(*SectionManagerModel).setSectionColor:saveProjectSync(proj)":                               rawSaveTUIMigration,
	"internal/tui/section_manager_actions.go:(*SectionManagerModel).setSectionColor:storage.SaveProjectLocked(p)":                        rawSaveRepair,
	"internal/tui/shared_secondary_ops_bulk.go:saveAffectedProjectsAsync:SaveMultipleProjectsAsync(projects, successMsg)":                rawSaveTUIMigration,
	"internal/tui/shared_task_ops_persist.go:persistWithWorkflow:opts.Persister.Save(opts.Project)":                                      rawSaveTUIRecorded,
	"internal/tui/shared_task_ops_persist.go:processTUIDeferredMoves:p.Save(destProject)":                                                rawSaveTUIRecorded,
	"internal/tui/shared_task_ops_persist.go:reapplySideEffects:persister.Save(project)":                                                 rawSaveRestoreResult,
	"internal/tui/shared_task_ops_persist.go:withPersist:p.Save(project)":                                                                rawSaveTUIRecorded,
	"internal/tui/snooze_input.go:(*SnoozeInputModel).saveAndReturn:saveProjectSync(project)":                                            rawSaveTUIMigration,
	"internal/tui/tag_manager.go:(*TagManagerModel).addProjectTag:saveProjectSync(project)":                                              rawSaveTUIMigration,
	"internal/tui/tag_manager.go:(*TagManagerModel).removeProjectTag:saveProjectSync(project)":                                           rawSaveTUIMigration,
	"internal/tui/tag_manager.go:(*TagManagerModel).renameTag:saveProjectSync(project)":                                                  rawSaveTUIMigration,
	"internal/tui/tag_manager.go:(*TagManagerModel).togglePinnedTag:saveProjectSync(project)":                                            rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).addCodeReference:saveProjectNoReload(project)":                     rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).addFileAttachment:saveProjectNoReload(project)":                    rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).addURLAttachment:saveProjectNoReload(project)":                     rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).executeDelete:saveProjectNoReload(project)":                        rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).executeKeep:saveProjectNoReload(project)":                          rawSaveTUIMigration,
	"internal/tui/task_attachments_actions.go:(*TaskAttachmentsModel).reorderAttachment:saveProjectNoReload(project)":                    rawSaveTUIMigration,
	"internal/tui/task_checklist_actions.go:(*TaskChecklistModel).executeDelete:saveProjectNoReload(project)":                            rawSaveTUIMigration,
	"internal/tui/task_checklist_actions.go:(*TaskChecklistModel).reorderItem:saveProjectNoReload(project)":                              rawSaveTUIMigration,
	"internal/tui/task_checklist_actions.go:(*TaskChecklistModel).saveEditedItem:saveProjectNoReload(project)":                           rawSaveTUIMigration,
	"internal/tui/task_checklist_actions.go:(*TaskChecklistModel).saveItem:saveProjectNoReload(project)":                                 rawSaveTUIMigration,
	"internal/tui/task_checklist_actions.go:(*TaskChecklistModel).toggleItem:saveProjectNoReload(project)":                               rawSaveTUIMigration,
	"internal/tui/task_checklist_editor.go:(*TaskChecklistModel).addItemInExternalEditor:saveProjectNoReload(project)":                   rawSaveTUIMigration,
	"internal/tui/task_checklist_editor.go:(*TaskChecklistModel).editInExternalEditor:saveProjectNoReload(project)":                      rawSaveTUIMigration,
	"internal/tui/task_checklist_editor.go:(*TaskChecklistModel).editItemInExternalEditorFromEditMode:saveProjectNoReload(project)":      rawSaveTUIMigration,
	"internal/tui/task_comments_actions.go:(*TaskCommentsModel).addCommentInExternalEditor:saveProjectNoReload(project)":                 rawSaveTUIMigration,
	"internal/tui/task_comments_actions.go:(*TaskCommentsModel).editCommentInExternalEditorFromEditMode:saveProjectNoReload(project)":    rawSaveTUIMigration,
	"internal/tui/task_comments_actions.go:(*TaskCommentsModel).editInExternalEditor:saveProjectNoReload(project)":                       rawSaveTUIMigration,
	"internal/tui/task_decisions_actions.go:(*TaskDecisionsModel).addDecisionInExternalEditor:saveProjectNoReload(project)":              rawSaveTUIMigration,
	"internal/tui/task_decisions_actions.go:(*TaskDecisionsModel).editDecisionInExternalEditorFromEditMode:saveProjectNoReload(project)": rawSaveTUIMigration,
	"internal/tui/task_decisions_actions.go:(*TaskDecisionsModel).editInExternalEditor:saveProjectNoReload(project)":                     rawSaveTUIMigration,
	"internal/tui/task_form_save.go:(*TaskFormModel).saveCreate:saveProjectSync(m.project)":                                              rawSaveTUIMigration,
	"internal/tui/task_form_save.go:(*TaskFormModel).saveEdit:saveProjectSync(m.project)":                                                rawSaveTUIMigration,
	"internal/tui/task_item_panel_shared.go:persistTaskItemMutation:saveProjectNoReload(project)":                                        rawSaveTUIMigration,
}

func TestRawProjectSaveCallsitesAreAllowlisted(t *testing.T) {
	root := findRepoRootForUndoAudit(t)
	calls := collectRawProjectSaveCalls(t, root, []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal", "tui"),
	})

	actual := make(map[string]rawProjectSaveCall, len(calls))
	var unexpected []rawProjectSaveCall
	for _, call := range calls {
		actual[call.Key] = call
		if _, ok := rawProjectSaveAllowlist[call.Key]; !ok {
			unexpected = append(unexpected, call)
		}
	}

	var stale []string
	for key := range rawProjectSaveAllowlist {
		if _, ok := actual[key]; !ok {
			stale = append(stale, key)
		}
	}

	if len(unexpected) > 0 || len(stale) > 0 {
		sort.Slice(unexpected, func(i, j int) bool { return unexpected[i].Key < unexpected[j].Key })
		sort.Strings(stale)
		var b strings.Builder
		if len(unexpected) > 0 {
			b.WriteString("unexpected raw project-save callsites; either route through undo-aware save APIs or add a justified allowlist entry:\n")
			for _, call := range unexpected {
				fmt.Fprintf(&b, "\t%q: \"TODO: explain why this raw save may bypass undo for now\",\n", call.Key)
			}
		}
		if len(stale) > 0 {
			b.WriteString("stale raw project-save allowlist entries:\n")
			for _, key := range stale {
				fmt.Fprintf(&b, "\t%s\n", key)
			}
		}
		t.Fatal(b.String())
	}
}

func TestRawProjectSaveAllowlistReasonsAreExplicit(t *testing.T) {
	for key, reason := range rawProjectSaveAllowlist {
		trimmed := strings.TrimSpace(reason)
		if trimmed == "" {
			t.Fatalf("%s has an empty raw-save allowlist reason", key)
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "todo") || strings.Contains(lower, "tbd") || strings.Contains(lower, "not yet") {
			t.Fatalf("%s has a placeholder raw-save allowlist reason: %q", key, reason)
		}
	}
}

type rawProjectSaveCall struct {
	Key string
}

func collectRawProjectSaveCalls(t *testing.T, root string, dirs []string) []rawProjectSaveCall {
	t.Helper()

	var calls []rawProjectSaveCall
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("stat audit dir %s: %v", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			calls = append(calls, collectRawProjectSaveCallsFromFile(t, root, path)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk audit dir %s: %v", dir, err)
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].Key < calls[j].Key })
	return calls
}

func collectRawProjectSaveCallsFromFile(t *testing.T, root, path string) []rawProjectSaveCall {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	relPath := slashPath(t, root, path)
	var calls []rawProjectSaveCall
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fnName := auditFuncName(fset, fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if rawProjectSaveKind(fset, relPath, call) == "" {
				return true
			}
			calls = append(calls, rawProjectSaveCall{
				Key: fmt.Sprintf("%s:%s:%s", relPath, fnName, compactNode(fset, call)),
			})
			return true
		})
	}
	return calls
}

func rawProjectSaveKind(fset *token.FileSet, relPath string, call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return rawProjectSaveIdentKind(relPath, fun.Name, call)
	case *ast.SelectorExpr:
		receiver := compactNode(fset, fun.X)
		return rawProjectSaveSelectorKind(receiver, fun.Sel.Name)
	default:
		return ""
	}
}

func rawProjectSaveIdentKind(relPath, name string, call *ast.CallExpr) string {
	if strings.HasPrefix(relPath, "cmd/") && name == "SaveProject" && !saveProjectCallHasUndoAction(call) {
		return "cmd-save-without-undo-action"
	}
	if strings.HasPrefix(relPath, "internal/tui/") {
		switch name {
		case "SaveProjectAsync", "SaveMultipleProjectsAsync", "saveProjectSync", "saveProjectNoReload", "saveProjectCore":
			return "tui-save-helper-without-undo-options"
		}
	}
	return ""
}

func rawProjectSaveSelectorKind(receiver, name string) string {
	if receiver == "storage" {
		switch name {
		case "SaveProject", "SaveProjectLocked":
			return "direct-storage-save"
		}
	}
	switch name {
	case "SaveWithBackup", "SaveWithBackupSync", "SaveMultiple", "SaveMultipleSync":
		return "persister-save-without-undo-options"
	case "Save":
		switch receiver {
		case "m.projectService", "m.persister", "opts.Persister", "p", "persister", "ps", "svc":
			return "persister-save-without-undo-options"
		}
	}
	return ""
}

func saveProjectCallHasUndoAction(call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}

	opts := call.Args[1]
	if unary, ok := opts.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		opts = unary.X
	}
	lit, ok := opts.(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if ok && key.Name == "UndoAction" {
			return true
		}
	}
	return false
}

func auditFuncName(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return fmt.Sprintf("(%s).%s", compactNode(fset, fn.Recv.List[0].Type), fn.Name.Name)
}

func compactNode(fset *token.FileSet, node any) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func slashPath(t *testing.T, root, path string) string {
	t.Helper()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("rel path for %s: %v", path, err)
	}
	return filepath.ToSlash(rel)
}

func findRepoRootForUndoAudit(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing go.mod")
		}
		dir = parent
	}
}
