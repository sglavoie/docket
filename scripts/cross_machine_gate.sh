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
		report_skip "tt at $ref already supports the current schema, so there is nothing to refuse"
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
		report_skip "tt at $ref already writes the current task schema, so there is nothing to migrate"
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
	if ! TT_HELPER_DATA_DIR="$shared" TT_HELPER_RESULT_FILE="$work_dir/case6-use.txt" \
		TT_HELPER_TASK_TITLE="Draft the migration plan" TT_HELPER_RESUME_NOTE="$note" \
		"$helper_binary" -test.run '^TestCrossMachineHelperUsesMigratedWorkspace$' \
		>"$work_dir/case6-use.log" 2>&1; then
		report_fail "the migrated workspace could not carry a resume note (see $work_dir/case6-use.log)"
		return
	fi
	if ! grep -qF "$note" "$shared/projects/delivery/tasks.md"; then
		report_fail "the resume note written to the migrated workspace is not on disk"
		return
	fi

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
	report_pass "a workspace minted at $ref migrates, carries a resume note, and is then refused by $ref"
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
