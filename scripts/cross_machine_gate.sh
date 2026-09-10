#!/usr/bin/env sh

# cross_machine_gate.sh exercises the hazards a synced workspace exposes that no
# in-process test can reach: two real tt processes with different HOME values
# driving one data directory at the same time, an older binary opening a catalog
# a newer one already advanced, and a workspace an older binary minted being
# migrated and then used at the schema the migration published.
#
# The two-process cases come in two shapes. The `doctor` cases fork the shipped
# binary, because a maintenance command is the whole interaction they need. The
# rename and resume-note cases fork a helper binary compiled from
# `internal/tui`'s test sources instead: the acts they race — freezing a rename
# preview, committing a catalog change under it, writing a resume note — have no
# CLI spelling and are not to gain one, so the second process is test
# infrastructure rather than a new command. Each helper role skips unless
# TT_HELPER_DATA_DIR names a workspace, which is what keeps them inert under an
# ordinary `go test ./...`.
#
# It is deliberately outside `just ci`. It builds binaries, forks processes, and
# in the old-binary cases checks out a git ref, so its runtime and its
# requirements are both unlike the rest of the gate. Run it before releasing a
# change to the storage, transaction, or migration layers.
#
# Usage: scripts/cross_machine_gate.sh
#
# Environment:
#   TT_OLD_BINARY_REF  git ref to build the older binary from. Defaults to the
#                      newest tag reachable from HEAD. The old-binary cases are
#                      reported as skipped, loudly, when neither is available.

