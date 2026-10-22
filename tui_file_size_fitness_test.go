package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/testutil/gotree"
)

// tuiPackageFileCeiling is the default bound on production files in a TUI
// package. File-only splits need more files without adding behavior or imports;
// the two bounded exceptions below pair that allowance with the 500-line guard.
const tuiPackageFileCeiling = 40

// The 500-line splits add 45 Outliner files and 21 shell files. These are
// fixed ceilings, not exemptions: another file still requires an explicit
// architectural decision, and the package graph remains independently checked.
var tuiPackageSplitFileCeilings = map[string]int{
	"outliner":     85,
	tuiRootPackage: 61,
}

// TestNoTUIPackageExceedsTheFileCeiling bounds every package, including the shell.
func TestNoTUIPackageExceedsTheFileCeiling(t *testing.T) {
	counts := tuiPackageFileCounts(t, repositoryRootDir(t))

	if _, measured := counts[tuiRootPackage]; !measured {
		t.Fatalf("the root shell was not measured, so this check would pass vacuously")
	}
	if len(counts) < 2 {
		t.Fatalf("measured %d packages under %s, so this check would pass vacuously", len(counts), tuiSubtree)
	}
	measured := map[string]bool{}
	for name := range counts {
		measured[name] = true
	}
	for _, name := range sortedNames(measured) {
		ceiling := tuiPackageFileCeiling
		if splitCeiling, ok := tuiPackageSplitFileCeilings[name]; ok {
			ceiling = splitCeiling
		}
		if counts[name] > ceiling {
			t.Errorf("package %s holds %d production .go files, over the ceiling of %d; move a concern out",
				name, counts[name], ceiling)
		}
	}
}

