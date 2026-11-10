# Contributing to l1ner

The CLI inspects local MP3 metadata on Linux and offers MusicBrainz lookup and interactive preview for one file. Start with the [README](README.md), [product vision](docs/vision.md), and [technical plan](docs/technical-plan.md). Upcoming work is tracked in [TODO.md](TODO.md).

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

Tests are organized by behavior: `inspect_test.go` uses generated MP3 tag fixtures to exercise reading, validation, and file integrity; `output_test.go` supplies in-memory metadata to check terminal rendering and writer failures directly. `batch_test.go` exercises directory traversal, filtering, recursion, summaries, failure continuation, and file integrity using temporary directories. `json_test.go` checks the public JSON format, single/batch results, partial failures, data fidelity, and write errors. No suite requires `music/`.

`lookup_test.go` uses generated tags to check extraction, CLI behavior, and file integrity. `musicbrainz_test.go` uses a local HTTP server to check request encoding, the rate limit, and provider responses. The [live sample checks](docs/sample-validation.md#musicbrainz-lookup) are manual.

`preview_test.go` uses generated tags and a local HTTP server. Linux pseudo-terminal tests exercise the keyboard workflow and verify terminal restoration and file integrity.

To focus on output behavior while developing:

```sh
go test -run 'TestDisplay|TestRunOutputFailure' -v ./...
```

To focus on malformed-input checks, valid padding/nested-data handling, and the 64 MiB boundary:

```sh
go test -run 'TestMalformedFrames|TestPaddingAndNestedData|TestTagSizeLimit' -v ./...
```

These inspection tests cover both supported ID3 versions, assert exact stdout/stderr and exit status, and compare file size, permissions, and a whole-file checksum before and after inspection. Size-limit cases use sparse temporary files; no large fixture is committed.

For refactors, preserve existing output, error messages, exit codes, and unchanged input files. Add regression coverage when changing parsing boundaries or fixing a bug. Initial dependency setup may require network access; inspection itself must stay offline.

### Parser fuzzing

`FuzzInspectTag` uses generated ID3v2.3 and ID3v2.4 fixtures to exercise tag reading and frame decoding in memory. It accepts parse errors for malformed inputs and checks that the parser does not panic. `go test ./...` runs the seed inputs. To run a one-minute fuzzing session locally, use:

```sh
go test -run='^$' -fuzz='^FuzzInspectTag$' -fuzztime=1m -parallel=2 .
```

## Continuous integration

[CI](.github/workflows/ci.yml) runs formatting checks, dependency verification, tests, vet, and a build in one standard Ubuntu job. It reads the Go version from `go.mod`. Tests use generated fixtures and do not require `music/`.

Runs start for pull requests, pushes to `main`, and manual dispatches. New runs cancel older runs for the same pull request or branch. Each job has a 10-minute timeout; there is no build matrix, schedule, cache storage, or artifact upload.

Standard hosted runners are [free for public repositories](https://docs.github.com/en/billing/concepts/product-billing/github-actions). Private repositories draw from the owner's included allowance; this workflow does not change account budgets or guarantee that private usage stays within that allowance.

## Issues and pull requests

For bugs, describe the command, expected and actual behavior, Go version, and relevant tag version. Prefer a small generated reproducer over uploading personal audio.

For feature proposals, describe the user need and acceptance criteria before expanding the MVP scope. Consult [Open Questions](docs/open-questions.md) for deferred decisions.

Keep pull requests focused. Explain the problem, the resulting behavior, and the checks performed. Include documentation updates when behavior or limitations change.

## Commit messages

Use [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) for new commits. Write the subject as `type(scope): summary` with an optional scope and a short, imperative summary. Use `feat` for a new capability, `fix` for a bug fix, `docs` for documentation, `test` for test-only work, `refactor` for restructuring that preserves behavior, `ci` for workflow changes, and `chore` for other maintenance.

```text
feat(lookup): add single-file lookup command
fix(id3): reject oversized frame sizes
test: fuzz tag parsing
docs(contributing): adopt Conventional Commits
```

Use the body to explain the reason for a change when the subject needs context. Mark breaking changes with `!` or a `BREAKING CHANGE:` footer as specified by the convention.

## Feature workflow

1. **Read the project context.** Check `git status --short --branch` first. Read [AGENTS.md](AGENTS.md), this guide, the [README](README.md), [TODO.md](TODO.md), and the relevant `docs/` pages. Preserve existing edits and check the branch history before choosing a base.
2. **Agree on the milestone.** Define the user-facing behavior, data boundaries, and acceptance criteria before implementing. Record decisions in [Open Questions](docs/open-questions.md). Break the work into ordered tasks in [TODO.md](TODO.md), each with a verification step and a commit boundary.
3. **Work on a milestone branch.** Use the branch name agreed with the user and the repository's default branch as the base. For each task, make the smallest complete change, run its checks, inspect the diff, and commit the task's files together using the [commit convention](#commit-messages). Keep each commit buildable and leave unrelated work out of it.
4. **Verify the complete workflow.** Exercise the feature through its normal entry point and run the checks in [Verification](#verification) for Go changes. Update usage docs, record any manual validation, and check off tasks as they pass. For external integrations, use a local test server in automated tests and record live checks separately.
5. **Open a pull request.** Push the verified milestone branch and use `gh pr create` with `main` as the base. Include the feature summary, check results, and any limitations or open decisions. Share the PR link so the user can review and merge it.
