# l1ner — First MVP Technical Plan

**Status:** First MVP implemented and validated locally.

## Agreed Decisions

- Platform: Linux.
- Language: Go.
- Execution: run from source on the user's local machine.
- Interface: `l1ner inspect [--recursive] <file-or-directory>`; recursion applies only to directories.
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
4. Validate frame boundaries and parse each frame separately to preserve repeated identifiers. Enumerate all exposed frames. Display decoded text in full, custom tags by name, and all exposed repeated values. Describe artwork and other binary data by type and size where available. Identify unknown frames by frame identifier and payload size; do not print raw binary data.
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

Implemented files:

| File | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Module definition and pinned dependencies. |
| `main.go` | Entry point, argument parsing, file/directory dispatch, help, and exit status. |
| `batch.go` | Directory traversal, sequential batch inspection, per-path errors, and summary counts. |
| `batch_test.go` | Generated directory fixtures, recursion, filtering, symlink handling, continuation, output errors, and integrity checks. |
| `inspect.go` | Read-only file access, tag detection, and tag-header validation. |
| `frames.go` | Frame-header validation, iteration, and library decoding adapter. |
| `output.go` | Stable ordering and readable text/binary frame presentation. |
| `inspect_test.go` | Behavior checks for metadata coverage, output, failures, and file integrity. |

Follow the Google Go style conventions documented in [CONTRIBUTING.md](../CONTRIBUTING.md). File access, tag validation, frame validation, and decoding are separate operations; coordinating functions remain focused.

Keep these in one package initially. Additional package layers are unnecessary for the agreed scope.

Local invocation from the repository root:

```sh
go run . inspect music/tom-lehrer-when-you-are-old-and-gray.mp3
```

Output behavior:

- Print successful inspection output to stdout and errors to stderr.
- Return exit code 0 for successful inspection, including a file with no tags; 1 for read, parse, or unsupported-version errors; 2 for invalid arguments.
- Sort frame identifiers for consistent output and retain every repeated value exposed by the parser.
- Keep original decoded text content, with terminal control characters escaped so metadata cannot issue terminal commands.

## Implementation Coverage and Validation

The CLI uses read-only file access and never calls metadata write APIs. Tag bodies are limited to 64 MiB. Tag-level flags and frame format flags are rejected explicitly because the selected reader does not reliably handle all transformations. CHAP/CTOC frames are summarized as unparsed data rather than passed to the limited nested-frame parser. Unknown frames are summarized; ordinary URL frames are displayed as text. A supported ID3v2 tag takes precedence over a coexisting ID3v1 tag.

The CLI checks the MP3 extension and recognizes ID3 or MPEG headers; it does not validate the entire audio stream.

Local validation passed:

- `go test ./...`: independently constructed ID3v2.3/v2.4 fixtures covering repeated text frames, custom tags, artwork, comments, unknown frames, terminal escaping, absent tags, malformed sizes, unsupported versions/flags, invalid arguments, missing/unreadable files, and unchanged input bytes.
- `go vet ./...` and a CLI build.
- Reference-file inspection compared with an independent byte-level check of embedded frames. The whole-file SHA-256 remained `9f34ee07c75be1c4a35a72293339f92707500bcaeed2e2510ee139ff20bc1d34` before and after inspection.
- Tests and reference inspection ran with `GOPROXY=off` after dependency setup; runtime inspection makes no network requests.

The reference includes a nonstandard `YEAR` frame. It is reported as unparsed data, not decoded as a standard date field.

## Directory Inspection Extension

`batch.go` uses standard-library `filepath.WalkDir` for lexical traversal. Immediate-folder scanning is the default; `--recursive` includes nested folders. Only regular `.mp3` files are selected, case-insensitively. Symlink entries are skipped, and a directory symlink root is rejected explicitly.

Discovery records paths and directory-read errors. Processing stays sequential and delegates each file to the existing read-only inspector and renderer. File errors and scan errors are reported separately; they do not stop other inspections. An output-write failure stops the batch. Successful output ends with counts of inspected files, failed files, and scan errors. The process exits 1 on any failure and 0 for a successful empty scan.

Tests use temporary directories and generated metadata; they cover nesting, uppercase extensions, ignored files, symlink loops, empty scans, per-file failure continuation, unreadable subdirectories, writer failures, and unchanged files.

Packaging, distribution, metadata writing, additional tag formats, and external services remain deferred.
