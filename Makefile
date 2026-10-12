.PHONY: build test check revive

REVIVE_VERSION := v1.17.0

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check: revive
	go vet ./...
	staticcheck ./...

revive:
	go run github.com/mgechev/revive@$(REVIVE_VERSION) -config revive.toml -set_exit_status ./...
