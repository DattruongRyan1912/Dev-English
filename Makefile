.PHONY: backend-run backend-test content-check flutter-check docker-dev-up docker-dev-down docker-dev-logs db-migrate db-backup db-restore check

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

check: backend-test content-check flutter-check
