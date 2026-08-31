# CyberOS machine gates — 2026-08-30

## Scope

Run the repository-configured CyberOS machine gates against the current dirty
checkout after the keyboard-accessibility and current APK verification. This
run is machine evidence only; it does not change task lifecycle state or grant
review, final acceptance, commit, push, merge or deployment authorization.

## Command and result

```text
rtk bash .cyberos/cuo/gates/run-gates.sh
exit 0

build: PASS — go build ./... and flutter build apk --debug
lint: PASS — gofmt, go vet, Dart format and flutter analyze
test: PASS — go test ./..., contentfactory generated 200 evaluation cases,
  flutter test passed 63 tests with 2 environment skips
coverage: PASS — configured gate completed successfully
doctor: SKIP — memory store present, CyberOS memory CLI not importable
summary: GATES: GREEN (machine gates only)
```

## Boundary

The command output explicitly retains human review and final acceptance as
required. The agent did not set any task to `done`, and no task state, commit,
push, merge, deployment or production data was changed by this run.
