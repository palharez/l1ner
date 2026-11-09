# Contributing to l1ner

The first MVP inspects existing metadata in one local MP3 file on Linux. Start with the [README](README.md), [product vision](docs/vision.md), and [technical plan](docs/technical-plan.md). Upcoming work is tracked in [TODO.md](TODO.md).

## Development setup

Use Linux and Go 1.27.3, the current development target. From the repository root:

```sh
go version
go mod download
go run . --help
```

Local audio belongs in the ignored `music/` directory. Tests must run without personal music files or external services; build metadata fixtures directly in tests.

## Go style

We follow the [Google Go Style Guide](https://google.github.io/styleguide/go/guide), with its [style decisions](https://google.github.io/styleguide/go/decisions) and [best practices](https://google.github.io/styleguide/go/best-practices). The [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) provide additional review guidance.

Apply these principles to this project:

- Prioritize clarity and simplicity. Keep each function focused on a coherent responsibility and use names that explain its purpose.
- Separate file access, format validation, frame decoding, and presentation. Keep coordinating functions readable without requiring readers to follow binary-format details.
- Extract helpers when they clarify a meaningful operation. There is no fixed function-length limit; avoid fragmenting straightforward code into trivial wrappers.
- Return errors explicitly and handle failures early. Add context when it helps identify a failed operation; preserve underlying errors with `%w` when wrapping.
- Explain why a workaround or unusual algorithm is necessary. Avoid comments that simply repeat the code.
- Keep APIs and types small. Add interfaces or package boundaries when actual use requires them.
- Format Go code with `gofmt`. Keep tests readable and focused on observable behavior, including errors and file integrity.

These project rules apply to both human contributors and coding agents. Agent-specific scope reminders are in [AGENTS.md](AGENTS.md).

## Verification

For Go code changes, format changed files and run:

```sh
gofmt -w *.go
go test ./...
go vet ./...
go build -o bin/l1ner .
```

Tests are organized by behavior: `inspect_test.go` uses generated MP3 tag fixtures to exercise reading, validation, and file integrity; `output_test.go` supplies in-memory metadata to check terminal rendering and writer failures directly. Neither suite requires `music/`.

To focus on output behavior while developing:

```sh
go test -run 'TestDisplay|TestRunOutputFailure' -v ./...
```

For refactors, preserve existing output, error messages, exit codes, and unchanged input files. Add regression coverage when changing parsing boundaries or fixing a bug. Initial dependency setup may require network access; inspection itself must stay offline.

## Continuous integration

[CI](.github/workflows/ci.yml) runs formatting checks, dependency verification, tests, vet, and a build in one standard Ubuntu job. It reads the Go version from `go.mod`. Tests use generated fixtures and do not require `music/`.

Runs start for pull requests, pushes to `main`, and manual dispatches. New runs cancel older runs for the same pull request or branch. Each job has a 10-minute timeout; there is no build matrix, schedule, cache storage, or artifact upload.

Standard hosted runners are [free for public repositories](https://docs.github.com/en/billing/concepts/product-billing/github-actions). Private repositories draw from the owner's included allowance; this workflow does not change account budgets or guarantee that private usage stays within that allowance.

## Issues and pull requests

For bugs, describe the command, expected and actual behavior, Go version, and relevant tag version. Prefer a small generated reproducer over uploading personal audio.

For feature proposals, describe the user need and acceptance criteria before expanding the MVP scope. Consult [Open Questions](docs/open-questions.md) for deferred decisions.

Keep pull requests focused. Explain the problem, the resulting behavior, and the checks performed. Include documentation updates when behavior or limitations change.
