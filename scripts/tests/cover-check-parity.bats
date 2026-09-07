#!/usr/bin/env bats
# Drift gate for the `cover-check` recipe in the Justfile.
#
# `just cover-check` enumerates every gated package by hand: an associative
# `targets` array for internal/* plus one `<name>_pkg=` variable per package
# outside internal/. Nothing forces that list to track the tree, so a newly
# added Go package is silently ungated while CI stays green.
#
# These tests assert both directions of the mapping:
#   * every Go package root on disk is enumerated in cover-check
#   * every package enumerated in cover-check still exists on disk
#
# Requires: bats-core (https://github.com/bats-core/bats-core)
#
# Run: bats scripts/tests/cover-check-parity.bats

REPO_ROOT="$BATS_TEST_DIRNAME/../.."
JUSTFILE="$REPO_ROOT/Justfile"

# Extract the `cover-check` recipe body: from the recipe header to the first
# line that starts a new top-level recipe or comment block.
cover_check_recipe() {
  awk '
    /^cover-check:/ { inrecipe = 1; next }
    inrecipe && /^[^[:space:]#]/ { exit }
    inrecipe { print }
  ' "$JUSTFILE"
}

# Packages named by cover-check, as repo-relative package roots.
declared_packages() {
  local recipe
  recipe=$(cover_check_recipe)

  # internal/* entries come from the `declare -A targets=( [pkg]=NN ... )` block.
  printf '%s\n' "$recipe" \
    | awk '
        /declare -A targets=\(/ { intargets = 1 }
        intargets { print }
        intargets && /^[[:space:]]*\)[[:space:]]*$/ { exit }
      ' \
    | grep -oE '\[[A-Za-z0-9_-]+\]=' \
    | tr -d '[]=' \
    | sed 's#^#internal/#'

  # Everything else is gated through a `<name>_pkg=<path>` assignment.
  printf '%s\n' "$recipe" \
    | grep -oE '^[[:space:]]*[A-Za-z0-9_]+_pkg=[^[:space:]]+' \
    | sed -E 's#^[[:space:]]*[A-Za-z0-9_]+_pkg=##'
}

# Go package roots on disk. internal/X/... and cmd/X/... collapse to their
# top-level directory because cover-check gates them with a `/...` wildcard.
actual_packages() {
  (
    cd "$REPO_ROOT" || exit 1
    find . -name '*.go' -not -path './.git/*' -print \
      | sed -E 's#^\./##; s#/[^/]*$##' \
      | sed -E 's#^(internal/[^/]+)/.*#\1#; s#^(cmd/[^/]+)/.*#\1#'
  ) | sort -u
}

@test "cover-check enumerates at least one internal package" {
  run declared_packages
  [ "$status" -eq 0 ]
  [[ "$output" == *"internal/"* ]]
}

@test "every Go package on disk is gated by cover-check" {
  local missing=()
  local pkg
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    if ! declared_packages | grep -qxF "$pkg"; then
      missing+=("$pkg")
    fi
  done < <(actual_packages)

  if [ "${#missing[@]}" -ne 0 ]; then
    printf 'ungated Go package(s) missing from Justfile cover-check: %s\n' \
      "${missing[*]}" >&2
    return 1
  fi
}

@test "every package named by cover-check exists on disk" {
  local stale=()
  local pkg
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    if [ ! -d "$REPO_ROOT/$pkg" ]; then
      stale+=("$pkg")
    fi
  done < <(declared_packages)

  if [ "${#stale[@]}" -ne 0 ]; then
    printf 'cover-check names package(s) that no longer exist: %s\n' \
      "${stale[*]}" >&2
    return 1
  fi
}

@test "every package named by cover-check contains Go sources" {
  local empty=()
  local pkg
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    [ -d "$REPO_ROOT/$pkg" ] || continue
    if [ -z "$(find "$REPO_ROOT/$pkg" -name '*.go' -print -quit)" ]; then
      empty+=("$pkg")
    fi
  done < <(declared_packages)

  if [ "${#empty[@]}" -ne 0 ]; then
    printf 'cover-check names package(s) with no Go sources: %s\n' \
      "${empty[*]}" >&2
    return 1
  fi
}

@test "cover-check declares no duplicate package entries" {
  local dupes
  dupes=$(declared_packages | sort | uniq -d)
  if [ -n "$dupes" ]; then
    printf 'duplicate cover-check entries: %s\n' "$dupes" >&2
    return 1
  fi
}

@test "every gated internal package has a numeric coverage target" {
  local recipe
  recipe=$(cover_check_recipe)

  local bad
  bad=$(printf '%s\n' "$recipe" \
    | awk '
        /declare -A targets=\(/ { intargets = 1 }
        intargets { print }
        intargets && /^[[:space:]]*\)[[:space:]]*$/ { exit }
      ' \
    | grep -oE '\[[A-Za-z0-9_-]+\]=[^[:space:])]*' \
    | grep -vE '\]=[0-9]+$' || true)

  if [ -n "$bad" ]; then
    printf 'non-numeric coverage target(s): %s\n' "$bad" >&2
    return 1
  fi
}

@test "every <name>_pkg gate has a matching <name>_target threshold" {
  local recipe
  recipe=$(cover_check_recipe)

  local missing=()
  local name
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    if ! printf '%s\n' "$recipe" | grep -qE "^[[:space:]]*${name}_target=[0-9]+[[:space:]]*$"; then
      missing+=("$name")
    fi
  done < <(printf '%s\n' "$recipe" \
    | grep -oE '^[[:space:]]*[A-Za-z0-9_]+_pkg=' \
    | sed -E 's#^[[:space:]]*##; s#_pkg=$##')

  if [ "${#missing[@]}" -ne 0 ]; then
    printf 'gate(s) without a _target threshold: %s\n' "${missing[*]}" >&2
    return 1
  fi
}

@test "cover-check exits with the accumulated failure status" {
  run bash -c "cd '$REPO_ROOT' && sed -n '/^cover-check:/,\$p' Justfile | grep -m1 -E '^[[:space:]]*exit \\\$fail'"
  [ "$status" -eq 0 ]
}
