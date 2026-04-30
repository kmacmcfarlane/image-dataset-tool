.PHONY: gen clear-gen build lint test run

gen:
	go generate ./internal/api/...

clear-gen:
	rm -rf internal/api/gen
	find . -type d -name mocks -exec rm -rf {} + 2>/dev/null || true

build:
	go build -o bin/server ./cmd/server

lint:
	go vet ./...

test:
	go test ./...

run: build
	./bin/server
