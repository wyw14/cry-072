.PHONY: run test test-race vet web-install web-test web-build migrate seed verify

run:
	go run ./cmd/server

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

web-install:
	cd web && npm ci

web-test:
	cd web && npm test

web-build:
	cd web && npm run build

migrate:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/001_initial.sql

seed:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f migrations/002_seed_demo.sql

verify: test test-race vet web-test web-build
