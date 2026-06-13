# AI Interaction Log

This document records how AI tools were used during the Go implementation of the Disk Paxos project.

## Initial Setup and Implementation

The initial repository setup and Go implementation were created manually, with ChatGPT/Codex used as an assistant in separate sessions for clarification and implementation questions.

The AI assistant was used to discuss:

- the structure of a Go project for Disk Paxos;
- how to represent disks using Redis instances;
- the `Block` structure corresponding to Disk Paxos state;
- concurrent read/write operations over disk quorums;
- the two main Paxos-style phases: scout/prepare and commit/accept.

The implementation decisions were manually reviewed and adapted. Redis was chosen as a practical representation of independent disks, while processors were represented as concurrent Go proposers.

## Formal Specification Reference

The official Disk Paxos TLA+ reference files from the `tlaplus/Examples` repository were added under `tla/`:

- `tla/Synod.tla`
- `tla/DiskSynod.tla`
- `tla/HDiskSynod.tla`
- `tla/MC_HDiskSynod.tla`
- `tla/MC_HDiskSynod.cfg`

These files were used as a formal reference for comparing the Go implementation against the model.

The important correspondence identified during this interaction was:

- `Proc` maps to Go processors/proposers.
- `Disk` maps to Redis instances.
- `DiskBlock` maps to `internal/block.Block`.
- `mbal`, `bal`, and `inp` map to `Mbal`, `Bal`, and `Inp`.
- TLA+ `output[p]` maps to the value returned by `Processor.Propose`.
- The TLA+ safety invariant `HInv6` corresponds to the Go requirement that all successful proposers decide the same value.

## Validation Against the TLA+ Model

During this conversation, the current Go implementation was compared with the TLA+ model.

The main differences found were:

- The TLA+ model has an explicit phase 0 recovery step, while the original Go code started each processor with an empty local block.
- The original Go implementation wrote the proposed input value during phase 1. In the TLA+ model, phase 1 updates the maximum ballot, while the input value is chosen/adopted at the end of phase 1 and written during phase 2.
- The TLA+ model reads other processors' blocks during phase 1 and phase 2. The original Go implementation read all blocks, including the processor's own block.
- The Go simulation did not yet verify the global chosen-value invariant checked by `HInv6`.
- The TLA+ model includes failure/recovery behavior. The Go implementation only had a simple concurrent proposer run.

## Fixes Made After the Comparison

The internal Go implementation was refactored to more closely follow the TLA+ phase structure.

Changes made:

- Added explicit processor phases:
  - `PhaseRecovery`
  - `PhaseScout`
  - `PhaseCommit`
  - `PhaseDone`
- Added phase 0 recovery logic that reads the processor's own persisted blocks from a quorum of disks before starting a new ballot.
- Changed phase 1 so that it no longer writes the fresh proposed value before the value has been accepted.
- Changed phase 1 and phase 2 reads to inspect other processors' blocks.
- Added read helpers for all blocks, own blocks, and other processors' blocks.
- Kept the existing quorum logic from the Go implementation: `DiskCount/2 + 1`.

The command package was intentionally not refactored during this step. It will be updated later when tests and result verification are added.

## Testing Roadmap and Implementation

After aligning the Go implementation more closely with the TLA+ phase structure, the next step was to add tests around the implementation.

The testing approach was discussed interactively. The proposed roadmap was:

- use Go table-driven tests for deterministic unit-level behavior;
- use GoMock for mocked disk clients so processor logic can be tested without Redis;
- use testcontainers for Redis-backed integration tests that are close to the real runtime environment;
- keep integration tests behind a build tag so the regular test suite remains fast and does not require Docker;
- run the Go race detector as part of the project validation.

The suggestion to use GoMock and testcontainers came from the project author. The AI assistant helped turn that testing approach into concrete code and Makefile commands.

During this process, the disk abstraction was first introduced in the producer package. The project author then pointed out the Go convention that interfaces should usually be defined by the consumer, not the producer. Following that suggestion, the `Store` interface was moved into the `processor` package, where it is consumed by the Paxos processor logic.

The resulting test structure is:

- unit/table tests for `block`, `config`, `disk`, `logger`, and `processor`;
- processor tests using generated GoMock mocks from the consumer-owned `processor.Store` interface;
- a Redis integration test for the disk client using testcontainers;
- Makefile targets for regular tests, table/unit tests, race tests, integration tests, and all tests.

The AI assistant helped with:

- defining the consumer-owned `processor.Store` interface;
- adding the `go:generate` directive for GoMock;
- generating the `mocks` package;
- writing processor tests for ballot generation, recovery, quorum failure, and value adoption;
- writing the Redis testcontainers integration test;
- adding `make test`, `make test-table`, `make test-race`, `make test-integration`, and `make test-all`;
- adding `-count=1` so test commands do not use cached results.
