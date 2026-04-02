.PHONY: run build test lint docker vendor

run:
	go run ./cmd/server

build:
	go build -o bin/sit-validator ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

vendor:
	GONOSUMDB=* GOPROXY="file://$$(go env GOPATH)/pkg/mod/cache/download,off" go mod vendor

docker:
	docker build -t briapi-sit-validator .
