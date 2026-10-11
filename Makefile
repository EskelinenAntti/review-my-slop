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

# Keep value-based TUI models, while flagging parameters of 512 bytes or more.
critic:
	go run github.com/go-critic/go-critic/cmd/go-critic@$(GO_CRITIC_VERSION) check -enableAll -disable '#experimental' -@hugeParam.sizeThreshold 512 ./...
