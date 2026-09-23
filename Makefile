.PHONY: build test check ast-size

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check:
	go vet ./...
	staticcheck ./...

ast-size:
	@go run ./scripts/ast_size.go
