.PHONY: run clear up down fumpt test test-table test-race test-integration test-all


run:
	go run cmd/concurrent_proposers/main.go

# Clear the disks
clear:
	go run cmd/clear_disks/main.go

# Start the redis disks
up:
	docker compose up -d

# Stop the redis disks
down:
	docker compose down

fumpt:
	gofumpt -w .

# Run the regular unit/table tests.
test:
	go test -count=1 ./...

# Alias for the regular table-driven/unit test suite.
test-table: test

# Run tests with the Go race detector.
test-race:
	go test -count=1 -race ./...

# Run tests that require external services through testcontainers.
test-integration:
	go test -count=1 -tags=integration ./...

# Run all local and integration test suites.
test-all: test test-race test-integration
