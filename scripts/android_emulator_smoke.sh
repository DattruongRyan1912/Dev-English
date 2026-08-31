#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
apk_path="${ANDROID_APK_PATH:-$repo_root/build/app/outputs/flutter-apk/app-release.apk}"
package_name="${DEVENGLISH_ANDROID_PACKAGE:-com.devenglish.devenglish}"
activity_name="${DEVENGLISH_ANDROID_ACTIVITY:-.MainActivity}"
device_id="${ANDROID_DEVICE_ID:-}"
remote_ui="/sdcard/devenglish-emulator-smoke.xml"

if [[ -n "${ANDROID_SMOKE_OUTPUT:-}" ]]; then
  output_dir="$ANDROID_SMOKE_OUTPUT"
else
  output_dir="$(mktemp -d "${TMPDIR:-/tmp}/devenglish-android-smoke.XXXXXX")"
fi

fail() {
  printf 'Android emulator smoke failed: %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

select_device() {
  if [[ -n "$device_id" ]]; then
    return
  fi

  device_id="$(adb devices | awk '$2 == "device" {print $1; exit}')"
  [[ -n "$device_id" ]] || fail "no booted Android device; run adb devices or launch an emulator first"
}

wait_for_boot() {
  local deadline=$((SECONDS + 90))
  local booted
  while (( SECONDS < deadline )); do
    booted="$(adb -s "$device_id" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)"
    if [[ "$booted" == "1" ]]; then
      return
    fi
    sleep 1
  done
  fail "device $device_id did not report sys.boot_completed=1"
}

dump_ui() {
  adb -s "$device_id" shell uiautomator dump "$remote_ui" >/dev/null 2>&1 || return 1
  adb -s "$device_id" shell cat "$remote_ui" >"$output_dir/window.xml"
}

assert_marker() {
  local marker="$1"
  grep -Fq "$marker" "$output_dir/window.xml" || fail "missing UI marker: $marker"
}

assert_visible() {
  local marker="$1"
  if ! visible_marker "$marker"; then
    fail "missing visible UI marker: $marker"
  fi
}

visible_marker() {
  local marker="$1"
  grep -Fq "content-desc=\"$marker\"" "$output_dir/window.xml" \
    || grep -Fq "text=\"$marker\"" "$output_dir/window.xml"
}

scroll_until_visible() {
  local marker="$1"
  for _ in 1 2 3 4; do
    if dump_ui && visible_marker "$marker"; then
      return
    fi
    adb -s "$device_id" shell input swipe 540 1900 540 500 400 >/dev/null
    sleep 1
  done
  fail "UI marker did not become visible after scrolling: $marker"
}

capture_screen() {
  local name="$1"
  adb -s "$device_id" exec-out screencap -p >"$output_dir/$name.png"
}

wait_for_screen() {
  local identity="$1"
  local deadline=$((SECONDS + 20))
  while (( SECONDS < deadline )); do
    if dump_ui && grep -Fq "content-desc=\"DEVENGLISH / $identity\"" "$output_dir/window.xml"; then
      return
    fi
    sleep 1
  done
  if dump_ui && grep -Fq 'Workspace is temporarily unavailable' "$output_dir/window.xml"; then
    fail "backend unavailable while waiting for screen $identity; inspect $output_dir/window.xml"
  fi
  fail "screen $identity did not render within 20 seconds"
}

tap_tab() {
  local tab="$1"
  local line
  line="$(grep -F "content-desc=\"$tab&#10;Tab" "$output_dir/window.xml" | head -n 1 || true)"
  [[ -n "$line" ]] || fail "navigation tab not found: $tab"

  local bounds
  bounds="$(sed -n "s/.*content-desc=\"${tab}&#10;Tab [^\"]*\"[^>]*bounds=\"\([^\"]*\)\".*/\1/p" <<<"$line")"
  if [[ "$bounds" =~ ^\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]$ ]]; then
    local left="${BASH_REMATCH[1]}"
    local top="${BASH_REMATCH[2]}"
    local right="${BASH_REMATCH[3]}"
    local bottom="${BASH_REMATCH[4]}"
    local center_x=$(((left + right) / 2))
    local center_y=$(((top + bottom) / 2))
    adb -s "$device_id" shell input tap "$center_x" "$center_y"
    return
  fi
  fail "navigation bounds not found for tab: $tab"
}

navigate_to() {
  local tab="$1"
  local identity="$2"
  local screenshot_name="$3"
  tap_tab "$tab"
  wait_for_screen "$identity"
  capture_screen "$screenshot_name"
}

main() {
  require_command adb
  [[ -f "$apk_path" ]] || fail "APK not found: $apk_path"
  mkdir -p "$output_dir"

  select_device
  wait_for_boot

  printf 'Installing %s on %s\n' "$apk_path" "$device_id"
  adb -s "$device_id" install -r "$apk_path" >/dev/null
  adb -s "$device_id" logcat -c
  adb -s "$device_id" shell am force-stop "$package_name"
  adb -s "$device_id" shell am start -n "$package_name/$activity_name" >/dev/null

  wait_for_screen TODAY

  local app_pid=""
  local pid_deadline=$((SECONDS + 10))
  while (( SECONDS < pid_deadline )); do
    app_pid="$(adb -s "$device_id" shell pidof "$package_name" 2>/dev/null | tr -d '\r' | awk '{print $1}' || true)"
    [[ -n "$app_pid" ]] && break
    sleep 1
  done
  [[ -n "$app_pid" ]] || fail "Android process was not found after launching $package_name"

  capture_screen today

  assert_marker 'DEVENGLISH / TODAY'
  assert_marker 'Canonical workspace'
  assert_marker 'Live'
  assert_marker 'content-desc="Today'
  assert_marker 'content-desc="Work'
  assert_marker 'content-desc="Knowledge'
  assert_marker 'content-desc="Learning'
  assert_marker '1 open task'
  assert_marker '1 source connected'

  navigate_to Work WORK work
  assert_visible 'Projects'
  scroll_until_visible 'Open tasks'
  assert_visible 'Open tasks'

  navigate_to Knowledge KNOWLEDGE knowledge
  scroll_until_visible 'Connected sources'
  assert_visible 'Connected sources'

  navigate_to Learning LEARNING learning
  scroll_until_visible 'Workflow overlay ready'
  assert_visible 'Workflow overlay ready'

  navigate_to Today TODAY today-return
  local composer_ready=0
  for _ in 1 2 3 4 5 6 7 8; do
    if dump_ui \
      && grep -Fq 'Send message' "$output_dir/window.xml" \
      && grep -Fq 'Hold to talk' "$output_dir/window.xml"; then
      composer_ready=1
      break
    fi
    adb -s "$device_id" shell input swipe 540 1900 540 500 400 >/dev/null
    sleep 1
  done
  [[ "$composer_ready" == "1" ]] || fail "assistant composer did not render after scrolling Today"
  capture_screen today-assistant

  adb -s "$device_id" logcat --pid="$app_pid" -d -v threadtime >"$output_dir/logcat.txt"

  if grep -Eq 'FATAL EXCEPTION|AndroidRuntime|Dart Error|Unhandled exception' "$output_dir/logcat.txt"; then
    fail "crash or unhandled exception found; inspect $output_dir/logcat.txt"
  fi

  printf 'ANDROID_EMULATOR_SMOKE=PASS\n'
  printf 'Device: %s\n' "$device_id"
  printf 'Package: %s\n' "$package_name"
  printf 'Evidence: %s\n' "$output_dir"
}

main "$@"
