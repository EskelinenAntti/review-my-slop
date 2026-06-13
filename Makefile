.PHONY: build test check

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check:
	go vet ./...
	staticcheck ./...