set -eu

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/tt-cross-machine.XXXXXX")
cleanup() {
	# A git worktree has to be released through git or it stays registered in
	# .git/worktrees even after its directory is gone.
	if [ -d "$work_dir/old-source" ]; then
		git worktree remove --force "$work_dir/old-source" >/dev/null 2>&1 || true
	fi
	rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

failures=0
skips=0

report_pass() {
	echo "PASS: $1"
}

report_fail() {
	echo "FAIL: $1" >&2
	failures=$((failures + 1))
}

report_skip() {
	echo "SKIP: $1" >&2
	skips=$((skips + 1))
}

# report_same_schema is the verdict for an older binary that turns out to already
# support the schema under test.
#
# Unpinned it is an honest skip: the default ref is the newest tag reachable from
# HEAD, which may well already be newer than the last schema bump, and there is
# then genuinely nothing to refuse or to migrate. Pinned through
# TT_OLD_BINARY_REF it is a failure, because naming a ref is a claim that it
# predates the schema under test — and a case that reported green while proving
# nothing is exactly how a release ships an unexercised migration.
report_same_schema() {
	if [ -n "${TT_OLD_BINARY_REF:-}" ]; then
		report_fail "$1 (TT_OLD_BINARY_REF pins a ref that must predate the current schema)"
		return
	fi
	report_skip "$1"
}

# machine_home_count counts the machine.json files under one simulated home, so a
# test can prove a machine wrote its own machine-local state and only its own.
machine_home_count() {
	find "$1/.tt/runtime" -name machine.json 2>/dev/null | wc -l | tr -d ' '
}

# await_file blocks until another process creates a file, which is the only
# barrier two unrelated processes here share. The bound turns a partner that died
# into a named failure instead of a gate that hangs.
await_file() {
	waited=0
	while [ ! -f "$1" ]; do
		if [ "$waited" -ge 600 ]; then
			return 1
		fi
		sleep 0.1
		waited=$((waited + 1))
	done
	return 0
}

# helper_outcome reads the single line a helper role recorded. An absent file is
# an empty outcome, so a role that died before recording is reported as one.
helper_outcome() {
	if [ -f "$1" ]; then
		tr -d '\n' <"$1"
	fi
}

# mint_workspace creates an empty workspace with the current binary, the way a
# first launch does. It is separate from seeding because several cases want a
# workspace one process minted and a different one filled.
mint_workspace() {
	mkdir -p "$1" "$2"
	HOME="$2" "$current_binary" doctor --data-dir "$1" >"$3" 2>&1
}

# The seed's own vocabulary, mirrored from internal/tui/cross_machine_helper_test.go:
# the project every race is fought over, the document it lives in, and the two
# project comments an edit or a removal addresses. They are spelled here because
# the helper roles take them as input — a role that minted its own target would be
# racing over a write it had just made itself rather than over the seed.
race_project_name="Delivery"
race_document_path="projects/delivery/tasks.md"
seeded_comment_a="Kickoff notes live in the shared drive"
seeded_comment_b="The vendor contract review is booked for the 12th"

# seed_workspace fills a workspace with the two projects and the one shared tag
# every helper role reads. The seed is a helper role rather than a fixture
# because the workspace it fills was minted by a different process.
seed_workspace() {
	TT_HELPER_DATA_DIR="$1" \
		"$helper_binary" -test.run '^TestCrossMachineHelperSeedsWorkspace$' >"$2" 2>&1
}

# task_schema_marker is the one line a tasks.md spells its schema on, which is
# what lets a migration case compare schemas without naming a version number the
# next bump would have to edit here.
task_schema_marker() {
	grep -m1 'tt-schema' "$1/projects/unfiled/tasks.md" 2>/dev/null || true
}

echo "Building the current binary..."
current_binary="$work_dir/tt-current"
go build -o "$current_binary" .

# The helper roles are compiled once and forked repeatedly. HOME is deliberately
# not set for them: internal/tui's TestMain gives every test process its own
# temporary home, so two helpers are always two machines without this script
# arranging it.
echo "Building the helper process binary..."
helper_binary="$work_dir/tt-helpers.test"
go test -c -o "$helper_binary" ./internal/tui

# --- Case 1: two homes over one workspace, one at a time ----------------------
#
# The sequential form first, because it establishes what the concurrent form is
# allowed to disturb: each home owns its own machine.json and the workspace owns
# one lenses.json both machines can read.

case_one() {
	shared="$work_dir/case1/workspace"
	home_a="$work_dir/case1/home-a"
	home_b="$work_dir/case1/home-b"
	mkdir -p "$shared" "$home_a" "$home_b"

	if ! HOME="$home_a" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case1-a.log" 2>&1; then
		report_fail "machine A could not open a fresh workspace (see $work_dir/case1-a.log)"
		return
	fi
	if ! HOME="$home_b" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case1-b.log" 2>&1; then
		report_fail "machine B could not join machine A's workspace (see $work_dir/case1-b.log)"
		return
	fi

	if [ "$(machine_home_count "$home_a")" != "1" ] || [ "$(machine_home_count "$home_b")" != "1" ]; then
		report_fail "each home must hold exactly one machine.json"
		return
	fi
	if [ ! -f "$shared/.tt/lenses.json" ]; then
		report_fail "joining should have published the portable lens home"
		return
	fi
	report_pass "two homes join one workspace and each keeps its own machine state"
}

# --- Case 2: two processes at once -------------------------------------------
#
# This is the case the Go tests cannot express. An in-process test can inject two
# machine homes, but both live behind one flock table: a lock the kernel grants
# per open file description is shared inside a process in ways it is not across
# processes. Only two processes prove the commit lock actually serializes.

case_two() {
	shared="$work_dir/case2/workspace"
	home_a="$work_dir/case2/home-a"
	home_b="$work_dir/case2/home-b"
	mkdir -p "$shared" "$home_a" "$home_b"

	# Mint the workspace first so the race is over an existing catalog rather
	# than over which process gets to create one.
	HOME="$home_a" "$current_binary" doctor --data-dir "$shared" >/dev/null 2>&1 || true

	rounds=6
	round=1
	while [ "$round" -le "$rounds" ]; do
		HOME="$home_a" "$current_binary" doctor --fix --data-dir "$shared" >"$work_dir/case2-a-$round.log" 2>&1 &
		pid_a=$!
		HOME="$home_b" "$current_binary" doctor --fix --data-dir "$shared" >"$work_dir/case2-b-$round.log" 2>&1 &
		pid_b=$!

		status_a=0
		status_b=0
		wait "$pid_a" || status_a=$?
		wait "$pid_b" || status_b=$?

		if [ "$status_a" -ne 0 ] || [ "$status_b" -ne 0 ]; then
			report_fail "concurrent round $round exited $status_a/$status_b (see $work_dir/case2-*-$round.log)"
			return
		fi
		round=$((round + 1))
	done

	# A quarantine is the one outcome that would wedge the workspace for every
	# machine, and it is exactly what an unowned rollforward produces.
	if [ -d "$shared/.tt/transactions/quarantine" ] &&
		[ -n "$(find "$shared/.tt/transactions/quarantine" -name '*.json' 2>/dev/null)" ]; then
		report_fail "concurrent machines quarantined a transaction"
		return
	fi

	# Doctor reads every synced domain and validates the lens collection, so a
	# clean exit here is the assertion that nothing the race wrote is unreadable.
	if ! HOME="$home_a" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case2-verify.log" 2>&1; then
		report_fail "the workspace is no longer clean after the race (see $work_dir/case2-verify.log)"
		return
	fi
	report_pass "two processes with different homes commit concurrently without wedging the workspace"
}

# --- Case 3: an older binary against a newer catalog -------------------------
#
# There is no down-migration, so an older build that opened a newer workspace
# would write a schema it cannot represent back over the newer one. Refusing is
# the contract, and the refusal has to leave the workspace byte-identical.

old_binary_ref() {
	if [ -n "${TT_OLD_BINARY_REF:-}" ]; then
		echo "$TT_OLD_BINARY_REF"
		return 0
	fi
	git describe --tags --abbrev=0 2>/dev/null || return 1
}

# ensure_old_binary builds tt at the older ref once and memoizes the answer, so
# the refusal case and the migration case share one checkout and one build. It
# reports through the two globals below rather than through standard output: a
# command substitution would run it in a subshell, and the memo — like the build
# it stands for — has to survive the call.
old_binary=""
old_binary_state=""
old_binary_ref_used=""
ensure_old_binary() {
	if [ "$old_binary_state" = "ready" ]; then
		return 0
	fi
	if [ -n "$old_binary_state" ]; then
		return 1
	fi
	old_binary_state="unavailable"
	candidate_ref=$(old_binary_ref) || return 1
	if ! git rev-parse --verify "$candidate_ref" >/dev/null 2>&1; then
		old_binary_state="unresolved"
		return 1
	fi
	old_source="$work_dir/old-source"
	if ! git worktree add --detach "$old_source" "$candidate_ref" >"$work_dir/old-worktree.log" 2>&1; then
		old_binary_state="checkout-failed"
		return 1
	fi
	old_binary="$work_dir/tt-old"
	if ! (cd "$old_source" && go build -o "$old_binary" .) >"$work_dir/old-build.log" 2>&1; then
		old_binary_state="build-failed"
		return 1
	fi
	old_binary_state="ready"
	old_binary_ref_used="$candidate_ref"
	return 0
}

# report_missing_old_binary turns the one memoized failure reason into this
# case's verdict, so an absent ref is a loud skip and a broken one is a failure.
report_missing_old_binary() {
	case "$old_binary_state" in
	unresolved)
		report_fail "TT_OLD_BINARY_REF=${TT_OLD_BINARY_REF:-} does not resolve to a git ref"
		;;
	checkout-failed)
		report_fail "could not check out the older ref (see $work_dir/old-worktree.log)"
		;;
	build-failed)
		report_fail "could not build tt at the older ref (see $work_dir/old-build.log)"
		;;
	*)
		report_skip "no git tag to build an older binary from; set TT_OLD_BINARY_REF to a ref older than the current schema"
		;;
	esac
}

