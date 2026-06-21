# Disk-Paxos

This repository contains an implementation of the [Disk-Paxos](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2003-35.pdf) consensus algorithm in Go. 
Instead of traditional distributed storage like shared disks or filesystems, this implementation utilizes multiple **Redis instances** as independent virtual disks.

## Overview

The Disk-Paxos algorithm allows a group of processors (proposers) to reach a consensus on a single value, using a set of independent disks for storage. The protocol does not rely on message passing between the processors directly, but rather uses read/write operations on the disks.

In this implementation:
- **Disks** are represented by separate Redis instances.
- **Processors** are Go routines that run the Disk-Paxos algorithm.

The algorithm progresses in two main phases (similarly to classic Paxos):
1. **Phase 1 (The Scout):** A processor attempts to become the leader by writing its block to the disks with a new, higher ballot number. It then reads back from the disks to ensure no other processor has preempted it and checks if it needs to adopt an already accepted value.
2. **Phase 2 (The Commit):** The processor writes its block (either its own proposed value or the adopted value) to the disks. It reads back once more to confirm no higher ballot numbers have been written by others. If successful, consensus is reached.

Fault tolerance is achieved by ensuring that all operations (reads and writes) are performed across all disks concurrently, and only require a majority (quorum) to succeed.

## Implementation Details

### Architecture
* **`cmd/`**: Contains the executable entry points.
  * `concurrent_proposers/main.go`: The main entry point that runs the simulation with multiple concurrent processors trying to reach consensus.
  * `clear_disks/main.go`: A utility script to clear the state (blocks) on all Redis disks.
* **`internal/disk/`**: Contains the `Client` logic which wraps the Redis operations. It exposes methods to read, write, and delete blocks on the underlying Redis instances.
* **`internal/processor/`**: Contains the core logic for the Paxos algorithm. The `Processor` struct executes Phase 1 and Phase 2.
* **`internal/block/`**: Defines the data structure `Block` that is written to the disks.
* **`tla/`**: Contains the TLA+ Disk Paxos specification files used as the formal reference model.
* **`docker-compose.yaml`**: Used to spin up multiple Redis instances to act as our independent disks.

### Block Structure
Each block written to Redis contains:
- `Mbal`: The maximum ballot number seen by the processor.
- `Bal`: The ballot number at which the current value was accepted.
- `Inp`: The proposed value.

### Redis as a Disk
Each Redis instance maps block keys to specific processors (e.g., `block-1`, `block-2`). The `Client` uses atomic operations to write the serialized JSON representation of the `Block` to the Redis disk.

## How to Run

### 1. Prerequisites
- Docker and Docker Compose
- Go 1.20+

### 2. Start the Disks (Redis Instances)
To start the Redis instances used as disks, run:
```bash
make up
```
This will spin up multiple Redis instances defined in your `docker-compose.yaml`. (You may need to configure environment variables like `DISK_COUNT`).

### 3. Run the Consensus Simulation
To run the main simulation where multiple concurrent proposers attempt to reach consensus:
```bash
make run
```
This will start multiple processors (based on your configuration). They will all attempt to propose their values and eventually agree on a single value, logging the process.

### 4. Clear the Disks
If you want to clear the Redis instances and reset the state between runs:
```bash
make clear
```

### 5. Stop the Disks
To stop the Redis containers:
```bash
make down
```

### 6. Run Tests
Run the regular unit/table tests:
```bash
make test
```

Run the same regular test suite through the table-test alias:
```bash
make test-table
```

Run the Go race detector:
```bash
make test-race
```

Run the intentionally broken race-condition demonstration:
```bash
make test-race-bug
```
This enables the `INJECT_RACE_BUG` config flag and includes the tagged race-demo test. The command is expected to fail with `WARNING: DATA RACE`.

Run Redis integration tests through testcontainers:
```bash
make test-integration
```
This requires Docker access.

Run all test suites:
```bash
make test-all
```

### 7. Run TLA+ Model Checking
Download the TLA+ tools jar:
```bash
make tla-tools
```

Run TLC on the Disk Paxos model in `tla/`:
```bash
make tla-check
```

Run TLC and save the verifier output to `tlc-output.txt`:
```bash
make tla-check-output
```

The TLC run uses the configuration in `tla/MC_HDiskSynod.cfg` and can take a long time because it explores many possible concurrent interleavings.
