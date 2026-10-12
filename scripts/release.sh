#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"

if [[ $# -ne 1 ]]; then
	echo "usage: $0 vX.Y.Z" >&2
	exit 1
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "version must match vX.Y.Z" >&2
	exit 1
fi

root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
	echo "release must run inside a git repository" >&2
	exit 1
}
cd "$root"

notes_file="docs/releases/${version}.md"
if [[ ! -f "$notes_file" ]]; then
	echo "missing release notes: $notes_file" >&2
	exit 1
fi
if ! git ls-files --error-unmatch "$notes_file" >/dev/null 2>&1; then
	echo "release notes are not tracked: $notes_file" >&2
	exit 1
fi

if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
	echo "worktree must be clean before release" >&2
	exit 1
fi

for command in git go gh; do
	if ! command -v "$command" >/dev/null 2>&1; then
		echo "$command is required to publish a release" >&2
		exit 1
	fi
done

if ! gh auth status >/dev/null 2>&1; then
	echo "gh is not authenticated" >&2
	exit 1
fi

branch="$(git branch --show-current)"
if [[ "$branch" != "main" ]]; then
	echo "release must run from main, current branch: ${branch:-detached HEAD}" >&2
	exit 1
fi

git fetch --quiet origin main
if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
	echo "main must be pushed and match origin/main before release" >&2
	exit 1
fi

git diff --check
go mod verify
# The player tests need libmpv. Without this variable they skip if it is absent.
export PINE_REQUIRE_LIBMPV=1
# The MPRIS server test needs dbus-daemon. Without this variable it skips if it is absent.
export PINE_REQUIRE_DBUS=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...

# Each binary must link only the shared packages and its own app packages.
if go list -deps ./cmd/spruce | grep -E 'Thelost77/pine/internal/(abs|app|cache|captions|config|db|screens)(/|$)'; then
	echo "spruce links pine-only packages" >&2
	exit 1
fi
if go list -deps . | grep 'Thelost77/pine/internal/spruce'; then
	echo "pine links spruce packages" >&2
	exit 1
fi

tag_ref="refs/tags/${version}"
remote_tag="$(git ls-remote --tags origin "$tag_ref" | awk 'NR == 1 {print $1}')"

if git show-ref --verify --quiet "$tag_ref"; then
	if [[ "$(git rev-list -n 1 "$version")" != "$(git rev-parse HEAD)" ]]; then
		echo "tag $version exists but does not point to HEAD" >&2
		exit 1
	fi
elif [[ -n "$remote_tag" ]]; then
	echo "tag $version exists on origin but not locally; fetch and inspect it" >&2
	exit 1
else
	git tag -a "$version" -m "$version"
fi

local_tag="$(git rev-parse "$tag_ref")"
remote_tag="$(git ls-remote --tags origin "$tag_ref" | awk 'NR == 1 {print $1}')"
if [[ -z "$remote_tag" ]]; then
	git push origin "$tag_ref"
elif [[ "$remote_tag" != "$local_tag" ]]; then
	echo "local and remote $version tags differ" >&2
	exit 1
fi

if gh release view "$version" >/dev/null 2>&1; then
	echo "GitHub Release $version already exists"
	exit 0
fi

gh release create "$version" \
	--verify-tag \
	--title "pine and spruce ${version}" \
	--notes-file "$notes_file"
