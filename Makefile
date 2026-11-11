.PHONY: test build up down bench

test:
	go test -race ./...

build:
	go build -o bin/ ./cmd/...

up:
	docker compose up -d --build

down:
	docker compose down

bench:
	go run ./cmd/bench -n 2000 -c 50 -keys 500
