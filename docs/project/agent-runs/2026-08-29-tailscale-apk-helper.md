# Tailscale Android packaging helper — 2026-08-29

## Objective

Make the local Docker backend usable from the development phone when the phone
and Mac are on different networks, without exposing the backend publicly.

## Implemented scope

- `scripts/android_local_apk.sh` now selects the API endpoint in this order:
  explicit `API_BASE_URL`, explicit or detected `TAILSCALE_IP`, then `LAN_IP`
  or the Mac's `en0` address;
- `ANDROID_BACKEND_PORT` changes the selected host port without changing the
  Flutter application contract;
- `DEVENGLISH_APK_OUTPUT` changes the copied APK path;
- the helper validates the endpoint scheme and checks `/healthz` before running
  `flutter build apk`;
- `scripts/android_local_apk_test.sh` covers precedence, slash normalization,
  Wi-Fi fallback and invalid endpoint rejection.

## Evidence

- `rtk bash -n scripts/android_local_apk.sh scripts/android_local_apk_test.sh`
  — exit `0`;
- `rtk bash scripts/android_local_apk_test.sh` — exit `0`, four cases passed;
- `rtk curl -fsS --connect-timeout 3 http://<tailscale-ip>:8080/healthz` —
  exit `0`, running backend returned HTTP `200`;
- `rtk env TAILSCALE_IP=100.126.52.73 DEVENGLISH_APK_OUTPUT=/Users/ryantruong/Desktop/DevEnglish-tailscale.apk scripts/android_local_apk.sh`
  — exit `0`; release APK built successfully (`52.2 MB`) and copied to the
  requested Desktop path;
- Android Emulator `Ledgerly_Pixel_8` booted as `emulator-5554`, installed the
  copied APK successfully, and launched `com.devenglish.devenglish/.MainActivity`;
- the emulator screenshot rendered the canonical `Today` surface with backend
  status `Live`, `1 open task` and `1 source connected`; UI hierarchy evidence
  exposed the four canonical tabs and their content;
- emulator navigation through `Today → Work → Knowledge → Learning` succeeded;
  the dumps exposed `S1 Runtime Smoke`, `S1 Canonical Runbook` and the
  work-derived learning prompt respectively;
- app-process logcat contained no `FATAL`, `AndroidRuntime` crash or Dart
  exception after launch; the emulator could reach the Tailscale backend port
  (`nc -z` exit `0`), and the app loaded backend-backed data;
- `rtk git diff --check` — exit `0`.

## Usage

```bash
TAILSCALE_IP="$(tailscale ip -4 | head -n 1)" \
  DEVENGLISH_APK_OUTPUT="$HOME/Desktop/DevEnglish-tailscale.apk" \
  scripts/android_local_apk.sh
```

Install Tailscale on the phone, sign in to the same tailnet, install the APK,
and keep the Mac's Docker backend and embedding sidecar running. The helper
does not provision Tailscale, publish a port, or embed a credential.

## Boundary

This proves endpoint selection and local backend reachability only. It does
not prove a real-device UI walkthrough, microphone permission, production
HTTPS, or release readiness. The emulator walkthrough is a separate local
APK/UI signal; it does not prove Tailscale routing on a physical phone.
