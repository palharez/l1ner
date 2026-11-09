# l1ner — Open Questions

**Status:** Pending discovery and technical planning

These questions accompany the [product vision](vision.md) and remain open for the next phase. Go has been selected as the implementation language for the Linux-only first MVP. The first MVP uses `github.com/bogem/id3v2/v2` for metadata reading. External services remain undecided and outside the first MVP.

Use the checklist to track progress. When a question is resolved, record the decision, rationale, and any supporting evidence beneath it before marking it complete.

## First MVP — Questions to Resolve Next

**Agreed scope:** A CLI accepts one local MP3 path, reads existing embedded metadata, and displays it in the terminal. It does not modify files or use external services.

- [x] Which existing metadata fields should the first MVP display?

  **Decision:** Display all existing metadata the reader can expose, including custom fields, rather than a fixed shortlist. This is the intended coverage; exact format support and reader limitations must be documented during technical planning.

- [x] How should non-text metadata be presented in the terminal?

  **Decision:** Display text metadata in full. For embedded artwork and other binary metadata, display a description including type and size where available, without printing raw binary content or rendering images.

- [x] How should absent metadata values be presented in the terminal?

  **Decision:** Show only metadata tags present in the file. Do not add placeholders for absent fields. If no metadata is present, display `No metadata found`.

  **Broader product direction:** Expose all metadata made available by future services as well. External lookups remain outside the first MVP unless its scope is explicitly expanded; no providers have been selected.
- [x] What command syntax and terminal layout should the first MVP use?

  **Decision:** Use `l1ner inspect <file-path>`. Display the file path first, then readable `Tag: Value` lines. Show custom tags by name and include all repeated values. Text values appear in full; binary values are described by type and size where available.

  Example layout (illustrative values, not the reference file's verified tags):

  ```text
  File: music/song.mp3

  Title: Example Song
  Artist: Example Artist
  Album: Example Album
  Artwork: image/jpeg, 125000 bytes
  ```
- [x] Which operating systems must the first MVP support?

  **Decision:** Linux only for the first MVP. Other platforms are deferred.

- [x] Which programming language should the first MVP use?

  **Decision:** Go, as selected by the user for the CLI.

- [x] Which metadata-reading library should the first MVP use?

  **Decision:** Use `github.com/bogem/id3v2/v2`, focusing on ID3v2.3 and ID3v2.4. Its frame-level API suits inspection of custom fields, repeated frames, artwork, and unknown data. Older ID3v1 and ID3v2.2 tags are deferred. Unsupported versions must be reported explicitly rather than presented as missing metadata. See the [technical plan](technical-plan.md) for the read-only approach and validation work.
- [x] How should users install and run the CLI?

  **Decision:** For the first MVP, work directly from the source code and run it locally on the user's Linux machine. Downloadable binaries, installers, package-manager distribution, and a release process are deferred. Exact development commands will be documented during implementation.
- [x] How will displayed values and an unchanged input file be verified, including missing metadata and unreadable inputs?

  **Decision:** Verify that the CLI displays existing tags in the reference MP3, leaves the input file unchanged, displays `No metadata found` for a file without tags, and reports a clear error for missing or unreadable files. Expected values must come from the embedded metadata, not just the source page. The verification tools and method remain implementation details.

The questions below are preserved for the broader product. Identification, enrichment, writing, and batch-processing questions do not block the first MVP. Overlapping technical questions can be revisited as the product expands.

## Directory Inspection — Agreed Extension

The user selected directory scanning and batch inspection as the next capability after the single-file MVP.

- [x] How should users inspect a directory?

  **Decision:** `l1ner inspect <directory>` scans the immediate folder; `l1ner inspect --recursive <directory>` also scans subfolders. Flags precede the path. Single-file inspection remains available.

- [x] Which files are included, and how are failures handled?

  **Decision:** Process regular `.mp3` files case-insensitively and sequentially in lexical directory traversal order. Skip discovered symlinks and non-MP3 files; reject a symlink directory root. Continue after per-file and directory-read errors, report affected paths to stderr, and summarize successful files, failed files, and scan errors. Any failure gives exit code 1. No matches is a successful empty scan. Output-write failure stops processing.

The extension remains local, Linux-only, offline, and read-only. Directory defaults are documented implementation choices that can be revisited with user feedback.

## Broader Product — Product Validation

- [ ] What reference collection represents the initial users' libraries, including difficult and unresolved cases?

  **Starting point:** We do not have a personal reference collection. Begin with one downloadable MP3, then expand the collection as validation needs become clearer. This file is the starting reference for the first MVP. The broader collection question remains open because one file does not represent difficult and unresolved cases.

  **First reference file:** Tom Lehrer — “When You Are Old and Gray,” from *The Remains of Tom Lehrer*, disc 2, track 10, as listed on the [artist's song page](https://tomlehrersongs.com/when-you-are-old-and-gray/).

  - [Download MP3](https://tomlehrersongs.com/wp-content/uploads/2021/10/10-When-You-Are-Old-and-Gray-1.mp3).
  - [Artist's rights statement](https://tomlehrersongs.com/disclaimer/): relinquishes copyrights and includes recording rights. Keep this provenance with the reference.
  - The download was verified as an MP3 on 2026-11-09. The local reference file is stored at `music/tom-lehrer-when-you-are-old-and-gray.mp3`. The `music/` directory is ignored by Git; contributors can download the sample from the source above.
  - Artist, title, album, disc, and track are reference information from the source page. Embedded tags, exact release date, artwork, and recognition by future metadata providers have not been validated.

- [ ] What correct-match coverage and incorrect-proposal thresholds will establish useful, trustworthy identification?
- [ ] Which operating systems and independent music players are the initial compatibility targets?
- [ ] Which metadata fields are essential for the MVP, and which are optional enrichment?

## Technology

- [x] Which programming language offers the best balance of portability, maintainability, and development productivity?

  **Decision:** Use Go, as selected for the first MVP. No language comparison is required at this stage.
- [ ] Which libraries should handle metadata reading and writing beyond the first MVP?

  **Partial decision:** First-MVP reading uses `github.com/bogem/id3v2/v2`. Writing and broader format coverage remain open.
- [ ] Which audio fingerprinting technologies are suitable?
- [ ] How should the CLI be packaged and distributed across operating systems?

## External Services

- [ ] Which metadata providers offer the most reliable and comprehensive information?
- [ ] What are their API limits and licensing requirements?
- [ ] How should the application handle unavailable or incomplete external services?
- [ ] Should metadata responses be cached locally?
- [ ] What lookup data may leave the device, how is it explained, and when is explicit consent required?
- [ ] Can identification meet the intended scope without sending original audio files to external services?

## Identification and Accuracy

- [ ] How should different recordings, remasters, and album editions be distinguished?
- [ ] How should match confidence be calculated and communicated?
- [ ] How should partially correct metadata be handled?
- [ ] How should the tool proceed when a recording is identifiable but its exact release or artwork is uncertain?
- [ ] When should the system request manual confirmation?

## Data Integrity

- [ ] What backup and recovery strategies should be supported?
- [ ] How can metadata modifications be made safely?
- [ ] How should interrupted batch operations be handled?
- [ ] How can the tool verify that audio content remains unchanged?

## Developer Experience

- [ ] What command structure will provide the most intuitive user experience?
- [ ] How should progress, errors, and match confidence be displayed?
- [ ] What output formats should be available for automation?
- [ ] How should testing and continuous integration be structured?