case_three() {
	if ! ensure_old_binary; then
		report_missing_old_binary
		return
	fi
	ref="$old_binary_ref_used"

	shared="$work_dir/case3/workspace"
	home_new="$work_dir/case3/home-new"
	home_old="$work_dir/case3/home-old"
	mkdir -p "$shared" "$home_new" "$home_old"

	# The current binary mints and migrates the workspace to today's schema.
	if ! HOME="$home_new" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case3-new.log" 2>&1; then
		report_fail "the current binary could not mint the workspace (see $work_dir/case3-new.log)"
		return
	fi

	before="$work_dir/case3-before.txt"
	after="$work_dir/case3-after.txt"
	snapshot_workspace "$shared" "$before"

	set +e
	HOME="$home_old" "$old_binary" doctor --data-dir "$shared" >"$work_dir/case3-old.log" 2>&1
	old_status=$?
	set -e

	if [ "$old_status" -eq 0 ]; then
		if grep -q "newer than supported" "$work_dir/case3-old.log"; then
			report_fail "the older binary named the newer schema but still exited 0"
			return
		fi
		report_same_schema "tt at $ref already supports the current schema, so there is nothing to refuse"
		return
	fi
	if ! grep -q "newer than supported" "$work_dir/case3-old.log"; then
		report_fail "the older binary failed for some other reason (see $work_dir/case3-old.log)"
		return
	fi

	snapshot_workspace "$shared" "$after"
	if ! diff "$before" "$after" >"$work_dir/case3-diff.log" 2>&1; then
		report_fail "the older binary's refusal modified the workspace (see $work_dir/case3-diff.log)"
		return
	fi
	report_pass "tt at $ref refuses a newer catalog and leaves it byte-identical"
}

# --- Case 4: a catalog change under a rename another process is holding -------
#
# A rename preview freezes a whole-workspace census and the apply commits every
# revision in it as a read-only guard, the catalog included. In one process that
# guard can only be staged by a harness; across two it is the ordinary case —
# machine B creates a project while machine A is still looking at the
# confirmation. Expiring is the contract: a rename written against a workspace
# nobody previewed is exactly what the guard exists to refuse.

case_four() {
	shared="$work_dir/case4/workspace"
	home_mint="$work_dir/case4/home-mint"
	if ! mint_workspace "$shared" "$home_mint" "$work_dir/case4-mint.log"; then
		report_fail "could not mint the workspace (see $work_dir/case4-mint.log)"
		return
	fi
	if ! seed_workspace "$shared" "$work_dir/case4-seed.log"; then
		report_fail "could not seed the workspace (see $work_dir/case4-seed.log)"
		return
	fi

	ready="$work_dir/case4-ready"
	proceed="$work_dir/case4-go"
	holder_result="$work_dir/case4-holder.txt"
	writer_result="$work_dir/case4-writer.txt"

	TT_HELPER_DATA_DIR="$shared" TT_HELPER_READY_FILE="$ready" \
		TT_HELPER_GO_FILE="$proceed" TT_HELPER_RESULT_FILE="$holder_result" \
		"$helper_binary" -test.run '^TestCrossMachineHelperPendingRenameExpiresOnCatalogChange$' \
		>"$work_dir/case4-holder.log" 2>&1 &
	holder_pid=$!
	TT_HELPER_DATA_DIR="$shared" TT_HELPER_READY_FILE="$ready" \
		TT_HELPER_GO_FILE="$proceed" TT_HELPER_RESULT_FILE="$writer_result" \
		"$helper_binary" -test.run '^TestCrossMachineHelperChangesCatalogDuringPendingRename$' \
		>"$work_dir/case4-writer.log" 2>&1 &
	writer_pid=$!

	holder_status=0
	writer_status=0
	wait "$holder_pid" || holder_status=$?
	wait "$writer_pid" || writer_status=$?
	if [ "$holder_status" -ne 0 ] || [ "$writer_status" -ne 0 ]; then
		report_fail "rename race exited $holder_status/$writer_status (see $work_dir/case4-*.log)"
		return
	fi
	if [ "$(helper_outcome "$holder_result")" != "expired" ]; then
		report_fail "the held rename reported '$(helper_outcome "$holder_result")', want expired"
		return
	fi
	if [ "$(helper_outcome "$writer_result")" != "catalog changed" ]; then
		report_fail "the interrupting machine reported '$(helper_outcome "$writer_result")'"
		return
	fi

	# The refusal has to have written nothing, and the workspace it refused in has
	# to still be one a maintenance pass calls clean.
	if grep -rq "effort" "$shared/projects" 2>/dev/null; then
		report_fail "an expired rename still wrote its destination name into a project"
		return
	fi
	if ! HOME="$home_mint" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case4-verify.log" 2>&1; then
		report_fail "the workspace is no longer clean after the rename race (see $work_dir/case4-verify.log)"
		return
	fi
	report_pass "a catalog change in another process expires a held rename and writes nothing"
}

# --- Case 5: two machines writing resume notes into one document --------------
#
# The two tasks share a tasks.md, so this is the lost-update hazard in its
# sharpest form: two processes rewriting one file, each having read it before the
# other committed. Distinct tasks are the point — a merge that resolves them
# keeps both notes, and the failure this catches is one machine's note quietly
# replacing the other's rather than either machine losing a race, which is a
# legitimate outcome and not a defect.
#
# The barrier each helper waits at is inside SetResumeNote, between the snapshot
# it resolves the task from and the commit that writes it, and each side
# publishes the project revision it is holding. Comparing those two revisions is
# what makes this a race and not two sequential writes: a helper released before
# its own snapshot could read a document the other machine had already rewritten,
# and every assertion below would still pass.

