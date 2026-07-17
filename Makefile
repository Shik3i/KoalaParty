.PHONY: dev backend frontend test verify build docker release-check

dev:
	docker compose up --build

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

verify:
	cd backend && gofmt -w . && go vet ./... && go run golang.org/x/vuln/cmd/govulncheck@latest ./... && go test -race -count=1 ./...
	cd frontend && npm run check && npm run lint && npm test -- --run && npm run build && npm audit --audit-level=high
	node --test scripts/*.test.mjs

build:
	cd frontend && npm run build
	cd backend && go build ./cmd/server

docker:
	docker compose build

release-check:
	node scripts/verify-release.mjs $(TAG)
