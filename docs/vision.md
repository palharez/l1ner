# l1ner — Product Vision

**Tagline:** Give your music its identity back.

**Status:** First MVP implemented; broader product remains in discovery.

This document defines the product concept, scope, principles, and intended outcomes. The first MVP targets Linux and uses Go, as agreed during discovery. Metadata reading uses `github.com/bogem/id3v2/v2`, as recorded in the [technical plan](technical-plan.md). External APIs remain undecided. Questions for the next phase are tracked in [Open Questions](open-questions.md).

## 1. Overview

l1ner is an open-source command-line interface (CLI) designed to identify music files, enrich their metadata, and organize local music libraries.

The long-term vision uses existing file metadata and external music databases to retrieve information such as track titles, artists, albums, release dates, track numbers, and album artwork.

The goal is to produce well-organized, portable music files that remain compatible with different media players without requiring users to continue using l1ner.

## 2. Problem Statement

Local music libraries often suffer from several common issues:

- Missing or incomplete metadata.
- Generic, inconsistent, or incorrect filenames.
- Missing album artwork.
- Tracks incorrectly associated with artists or albums.
- Time-consuming manual organization of large collections.
- Dependence on specific applications to maintain organized libraries.

Although established standards exist for embedding metadata in audio files, not every music file implements them correctly or consistently.

As a result, users may own their music files but still struggle to maintain an accurate, organized, and portable collection.

## 3. Proposed Solution

l1ner aims to automate the identification and metadata enrichment of existing music files.

Beyond the first MVP, the tool should allow users to:

1. Scan individual music files or entire directories.
2. Inspect existing metadata and detect missing or potentially inconsistent information.
3. Identify possible matches using external music catalogs.
4. Retrieve available metadata and album artwork.
5. Preview proposed changes before applying them.
6. Write approved metadata directly into supported music files.
7. Generate reports describing the results.

Music identification should leverage existing metadata, filenames, and, when necessary, techniques that analyze the audio content itself.

The tool should prioritize accuracy and user control over aggressive automation.

## 4. Target Audience

l1ner is intended for:

- People who maintain personal collections of locally stored music.
- Users of offline music players.
- Digital music collectors and archivists.
- Developers interested in automating music library management.
- Users who prefer owning their music files rather than depending entirely on streaming platforms.

### Primary User and Core Workflow

The initial focus is a person who wants to inspect the metadata already present in a local MP3 file. The first MVP workflow is to provide one file path to the CLI and read its metadata in the terminal.

The longer-term workflow is: inspect a collection, review candidate matches and proposed changes, approve changes, and use the enriched files in an existing music player. Users should be able to leave unresolved files untouched and still complete useful work on the rest of the collection.

## 5. Core Capabilities

These capabilities describe the broader product direction. Only reading and displaying existing metadata is included in the first MVP.

### 5.1 Metadata Inspection

Read existing metadata from supported audio files and identify missing, incomplete, or potentially inconsistent fields.

### 5.2 Music Identification

Identify music tracks using available metadata, filenames, and, when necessary, audio fingerprinting techniques.

The identification process must explicitly handle ambiguous matches.

Identifying a recording does not necessarily establish its exact album edition, release, or remaster.

### 5.3 Metadata Enrichment

Retrieve relevant information from external metadata providers and associate it with the corresponding music file.

Metadata enrichment may include:

- Track title.
- Artist and album artist.
- Album title.
- Release date.
- Track and disc numbers.
- Genre.
- Composer.
- Recording identifiers.
- Other supported metadata fields.

The broader product should expose all available metadata from future providers rather than limit it to this list. Availability depends on the selected metadata providers and the completeness of their records.

### 5.4 Artwork Enrichment

Retrieve album artwork when available and associate it with the corresponding audio files.

Artwork should only be embedded when a sufficiently reliable match has been established.

### 5.5 Safe Metadata Writing

Write metadata directly into supported audio files without altering their underlying audio content.

The system should prioritize predictable and reversible operations wherever technically feasible.

Users must be able to preview changes before applying them.

### 5.6 Batch Processing

Support processing multiple music files in a single execution.

Batch operations should provide meaningful progress information, clear error reporting, and a summary of successful, skipped, and failed operations.

## 6. Product Principles

### Local-First

Music files remain under the user's ownership and control.

External services may provide metadata, but users should not be required to upload or store their music libraries on third-party platforms.

