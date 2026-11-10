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
3. [x] **Wire up `lookup`.** Accept one MP3 path and show candidates in service order. Preserve separate recordings even when they share a title and credit. Escape control characters in terminal output. Use exit `0` for a completed search including zero results, `1` for input, service, or output failures, and `2` for invalid arguments. Test the command with a local server, including zero requests on invalid tags and identical file checksums before and after lookup. Keep existing `inspect` tests passing. Commit as `feat(lookup): add single-file lookup command`.
4. [x] **Verify and document the milestone.** Run `gofmt -w *.go`, `go test ./...`, `go vet ./...`, and `go build -o bin/l1ner .`. The original reference lookup missed its expected version. “My Home Town” returned the expected recording, and all checked input SHA-256 hashes matched before and after. See [usage](README.md#musicbrainz-lookup) and the [live validation results](docs/sample-validation.md#musicbrainz-lookup). Commit as `docs(lookup): document usage and live validation`.

Fingerprinting and release selection are future milestones.

## Milestone — Interactive song preview

**Branch** `milestone/interactive-preview`. The first MusicBrainz lookup is accepted. This milestone moves from finding candidates to reviewing a proposed change for one song.

**Implemented scope** `l1ner preview <file.mp3>` opens a keyboard-driven review in the terminal. Use the existing MusicBrainz search, pick a recording, and compare its title and artist credit with the MP3's tags. Accept, skip, or quit the preview with the file opened read-only. See the [review flow](docs/open-questions.md#interactive-song-review).

1. [x] **Build the song proposal.** Use the selected recording ID and the existing title and artist to show current and proposed values. Flag changed values and show a clear outcome when both tags already agree. Test changed and unchanged tags with generated fixtures. Commit as `feat(preview): compare current and proposed song tags`.
2. [x] **Add the interactive review.** Validate that input and output are terminals before searching. Reuse the existing search and rate limit. Show candidate IDs and available version details, then present the proposal and review choices. Verify selection, changing candidates, accepting, skipping, quitting, and cancellation. Commit as `feat(preview): add interactive song review`.
3. [x] **Verify and document the milestone.** Tests use generated MP3s and a local MusicBrainz server. The [live terminal check](docs/sample-validation.md#interactive-preview) confirmed acceptance, terminal restoration, and an unchanged checksum. See the [usage](README.md#interactive-song-preview) and [PR workflow](CONTRIBUTING.md#feature-workflow). Commit as `docs(preview): document and validate song review`.

## Milestone — Safe single-song fix

**Branch** `milestone/single-song-fix`. `l1ner fix <file.mp3>` uses the existing MusicBrainz review. The displayed title and artist changes are applied only after explicit approval for that song. Skip and Quit leave it intact.

1. [x] **Rewrite approved ID3 frames.** Keep the existing tag version and preserve every unrelated frame and the audio. Test ID3v2.3 and ID3v2.4, repeated frames, Unicode, and unsupported features. Commit as `feat(fix): rewrite approved title and artist frames`.
2. [x] **Stage and verify the replacement.** Work in the source directory, reject unsafe file layouts, check that the reviewed file has stayed the same, verify the staged tags and audio, and publish an exact `.l1ner.bak` backup before replacement. Test restoration, backup collisions, and failures. Commit as `feat(fix): verify and back up song edits`.
3. [x] **Connect the wizard to the writer.** Make Apply an explicit choice with Skip highlighted, and test approval, skip, quit, cancellation, and the applied result through the terminal command. Commit as `feat(fix): apply approved single-song changes`.
4. [x] **Verify and document the milestone.** The [live terminal check](docs/sample-validation.md#single-song-fix) verified the applied tags, unchanged audio and other frames, and exact restoration. See [usage and recovery](README.md#fix-one-song) and the [PR workflow](CONTRIBUTING.md#feature-workflow). Commit as `docs(fix): document single-song edits and recovery`.

## Milestone — Batch song fixes

**Branch** `milestone/batch-fix`. `l1ner fix <directory>` reviews the immediate MP3s in lexical order. Each Apply approves only the displayed song. Quit leaves the current and remaining songs pending.

1. [x] Pace the MusicBrainz requests with a cancellable wait and keep the existing shared rate limit. Commit as `feat(fix): pace MusicBrainz searches during batch review`.
2. [x] Reuse the one-song review and writer for directories. Report progress and counts for applied, skipped, unchanged, unresolved, failed, and pending songs. Continue after per-file failures. Commit as `feat(fix): review and apply fixes one song at a time`.
3. [x] Verified all six outcomes with generated MP3s, a local server, and terminal input. Documented [folder usage](README.md#fix-a-folder) and the [PR workflow](CONTRIBUTING.md#feature-workflow). Commit as `docs(fix): document batch review and outcomes`.

## Milestone — Identify a song from audio

**Branch** `milestone/audio-identification`. `l1ner identify <file.mp3>` fingerprints one MP3 locally, looks up the fingerprint and duration with AcoustID, and displays MusicBrainz recording candidates for review. The file stays read-only.

1. [x] Move the existing CLI and tests into `internal/cli` and keep the root entry point. Commit as `refactor(cli): move command workflows into internal`.
2. [ ] Add the local Chromaprint adapter and AcoustID client, with deterministic tests for ambiguous and missing results. Commit as `feat(identify): look up recordings from audio fingerprints`.
3. [ ] Connect `identify` to the CLI and verify recognition with missing or incorrect tags. Document the application key and open a PR. Commit as `feat(cli): add single-song audio identification`.

## Future directions — not committed scope

- [ ] Revisit older tag versions and currently unsupported ID3 features.
- [ ] Evaluate recording retrieval across a broader sample and research additional metadata services using `docs/open-questions.md`.
- [ ] Agree on recursive fixing, revisiting decisions, and resuming reviews.
- [ ] Consider packaging and other operating systems when local Linux usage is established.

See the [product vision](docs/vision.md), [open questions](docs/open-questions.md), and [technical plan](docs/technical-plan.md) for context. This checklist tracks work; the question tracker records decisions and their rationale.