case_five() {
	shared="$work_dir/case5/workspace"
	home_mint="$work_dir/case5/home-mint"
	if ! mint_workspace "$shared" "$home_mint" "$work_dir/case5-mint.log"; then
		report_fail "could not mint the workspace (see $work_dir/case5-mint.log)"
		return
	fi
	if ! seed_workspace "$shared" "$work_dir/case5-seed.log"; then
		report_fail "could not seed the workspace (see $work_dir/case5-seed.log)"
		return
	fi

	note_a="picking the migration plan back up"
	note_b="waiting on legal to answer the vendor"
	proceed="$work_dir/case5-go"

	TT_HELPER_DATA_DIR="$shared" TT_HELPER_READY_FILE="$work_dir/case5-ready-a" \
		TT_HELPER_GO_FILE="$proceed" TT_HELPER_RESULT_FILE="$work_dir/case5-a.txt" \
		TT_HELPER_TASK_TITLE="Draft the migration plan" TT_HELPER_RESUME_NOTE="$note_a" \
		"$helper_binary" -test.run '^TestCrossMachineHelperSetsResumeNote$' \
		>"$work_dir/case5-a.log" 2>&1 &
	pid_a=$!
	TT_HELPER_DATA_DIR="$shared" TT_HELPER_READY_FILE="$work_dir/case5-ready-b" \
		TT_HELPER_GO_FILE="$proceed" TT_HELPER_RESULT_FILE="$work_dir/case5-b.txt" \
		TT_HELPER_TASK_TITLE="Review the vendor contract" TT_HELPER_RESUME_NOTE="$note_b" \
		"$helper_binary" -test.run '^TestCrossMachineHelperSetsResumeNote$' \
		>"$work_dir/case5-b.log" 2>&1 &
	pid_b=$!

	# Both sides hold their mutation snapshot at the barrier and commit after it,
	# so the barrier is what makes this a race rather than a sequence. This script
	# is the only party that can see both are there.
	if ! await_file "$work_dir/case5-ready-a" || ! await_file "$work_dir/case5-ready-b"; then
		kill "$pid_a" "$pid_b" >/dev/null 2>&1 || true
		wait "$pid_a" >/dev/null 2>&1 || true
		wait "$pid_b" >/dev/null 2>&1 || true
		report_fail "a resume-note helper never reached the barrier (see $work_dir/case5-*.log)"
		return
	fi

	# Each helper published the project revision its snapshot was taken at. Two
	# different revisions would mean one machine read a document the other had
	# already rewritten, which is the two-sequential-writes case this is not.
	revision_a=$(helper_outcome "$work_dir/case5-ready-a")
	revision_b=$(helper_outcome "$work_dir/case5-ready-b")
	if [ -z "$revision_a" ] || [ "$revision_a" != "$revision_b" ]; then
		kill "$pid_a" "$pid_b" >/dev/null 2>&1 || true
		wait "$pid_a" >/dev/null 2>&1 || true
		wait "$pid_b" >/dev/null 2>&1 || true
		report_fail "the two machines held different base revisions: '$revision_a' / '$revision_b'"
		return
	fi
	echo "go" >"$proceed"

	status_a=0
	status_b=0
	wait "$pid_a" || status_a=$?
	wait "$pid_b" || status_b=$?
	if [ "$status_a" -ne 0 ] || [ "$status_b" -ne 0 ]; then
		report_fail "resume-note race exited $status_a/$status_b (see $work_dir/case5-*.log)"
		return
	fi

	outcome_a=$(helper_outcome "$work_dir/case5-a.txt")
	outcome_b=$(helper_outcome "$work_dir/case5-b.txt")
	case "$outcome_a$outcome_b" in
	*lost*)
		report_fail "a resume-note commit was silently replaced: $outcome_a / $outcome_b"
		return
		;;
	esac
	if [ "$outcome_a" != "committed" ] && [ "$outcome_b" != "committed" ]; then
		report_fail "neither machine committed a resume note: $outcome_a / $outcome_b"
		return
	fi

	# The helpers' own reads can only see the disk as it was when each finished.
	# This runs after both exited, so it is the assertion that a committed note is
	# still there once the other machine is done.
	document="$shared/projects/delivery/tasks.md"
	if [ "$outcome_a" = "committed" ] && ! grep -qF "$note_a" "$document"; then
		report_fail "machine A committed a resume note that is not in $document"
		return
	fi
	if [ "$outcome_b" = "committed" ] && ! grep -qF "$note_b" "$document"; then
		report_fail "machine B committed a resume note that is not in $document"
		return
	fi
	if [ -d "$shared/.tt/transactions/quarantine" ] &&
		[ -n "$(find "$shared/.tt/transactions/quarantine" -name '*.json' 2>/dev/null)" ]; then
		report_fail "concurrent resume notes quarantined a transaction"
		return
	fi
	if ! HOME="$home_mint" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case5-verify.log" 2>&1; then
		report_fail "the workspace is no longer clean after concurrent resume notes (see $work_dir/case5-verify.log)"
		return
	fi
	report_pass "two machines write resume notes into one document without losing either ($outcome_a / $outcome_b)"
}

# --- Case 6: a workspace an older binary minted, migrated and then used -------
#
# The in-process migration tests build their input; this one is handed one. An
# older binary mints the workspace at the schema it knows, the current binary
# migrates it, and a helper then writes the field the migration admitted and
# reads the block back out of the bytes. That last step is what separates
# "migrated" from "readable": a workspace that opens but cannot carry the new
# field would pass every check the refusal case makes.

