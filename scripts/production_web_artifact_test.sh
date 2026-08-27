#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
output_root=''
api_origin='https://devenglish-artifact.invalid'
synthetic_sentinel='DEVENGLISH_SYNTHETIC_ARTIFACT_SENTINEL_20260827'

fail() {
  printf '%s\n' "$1" >&2
  exit 1
}

cleanup() {
  if [[ -n "$output_root" && -d "$output_root" ]]; then
    rm -rf "$output_root"
  fi
}
trap cleanup EXIT

for required in flutter git mktemp find grep shasum cp cmp awk stat; do
  if ! command -v "$required" >/dev/null 2>&1; then
    fail "Required command is unavailable: $required"
  fi
done

flutter_bin="$(command -v flutter)"
if [[ ! -f "$repo_root/pubspec.yaml" ]]; then
  fail 'Flutter repository root could not be verified.'
fi
output_root="$(mktemp -d)"
artifact_dir="$output_root/artifact"
negative_artifact_dir="$output_root/negative-artifact"
safe_home="$output_root/home"
mkdir -p "$artifact_dir" "$negative_artifact_dir" "$safe_home" "$output_root/tmp" \
  "$output_root/config"

snapshot_repo_state() {
  local tracked_output="$1"
  local untracked_output="$2"
  local untracked_list="$output_root/untracked-files"
  local path=''
  local file_mode=''
  local file_digest=''

  if ! git -C "$repo_root" diff --binary --no-ext-diff HEAD >"$tracked_output"; then
    fail 'Could not snapshot tracked repository state.'
  fi
  if ! git -C "$repo_root" ls-files --others --exclude-standard -z >"$untracked_list"; then
    fail 'Could not enumerate non-ignored untracked files.'
  fi
  : >"$untracked_output"
  while IFS= read -r -d '' path; do
    if ! file_mode="$(stat -f '%OLp' "$repo_root/$path" 2>/dev/null)"; then
      if ! file_mode="$(stat -c '%a' "$repo_root/$path" 2>/dev/null)"; then
        fail 'Could not snapshot an untracked file mode.'
      fi
    fi
    if ! file_digest="$(git -C "$repo_root" hash-object --no-filters -- "$path")"; then
      fail 'Could not snapshot an untracked file digest.'
    fi
    printf '%s\t%s\t%s\0' "$path" "$file_mode" "$file_digest" >>"$untracked_output"
  done <"$untracked_list"
}

git -C "$repo_root" status --porcelain=v1 >"$output_root/status-before"
snapshot_repo_state "$output_root/tracked-before" "$output_root/untracked-before"

run_flutter_build() {
  (
    cd "$repo_root"
    env -i \
      PATH="$PATH" \
      HOME="$safe_home" \
      TMPDIR="$output_root/tmp" \
      XDG_CONFIG_HOME="$output_root/config" \
      CI=1 \
      FLUTTER_SUPPRESS_ANALYTICS=true \
      DEVENGLISH_ENV=production \
      API_BASE_URL="$api_origin" \
      "$flutter_bin" build web \
        --release \
        --no-pub \
        --output "$artifact_dir" \
        --dart-define=DEVENGLISH_ENV=production \
        --dart-define=API_BASE_URL="$api_origin"
  )
}

if ! run_flutter_build >"$output_root/flutter-build.log" 2>&1; then
  fail 'Flutter production web build failed; build output was kept private.'
fi

scan_file_for_marker() {
  local marker="$1"
  local file="$2"
  local grep_status=0

  if grep -aFq -- "$marker" "$file"; then
    return 0
  else
    grep_status=$?
  fi
  if (( grep_status == 1 )); then
    return 1
  fi
  printf 'Artifact scan could not read a regular file.\n' >&2
  return "$grep_status"
}

scan_artifact() {
  local root="$1"
  local expected_origin="$2"
  local file_list="$3"
  local file=''
  local file_count=0
  local origin_found=0
  local scan_status=0

  if ! find "$root" -type f -print0 >"$file_list"; then
    printf 'Artifact file traversal failed.\n' >&2
    return 1
  fi

  while IFS= read -r -d '' file; do
    file_count=$((file_count + 1))

    case "$file" in
      *.map)
        printf 'Source-map artifact detected.\n' >&2
        return 1
        ;;
    esac

    if scan_file_for_marker "$expected_origin" "$file"; then
      origin_found=1
    else
      scan_status=$?
      if (( scan_status != 1 )); then
        return 1
      fi
    fi

    for marker in \
      "$synthetic_sentinel" \
      'http://localhost:8080' \
      'API_TOKEN' \
      'DEVENGLISH_BOOTSTRAP_KEY' \
      'DEVENGLISH_LOGIN_SECRET' \
      'DEEPSEEK_API_KEY' \
      'GROQ_API_KEY' \
      'AZURE_SPEECH_KEY'; do
      if scan_file_for_marker "$marker" "$file"; then
        printf 'Forbidden production artifact marker detected.\n' >&2
        return 1
      else
        scan_status=$?
        if (( scan_status != 1 )); then
          return 1
        fi
      fi
    done
  done <"$file_list"

  if (( file_count == 0 )); then
    printf 'Production artifact contains no regular files.\n' >&2
    return 1
  fi
  if (( origin_found == 0 )); then
    printf 'Synthetic public API origin was not found in the artifact.\n' >&2
    return 1
  fi
}

positive_file_list="$output_root/positive-files"
if ! scan_artifact "$artifact_dir" "$api_origin" "$positive_file_list" >/dev/null 2>&1; then
  fail 'Production artifact security scan failed; matching content was kept private.'
fi

cp -R "$artifact_dir/." "$negative_artifact_dir/"
negative_file="$(find "$negative_artifact_dir" -type f -name 'index.html' -print -quit)"
if [[ -z "$negative_file" ]]; then
  negative_file="$(find "$negative_artifact_dir" -type f -print -quit)"
fi
if [[ -z "$negative_file" ]]; then
  fail 'Negative artifact control could not find a regular artifact file.'
fi
printf '%s\n' "$synthetic_sentinel" >>"$negative_file"

negative_file_list="$output_root/negative-files"
if scan_artifact "$negative_artifact_dir" "$api_origin" "$negative_file_list" >/dev/null 2>&1; then
  fail 'Negative artifact control unexpectedly passed.'
fi

artifact_manifest="$output_root/artifact-files.txt"
(
  cd "$artifact_dir"
  find . -type f -print | LC_ALL=C sort >"$artifact_manifest"
)
artifact_file_count="$(wc -l <"$artifact_manifest" | tr -d ' ')"
artifact_digest="$(
  (
    cd "$artifact_dir"
    while IFS= read -r file; do
      shasum -a 256 "$file"
    done <"$artifact_manifest"
  ) | shasum -a 256 | awk '{print $1}'
)"

git -C "$repo_root" status --porcelain=v1 >"$output_root/status-after"
snapshot_repo_state "$output_root/tracked-after" "$output_root/untracked-after"
if ! cmp -s "$output_root/status-before" "$output_root/status-after" ||
  ! cmp -s "$output_root/tracked-before" "$output_root/tracked-after" ||
  ! cmp -s "$output_root/untracked-before" "$output_root/untracked-after"; then
  fail 'Repository state changed during the production artifact build.'
fi

printf 'Production artifact gate passed: files=%s digest=%s\n' \
  "$artifact_file_count" "$artifact_digest"
printf 'Negative artifact control: sentinel rejected without content disclosure.\n'
printf 'Repository status unchanged after production build.\n'
