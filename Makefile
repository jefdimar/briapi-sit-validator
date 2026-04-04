.PHONY: run build test lint docker vendor

run:
	go run -mod=vendor ./cmd/server

build:
	go build -mod=vendor -o bin/sit-validator ./cmd/server

test:
	go test -mod=vendor ./...

lint:
	golangci-lint run

vendor:
	GONOSUMDB=* GOPROXY="file://$$(go env GOPATH)/pkg/mod/cache/download,off" go mod vendor

docker:
	docker build -t briapi-sit-validator .
