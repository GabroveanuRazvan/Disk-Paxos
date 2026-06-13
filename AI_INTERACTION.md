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

## TLA+ Model Checking

The project also uses the TLA+ specifications as a formal model of the concurrent Disk Paxos algorithm.

The motivation for using TLA+ is to describe the algorithm independently of the Go implementation. Instead of testing one concrete execution, the TLA+ model defines:

- the possible states of the algorithm;
- the initial states;
- the valid transitions between states;
- invariants that must hold in every reachable state.

TLC then explores the reachable state space generated by those transitions. This is useful for concurrent algorithms because many bugs appear only under unusual interleavings of actions. For Disk Paxos, TLC can explore executions where different processors write, read, get preempted, fail, or recover in many possible orders.

In this project, the key invariant is `HInv6` from `tla/HDiskSynod.tla`. It corresponds to the consensus safety property:

- once a value is chosen, every non-empty processor output must be that same value;
- in the Go implementation, this maps to all successful proposers returning the same decided value.

The TLC model is configured by `tla/MC_HDiskSynod.cfg`. The input values used by the verifier are:

- `N = 3`: three processors;
- `Inputs = {in1, in2}`: two possible proposed values;
- `BallotCountPerProcess = 2`: each processor has two ballot values available in the finite model;
- `Disk = {1, 2}`: two modeled disks;
- `Ballot <- BallotImpl`: the abstract ballot function is replaced with the finite implementation from `MC_HDiskSynod.tla`;
- `IsMajority <- IsMajorityImpl`: the abstract majority predicate is replaced with the finite implementation from `MC_HDiskSynod.tla`;
- `INIT HInit`: model checking starts from `HInit`;
- `NEXT HNext`: every transition must follow `HNext`;
- `INVARIANTS HInv1 HInv2 HInv3 HInv4 HInv6`: TLC checks these invariants in every reachable state.

TLC was installed locally by downloading the official `tla2tools.jar` into `tools/`. Makefile commands were added:

- `make tla-tools`: downloads the TLA+ tools jar;
- `make tla-check`: runs TLC on the Disk Paxos model;
- `make tla-check-output`: runs TLC and writes the output to `tlc-output.txt`.

The TLC command uses parallel workers:

```bash
java -XX:+UseParallelGC -cp ../tools/tla2tools.jar tlc2.TLC -workers auto -config MC_HDiskSynod.cfg MC_HDiskSynod.tla
```

The model can take a long time to run because TLC is not running a single scenario. It systematically generates reachable states and checks the invariants for each state. Even with small inputs, the number of interleavings grows quickly. During the observed run, TLC generated tens of millions of states and millions of distinct states without reporting an invariant violation before the run was stopped manually due to runtime.

For the final project report, the useful TLC output is:

- whether TLC finishes with `Model checking completed. No error has been found.`;
- the number of generated states;
- the number of distinct states;
- whether any invariant, especially `HInv6`, is violated.
