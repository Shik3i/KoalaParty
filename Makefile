.PHONY: dev backend frontend test verify build docker

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
	cd backend && gofmt -w . && go vet ./... && go test ./...
	cd frontend && npm run check && npm run lint && npm test -- --run && npm run build

build:
	cd frontend && npm run build
	cd backend && go build ./cmd/server

docker:
	docker compose build

