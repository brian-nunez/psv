#!/usr/bin/env bash

set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/check-commit.sh"
failures=0

expect() {
	local name=$1 want=$2 msg=$3 file got
	file=$(mktemp)
	printf '%s\n' "$msg" >"$file"

	if bash "$script" "$file" 2>/dev/null; then
		got=pass
	else
		got=fail
	fi

	rm -f "$file"

	if [[ "$got" == "$want" ]]; then
		echo "ok   $name"
	else
		echo "FAIL $name: want $want, got $got"
		failures=$((failures + 1))
	fi
}

long=$(printf 'a%.0s' {1..73})
max=$(printf 'a%.0s' {1..72})

expect "type and summary" pass "fix: count flashing area per frame"
expect "type, scope and summary" pass "feat(server): delete the upload on disconnect"
expect "body after blank line" pass $'docs: refresh the readme\n\nThe examples were stale.'
expect "breaking change with bang and footer" pass $'feat(core)!: drop the v1 report\n\nBREAKING CHANGE: read psv.report/v2 instead'
expect "summary of 72 characters" pass "fix: $max"
expect "git comments are ignored" pass $'fix: a\n# Please enter the commit message'
expect "verbose diff is ignored" pass $'fix: a\n# ------------------------ >8 ------------------------\n'"$(printf 'x%.0s' {1..100})"

expect "no type" fail "count flashing area per frame"
expect "unknown type" fail "feature: add watch"
expect "unknown scope" fail "fix(vad): keep lead-in"
expect "empty scope" fail "fix(): a"
expect "capitalised summary" fail "fix: Count flashing area"
expect "trailing period" fail "fix: count flashing area."
expect "summary of 73 characters" fail "fix: $long"
expect "missing space after colon" fail "fix:count"
expect "merge commit" fail "Merge pull request #1 from brian-nunez/feat/x"
expect "line 2 not blank" fail $'fix: a\nbody'
expect "body line over 72" fail $'fix: a\n\n'"$long"
expect "bang without footer" fail "feat(core)!: drop the v1 report"
expect "footer without bang" fail $'feat(core): drop the v1 report\n\nBREAKING CHANGE: read v2'
expect "malformed footer" fail $'feat(core)!: drop\n\nBREAKING-CHANGE: read v2'

if ((failures > 0)); then
	echo "$failures failed"
	exit 1
fi