case_six() {
	if ! ensure_old_binary; then
		report_missing_old_binary
		return
	fi
	ref="$old_binary_ref_used"

	shared="$work_dir/case6/workspace"
	home_old="$work_dir/case6/home-old"
	home_new="$work_dir/case6/home-new"
	mkdir -p "$shared" "$home_old" "$home_new"

	if ! HOME="$home_old" "$old_binary" doctor --data-dir "$shared" >"$work_dir/case6-old.log" 2>&1; then
		report_fail "tt at $ref could not mint a workspace of its own (see $work_dir/case6-old.log)"
		return
	fi
	minted_marker=$(task_schema_marker "$shared")
	if [ -z "$minted_marker" ]; then
		report_fail "tt at $ref minted no readable task document"
		return
	fi

	# What the current binary writes into a workspace of its own is the schema the
	# migration has to land on, so no version number is spelled here.
	reference="$work_dir/case6/reference"
	if ! mint_workspace "$reference" "$work_dir/case6/home-reference" "$work_dir/case6-reference.log"; then
		report_fail "could not mint a current reference workspace (see $work_dir/case6-reference.log)"
		return
	fi
	current_marker=$(task_schema_marker "$reference")
	if [ "$minted_marker" = "$current_marker" ]; then
		report_same_schema "tt at $ref already writes the current task schema, so there is nothing to migrate"
		return
	fi

	if ! HOME="$home_new" "$current_binary" doctor --data-dir "$shared" >"$work_dir/case6-migrate.log" 2>&1; then
		report_fail "the current binary could not migrate the older workspace (see $work_dir/case6-migrate.log)"
		return
	fi
	migrated_marker=$(task_schema_marker "$shared")
	if [ "$migrated_marker" != "$current_marker" ]; then
		report_fail "migrated task schema is '$migrated_marker', want '$current_marker'"
		return
	fi

	# A migration publishes an immutable archive/audit pair before it activates,
	# and the pair is the only durable evidence the boundary was crossed properly.
	if [ -z "$(find "$shared/.tt/backups" -name '*.tar.gz' 2>/dev/null)" ] ||
		[ -z "$(find "$shared/.tt/backups" -name '*.audit.md' 2>/dev/null)" ]; then
		report_fail "the migration published no archive/audit pair under $shared/.tt/backups"
		return
	fi

	if ! seed_workspace "$shared" "$work_dir/case6-seed.log"; then
		report_fail "could not seed the migrated workspace (see $work_dir/case6-seed.log)"
		return
	fi
	note="resumed on a workspace an older binary minted"
	# The comment is written by the same helper run, because the two fields the
	# migration admitted arrive at one document and a workspace that carried one
	# but refused the other would still pass a run that only wrote the note.
	comment="commented on a workspace an older binary minted"
	if ! TT_HELPER_DATA_DIR="$shared" TT_HELPER_RESULT_FILE="$work_dir/case6-use.txt" \
		TT_HELPER_TASK_TITLE="Draft the migration plan" TT_HELPER_RESUME_NOTE="$note" \
		TT_HELPER_TEXT="$comment" \
		"$helper_binary" -test.run '^TestCrossMachineHelperUsesMigratedWorkspace$' \
		>"$work_dir/case6-use.log" 2>&1; then
		report_fail "the migrated workspace could not carry a resume note and a project comment (see $work_dir/case6-use.log)"
		return
	fi
	migrated_document="$shared/$race_document_path"
	if ! grep -qF "$note" "$migrated_document"; then
		report_fail "the resume note written to the migrated workspace is not on disk"
		return
	fi
	# The helper already reread its comment through the service; this reads the
	# same claim off the bytes, so a comment held only in memory cannot pass.
	require_comment_occurrences case6 "$migrated_document" "$comment" 1 || return
	# Three: the two the seed wrote after the migration, plus the one the helper
	# added and read back. Counting them all is what proves the block the upgrade
	# admitted accumulates rather than being rewritten to hold the newest entry.
	require_stored_comment_count case6 "$migrated_document" 3 || return

	# And the workspace the migration produced is one the older binary now refuses,
	# which is the refusal case's contract asserted where the schema actually moved.
	set +e
	HOME="$home_old" "$old_binary" doctor --data-dir "$shared" >"$work_dir/case6-old-again.log" 2>&1
	old_status=$?
	set -e
	if [ "$old_status" -eq 0 ] || ! grep -q "newer than supported" "$work_dir/case6-old-again.log"; then
		report_fail "tt at $ref did not refuse the workspace it minted and this binary migrated (see $work_dir/case6-old-again.log)"
		return
	fi
	report_pass "a workspace minted at $ref migrates, carries a resume note and project comments, and is then refused by $ref"
}

# --- Case 7: two machines applying the release's new mutations at once --------
#
# Case 5 proved one write into one shared document. This one proves the shape
# generalizes to the three writes the release added a key for, because the hazard
# is a property of the document, not of the field: two machines that resolved the
# same base revision both rewrite the whole tasks.md, so whichever commits second
# is merging over bytes it never read.
#
# Creation is the sharpest of the three and the reason the case is not folded
# into case 5. The other two rewrite a task both machines can name; two creations
# mint two identities against a document that named neither, so a merge that
# silently dropped one would leave the losing machine with no evidence at all.

# race_two_helpers forks one helper role twice, holds both at the commit barrier,
# releases them together, and reports the pair of outcomes on stdout. It is the
# whole of what makes these cases races, so it is written once: a case that
# arranged its own barrier could differ from case 5's in ways nobody would see
# until a lost update was blamed on the wrong layer.
#
# It echoes "<outcome_a>|<outcome_b>" on success and nothing on failure, having
# already reported the failure itself.
start_race_helper() {
	side=$1
	role=$2
	title=$3
	text=$4
	target=$5
	extra=$6
	shift 6
	# The remaining arguments are extra environment assignments applied to both
	# roles; $extra is this side's own, prepended so a mixed race can hand each
	# machine a different selector. Passing them through `env` rather than inline
	# keeps one spelling for a role that takes no extras and a role that takes
	# several, and an empty $extra contributes no argument at all — `env` would
	# read one as a program name.
	if [ -n "$extra" ]; then
		set -- "$extra" "$@"
	fi
	TT_HELPER_DATA_DIR="$shared" TT_HELPER_READY_FILE="$work_dir/$tag-ready-$side" \
		TT_HELPER_GO_FILE="$work_dir/$tag-go" TT_HELPER_RESULT_FILE="$work_dir/$tag-$side.txt" \
		TT_HELPER_TASK_TITLE="$title" TT_HELPER_PROJECT="$helper_project" \
		env "$@" TT_HELPER_TEXT="$text" TT_HELPER_TARGET_TEXT="$target" \
		"$helper_binary" -test.run "^$role\$" >"$work_dir/$tag-$side.log" 2>&1 &
}

# race_two_roles is the general form: two named roles over one document, each
# with its own title, text and comment target. Two different roles are not a
# variation on the shape but the only way to reach one hazard — a project comment
# and a task edit rewriting one tasks.md — so the fork is written once and the
# same-role spelling below is what every earlier case still calls.
race_two_roles() {
	tag=$1
	role_a=$2
	role_b=$3
	title_a=$4
	title_b=$5
	shift 5
	proceed="$work_dir/$tag-go"

	start_race_helper a "$role_a" "$title_a" "$helper_text_a" "$helper_target_a" "$helper_extra_a" "$@"
	pid_a=$!
	start_race_helper b "$role_b" "$title_b" "$helper_text_b" "$helper_target_b" "$helper_extra_b" "$@"
	pid_b=$!

	if ! await_file "$work_dir/$tag-ready-a" || ! await_file "$work_dir/$tag-ready-b"; then
		kill "$pid_a" "$pid_b" >/dev/null 2>&1 || true
		wait "$pid_a" >/dev/null 2>&1 || true
		wait "$pid_b" >/dev/null 2>&1 || true
		report_fail "$tag: a helper never reached the barrier (see $work_dir/$tag-*.log)"
		return 1
	fi

	revision_a=$(helper_outcome "$work_dir/$tag-ready-a")
	revision_b=$(helper_outcome "$work_dir/$tag-ready-b")
	if [ -z "$revision_a" ] || [ "$revision_a" != "$revision_b" ]; then
		kill "$pid_a" "$pid_b" >/dev/null 2>&1 || true
		wait "$pid_a" >/dev/null 2>&1 || true
		wait "$pid_b" >/dev/null 2>&1 || true
		report_fail "$tag: the two machines held different base revisions: '$revision_a' / '$revision_b'"
		return 1
	fi
	echo "go" >"$proceed"

	status_a=0
	status_b=0
	wait "$pid_a" || status_a=$?
	wait "$pid_b" || status_b=$?
	if [ "$status_a" -ne 0 ] || [ "$status_b" -ne 0 ]; then
		report_fail "$tag: the race exited $status_a/$status_b (see $work_dir/$tag-*.log)"
		return 1
	fi

	outcome_a=$(helper_outcome "$work_dir/$tag-a.txt")
	outcome_b=$(helper_outcome "$work_dir/$tag-b.txt")
	case "$outcome_a$outcome_b" in
	*lost*)
		report_fail "$tag: a commit was silently replaced: $outcome_a / $outcome_b"
		return 1
		;;
	esac
	if [ "$outcome_a" != "committed" ] && [ "$outcome_b" != "committed" ]; then
		report_fail "$tag: neither machine committed: $outcome_a / $outcome_b"
		return 1
	fi
	echo "$outcome_a|$outcome_b"
	return 0
}

