.PHONY: build test check

GOCACHE ?= /tmp/review-my-slop-go-cache

build:
	GOCACHE=$(GOCACHE) go build ./cmd/review-my-slop ./cmd/review-my-comments

test:
	GOCACHE=$(GOCACHE) go test -race ./...

check:
	GOCACHE=$(GOCACHE) go vet ./...
	GOCACHE=$(GOCACHE) staticcheck ./...

