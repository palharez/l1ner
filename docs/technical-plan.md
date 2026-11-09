# l1ner — First MVP Technical Plan

**Status:** Planning; implementation has not started.

## Agreed Decisions

- Platform: Linux.
- Language: Go.
- Execution: run from source on the user's local machine.
- Interface: `l1ner inspect <file-path>`.
- Metadata reader: `github.com/bogem/id3v2/v2`, pinned to `v2.1.4` for initial implementation.
- Supported tags: ID3v2.3 and ID3v2.4 in MP3 files.
- File access: read-only; no external services or network lookups.

The [product vision](vision.md) defines the scope and acceptance criteria. The [question tracker](open-questions.md) records decisions and deferred questions.

## Library Rationale

The selected library exposes frames through `AllFrames()` and provides types for text, custom text, pictures, and unknown frames. This fits the goal of exposing available metadata without relying on a fixed list of title/artist getters.

`dhowden/tag` was considered for its broader format coverage. The selected library favors direct frame inspection for this MP3-focused MVP. No claim is made that it decodes every possible MP3 metadata field.

Sources consulted during selection:

- [Selected library API](https://pkg.go.dev/github.com/bogem/id3v2/v2).
- [Selected library repository](https://github.com/bogem/id3v2), which redirects to `n10v/id3v2`; the Go module proxy confirms the original module path for release `v2.1.4`.
- [Alternative: dhowden/tag](https://github.com/dhowden/tag).

## Intended Read Path

1. Validate the invocation and open the input file read-only.
2. Detect the tag version before parsing. Report unsupported versions explicitly; do not interpret a parser's empty result alone as proof that metadata is absent. Account for legacy ID3v1 tags when distinguishing unsupported metadata from no metadata.
3. Parse supported tags using the library's reader API, without calling save or write methods.
4. Enumerate all exposed frames. Display decoded text in full, custom tags by name, and all exposed repeated values. Describe artwork and other binary data by type and size where available. Identify unknown frames by frame identifier and payload size; do not print raw binary data.
5. Print the file path followed by `Tag: Value` lines. Use `No metadata found` only when metadata is absent. Report malformed tags or read failures clearly.
6. Close the file without changing it.

## Validation Before Acceptance

- Compare terminal output with independently inspected embedded values in the reference MP3.
- Check text, custom fields, repeated values, artwork descriptions, and unknown-frame summaries using suitable fixtures; do not assume one reference file covers all cases.
- Compare a whole-file checksum before and after inspection.
- Exercise missing metadata, missing files, unreadable files, malformed tags, and unsupported versions.
- Verify runtime inspection works without network access; initial Go dependency setup may require a download.

## Dependency and Environment Verification

Checked on 2026-11-09:

- The [Go module proxy](https://proxy.golang.org/github.com/bogem/id3v2/v2/@latest) reports `v2.1.4` as the latest release for the selected module path.
- Its [module declaration](https://proxy.golang.org/github.com/bogem/id3v2/v2/@v/v2.1.4.mod) declares `github.com/bogem/id3v2/v2`, Go 1.13, and `golang.org/x/text v0.3.8`. The declared Go version is not a recommendation to install that old toolchain.
- The [official Go download metadata](https://go.dev/dl/?mode=json) lists Go 1.27.3 as stable. Use that as the initial development toolchain target; the selected library's upstream tests pass with this toolchain.

The selected library's upstream `go test ./...` passed with network access disabled after dependency setup. Tests were run from a writable temporary copy because the upstream tests modify their own fixtures and cannot run directly in Go's read-only module cache. This verifies the dependency/toolchain combination, not the unimplemented l1ner CLI.

## Minimal CLI Structure

Use Go's standard library for argument handling and terminal output; no CLI framework is needed for this single-command MVP.

Planned files (not yet created):

| File | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Module definition and pinned dependencies. |
| `main.go` | Entry point, `inspect` argument validation, help, and exit status. |
| `inspect.go` | Read-only file access, tag-version checks, parsing, and frame enumeration. |
| `output.go` | Stable ordering and readable text/binary frame presentation. |
| `inspect_test.go`, `output_test.go` | Behavior checks for metadata coverage, output, failures, and file integrity. |

Keep these in one package initially. Additional package layers are unnecessary for the agreed scope.

Planned local invocation from the repository root:

```sh
go run . inspect music/tom-lehrer-when-you-are-old-and-gray.mp3
```

Proposed output behavior:

- Print successful inspection output to stdout and errors to stderr.
- Return exit code 0 for successful inspection, including a file with no tags; 1 for read, parse, or unsupported-version errors; 2 for invalid arguments.
- Sort frame identifiers for consistent output and retain every repeated value exposed by the parser.
- Keep original decoded text content, with terminal control characters escaped so metadata cannot issue terminal commands.

## Remaining Technical Details

- Validate parser behavior for repeated values and unsupported frame features; document limitations explicitly.
- Validate the proposed output and error behavior during implementation.
- Execute the planned source-based run command and document the verified procedure.

These details remain planning work. Packaging, distribution, metadata writing, additional tag formats, and external services are deferred.
