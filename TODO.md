# Next steps

The implemented CLI inspects local MP3 files and directories on Linux, read-only and offline. The original MVP covered one file; directory inspection is its first extension. Work through the immediate tasks in order; check items off when completed.

## Improve the current MVP

- [x] Add GitHub Actions CI on Linux to check formatting, run `go test ./...` and `go vet ./...`, and build the CLI. Tests must work without the ignored `music/` directory.
- [x] Add `CONTRIBUTING.md` with setup instructions, local checks, the current scope, and guidance for reporting bugs and proposing changes.
- [x] Add dedicated output tests for stable ordering, repeated values, Unicode, terminal control escaping, binary summaries, and output-write failures.
- [x] Expand inspection tests for truncated frame headers, invalid identifiers and sizes, unsupported frame flags, malformed padding, and the tag-size limit. Verify clear failures and unchanged input files.
- [x] Validate against additional legally usable MP3 samples and document observed parser limitations. Keep personal audio in the ignored `music/` directory; use generated fixtures for automated tests. See [20-file validation results](docs/sample-validation.md) and the [download manifest](docs/mp3-samples.tsv).
- [x] Reconcile the question tracker and technical plan with implementation findings, separating completed decisions from unresolved work.
- [x] Add a parser fuzz target for tag reading and frame decoding, and run a one-minute local session.

## Decide the next milestone together

- [x] Review the first MVP's terminal output and error messages in everyday use. Accepted on 2026-11-10; batch output can be difficult to review per song, but this does not block the inspection milestone.
- [x] Agree on the next user-facing capability after directory inspection: `--json` writes one report to stdout, including per-file results, errors, and summary counts; no automatic storage.

## Completed extensions

- [x] Add directory scanning and batch inspection: immediate-folder scanning, optional `--recursive`, deterministic order, per-path errors, and summary counts.

- [x] Add optional JSON output for files and directories, document the versioned format, and test error reporting, repeated metadata, binary summaries, and input integrity.

## Milestone — MusicBrainz lookup

**Branch** `milestone/musicbrainz-lookup`. Follow the [feature workflow](CONTRIBUTING.md#feature-workflow).

**Agreed scope** `l1ner lookup <file.mp3>` uses one file's embedded title and artist to search MusicBrainz and displays up to five recording candidates. Each candidate shows its title, artist credit, and recording ID. The file stays read-only, and `inspect` stays offline. See the [provider and data-sharing decisions](docs/open-questions.md#first-provider--musicbrainz).

1. [x] **Read lookup tags.** Reuse the inspector and extract `TIT2` and `TPE1`. Trim whitespace, ignore blank values, and accept identical repeats. Report missing or conflicting values before any request. Test with generated ID3v2.3 and ID3v2.4 fixtures. Commit as `feat(lookup): extract title and artist for recording lookup`.
2. [x] **Add the MusicBrainz client.** Search the HTTPS `/ws/2/recording` endpoint for title and credited artist with `fmt=json` and `limit=5`. Escape tag values as search literals and URL-encode the query. Identify l1ner in the `User-Agent`, set a timeout, and enforce the one-request-per-second limit across concurrent local invocations. Test the request, credited names and join phrases, empty results, throttling, timeouts, and malformed responses with a local server. Commit as `feat(musicbrainz): add recording search client`.
3. [ ] **Wire up `lookup`.** Accept one MP3 path and show candidates in service order. Preserve separate recordings even when they share a title and credit. Escape control characters in terminal output. Use exit `0` for a completed search including zero results, `1` for input, service, or output failures, and `2` for invalid arguments. Test the command with a local server, including zero requests on invalid tags and identical file checksums before and after lookup. Keep existing `inspect` tests passing. Commit as `feat(lookup): add single-file lookup command`.
4. [ ] **Verify and document the milestone.** Run `gofmt -w *.go`, `go test ./...`, `go vet ./...`, and `go build -o bin/l1ner .`. Run one live lookup on `music/tom-lehrer-when-you-are-old-and-gray.mp3` and check for the [catalog recording linked to disc 2, track 10](https://musicbrainz.org/recording/9bd8d12b-188c-4b2a-8667-ecc035aa4302) among the candidates. Compare the file's SHA-256 before and after. Document usage, what leaves the device, and the outcome of the live check. Commit as `docs(lookup): document usage and live validation`.

Batch lookup, fingerprinting, release selection, and metadata writing are future milestones.

## Future directions — not committed scope

- [ ] Revisit older tag versions and currently unsupported ID3 features.
- [ ] Research identification and metadata services using the questions in `docs/open-questions.md`.
- [ ] Design per-song review and acceptance of proposed changes before adding metadata writing. Clarify skipping/rejection, batch navigation, and integrity/recovery behavior.
- [ ] Consider packaging and other operating systems when local Linux usage is established.
- [ ] Revisit `cmd/l1ner/` and `internal/` packages when additional commands or responsibilities justify them. Keep the current layout for the single-command MVP.

See the [product vision](docs/vision.md), [open questions](docs/open-questions.md), and [technical plan](docs/technical-plan.md) for context. This checklist tracks work; the question tracker records decisions and their rationale.
