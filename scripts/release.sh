#!/usr/bin/env sh
#
# Cut a release: tag the next version of the commit main is at and push the tag,
# which starts .github/workflows/release.yml (CI, then GoReleaser's CLI archives,
# then the notarized Docket.app and the Homebrew cask). It then follows that run
# and confirms the tap's cask moved to the new version.
#
# Refuses unless this is a clean main identical to origin/main, the commit is
# new since the last release, and CI passed on it: the release workflow reruns
# CI anyway, but a tag pushed over a red commit publishes nothing and has to be
# deleted by hand.
#
# Usage: release.sh patch|minor|major [--yes] [--no-watch]
#
#   --yes        tag without asking
#   --no-watch   push the tag and return without following the release

set -eu

usage() {
	echo "usage: release.sh patch|minor|major [--yes] [--no-watch]" >&2
	exit 2
}

bump=
assume_yes=
watch=1
for arg in "$@"; do
	case "$arg" in
	patch | minor | major)
		[ -z "$bump" ] || usage
		bump=$arg
		;;
	--yes) assume_yes=1 ;;
	--no-watch) watch= ;;
	*) usage ;;
	esac
done
[ -n "$bump" ] || usage

for tool in git gh; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "required tool not found: $tool" >&2
		exit 1
	fi
done

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

fail() {
	echo "release: $*" >&2
	exit 1
}

# --- Is this a commit that may be released? ---------------------------------------

branch=$(git symbolic-ref --quiet --short HEAD || true)
[ "$branch" = main ] || fail "releases are cut from main, not ${branch:-a detached HEAD}"
[ -z "$(git status --porcelain)" ] || fail "the working tree has uncommitted or untracked changes"

git fetch --quiet --tags origin main
head=$(git rev-parse HEAD)
[ "$head" = "$(git rev-parse origin/main)" ] ||
	fail "main is not origin/main: push or pull first, since the tag must name a commit CI has seen"

# Only plain vX.Y.Z tags count: a prerelease never sets the base of the next bump.
last=$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1 || true)
[ -n "$last" ] || fail "no vX.Y.Z tag to bump from"
[ "$(git rev-list --count "$last..HEAD")" -gt 0 ] || fail "nothing to release: HEAD is $last"

ci=$(gh run list --workflow CI --commit "$head" --limit 1 --json status,conclusion,url \
	--jq '.[0] | "\(.status) \(.conclusion) \(.url)"')
case "$ci" in
"completed success "*) ;;
"")
	fail "CI has not run on $(git rev-parse --short HEAD)"
	;;
"completed "*)
	fail "CI did not pass on $(git rev-parse --short HEAD): ${ci##* }"
	;;
*)
	fail "CI is still running on $(git rev-parse --short HEAD); wait with: gh run watch ${ci##*/}"
	;;
esac

# --- The next version -------------------------------------------------------------

version=${last#v}
major=${version%%.*}
rest=${version#*.}
minor=${rest%%.*}
patch=${rest#*.}
case "$bump" in
major) next="v$((major + 1)).0.0" ;;
minor) next="v$major.$((minor + 1)).0" ;;
patch) next="v$major.$minor.$((patch + 1))" ;;
esac
if git rev-parse --quiet --verify "refs/tags/$next" >/dev/null ||
	[ -n "$(git ls-remote --tags origin "refs/tags/$next")" ]; then
	fail "$next already exists"
fi

echo "Release $next ($bump after $last) at $(git rev-parse --short HEAD):"
git log --oneline --no-decorate "$last..HEAD" | sed 's/^/  /'
echo

if [ -z "$assume_yes" ]; then
	[ -t 0 ] || fail "not a terminal: pass --yes to tag without asking"
	printf 'Tag and push %s? This publishes it. [y/N] ' "$next"
	read -r answer || answer=
	case "$answer" in
	[yY] | [yY][eE][sS]) ;;
	*)
		echo "Nothing tagged."
		exit 1
		;;
	esac
fi

# --- Tag, push, follow ------------------------------------------------------------

git tag -a "$next" -m "$next"
if ! git push --quiet origin "$next"; then
	git tag -d "$next" >/dev/null
	fail "pushing $next failed; the local tag was removed"
fi
echo "Pushed $next."

# The run appears a few seconds after the push; a tag push names the tag as its branch.
run=
tries=0
while [ -z "$run" ] && [ "$tries" -lt 30 ]; do
	sleep 2
	tries=$((tries + 1))
	run=$(gh run list --workflow Release --branch "$next" --limit 1 --json databaseId --jq '.[0].databaseId // empty')
done
if [ -z "$run" ]; then
	echo "The Release run has not appeared yet; find it with: gh run list --workflow Release" >&2
	exit 0
fi
url=$(gh run view "$run" --json url --jq .url)
echo "Release run: $url"
if [ -z "$watch" ]; then
	exit 0
fi

echo "Following it (Ctrl+C stops following; the release carries on)..."
if ! gh run watch "$run" --exit-status --interval 30 >/dev/null; then
	fail "the Release run failed: $url (rerun a failed job with: gh run rerun $run --failed)"
fi

cask_version=$(gh api repos/sglavoie/homebrew-tap/contents/Casks/docket.rb --jq .content |
	base64 --decode | sed -n 's/^  version "\(.*\)"$/\1/p')
[ "$cask_version" = "${next#v}" ] ||
	fail "the run passed but the cask names version '${cask_version}', not ${next#v}"
echo "Released $next: the GitHub release and the Homebrew cask are both at ${next#v}."
echo "Upgrade with: brew upgrade --cask docket"