# race_two_helpers is the same-role spelling: one role forked twice.
race_two_helpers() {
	tag=$1
	role=$2
	shift 2
	race_two_roles "$tag" "$role" "$role" "$@"
}

# require_clean_workspace is the shared tail of every race case: nothing
# quarantined, and the shipped maintenance command still calls the workspace
# healthy. A race that merged incorrectly but left plausible bytes is exactly
# what doctor is for.
require_clean_workspace() {
	tag=$1
	if [ -d "$shared/.tt/transactions/quarantine" ] &&
		[ -n "$(find "$shared/.tt/transactions/quarantine" -name '*.json' 2>/dev/null)" ]; then
		report_fail "$tag: the race quarantined a transaction"
		return 1
	fi
	if ! HOME="$home_mint" "$current_binary" doctor --data-dir "$shared" >"$work_dir/$tag-verify.log" 2>&1; then
		report_fail "$tag: the workspace is no longer clean (see $work_dir/$tag-verify.log)"
		return 1
	fi
	return 0
}

# prepare_race_workspace mints and seeds a workspace of its own for one race, so
# a case that leaves a document merged cannot change what the next one races
# over. It sets $shared and $home_mint for the helpers that follow.
prepare_race_workspace() {
	tag=$1
	shared="$work_dir/$tag/workspace"
	home_mint="$work_dir/$tag/home-mint"
	# The per-side race inputs are reset here rather than defaulted at the fork,
	# so a case that does not set one is racing over the seed rather than over
	# whatever the previous case left in the variable.
	helper_project="$race_project_name"
	helper_target_a=""
	helper_target_b=""
	helper_extra_a=""
	helper_extra_b=""
	if ! mint_workspace "$shared" "$home_mint" "$work_dir/$tag-mint.log"; then
		report_fail "$tag: could not mint the workspace (see $work_dir/$tag-mint.log)"
		return 1
	fi
	if ! seed_workspace "$shared" "$work_dir/$tag-seed.log"; then
		report_fail "$tag: could not seed the workspace (see $work_dir/$tag-seed.log)"
		return 1
	fi
	return 0
}

case_seven_comments() {
	tag=case7-comment
	prepare_race_workspace "$tag" || return
	helper_text_a="Machine A saw this in the migration review"
	helper_text_b="Machine B heard back from the vendor"

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesTaskMutation \
		"Draft the migration plan" "Review the vendor contract" \
		TT_HELPER_MUTATION=comment) || return

	document="$shared/projects/delivery/tasks.md"
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	if [ "$outcome_a" = "committed" ] && ! grep -qF "$helper_text_a" "$document"; then
		report_fail "$tag: machine A committed a comment that is not in $document"
		return
	fi
	if [ "$outcome_b" = "committed" ] && ! grep -qF "$helper_text_b" "$document"; then
		report_fail "$tag: machine B committed a comment that is not in $document"
		return
	fi
	require_clean_workspace "$tag" || return
	report_pass "two machines comment on one document without losing either ($outcome_a / $outcome_b)"
}

case_seven_reviews() {
	tag=case7-review
	prepare_race_workspace "$tag" || return
	# Acknowledgement takes no text; the seed flags both tasks, so each machine's
	# write is the clearing of one flag and the claim is the key's absence.
	helper_text_a=""
	helper_text_b=""

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesTaskMutation \
		"Draft the migration plan" "Review the vendor contract" \
		TT_HELPER_MUTATION=review) || return

	document="$shared/projects/delivery/tasks.md"
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	flagged=$(grep -c 'review: true' "$document" || true)
	committed=0
	[ "$outcome_a" = "committed" ] && committed=$((committed + 1))
	[ "$outcome_b" = "committed" ] && committed=$((committed + 1))
	if [ "$flagged" -ne $((2 - committed)) ]; then
		report_fail "$tag: $flagged task(s) still flagged after $committed acknowledgement(s)"
		return
	fi
	require_clean_workspace "$tag" || return
	report_pass "two machines acknowledge reviews in one document without restoring either flag ($outcome_a / $outcome_b)"
}

