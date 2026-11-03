.PHONY: run build test cover integration proto docker-up docker-down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

# -race needs cgo (gcc); available by default on Linux/macOS.
test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

integration:
	go test -tags=integration ./test/...

proto:
	buf lint && buf generate

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
