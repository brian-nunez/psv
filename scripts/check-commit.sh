#!/usr/bin/env bash

set -euo pipefail

types='feat|fix|docs|refactor|perf|test|build|ci|style|chore|revert'
scopes='core|report|cli|server|ui|testdata|scripts'
subject_re="^($types)(\(($scopes)\))?(!)?: (.+)$"

msg=$(sed -e '/^# -\{24\} >8 -\{24\}$/,$d' -e '/^#/d' "$1")
subject=${msg%%$'\n'*}
errors=()

if [[ "$subject" =~ $subject_re ]]; then
	bang=${BASH_REMATCH[4]}
	summary=${BASH_REMATCH[5]}

	if [[ ! "$summary" =~ ^[a-z0-9] ]]; then
		errors+=("summary must start with a lowercase letter or digit: '$summary'")
	fi

	if [[ "$summary" == *. ]]; then
		errors+=("summary must not end with a period")
	fi

	if ((${#summary} > 72)); then
		errors+=("summary is ${#summary} characters; the limit is 72")
	fi
else
	bang=
	errors+=("subject must be '<type>(<scope>): <summary>' with type one of $types and scope one of $scopes: '$subject'")
fi

line_no=0
breaking=

while IFS= read -r line; do
	line_no=$((line_no + 1))

	if ((line_no == 2)) && [[ -n "$line" ]]; then
		errors+=("line 2 must be blank")
	fi

	if ((line_no > 1 && ${#line} > 72)); then
		errors+=("line $line_no is ${#line} characters; the limit is 72")
	fi

	if [[ "$line" =~ ^BREAKING[\ -]CHANGE ]]; then
		if [[ "$line" =~ ^BREAKING\ CHANGE:\ .+ ]]; then
			breaking=1
		else
			errors+=("line $line_no must be 'BREAKING CHANGE: <what breaks and how to migrate>'")
		fi
	fi
done <<<"$msg"

if [[ -n "$bang" && -z "$breaking" ]]; then
	errors+=("'!' needs a 'BREAKING CHANGE: ' footer")
fi

if [[ -n "$breaking" && -z "$bang" ]]; then
	errors+=("a 'BREAKING CHANGE: ' footer needs '!' after the type or scope")
fi

if ((${#errors[@]} > 0)); then
	printf 'commit message: %s\n' "${errors[@]}" >&2
	exit 1
fi
