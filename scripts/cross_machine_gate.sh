#!/usr/bin/env sh

# cross_machine_gate.sh exercises the hazards a synced workspace exposes that no
# in-process test can reach: two real tt processes with different HOME values
# driving one data directory at the same time, and an older binary opening a
# catalog a newer one already advanced.
#
# It is deliberately outside `just ci`. It builds binaries, forks processes, and
# in the old-binary case checks out a git tag, so its runtime and its
# requirements are both unlike the rest of the gate. Run it before releasing a
# change to the storage, transaction, or migration layers.
#
# Usage: scripts/cross_machine_gate.sh
#
# Environment:
#   TT_OLD_BINARY_REF  git ref to build the older binary from. Defaults to the
#                      newest tag reachable from HEAD. The old-binary case is
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

echo "Building the current binary..."
current_binary="$work_dir/tt-current"
go build -o "$current_binary" .

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

case_three() {
	ref=$(old_binary_ref) || {
		report_skip "no git tag to build an older binary from; set TT_OLD_BINARY_REF to a ref older than the current schema"
		return
	}
	if ! git rev-parse --verify "$ref" >/dev/null 2>&1; then
		report_fail "TT_OLD_BINARY_REF=$ref does not resolve to a git ref"
		return
	fi

	old_source="$work_dir/old-source"
	if ! git worktree add --detach "$old_source" "$ref" >"$work_dir/case3-worktree.log" 2>&1; then
		report_fail "could not check out $ref (see $work_dir/case3-worktree.log)"
		return
	fi
	old_binary="$work_dir/tt-old"
	if ! (cd "$old_source" && go build -o "$old_binary" .) >"$work_dir/case3-build.log" 2>&1; then
		report_fail "could not build tt at $ref (see $work_dir/case3-build.log)"
		return
	fi

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
