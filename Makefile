.PHONY: build test check lint

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check:
	golangci-lint run ./...

lint: check