# case_seven_creations is the one race with a guaranteed loser, which is why it
# gets a case of its own rather than a third pass through the comment shape.
#
# Both machines name the same anchor task, so both creations are minted against
# one document that named neither, and each takes the same next-free short id. The
# second commit's three-way merge is therefore textually clean — two additions
# that overlap nowhere — and the whole it produces is invalid, because a short id
# is the reference every `#N` spelling resolves through and no project may hold
# two tasks answering to one. The contract is that this is refused as a recorded
# conflict rather than as a rendering failure with nothing to show, and that the
# retry, reading what the winner left, completes the losing machine's creation.
case_seven_creations() {
	tag=case7-create
	prepare_race_workspace "$tag" || return
	helper_text_a="Draft the rollout announcement"
	helper_text_b="Book the contract review slot"

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesTaskMutation \
		"Draft the migration plan" "Draft the migration plan" \
		TT_HELPER_MUTATION=create) || return

	document="$shared/projects/delivery/tasks.md"
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	case "$outcome_a$outcome_b" in
	*retried:*) ;;
	*)
		report_fail "$tag: no creation was refused and retried: $outcome_a / $outcome_b"
		return
		;;
	esac
	if ! grep -qF "$helper_text_a" "$document" || ! grep -qF "$helper_text_b" "$document"; then
		report_fail "$tag: a creation is missing from $document"
		return
	fi
	# Read off the bytes rather than from either machine's report: a merge that
	# kept both tasks and both numbers would satisfy every claim above.
	duplicates=$(grep -o 'short_id: [0-9]*' "$document" | sort | uniq -d)
	if [ -n "$duplicates" ]; then
		report_fail "$tag: two tasks answer to one short id after the race ($duplicates)"
		return
	fi
	# The refusal has to have left something for `tt conflict` to list. That is the
	# whole difference between this classification and the bare error it replaced.
	if [ -z "$(find "$shared/.tt/conflicts" -name '*.json' 2>/dev/null)" ]; then
		report_fail "$tag: a creation was refused as a conflict but recorded no bundle"
		return
	fi
	require_clean_workspace "$tag" || return
	report_pass "two machines create into one document; the refused one is recorded and completes on retry ($outcome_a / $outcome_b)"
}

# --- Case 8: two machines commenting on one project ---------------------------
#
# A project comment lives in the same tasks.md a task does, so it carries the
# same lost-update hazard — but nothing about how it is reconciled follows from
# case 7 passing. A task is merged field by field against a heading the document
# already names; a comment collection is merged by minted identity against a
# block that may not exist yet, and the five ways two machines can collide over
# one collection resolve by five different rules.
#
# So there are five cases. Two additions and two edits to different comments must
# all land, because the identities differ and independent writes never collide.
# Two rewrites of one comment, and a rewrite against a deletion, must each be
# refused whole with the divergence recorded against the comment rather than
# against a task nobody touched. And a project comment racing a task comment is
# the one hazard neither axis can reach alone: two machines rewriting one
# document along two axes that have to reconcile independently.

# require_comment_occurrences asserts a text appears exactly the expected number
# of times in one document. "Exactly once" is the claim an identity-keyed merge
# owes and a presence check cannot make: a merge that kept one machine's addition
# twice would satisfy every other assertion here.
require_comment_occurrences() {
	occ_tag=$1
	occ_document=$2
	occ_text=$3
	occ_want=$4
	occ_found=$(grep -cF "$occ_text" "$occ_document" 2>/dev/null || true)
	[ -n "$occ_found" ] || occ_found=0
	if [ "$occ_found" -ne "$occ_want" ]; then
		report_fail "$occ_tag: '$occ_text' appears $occ_found time(s) in $occ_document, want $occ_want"
		return 1
	fi
	return 0
}

# require_stored_comment_count counts the block's per-entry header lines, which
# is how many comments the document holds regardless of what any of them say.
require_stored_comment_count() {
	cnt_tag=$1
	cnt_document=$2
	cnt_want=$3
	cnt_found=$(grep -c '<!-- project-comment: ' "$cnt_document" 2>/dev/null || true)
	[ -n "$cnt_found" ] || cnt_found=0
	if [ "$cnt_found" -ne "$cnt_want" ]; then
		report_fail "$cnt_tag: $cnt_document holds $cnt_found project comment(s), want $cnt_want"
		return 1
	fi
	return 0
}

# require_recorded_comment_conflict proves a refusal left evidence and that the
# evidence names the comment. A refused commit that recorded nothing would leave
# `tt conflict` with nothing to list, and one recorded as a task conflict would
# have it naming a task title nobody edited.
require_recorded_comment_conflict() {
	cf_tag=$1
	cf_field=$2
	if [ -z "$(find "$shared/.tt/conflicts" -name '*.json' 2>/dev/null)" ]; then
		report_fail "$cf_tag: a refused project comment recorded no conflict bundle"
		return 1
	fi
	if ! grep -rq "$cf_field" "$shared/.tt/conflicts" 2>/dev/null; then
		report_fail "$cf_tag: the recorded bundle does not name a $cf_field divergence"
		return 1
	fi
	return 0
}

# require_one_refused_comment is the outcome shape of the two races the merge
# cannot resolve, and it sets $landed_text and $refused_text so the caller can
# read the document without knowing which machine won.
require_one_refused_comment() {
	ref_tag=$1
	ref_a=$2
	ref_b=$3
	case "$ref_a$ref_b" in
	*conflicted:*) ;;
	*)
		report_fail "$ref_tag: a collision over one comment recorded no conflict: $ref_a / $ref_b"
		return 1
		;;
	esac
	if [ "$ref_a" = "committed" ]; then
		landed_text=$helper_text_a
		refused_text=$helper_text_b
	elif [ "$ref_b" = "committed" ]; then
		landed_text=$helper_text_b
		refused_text=$helper_text_a
	else
		report_fail "$ref_tag: neither machine committed: $ref_a / $ref_b"
		return 1
	fi
	return 0
}

case_eight_additions() {
	tag=case8-add
	prepare_race_workspace "$tag" || return
	helper_extra_a=TT_HELPER_PROJECT_COMMENT=add
	helper_extra_b=TT_HELPER_PROJECT_COMMENT=add
	helper_text_a="Machine A logged the migration review"
	helper_text_b="Machine B heard back from the vendor"

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesProjectCommentMutation "" "") || return
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	if [ "$outcome_a" != "committed" ] || [ "$outcome_b" != "committed" ]; then
		report_fail "$tag: independent additions must both land: $outcome_a / $outcome_b"
		return
	fi

	document="$shared/$race_document_path"
	require_comment_occurrences "$tag" "$document" "$helper_text_a" 1 || return
	require_comment_occurrences "$tag" "$document" "$helper_text_b" 1 || return
	require_stored_comment_count "$tag" "$document" 4 || return
	require_clean_workspace "$tag" || return
	report_pass "two machines add project comments and each survives exactly once ($outcome_a / $outcome_b)"
}

