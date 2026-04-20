#!/usr/bin/env bash
# check-migration-collision.sh
#
# Scans both migrations directories and fails (non-zero exit) if any
# 6-digit prefix appears more than once across the two services, or if
# a .up.sql has no matching .down.sql.
#
# Usage:
#   ./scripts/check-migration-collision.sh             # collision + pairing check
#   ./scripts/check-migration-collision.sh --strict    # also validate docs/MIGRATIONS.md coverage
#
# Exit codes:
#   0  no collisions, all pairs matched, (strict) registry covers all files
#   1  check failed (collision / missing down / registry gap)
#   2  bad flag / usage error

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

dirs=(
    "$repo_root/cmdb-core/db/migrations"
    "$repo_root/ingestion-engine/db/migrations"
)
registry="$repo_root/docs/MIGRATIONS.md"

strict=0
for arg in "$@"; do
    case "$arg" in
        --strict) strict=1 ;;
        -h|--help)
            sed -n '2,12p' "$0"
            exit 0
            ;;
        *) echo "unknown flag: $arg" >&2; exit 2 ;;
    esac
done

# -----------------------------------------------------------------------
# 1. Gather (version, service, relpath) triples
# -----------------------------------------------------------------------
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

for d in "${dirs[@]}"; do
    if [[ ! -d "$d" ]]; then
        echo "warning: migrations dir not found: $d" >&2
        continue
    fi
    # Service name = the top-level dir containing db/migrations
    # e.g. /cmdb-platform/cmdb-core/db/migrations -> cmdb-core
    service="$(basename "$(dirname "$(dirname "$d")")")"
    while IFS= read -r -d '' up; do
        file="$(basename "$up")"
        # Extract leading 6-digit version via parameter expansion
        version="${file:0:6}"
        if [[ ! "$version" =~ ^[0-9]{6}$ ]]; then
            continue
        fi
        relpath="${up#"$repo_root/"}"
        printf '%s\t%s\t%s\n' "$version" "$service" "$relpath" >> "$tmp"
    done < <(find "$d" -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9][0-9][0-9]_*.up.sql' -print0)
done

# -----------------------------------------------------------------------
# 2. Collision detection: same version appearing more than once
# -----------------------------------------------------------------------
collisions="$(awk -F'\t' '{print $1}' "$tmp" | sort | uniq -d)"
if [[ -n "$collisions" ]]; then
    echo "MIGRATION NUMBER COLLISION DETECTED" >&2
    echo "===================================" >&2
    while IFS= read -r v; do
        echo "version $v occupied by:" >&2
        awk -F'\t' -v v="$v" '$1 == v { printf "  - %s (%s)\n", $3, $2 }' "$tmp" >&2
    done <<< "$collisions"
    echo >&2
    echo "Resolution: pick the next available number from docs/MIGRATIONS.md" >&2
    echo "            and rename the later-to-land file pair." >&2
    exit 1
fi

# -----------------------------------------------------------------------
# 3. Up / down pairing: every .up.sql needs a matching .down.sql
# -----------------------------------------------------------------------
missing_down=""
while IFS=$'\t' read -r _ _ path; do
    up="$repo_root/$path"
    down="${up%.up.sql}.down.sql"
    if [[ ! -f "$down" ]]; then
        missing_down+="  - $path"$'\n'
    fi
done < "$tmp"
if [[ -n "$missing_down" ]]; then
    echo "UP migration missing matching DOWN:" >&2
    printf '%s' "$missing_down" >&2
    exit 1
fi

# -----------------------------------------------------------------------
# 4. Registry coverage (strict only)
# -----------------------------------------------------------------------
if [[ "$strict" == "1" ]]; then
    if [[ ! -f "$registry" ]]; then
        echo "registry missing: $registry" >&2
        exit 1
    fi
    not_in_registry=""
    while IFS=$'\t' read -r v _ _; do
        # Match a markdown table row whose first column is "| NNNNNN |"
        if ! grep -qE "^\| ${v} +\|" "$registry"; then
            not_in_registry+="  - $v"$'\n'
        fi
    done < "$tmp"
    if [[ -n "$not_in_registry" ]]; then
        echo "migrations present on disk but missing from registry:" >&2
        printf '%s' "$not_in_registry" >&2
        echo "edit docs/MIGRATIONS.md and add one row per listed number." >&2
        exit 1
    fi
fi

# -----------------------------------------------------------------------
# 5. Summary
# -----------------------------------------------------------------------
count="$(wc -l < "$tmp" | tr -d ' ')"
mode="collision+pairing"
[[ "$strict" == "1" ]] && mode="$mode+registry"
echo "OK — $count migrations across ${#dirs[@]} services, no collisions, all pairs matched ($mode)."
