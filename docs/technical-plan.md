# l1ner — First MVP Technical Plan

**Status:** Single-file and directory inspection implemented and validated locally.

## Agreed Decisions

- Platform: Linux.
- Language: Go.
- Execution: run from source on the user's local machine.
- Interface: `l1ner inspect [--recursive] [--json] <file-or-directory>`; recursion applies only to directories.
- Metadata reader: `github.com/bogem/id3v2/v2`, pinned to `v2.1.4` for initial implementation.
- Supported tags: ID3v2.3 and ID3v2.4 in MP3 files.
- File access: read-only; no external services or network lookups.

The [product vision](vision.md) defines the scope and acceptance criteria. The [question tracker](open-questions.md) records decisions and deferred questions.

## Library Rationale

The selected library provides typed frames for text, custom text, pictures, and unknown data. The implementation validates frame boundaries and wraps each frame in its own tag for `ParseReader`, then retrieves it with `GetFrames`. This fits the goal of exposing available metadata without relying on a fixed list of title/artist getters.

`dhowden/tag` was considered for its broader format coverage. The selected library favors direct frame inspection for this MP3-focused MVP. No claim is made that it decodes every possible MP3 metadata field.

Sources consulted during selection:

- [Selected library API](https://pkg.go.dev/github.com/bogem/id3v2/v2).
- [Selected library repository](https://github.com/bogem/id3v2), which redirects to `n10v/id3v2`; the Go module proxy confirms the original module path for release `v2.1.4`.
- [Alternative: dhowden/tag](https://github.com/dhowden/tag).

## Implemented Read Path

1. Validate the invocation and open the input file read-only.
2. Detect the tag version before parsing. Report unsupported versions explicitly; do not interpret a parser's empty result alone as proof that metadata is absent. Account for legacy ID3v1 tags when distinguishing unsupported metadata from no metadata.
3. Parse supported tags using the library's reader API, without calling save or write methods.
4. Validate frame boundaries and parse each frame separately to preserve repeated identifiers. Enumerate all exposed frames. Display decoded text in full, custom tags by name, and all exposed repeated values. Describe artwork and other binary data by type and size where available. Identify unknown frames by frame identifier and payload size; do not print raw binary data.
5. Print the file path followed by `Tag: Value` lines. Use `No metadata found` only when metadata is absent. Report malformed tags or read failures clearly.
6. Close the file without changing it.

## Validation Criteria

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

The selected library's upstream `go test ./...` passed with network access disabled after dependency setup. Tests were run from a writable temporary copy because the upstream tests modify their own fixtures and cannot run directly in Go's read-only module cache. That check verified the dependency/toolchain combination. The project tests and CLI validation are recorded below.

## CLI Structure

Use Go's standard library for argument handling and output. Keep the root entry point for `go run .` and `go build -o bin/l1ner .`.

| Path | Responsibility |
| --- | --- |
| `main.go` | Pass command-line arguments and terminal streams to the CLI and return its exit status. |
| `internal/cli` | Command dispatch, inspection, rendering, MusicBrainz search, interactive review, and approved tag writes. Tests stay beside the workflows they exercise. |
| `internal/identify` | Local audio fingerprinting, AcoustID requests, and recording candidate normalization. |
| `internal/ratelimit` | Shared Linux file locking and request timing for the two services. |
| `go.mod`, `go.sum` | Module definition and pinned dependencies. |

Within `internal/cli`, `inspect.go` handles read-only file access and tag headers, `frames.go` handles frame validation and decoding, `batch.go` handles discovery and per-file results, and `output.go` and `json.go` handle presentation. Follow the [Go style conventions](../CONTRIBUTING.md#go-style).

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

`inspect` uses read-only file access. Tag bodies are limited to 64 MiB. Tag-level flags and frame format flags are rejected explicitly because the selected reader does not reliably handle all transformations. CHAP/CTOC frames are summarized as unparsed data rather than passed to the limited nested-frame parser. Unknown frames are summarized; ordinary URL frames are displayed as text. A supported ID3v2 tag takes precedence over a coexisting ID3v1 tag.

The CLI checks the MP3 extension and recognizes ID3 or MPEG headers; it does not validate the entire audio stream.

Local validation passed:

- `go test ./...`: independently constructed ID3v2.3/v2.4 fixtures covering repeated text frames, custom tags, artwork, comments, unknown frames, terminal escaping, absent tags, malformed sizes, unsupported versions/flags, invalid arguments, missing/unreadable files, and unchanged input bytes.
- `go vet ./...` and a CLI build. CI runs these checks alongside formatting, module verification, and tests; remote run status must be checked on GitHub separately.
- Reference-file inspection compared with an independent byte-level check of embedded frames. The whole-file SHA-256 remained `9f34ee07c75be1c4a35a72293339f92707500bcaeed2e2510ee139ff20bc1d34` before and after inspection.
- Tests and reference inspection ran with `GOPROXY=off` after dependency setup; runtime inspection makes no network requests.

The [sample-validation notes](sample-validation.md) record the 20-file local collection and its limits; the [manifest](mp3-samples.tsv) records provenance and checksums.

The reference includes a nonstandard `YEAR` frame. It is reported as unparsed data, not decoded as a standard date field.

## Directory Inspection Extension

`batch.go` uses standard-library `filepath.WalkDir` for lexical traversal. Immediate-folder scanning is the default; `--recursive` includes nested folders. Only regular `.mp3` files are selected, case-insensitively. Symlink entries are skipped, and a directory symlink root is rejected explicitly.

Discovery records paths and directory-read errors. Processing stays sequential and delegates each file to the existing read-only inspector and renderer. File errors and scan errors are reported separately; they do not stop other inspections. An output-write failure stops the batch. Successful output ends with counts of inspected files, failed files, and scan errors. The process exits 1 on any failure and 0 for a successful empty scan.

Tests use temporary directories and generated metadata; they cover nesting, uppercase extensions, ignored files, symlink loops, empty scans, per-file failure continuation, unreadable subdirectories, writer failures, and unchanged files.

## JSON Output Extension

The optional `--json` flag switches rendering to one versioned JSON report without changing metadata-reader coverage or directory discovery rules. `json.go` collects per-file results and directory scan errors, maps exposed frames to structured summaries, and encodes with Go's standard `encoding/json` package. No new dependency is needed. Text rendering remains unchanged.

The report schema and failure behavior are documented in [JSON output](json-output.md). Reports contain metadata summaries rather than raw image or unknown-frame payloads. Results are accumulated in memory before encoding; output is not streamed per file. File/scan failures remain machine-readable in the report with exit status 1; invalid arguments produce stderr and exit 2, and write errors produce stderr and exit 1.

Generated tests cover single files, shallow/recursive directories, mixed success/failure, empty scans, missing/malformed input, scan errors, writer failures, typed metadata, Unicode/control escaping, repeated values, and unchanged files. Existing text-mode regression tests remain in place.

## MusicBrainz Lookup Extension

`lookup.go` reuses the inspector to extract one unambiguous title and artist, then renders the recording candidates as escaped terminal text. `musicbrainz.go` makes one bounded HTTPS request with Go's standard HTTP and JSON libraries. It keeps candidates in service order and caps the list at five. A Linux file lock and the marker's modification time coordinate the request limit across invocations that use the same user cache.

Tests exercise the command with generated MP3s and a local HTTP server. The [sample-validation notes](sample-validation.md#musicbrainz-lookup) record the live catalog results and limits.

## Audio identification

`identify` validates one regular MP3 and invokes `fpcalc` with the MP3 demuxer, JSON output, and an absolute path. The process has a 30-second timeout and bounded output. The CLI keeps the fingerprint and results in memory and posts the fingerprint and whole-file duration to AcoustID with `meta=recordings`. The HTTPS request has a 10-second timeout and declines redirects. The two services use independent rate markers.

The CLI displays distinct MusicBrainz recording IDs in descending AcoustID score order. Repeated IDs keep the highest score. `identify` ends after showing candidates. Interactive `preview --from-audio` and `fix --from-audio` send the same request for one song and use candidates with both title and artist in the picker. Those commands pass the opened file to `fpcalc` via an inherited descriptor so a path swap cannot change the audio being matched. See the [provider decision](open-questions.md#first-audio-lookup--acoustid) and [live validation](sample-validation.md#audio-assisted-repair).

## Single-song fix

`fix` shares the interactive review and requires the user to select Apply for the displayed file. It rewrites the approved `TIT2` and `TPE1` frames individually and copies every unrelated frame byte for byte in its original order. Repeated frames keep their positions. ID3v2.3 uses Latin-1 or BOM-marked UCS-2, and ID3v2.4 uses UTF-8. The writer rejects protected or flagged frames, signatures, unrepresentable text, and tag growth when `CHAP`, `ASPI`, or `RBUF` may depend on file offsets. Existing padding is reused to preserve offsets where possible. [ID3v2.3](https://id3.org/id3v2.3.0) · [ID3v2.4 structure](https://id3.org/id3v2.4.0-structure) · [ID3 chapters](https://id3.org/id3v2-chapters-1.0)

The writer uses an advisory lock and rechecks the path and SHA-256 before replacement. It copies the reviewed file into a backup stage and verifies that copy. The replacement stage is checked against the proposal and the audio and trailing bytes from the backup. The original owner, group, and permission bits are preserved. Files with extended attributes, special mode bits, or multiple hard links are refused so those properties cannot be lost during replacement.

For audio-assisted fixes, the writer adds an approved missing title or artist within the existing tag version. If the MP3 has no tag, it prepends an ID3v2.4.0 tag with the approved fields and verifies the entire original byte stream after that tag.

The backup is published under an exclusive `.l1ner.bak` name and synced before the verified stage is renamed over the MP3. A failure before rename preserves the original file. If the backup was already published, it remains available for recovery. A directory sync failure after rename is reported as a warning that the replacement was applied. Abrupt termination can leave `.l1ner-stage-*` files beside the MP3. Use the reported backup path for recovery. See the [recovery command](../README.md#fix-one-song).

## Folder fixes

`fixbatch.go` uses the existing immediate-directory scan and processes paths sequentially. Each song goes through the same review and verified write path as a single file. The batch paces lookups using a full second after each completed request and waits when the existing local reservation is busy. The wait and HTTP request can be canceled. Service failures are reported for the affected file and the batch continues.

An approved replacement and its backup are kept when the review ends. The batch tracks six outcomes in memory and counts unvisited songs as pending. A terminal or output failure stops further reviews. The [folder usage](../README.md#fix-a-folder) describes scope and exit codes.

Packaging, distribution, recursive editing, and additional tag formats remain deferred.
