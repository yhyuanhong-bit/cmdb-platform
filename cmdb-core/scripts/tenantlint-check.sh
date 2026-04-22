#!/usr/bin/env bash
# tenantlint-check.sh
#
# CI gate for tools/tenantlint. We inherit 180 pre-existing violations
# across 44 files (see tools/tenantlint/baseline.txt). This script runs
# tenantlint, groups its findings by file, and fails ONLY when:
#
#   (a) a file not in the baseline has at least one violation, or
#   (b) a file in the baseline exceeds its recorded count.
#
# Fixes that LOWER a file's count are fine — the script prints a
# reminder to update baseline.txt when that happens so the backlog
# burns down monotonically instead of re-accumulating silently.

set -euo pipefail

cd "$(dirname "$0")/.."

BASELINE_FILE="tools/tenantlint/baseline.txt"
BIN_DIR="$(mktemp -d)"
trap 'rm -rf "$BIN_DIR"' EXIT

echo "[tenantlint] building analyzer…"
go build -o "$BIN_DIR/tenantlint" ./tools/tenantlint/cmd/tenantlint

echo "[tenantlint] running against ./..."
# go vet with a -vettool exits non-zero when findings are present;
# we want the findings regardless, so capture stderr and keep going.
CURRENT=$(mktemp)
set +e
go vet -vettool="$BIN_DIR/tenantlint" ./... 2>"$CURRENT"
set -e

# Extract findings: lines of the form
#   path/to/file.go:LINE:COL: direct pool.X call — …
# Aggregate into "path count" pairs sorted by path.
CURRENT_COUNTS=$(mktemp)
grep -E 'direct pool\.' "$CURRENT" \
  | awk -F: '{print $1}' \
  | sort | uniq -c \
  | awk '{printf "%s %d\n",$2,$1}' \
  | sort > "$CURRENT_COUNTS"

# Strip comments/blank lines from the baseline for comparison.
BASELINE_COUNTS=$(mktemp)
grep -Ev '^(#|$)' "$BASELINE_FILE" | sort > "$BASELINE_COUNTS"

echo "[tenantlint] comparing against baseline ($BASELINE_FILE)…"

FAIL=0
DROPPED=()

# For every file currently producing findings, check it is in baseline
# AND its count has not exceeded the recorded ceiling.
while read -r line; do
  file=$(echo "$line" | awk '{print $1}')
  cur=$(echo "$line" | awk '{print $2}')
  base=$(awk -v f="$file" '$1==f {print $2}' "$BASELINE_COUNTS")
  if [ -z "$base" ]; then
    echo "::error file=$file::tenantlint: new file with direct pool calls ($cur finding(s)). Use database.TenantScoped or add //tenantlint:allow-direct-pool with justification."
    FAIL=1
  elif [ "$cur" -gt "$base" ]; then
    echo "::error file=$file::tenantlint: $cur direct pool calls, baseline allows $base. A new direct call was introduced — do not raise the baseline to silence it."
    FAIL=1
  elif [ "$cur" -lt "$base" ]; then
    DROPPED+=("$file: $base → $cur")
  fi
done < "$CURRENT_COUNTS"

# For every file in baseline that no longer appears in current, flag it
# so contributors clean up baseline.txt when they fully eliminate a file.
while read -r line; do
  file=$(echo "$line" | awk '{print $1}')
  cur=$(awk -v f="$file" '$1==f {print $2}' "$CURRENT_COUNTS")
  if [ -z "$cur" ]; then
    DROPPED+=("$file: cleared — remove from baseline.txt")
  fi
done < "$BASELINE_COUNTS"

if [ "${#DROPPED[@]}" -gt 0 ]; then
  echo ""
  echo "[tenantlint] nice — some files dropped below baseline:"
  for entry in "${DROPPED[@]}"; do
    echo "  $entry"
  done
  echo ""
  echo "Please update tools/tenantlint/baseline.txt in this PR to reflect the new (lower) ceiling."
  # Not a failure — we never block a burndown.
fi

TOTAL_CUR=$(awk '{s+=$2} END {print s+0}' "$CURRENT_COUNTS")
TOTAL_BASE=$(awk '{s+=$2} END {print s+0}' "$BASELINE_COUNTS")
echo ""
echo "[tenantlint] total findings: current=$TOTAL_CUR  baseline=$TOTAL_BASE"

if [ "$FAIL" -ne 0 ]; then
  echo "[tenantlint] FAIL — new direct-pool calls introduced."
  exit 1
fi

echo "[tenantlint] OK — no new violations."
