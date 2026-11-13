export PATH := $(shell pwd)/.tmp/protoc/bin:$(PATH)
export PROTOC_VERSION := 22.0

.PHONY: ci
ci: deps lint test

.PHONY: deps
deps:
	go get -d -v -t ./...
	go mod tidy

.PHONY: updatedeps
updatedeps:
	go get -d -v -t -u -f ./...
	go mod tidy

.PHONY: install
install:
	go install ./...

.PHONY: generate
generate: .tmp/protoc/bin/protoc
	@go install ./cmd/protoc-gen-grpchan
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1.0
	go generate ./...

.PHONY: lint
lint:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
	golangci-lint run ./...

.PHONY: test
test:
	go test -race ./...

.tmp/protoc/bin/protoc: ./Makefile ./download_protoc.sh
	./download_protoc.sh
