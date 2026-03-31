.PHONY: run build test lint docker

run:
	go run ./cmd/server

build:
	go build -o bin/sit-validator ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

docker:
	docker build -t briapi-sit-validator .
