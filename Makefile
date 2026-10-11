.PHONY: build test check critic

GO_CRITIC_VERSION := v0.14.4

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check:
	go vet ./...
	staticcheck ./...
	$(MAKE) critic

critic:
	go run github.com/go-critic/go-critic/cmd/go-critic@$(GO_CRITIC_VERSION) check -enableAll -disable '#experimental' ./...
