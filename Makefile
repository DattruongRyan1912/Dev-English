.PHONY: backend-run backend-test content-check flutter-check ui-goldens production-runtime-smoke docker-dev-up docker-dev-down docker-dev-logs db-migrate db-backup db-restore android-local-apk android-emulator-smoke check

backend-run:
	go run ./backend/cmd/server

backend-test:
	gofmt -w backend
	go test ./...
	go vet ./...

content-check:
	go run ./backend/cmd/contentfactory --root . --write-evaluation-cases tests/evaluation_cases.generated.json --target 200

flutter-check:
	dart format lib test
	flutter analyze
	flutter test

ui-goldens:
	flutter test test/workspace_golden_test.dart test/workspace_state_golden_test.dart

production-runtime-smoke:
	DEVENGLISH_PRODUCTION_SMOKE=YES ./scripts/production_runtime_smoke.sh

docker-dev-up:
	docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build

docker-dev-down:
	docker compose --env-file .env.local -f infra/docker-compose.yml down

docker-dev-logs:
	docker compose --env-file .env.local -f infra/docker-compose.yml logs -f backend

db-migrate:
	./scripts/db_migrate.sh

db-backup:
	./scripts/db_backup.sh

db-restore:
	./scripts/db_restore.sh

android-local-apk:
	./scripts/android_local_apk.sh

android-emulator-smoke:
	./scripts/android_emulator_smoke.sh

check: backend-test content-check flutter-check