// tuiPackageFileCounts counts the production .go files each package under
// internal/tui holds. Test files are excluded: the ceiling is about how much
// production behavior one package carries, and a package's tests are as many
// files as its tests need.
func tuiPackageFileCounts(t *testing.T, root string) map[string]int {
	t.Helper()

	subtree := filepath.Join(root, filepath.FromSlash(tuiSubtree))
	counts := map[string]int{}
	err := filepath.WalkDir(subtree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if gotree.Ignored(entry.Name()) && path != subtree {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		owner, err := tuiPackageForDir(subtree, filepath.Dir(path))
		if err != nil {
			return err
		}
		counts[owner]++
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", tuiSubtree, err)
	}
	return counts
}

// TestRefactoredFilesStayWithinLineBudget keeps the file-count allowances from
// becoming permission to grow the extracted concerns back into large files.
// Count physical lines, including comments and imports, as in the split target.
func TestRefactoredFilesStayWithinLineBudget(t *testing.T) {
	root := repositoryRootDir(t)
	for _, name := range []string{
		"cmd/doctor_command_test.go",
		"cmd/doctor_repair_test.go",
		"cmd/doctor_test.go",
		"cmd/doctor_workspace_test.go",
		"contributor_guidance_docs_test.go",
		"internal/app/checklist_promotion_test.go",
		"internal/app/checklist_promotion_validation_test.go",
		"internal/app/due_step_test.go",
		"internal/app/due_step_transactions_test.go",
		"internal/bridge/organize_ops.go",
		"internal/bridge/organize_projects.go",
		"internal/bridge/organize_structure.go",
		"internal/bridge/rich_attachments.go",
		"internal/bridge/rich_notes.go",
		"internal/bridge/rich_ops.go",
		"internal/compat/compat_fixture_coverage_test.go",
		"internal/compat/compat_fixture_test.go",
		"internal/config/v21_migration_activation_test.go",
		"internal/config/v21_migration_recovery_test.go",
		"internal/engine/conflicts.go",
		"internal/engine/conflicts_reader.go",
		"internal/engine/sync_diagnostics.go",
		"internal/mobileparity/parity_helpers_test.go",
		"internal/mobileparity/parity_test.go",
		"internal/models/task_check_in_review_test.go",
		"internal/models/task_check_in_test.go",
		"internal/storage/sync_fault_harness_test.go",
		"internal/storage/sync_fault_invariants_test.go",
		"internal/storage/sync_fault_mutations_test.go",
		"internal/storage/sync_fault_transport_test.go",
		"internal/storage/sync_history_helpers_test.go",
		"internal/storage/sync_history_incoming_test.go",
		"internal/storage/sync_history_test.go",
		"internal/syncexchange/folder.go",
		"internal/syncexchange/folder_fetch.go",
		"internal/synclog/reconcile.go",
		"internal/synclog/reconcile_materialize.go",
		"internal/synclog/reconcile_resolution.go",
		"internal/syncmerge/rich_artifacts_test.go",
		"internal/syncmerge/rich_external_test.go",
		"internal/syncmerge/rich_test.go",
		"internal/syncmerge/rich_undo_test.go",
		"internal/tui/app_advisories.go",
		"internal/tui/app_footer_flash.go",
		"internal/tui/app_footer_guidance.go",
		"internal/tui/app_hosted_panel.go",
		"internal/tui/app_navigation.go",
		"internal/tui/app_navigation_routes.go",
		"internal/tui/app_update_messages.go",
		"internal/tui/app_update_transactions.go",
		"internal/tui/app_view.go",
		"internal/tui/app_view_overlays.go",
		"internal/tui/app_view_status.go",
		"internal/tui/app_view_status_events.go",
		"internal/tui/core_relation_navigation_escape_test.go",
		"internal/tui/core_relation_navigation_helpers_test.go",
		"internal/tui/core_relation_navigation_lifecycle_test.go",
		"internal/tui/cross_machine_barrier_harness_test.go",
		"internal/tui/cross_machine_barrier_test.go",
		"internal/tui/cross_machine_helper_test.go",
		"internal/tui/cross_machine_mutation_helper_test.go",
		"internal/tui/cross_machine_project_comment_barrier_test.go",
		"internal/tui/cross_machine_workspace_helper_test.go",
		"internal/tui/lifecycle_control.go",
		"internal/tui/lifecycle_control_capture.go",
		"internal/tui/lifecycle_control_tasks.go",
		"internal/tui/lifecycle_controller.go",
		"internal/tui/lifecycle_external_reload.go",
		"internal/tui/lifecycle_sync.go",
		"internal/tui/outliner/cross_project.go",
		"internal/tui/outliner/cross_project_cursor.go",
		"internal/tui/outliner/cross_project_projection.go",
		"internal/tui/outliner/cross_project_scope.go",
		"internal/tui/outliner/inline_peek_content.go",
		"internal/tui/outliner/inline_peek_dock.go",
		"internal/tui/outliner/inline_peek_document.go",
		"internal/tui/outliner/inline_peek_fields.go",
		"internal/tui/outliner/inline_peek_paging.go",
		"internal/tui/outliner/inline_preview_content_test.go",
		"internal/tui/outliner/inline_preview_schedule_test.go",
		"internal/tui/outliner/inline_preview_test.go",
		"internal/tui/outliner/menu_bar.go",
		"internal/tui/outliner/menu_bar_input.go",
		"internal/tui/outliner/menu_bar_render.go",
		"internal/tui/outliner/outliner_axis_state.go",
		"internal/tui/outliner/outliner_empty_state.go",
		"internal/tui/outliner/outliner_focus_navigation.go",
		"internal/tui/outliner/outliner_focus_state.go",
		"internal/tui/outliner/outliner_fold_state.go",
		"internal/tui/outliner/outliner_guidance.go",
		"internal/tui/outliner/outliner_jump_history.go",
		"internal/tui/outliner/outliner_keys.go",
		"internal/tui/outliner/outliner_lens.go",
		"internal/tui/outliner/outliner_lens_loading.go",
		"internal/tui/outliner/outliner_mode_input.go",
		"internal/tui/outliner/outliner_mode_sections.go",
		"internal/tui/outliner/outliner_mode_switch.go",
		"internal/tui/outliner/outliner_mode_wiring.go",
		"internal/tui/outliner/outliner_navigation.go",
		"internal/tui/outliner/outliner_navigation_groups.go",
		"internal/tui/outliner/outliner_navigation_tree.go",
		"internal/tui/outliner/outliner_pinned_section.go",
		"internal/tui/outliner/outliner_planning_sections.go",
		"internal/tui/outliner/outliner_population_state.go",
		"internal/tui/outliner/outliner_projection.go",
		"internal/tui/outliner/outliner_projection_groups.go",
		"internal/tui/outliner/outliner_projection_modes.go",
		"internal/tui/outliner/outliner_projection_primary.go",
		"internal/tui/outliner/outliner_query_narrowing.go",
		"internal/tui/outliner/outliner_quick_edit.go",
		"internal/tui/outliner/outliner_relocation.go",
		"internal/tui/outliner/outliner_render.go",
		"internal/tui/outliner/outliner_render_headers.go",
		"internal/tui/outliner/outliner_render_notes.go",
		"internal/tui/outliner/outliner_render_task_indicators.go",
		"internal/tui/outliner/outliner_render_task_row.go",
		"internal/tui/outliner/outliner_render_task_title.go",
		"internal/tui/outliner/outliner_reset.go",
		"internal/tui/outliner/outliner_rows.go",
		"internal/tui/outliner/outliner_rows_rebuild.go",
		"internal/tui/outliner/outliner_scope.go",
		"internal/tui/outliner/outliner_search.go",
		"internal/tui/outliner/outliner_search_background.go",
		"internal/tui/outliner/outliner_search_history.go",
		"internal/tui/outliner/outliner_search_rows.go",
		"internal/tui/outliner/outliner_search_scoring.go",
		"internal/tui/outliner/outliner_search_state.go",
		"internal/tui/outliner/outliner_search_suggestions.go",
		"internal/tui/outliner/outliner_search_vocabulary.go",
		"internal/tui/outliner/outliner_sections.go",
		"internal/tui/outliner/outliner_status_strip.go",
		"internal/tui/outliner/outliner_task_folds.go",
		"internal/tui/outliner/outliner_view_context.go",
		"internal/tui/outliner/outliner_viewport.go",
		"internal/tui/outliner/query_readout.go",
		"internal/tui/outliner/query_readout_view.go",
		"internal/tui/outliner/relation_navigation_baseline_test.go",
		"internal/tui/outliner/relation_navigation_parked_origin_test.go",
		"internal/tui/outliner/relation_navigation_read_failures_test.go",
		"internal/tui/outliner/relation_navigation_reload_outlinersplit_test.go",
		"internal/tui/outliner/shared_render_helpers_test.go",
		"internal/tui/outliner/shared_search_helpers_test.go",
		"internal/tui/outliner/shared_shell_helpers_test.go",
		"internal/tui/outliner/shared_storage_helpers_test.go",
		"internal/tui/query_readout_scroll_test.go",
		"internal/tui/query_readout_test.go",
		"internal/tui/task_operation_clipboard.go",
		"internal/tui/task_operation_delete.go",
		"internal/tui/task_operation_execute.go",
		"internal/tui/task_operation_request.go",
		"internal/tui/taskdetail/comments_actions.go",
		"internal/tui/taskdetail/comments_composer.go",
		"internal/tui/taskdetail/comments_drafts.go",
		"internal/tui/taskdetail/comments_external_editor.go",
		"internal/tui/taskdetail/comments_update.go",
		"internal/tui/taskform/task_composer.go",
		"internal/tui/taskform/task_composer_capture.go",
		"internal/tui/taskform/task_composer_panel_test.go",
		"internal/tui/taskform/task_composer_test.go",
		"internal/tui/taskform/task_composer_text.go",
		"internal/tui/taskform/task_composer_validation_test.go",
		"internal/tui/ux_clarity_review_matrix_test.go",
		"internal/tui/ux_daily_use_baseline_capture_test.go",
		"internal/tui/ux_daily_use_baseline_guidance_test.go",
		"internal/tui/ux_daily_use_baseline_tags_test.go",
		"internal/tui/ux_daily_use_baseline_test.go",
		"internal/tui/ux_daily_use_integrated_acceptance_test.go",
		"internal/tui/ux_daily_use_integrated_layout_test.go",
		"internal/tui/ux_daily_use_integrated_release_test.go",
		"internal/tui/ux_test_helpers_test.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		lines := strings.Count(string(data), "\n")
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		if lines > 500 {
			t.Errorf("%s has %d lines, over the 500-line target; split the responsibility further", name, lines)
		}
	}
}
