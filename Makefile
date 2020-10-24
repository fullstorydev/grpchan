# TODO: run golint and errcheck, but only to catch *new* violations and
# decide whether to change code or not (e.g. we need to be able to whitelist
# violations already in the code). They can be useful to catch errors, but
# they are just too noisy to be a requirement for a CI -- we don't even *want*
# to fix some of the things they consider to be violations.
.PHONY: ci
ci: deps checkgofmt vet staticcheck ineffassign predeclared test

.PHONY: deps
deps:
	go get -d -v -t ./...

.PHONY: updatedeps
updatedeps:
	go get -d -v -t -u -f ./...

.PHONY: install
install:
	go install ./...

.PHONY: checkgofmt
checkgofmt:
	gofmt -s -l .
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		exit 1; \
	fi

.PHONY: generate
generate:
	@GO111MODULE=on go install ./cmd/protoc-gen-grpchan
	@GO111MODULE=on go install github.com/golang/protobuf/protoc-gen-go
	go generate ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: staticcheck
staticcheck:
	@GO111MODULE=on go install honnef.co/go/tools/cmd/staticcheck
	staticcheck ./...

.PHONY: ineffassign
ineffassign:
	@GO111MODULE=on go install github.com/gordonklaus/ineffassign
	ineffassign .

.PHONY: predeclared
predeclared:
	@GO111MODULE=on go install github.com/nishanths/predeclared
	predeclared .

# Intentionally omitted from CI, but target here for ad-hoc reports.
.PHONY: golint
golint:
	@GO111MODULE=on go install golang.org/x/lint/golint
	golint -min_confidence 0.9 -set_exit_status ./...

# Intentionally omitted from CI, but target here for ad-hoc reports.
.PHONY: errcheck
errcheck:
	@GO111MODULE=on go install github.com/kisielk/errcheck
	errcheck ./...

.PHONY: test
test:
	go test -race ./...