case_eight_distinct_edits() {
	tag=case8-edit
	prepare_race_workspace "$tag" || return
	helper_extra_a=TT_HELPER_PROJECT_COMMENT=edit
	helper_extra_b=TT_HELPER_PROJECT_COMMENT=edit
	helper_target_a="$seeded_comment_a"
	helper_target_b="$seeded_comment_b"
	helper_text_a="Kickoff notes moved to the team wiki"
	helper_text_b="The contract review slipped to the 19th"

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesProjectCommentMutation "" "") || return
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	if [ "$outcome_a" != "committed" ] || [ "$outcome_b" != "committed" ]; then
		report_fail "$tag: edits to different comments must both land: $outcome_a / $outcome_b"
		return
	fi

	document="$shared/$race_document_path"
	require_comment_occurrences "$tag" "$document" "$helper_text_a" 1 || return
	require_comment_occurrences "$tag" "$document" "$helper_text_b" 1 || return
	require_comment_occurrences "$tag" "$document" "$seeded_comment_a" 0 || return
	require_comment_occurrences "$tag" "$document" "$seeded_comment_b" 0 || return
	require_stored_comment_count "$tag" "$document" 2 || return
	require_clean_workspace "$tag" || return
	report_pass "two machines edit different project comments and both edits land ($outcome_a / $outcome_b)"
}

case_eight_rival_edits() {
	tag=case8-rival
	prepare_race_workspace "$tag" || return
	helper_extra_a=TT_HELPER_PROJECT_COMMENT=edit
	helper_extra_b=TT_HELPER_PROJECT_COMMENT=edit
	helper_target_a="$seeded_comment_a"
	helper_target_b="$seeded_comment_a"
	helper_text_a="Kickoff notes moved to the team wiki"
	helper_text_b="Kickoff notes are in the shared drive still"

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesProjectCommentMutation "" "") || return
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	require_one_refused_comment "$tag" "$outcome_a" "$outcome_b" || return

	document="$shared/$race_document_path"
	require_comment_occurrences "$tag" "$document" "$landed_text" 1 || return
	require_comment_occurrences "$tag" "$document" "$refused_text" 0 || return
	require_stored_comment_count "$tag" "$document" 2 || return
	require_recorded_comment_conflict "$tag" '"comment.text"' || return
	require_clean_workspace "$tag" || return
	report_pass "two rewrites of one project comment leave the winner's text and a recorded conflict ($outcome_a / $outcome_b)"
}

case_eight_edit_versus_delete() {
	tag=case8-delete
	prepare_race_workspace "$tag" || return
	helper_extra_a=TT_HELPER_PROJECT_COMMENT=edit
	helper_extra_b=TT_HELPER_PROJECT_COMMENT=remove
	helper_target_a="$seeded_comment_a"
	helper_target_b="$seeded_comment_a"
	helper_text_a="Kickoff notes moved to the team wiki"
	helper_text_b=""

	outcomes=$(race_two_helpers "$tag" TestCrossMachineHelperAppliesProjectCommentMutation "" "") || return
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	case "$outcome_a$outcome_b" in
	*conflicted:*) ;;
	*)
		report_fail "$tag: an edit against a deletion recorded no conflict: $outcome_a / $outcome_b"
		return
		;;
	esac

	# Both survivors are legitimate and they leave different documents, so the
	# assertion is per winner: the edit landing keeps two comments and its own new
	# text, the deletion landing keeps one and neither machine's text.
	document="$shared/$race_document_path"
	if [ "$outcome_a" = "committed" ]; then
		require_comment_occurrences "$tag" "$document" "$helper_text_a" 1 || return
		require_stored_comment_count "$tag" "$document" 2 || return
	else
		require_comment_occurrences "$tag" "$document" "$helper_text_a" 0 || return
		require_comment_occurrences "$tag" "$document" "$seeded_comment_a" 0 || return
		require_stored_comment_count "$tag" "$document" 1 || return
	fi
	require_recorded_comment_conflict "$tag" '"comment"' || return
	require_clean_workspace "$tag" || return
	report_pass "an edit and a deletion of one project comment settle on one winner and a recorded conflict ($outcome_a / $outcome_b)"
}

# case_eight_comment_versus_task is the mixed race, and the only one here that
# forks two different roles. The comment collection and the task list are merged
# by separate rules inside one document, so neither write may cost the other —
# and a single-axis race, however many times it is repeated, cannot say so.
case_eight_comment_versus_task() {
	tag=case8-mixed
	prepare_race_workspace "$tag" || return
	helper_extra_a=TT_HELPER_PROJECT_COMMENT=add
	helper_extra_b=TT_HELPER_MUTATION=comment
	helper_text_a="Machine A logged the migration review"
	helper_text_b="Machine B checked this against the vendor thread"

	outcomes=$(race_two_roles "$tag" \
		TestCrossMachineHelperAppliesProjectCommentMutation \
		TestCrossMachineHelperAppliesTaskMutation \
		"" "Review the vendor contract") || return
	outcome_a=${outcomes%%|*}
	outcome_b=${outcomes##*|}
	if [ "$outcome_a" != "committed" ] || [ "$outcome_b" != "committed" ]; then
		report_fail "$tag: a project comment and a task comment must both land: $outcome_a / $outcome_b"
		return
	fi

	document="$shared/$race_document_path"
	require_comment_occurrences "$tag" "$document" "$helper_text_a" 1 || return
	require_comment_occurrences "$tag" "$document" "$helper_text_b" 1 || return
	require_stored_comment_count "$tag" "$document" 3 || return
	require_clean_workspace "$tag" || return
	report_pass "a project comment and a task comment land together in one document ($outcome_a / $outcome_b)"
}

# snapshot_workspace writes a sorted path/size/checksum listing of every file
# under a workspace, which is what "byte-identical" is checked against.
snapshot_workspace() {
	root=$1
	out=$2
	(cd "$root" && find . -type f -exec cksum {} \; | sort) >"$out"
}

case_one
case_two
case_three
case_four
case_five
case_six
case_seven_comments
case_seven_reviews
case_seven_creations
case_eight_additions
case_eight_distinct_edits
case_eight_rival_edits
case_eight_edit_versus_delete
case_eight_comment_versus_task

echo
if [ "$failures" -gt 0 ]; then
	echo "cross-machine gate failed: $failures case(s)" >&2
	exit 1
fi
if [ "$skips" -gt 0 ]; then
	echo "cross-machine gate passed with $skips skipped case(s)."
	exit 0
fi
echo "cross-machine gate passed."