Local-first does not imply fully offline identification. Users should understand what information leaves their device during an external lookup. The permitted lookup data and any consent requirements remain open for the next phase.

### Non-Destructive

The tool must preserve the original audio content and minimize the risk of unintended metadata changes.

Potentially destructive operations should require explicit authorization.

### Portable

Enriched metadata should follow established audio metadata standards and remain usable by compatible music players.

### Transparent

Users should be able to understand where metadata originates, which changes are proposed, and why a particular match was selected.

### Predictable

The system should not silently apply uncertain matches or overwrite potentially valid information.

Proposed replacements of existing values must be visible in the preview and covered by the user's approval. Missing information from a provider must not be treated as a reason to erase existing metadata.

### Open Source

The project should encourage transparency, learning, collaboration, and community contributions.

## 7. Initial Scope — First MVP

The first MVP is a Linux CLI written in Go that reads metadata already present in one local MP3 file and displays it in the terminal.

### Included

- Run from source on the user's local Linux machine.
- Accept a path to a single local MP3 file through `l1ner inspect <file-path>`; this describes the CLI interface, with the source-based invocation to be documented during implementation.
- Read existing ID3v2.3 and ID3v2.4 metadata the reader can expose, including custom fields, rather than a fixed shortlist.
- Clearly report unsupported tag versions; ID3v1 and ID3v2.2 support is deferred.
- Display the file path first, followed by readable `Tag: Value` lines. Display text metadata in full, custom tags by name, and all repeated values.
- Describe embedded artwork and other binary metadata by type and size where available, without printing raw binary content or rendering images.
- Show only metadata tags present in the file, without placeholders for absent fields. Display `No metadata found` when no metadata is present.
- Clearly report when the file cannot be read.

The CLI reads the file without modifying it. It works locally without external lookups. The remaining CLI presentation and technical questions are tracked in [Open Questions](open-questions.md).

### Excluded

- Music identification, audio fingerprinting, or assessing whether existing metadata is correct.
- External metadata services and network lookups.
- Metadata enrichment, editing, or writing.
- Artwork retrieval, extraction, or display.
- Change previews and approval workflows.
- Metadata enrichment or writing in batches (read-only directory inspection is available as an extension).
- Report files and structured output for automation.
- Renaming, moving, deleting, or reorganizing files.
- Duplicate detection.
- Music downloading, playback, or streaming.
- Cloud libraries and cross-device synchronization.
- Graphical interfaces.
- Audio editing or transcoding.
- Downloadable binaries, installers, package-manager distribution, and a release process.
- Support for other operating systems.
- Support for other audio formats.
- Playlist management.

The broader capabilities remain future directions; they are not requirements for this first MVP.

### Implemented Extension — Directory Inspection

Following completion of the first MVP, read-only inspection also accepts directories. The CLI scans MP3s in the immediate folder by default or includes subfolders with `--recursive`. It continues after individual failures, reports errors with file paths, and summarizes successes, failures, and scan errors. Existing single-file output remains unchanged. See [the recorded decisions](open-questions.md#directory-inspection--agreed-extension).

## 8. Success Criteria

The first MVP is successful when a user can:

1. Run the CLI with the path to a local MP3 file.
2. See readable terminal output reflecting its existing embedded metadata, without guessed or externally retrieved values.
3. See `No metadata found` for a file without tags, and a clear error for a missing or unreadable file.
4. Verify that the entire input file remains unchanged after execution.
5. Complete the operation without network access.
6. Receive an explicit unsupported-version message for older tag versions, rather than `No metadata found`.

Start validation with `music/tom-lehrer-when-you-are-old-and-gray.mp3`. The sample is local and ignored by Git; its download source and reuse terms are recorded in [Open Questions](open-questions.md). Expected output must be checked against the file's embedded values; source-page information alone does not establish what tags the file contains. Validation must also cover a file without tags and missing or unreadable inputs, and confirm that inspection leaves the input file unchanged.

## 9. Long-Term Vision

l1ner aims to become a simple, reliable, and extensible tool for preserving and organizing personal digital music collections.

Future development may expand supported audio formats, improve identification accuracy, introduce additional metadata providers, and provide advanced library management capabilities.

The project does not aim to replace music streaming services.

Instead, it seeks to improve the experience of people who value ownership, portability, and control over their music collections.

At its core, l1ner should help users maintain a music library that remains organized and useful regardless of which media player or platform they choose.

---

**l1ner — Give your music its identity back.**
