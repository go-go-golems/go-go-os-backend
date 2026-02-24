.PHONY: gifs proto ts-proto codegen frontend-check buf-lint test build install ci ui-build dev-backend dev-frontend dev-tmux

all: gifs

VERSION=v0.1.14
GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target

AGENT_UI_DIR=agent-ui-system
AGENT_UI_TSC_BIN=$(AGENT_UI_DIR)/node_modules/.bin/tsc
AGENT_UI_TS_PROTO_BIN=$(AGENT_UI_DIR)/node_modules/.bin/protoc-gen-ts_proto

TAPES=$(wildcard doc/vhs/*tape)
gifs: $(TAPES)
	for i in $(TAPES); do vhs < $$i; done

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:latest golangci-lint run -v

lint:
	golangci-lint run -v

lintmax:
	golangci-lint run -v --max-same-issues=100

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude-generated -exclude=G101,G304,G301,G306 -exclude-dir=.history -exclude-dir=proto/generated/go ./...

govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

test:
	go test ./... -count=1


build: codegen ui-build
	go build -tags embed ./...

ci: buf-lint test

DEV_API_ADDR ?= :3001
DEV_UI_PORT ?= 3000

goreleaser:
	goreleaser release $(GORELEASER_ARGS) $(GORELEASER_TARGET)

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

bump-glazed:
	go get github.com/go-go-golems/glazed@latest
	go get github.com/go-go-golems/clay@latest
	go get github.com/go-go-golems/plz-confirm@latest
	go get github.com/go-go-golems/geppetto@latest
	go get github.com/go-go-golems/pinocchio@latest
	go mod tidy

