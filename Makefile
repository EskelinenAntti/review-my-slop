.PHONY: build test check ast-size coverage metrics

COVERAGE_PROFILE := /tmp/review-my-slop-coverage.out
COVERAGE_REPORT := /tmp/review-my-slop-coverage.txt

build:
	go build ./cmd/review-my-slop

test:
	go test -race ./...

check:
	go vet ./...
	staticcheck ./...

ast-size:
	@go run ./scripts/ast_size.go

coverage:
	@go test -coverprofile=$(COVERAGE_PROFILE) ./...
	@go tool cover -func=$(COVERAGE_PROFILE) -o=$(COVERAGE_REPORT)
	@tail -n 1 $(COVERAGE_REPORT)
	@rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)

metrics:
	@echo "essential-ast:"
	@$(MAKE) --no-print-directory ast-size
	@echo "coverage:"
	@$(MAKE) --no-print-directory coverage
