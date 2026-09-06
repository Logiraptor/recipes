#!/usr/bin/env bash
# Validate every data file against the CUE schemas.
#
#   ./schema/validate.sh
#
# Requires: cue (go install cuelang.org/go/cmd/cue@latest)
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

for f in recipes/*.json; do
	slug=$(basename "$f" .json)
	if ! cue vet -d '#Recipe' ./schema "$f"; then
		echo "invalid recipe: $f" >&2
		fail=1
	fi
	if ! [[ $slug =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
		echo "bad slug (filename): $f" >&2
		fail=1
	fi
done

for f in meal-plans/*.json; do
	[ -e "$f" ] || continue
	base=$(basename "$f" .json)
	if ! cue vet -d '#MealPlan' ./schema "$f"; then
		echo "invalid meal plan: $f" >&2
		fail=1
	fi
	if [ "$(jq -r .weekStart "$f")" != "$base" ]; then
		echo "weekStart does not match filename: $f" >&2
		fail=1
	fi
	# Every referenced recipe slug must exist in recipes/.
	while read -r slug; do
		[ -n "$slug" ] || continue
		if [ ! -f "recipes/$slug.json" ]; then
			echo "$f: unknown recipe slug: $slug" >&2
			fail=1
		fi
	done < <(jq -r '.entries[].recipe // empty' "$f")
done

if [ $fail -eq 0 ]; then
	echo "ok"
fi
exit $fail
