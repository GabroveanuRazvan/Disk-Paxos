TLA_TOOLS := tools/tla2tools.jar
TLC_WORKERS ?= auto

.PHONY: run clear up down fumpt test test-table test-race test-integration test-all tla-tools tla-check tla-check-output


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

# Download the TLA+ tools jar used to run TLC.
tla-tools:
	mkdir -p tools
	curl -L -o $(TLA_TOOLS) https://github.com/tlaplus/tlaplus/releases/latest/download/tla2tools.jar

# Run TLC against the Disk Paxos model.
tla-check: $(TLA_TOOLS)
	cd tla && java -XX:+UseParallelGC -cp ../$(TLA_TOOLS) tlc2.TLC -workers $(TLC_WORKERS) -config MC_HDiskSynod.cfg MC_HDiskSynod.tla

# Run TLC and save the output for the report.
tla-check-output: $(TLA_TOOLS)
	cd tla && java -XX:+UseParallelGC -cp ../$(TLA_TOOLS) tlc2.TLC -workers $(TLC_WORKERS) -config MC_HDiskSynod.cfg MC_HDiskSynod.tla | tee ../tlc-output.txt

$(TLA_TOOLS):
	$(MAKE) tla-tools
