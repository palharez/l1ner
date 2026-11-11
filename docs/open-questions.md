# l1ner — Open Questions

**Status:** Inspection decisions implemented; broader product questions remain open.

This tracker accompanies the [product vision](vision.md). Checked items record decisions, and unchecked items track future questions. The Linux-only inspection MVP uses Go and `github.com/bogem/id3v2/v2` for metadata reading. The first external integration is [MusicBrainz lookup](#first-provider--musicbrainz).

Use the checklist to track progress. When a question is resolved, record the decision, rationale, and any supporting evidence beneath it before marking it complete.

## First MVP — Resolved Decisions

**Agreed scope:** A CLI accepts one local MP3 path, reads existing embedded metadata, and displays it in the terminal. It does not modify files or use external services.

- [x] Which existing metadata fields should the first MVP display?

  **Decision:** Display all existing metadata the reader can expose, including custom fields, rather than a fixed shortlist. Implemented coverage and reader limitations are documented in the [README](../README.md#metadata-coverage): unknown frames are summarized, and unsupported versions/features are reported explicitly.

- [x] How should non-text metadata be presented in the terminal?

  **Decision:** Display text metadata in full. For embedded artwork and other binary metadata, display a description including type and size where available, without printing raw binary content or rendering images.

- [x] How should absent metadata values be presented in the terminal?

  **Decision:** Show only metadata tags present in the file. Do not add placeholders for absent fields. If no metadata is present, display `No metadata found`.

  **Broader product direction:** Expose metadata made available by future services. The [first lookup milestone](#first-provider--musicbrainz) starts with a recording candidate list.

- [x] What command syntax and terminal layout should the first MVP use?

  **Decision:** Use `l1ner inspect <file-path>`. Display the file path first, then readable `Tag: Value` lines. Show custom tags by name and include all repeated values. Text values appear in full; binary values are described by type and size where available.

  Example layout (illustrative values, not the reference file's verified tags):

  ```text
  File: music/song.mp3

  Album [TALB]: Example Album
  Title [TIT2]: Example Song
  Artist [TPE1]: Example Artist
  ```

- [x] Which operating systems must the first MVP support?

  **Decision:** Linux only for the first MVP. Other platforms are deferred.

- [x] Which programming language should the first MVP use?

  **Decision:** Go, as selected by the user for the CLI.

- [x] Which metadata-reading library should the first MVP use?

  **Decision:** Use `github.com/bogem/id3v2/v2`, focusing on ID3v2.3 and ID3v2.4. Its frame-level API suits inspection of custom fields, repeated frames, artwork, and unknown data. Older ID3v1 and ID3v2.2 tags are deferred. Unsupported versions must be reported explicitly rather than presented as missing metadata. See the [technical plan](technical-plan.md) for the read-only approach and validation work.

- [x] How should users install and run the CLI?

  **Decision:** For the first MVP, work directly from the source code and run it locally on the user's Linux machine. Downloadable binaries, installers, package-manager distribution, and a release process are deferred. Working commands are documented in the [README](../README.md#usage).
- [x] How will displayed values and an unchanged input file be verified, including missing metadata and unreadable inputs?

  **Decision:** Verify that the CLI displays existing tags in the reference MP3, leaves the input file unchanged, displays `No metadata found` for a file without tags, and reports a clear error for missing or unreadable files. Expected values must come from the embedded metadata, not just the source page. Generated fixtures cover output, parsing errors, and file integrity; checks compare file size, permissions, and whole-file SHA-256. Tests, vet, and build checks run in CI; see [CONTRIBUTING.md](../CONTRIBUTING.md#verification).

The original single-file inspection decisions remain recorded above. Directory inspection, MusicBrainz lookup, and single-song editing each have a separate scope.

## Directory Inspection — Agreed Extension

The user selected directory scanning and batch inspection as the next capability after the single-file MVP.

- [x] How should users inspect a directory?

  **Decision:** `l1ner inspect <directory>` scans the immediate folder; `l1ner inspect --recursive <directory>` also scans subfolders. Flags precede the path. Single-file inspection remains available.

- [x] Which files are included, and how are failures handled?

  **Decision:** Process regular `.mp3` files case-insensitively and sequentially in lexical directory traversal order. Skip discovered symlinks and non-MP3 files; reject a symlink directory root. Continue after per-file and directory-read errors, report affected paths to stderr, and summarize successful files, failed files, and scan errors. Any failure gives exit code 1. No matches is a successful empty scan. Output-write failure stops processing.

The extension remains local, Linux-only, offline, and read-only. Directory defaults are documented implementation choices that can be revisited with user feedback.

## Inspection Acceptance and Future Review

- [x] Is the current inspection workflow acceptable for the completed milestone?

  **User acceptance — 2026-11-10:** Yes. Batch output is difficult to review song by song, but it is acceptable for read-only inspection. This usability limitation does not block completion of the inspection milestone.

- [x] Should proposed fixes require acceptance per song?

  **Decision:** Present proposed changes for each song and require the user's acceptance before modifying that file. Both file and directory forms of `fix` enforce this requirement.

- [ ] How should per-song review show existing and proposed values, support skipping/rejection, and navigate a batch?

### Interactive song review

**Accepted starting point.** The first MusicBrainz lookup is accepted as shipped. The next step is to let a listener choose a candidate and see how it would change the MP3's tags.

**Implemented one-song flow.** Use `l1ner preview <file.mp3>` in a terminal. The command uses the existing lookup's title and artist query. The picker shows the title, credited artist, and available duration for each option. Arrow keys move focus and reveal that recording's ID and version description. Enter leaves a one-line confirmation and shows the file path and comparison. The same picker drives `fix`. See the [live terminal check](sample-validation.md#interactive-preview).

```text
$ l1ner preview music/song.mp3

? Choose a recording  (↑/↓, Enter)
❯ 1. Example Song — Example Artist · 2:27
    Recording ID: 01234567-89ab-4cde-8f01-23456789abcd
    Version: live
  2. Example Song — Example Artist · 2:19
  Skip song
  Quit

✓ 1. Example Song — Example Artist · 2:27

File       music/song.mp3
Field      Current          Proposed
Title      example song     Example Song    changed
Artist     Example Artist   Example Artist  same

? Review this proposal
❯ Skip song
  Accept proposal (preview only)
  Choose another recording
  Quit
```

Highlight Skip by default on the proposal screen. Accept records the decision for the current run and prints a summary of the accepted proposal. Skip ends review of that song. Quit and Ctrl+C exit the review. If the tags already agree, show that outcome and finish. The milestone opens the MP3 read-only, and the terminal is restored when the wizard exits.

**Implemented batch behavior.** `l1ner fix <directory>` reviews immediate MP3s in lexical order with progress such as `Song 3 of 20`. Apply writes and backs up the displayed song, Skip advances, and Quit leaves the current and later songs pending. The summary counts applied, skipped, unchanged, unresolved, failed, and pending songs. Revisiting decisions and saved sessions remain open. See [folder usage](../README.md#fix-a-folder).

## Broader Product — Product Validation

- [ ] What reference collection represents the initial users' libraries, including difficult and unresolved cases?

  **Current evidence:** The initial sample grew into a local collection of 20 MP3s from one artist and album family. Sources and observed limitations are documented in the [validation notes](sample-validation.md) and [manifest](mp3-samples.tsv). A representative collection covering difficult and unresolved cases remains open.

  **First reference file:** Tom Lehrer — “When You Are Old and Gray,” from *The Remains of Tom Lehrer*, disc 2, track 10, as listed on the [artist's song page](https://tomlehrersongs.com/when-you-are-old-and-gray/).

  - [Download MP3](https://tomlehrersongs.com/wp-content/uploads/2021/10/10-When-You-Are-Old-and-Gray-1.mp3).
  - [Artist's rights statement](https://tomlehrersongs.com/disclaimer/): relinquishes copyrights and includes recording rights. Keep this provenance with the reference.
  - The download was verified as an MP3 on 2026-11-09. The local reference file is stored at `music/tom-lehrer-when-you-are-old-and-gray.mp3`. The `music/` directory is ignored by Git; contributors can download the sample from the source above.
  - Embedded album, title, artist, and track fields were inspected and checked independently. Disc 2 is source-page context, not a verified embedded disc tag. The nonstandard `YEAR` frame is summarized rather than decoded. Artwork and future service recognition have not been validated.

- [ ] What correct-match coverage and incorrect-proposal thresholds will establish useful, trustworthy identification?
- [ ] Which additional operating systems and independent music players should future enrichment support?

  **Current scope:** Inspection targets Linux and does not require player integration.
- [ ] Which metadata fields should future enrichment retrieve and propose writing?

  **Current decision:** Inspection displays all exposed metadata, with unknown data summarized; it has no fixed field shortlist.

## Technology

- [x] Which programming language offers the best balance of portability, maintainability, and development productivity?

  **Decision:** Use Go, as selected for the first MVP. No language comparison is required at this stage.

- [ ] Which libraries should handle metadata reading and writing beyond the first MVP?

  **Partial decision:** First-MVP reading uses `github.com/bogem/id3v2/v2`. Writing and broader format coverage remain open.
- [x] Which fingerprinting technology should the first audio lookup use?

  **Decision.** Use Chromaprint through its `fpcalc` executable on Linux. See the [AcoustID decision](#first-audio-lookup--acoustid).
- [ ] How should the CLI be packaged and distributed across operating systems?

## External Services

### First provider — MusicBrainz

- [x] Which provider and data should the first lookup use?

  **Decision:** A separate, explicitly invoked `lookup` command sends the embedded title and artist of one MP3 to MusicBrainz and displays up to five recording candidates for human review. The file path, audio, and other tags stay local. Inspection is offline, and both commands read the music file without modifying it. See the [live results](sample-validation.md#musicbrainz-lookup).

- [x] What are the first provider's access requirements?

  **Decision:** Use MusicBrainz public recording search. The hosted API permits free non-commercial use and requires a meaningful `User-Agent` and a maximum of one request per second. Public search requires no API key. [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API) · [Rate limiting](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting)

  **Implementation:** Lookup sends one request per invocation with a 10-second timeout. A per-user cache marker coordinates the rate limit across local invocations. The marker stores request timing through its modification time.

### First audio lookup — AcoustID

- [x] Agree on the provider and what data may leave the machine.

  **Decision.** Audio identification is opt-in through `identify` or `preview` or `fix` with `--from-audio`. These lookups send a locally calculated audio fingerprint and the MP3 duration to AcoustID, authenticated with an application key from `L1NER_ACOUSTID_KEY`. The original audio, path, and embedded tags stay local. Tag-based commands work without AcoustID setup. [Setup guide](audio-identification.md)

- [x] Agree on the first presentation of fingerprint matches.

  **Decision.** Show each distinct linked MusicBrainz recording for review, ordered by AcoustID score. The score ranks fingerprint hits. Audio-assisted preview and fix offer recordings with both title and artist, and require a selection before displaying a proposal. Only explicit Apply in `fix` can write the approved fields. Confidence thresholds remain open. See the [live validation](sample-validation.md#audio-identification).

- [x] How should a folder use audio matching?

  **Decision.** `fix --from-audio <directory>` looks up each immediate MP3 sequentially and uses the existing per-song approval and backup workflow. The local AcoustID wait can be canceled. The batch reports per-file errors and the existing six outcome counts. [Batch usage](audio-identification.md#review-a-folder)

### Broader provider questions

- [ ] Which metadata providers offer the most reliable and comprehensive information?
- [ ] What are their API limits and licensing requirements?
- [ ] How should the application handle unavailable or incomplete external services?
- [ ] Should metadata responses be cached locally?
- [ ] What lookup data may leave the device, how is it explained, and when is explicit consent required?
- [ ] Can identification meet the intended scope without sending original audio files to external services?

## Identification and Accuracy

- [x] How should the first release-tagging workflow choose an edition?

  **Decision.** `--release` adds a release picker after recording selection in `preview` and `fix`. MusicBrainz receives the selected recording ID. Review the matching tracks from the first page of up to 20 releases, with disc and track position and available edition details. The selected release contributes album, album artist when credited, track number, and disc number. A folder uses the same selection and approval for each song. [Release tagging usage](../README.md#add-album-and-track-tags)

- [ ] How should different recordings, remasters, and album editions be distinguished?
- [ ] How should match confidence be calculated and communicated?
- [ ] How should partially correct metadata be handled?
- [ ] How should the tool proceed when a recording is identifiable but its exact release or artwork is uncertain?
- [ ] When should the system request manual confirmation?

## Album Organization

- [ ] Which folder and tags define an album group?

  **Proposed layout.** Use the selected directory as both the source and destination root. Review the regular MP3s directly inside it in lexical order. Place each eligible file at `<album artist>/<album>/<original filename>` relative to that root. Read the album artist from `TPE2` and the album from `TALB`. This puts compilation tracks with the same album artist and album together. Missing or conflicting values need attention. The layout groups by those two tags, so editions with the same credited album artist and album name share a folder.

- [ ] How should a preview handle unsafe paths and collisions?

  **Proposed behavior.** Reuse the offline ID3 reader and the existing directory discovery rules. Validate each proposed folder name as a single path component. Report invalid names, symlinks in destination paths, occupied destinations, and names that would collide on a case-insensitive filesystem. Show the source path and the reason for each blocked item. The preview reads the filesystem and prints its plan to the terminal.

- [ ] What should approval and undo cover?

  **Proposed behavior.** Show the exact source and destination for each song. Apply approves that move only. Skip advances, and Quit leaves the remaining songs pending. Before moving, verify the source still matches the reviewed file and use a move that rejects an occupied destination. Keep recovery information for each approved move so undo can restore completed moves in reverse order and detect conflicts. Undo restores the MP3 to its original path beside any existing `.l1ner.bak` from `fix`. Remove folders created by the run during undo when they are empty.

See the [preview and apply milestones](../TODO.md#milestone--preview-album-folders).

## Data Integrity

**Single-song decision.** `fix` stages a replacement in the same directory, verifies the proposed tags and unchanged audio, and publishes an exact `.l1ner.bak` backup before replacing the source. It refuses existing backups and changes detected during review. Restore by moving the backup over the MP3. See the [write and recovery details](technical-plan.md#single-song-fix).

- [ ] What backup and recovery strategies should be supported?
- [ ] How can metadata modifications be made safely?
- [x] How does the first batch workflow handle interruption?

  **Decision:** Stop the review and keep completed replacements and their backups. Count the current song as pending if its replacement had yet to commit. Resume behavior and persistent review state remain open.
- [x] How does current inspection verify that files remain unchanged?

  **Implemented:** Tests compare whole-file SHA-256, size, and permissions before and after inspection.

- [ ] What additional integrity verification will metadata writing require? Whole-file hashes will change when tags are edited, so audio preservation needs a separate strategy.

## Developer Experience

- [x] What is the current command structure?

  **Implemented.** `inspect` accepts a file or directory with optional `--recursive` and `--json`. `lookup`, `identify`, and `preview` accept one MP3. `fix` accepts a file or directory. Both `preview` and `fix` accept `--from-audio` and `--release`. Flags precede the path.
- [ ] How should progress and match confidence be displayed in future workflows?

  **Current behavior:** Inspection emits per-file metadata, path-specific errors, and a directory summary. It computes no match confidence.
- [x] What output format should be available for automation?

  **Decision — 2026-11-10:** Add `--json` for single-file and directory inspection. Write one report to stdout with per-file metadata/errors, directory scan errors, and summary counts. Preserve repeated metadata as arrays and exclude raw binary payloads. Users choose where to save output with shell redirection; there is no automatic `/tmp` storage. Existing text mode stays the default. See the [version 1 report format](json-output.md).
- [x] How are current testing and continuous integration structured?

  **Implemented:** Standard Go tests cover inspection, rendering, and directory batches with generated fixtures. GitHub Actions runs formatting, dependency verification, tests, vet, and build in one Linux job, with a timeout and cancellation of outdated runs. See [CONTRIBUTING.md](../CONTRIBUTING.md#continuous-integration).
