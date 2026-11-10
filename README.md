# l1ner

**Give your music its identity back.**

l1ner is an open-source CLI for inspecting local music files, with a longer-term vision of identifying tracks, enriching metadata, and helping users organize portable music collections.

**Status.** Inspect MP3s offline, search by embedded tags, and review or apply title and artist changes one song at a time in the terminal. Audio identification is optional.

## First MVP

Read existing metadata from a local MP3 file or directory and display it in the terminal.

- Built in Go for Linux, running locally from source.
- Supports ID3v2.3 and ID3v2.4 tags.
- Displays all metadata exposed by the reader, including custom fields and repeated values.
- Shows text values in full and describes embedded artwork and other binary data by type and size where available.
- Shows only existing tags, or `No metadata found` in text mode when none are present. JSON uses an empty metadata array.
- Reports unreadable files and unsupported tag versions clearly.
- Leaves the file unchanged and performs no network lookups.

Directory scanning and optional JSON output extend the original single-file MVP. MusicBrainz lookup is a separate command for reviewing possible recordings.

## Usage

From the repository root, download dependencies once:

```sh
go mod download
```

Inspect a local file:

```sh
go run . inspect music/song.mp3
```

Inspect MP3s directly inside a directory:

```sh
go run . inspect music/
```

Include subdirectories:

```sh
go run . inspect --recursive music/
```

Or build and run the CLI:

```sh
go build -o bin/l1ner .
./bin/l1ner inspect music/song.mp3
```

Use `go run . --help` for usage. Put flags before the path; quote paths containing spaces.

### JSON output

Write a machine-readable report to stdout or redirect it to a file:

```sh
./bin/l1ner inspect --json music/song.mp3
./bin/l1ner inspect --json --recursive music/ > inspection.json
```

Both input modes produce one document containing `schema_version`, `files`, `scan_errors`, and `summary`. Metadata is an array to preserve repeated tags; binary data is summarized, never embedded. No report is saved automatically. See the [JSON output format](docs/json-output.md) for fields, examples, and error handling.

### Directory behavior

- Scan only the selected folder unless `--recursive` is supplied.
- Inspect regular files with a case-insensitive `.mp3` extension in lexical directory traversal order. Other files and discovered symlinks are skipped. A directory symlink supplied as the root is rejected explicitly.
- Process files sequentially and keep each file's existing metadata layout, separated by a blank line.
- In text mode, report per-file errors with their paths to stderr and continue inspecting other files. Unreadable directories are scan errors; readable sibling directories still get scanned.
- In text mode, finish with `Summary: N inspected, N failed, N scan errors`. `inspected` counts successful files, including files without tags; `failed` counts file inspection failures. Skipped non-MP3 files are not counted.
- If there are no matching MP3s and no scan errors, print `No MP3 files found.` and a zero summary, exiting successfully.
- Exit with status 1 if any file or scan fails. An output-write failure stops the batch immediately, so a complete summary may not be available. Single-file output stays unchanged.

Illustrative output:

```text
File: music/song.mp3

Album [TALB]: Example Album
Artwork [APIC]: image/jpeg, 125000 bytes; picture type 3; description: Front cover
Title [TIT2]: Example Song
Artist [TPE1]: Example Artist
```

### MusicBrainz lookup

```sh
./bin/l1ner lookup music/song.mp3
```

`lookup` uses the embedded title (`TIT2`) and artist (`TPE1`) as the only file data in an HTTPS search to MusicBrainz. Missing or conflicting values stop the lookup before a request. Use `inspect` for offline reads. Public search requires no API key and is free for non-commercial use. [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API)

The command displays up to five recording candidates in MusicBrainz order, including title, artist credit, and recording ID. Review candidates before treating any as a match. The catalog can offer several recordings with the same title and artist. See the [live validation results](docs/sample-validation.md#musicbrainz-lookup).

A completed search exits `0`, including when it returns zero candidates. Input, service, and output errors exit `1`. Invalid arguments exit `2`. The request times out after 10 seconds. Concurrent lookups that reach the local limit report an error. l1ner uses a one-byte rate marker at `$XDG_CACHE_HOME/l1ner/musicbrainz-rate`, defaulting to `$HOME/.cache/l1ner/musicbrainz-rate`, to coordinate the [MusicBrainz rate limit](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting) across invocations. Search terms and results are held in memory, and the MP3 is opened read-only.

### Identify a song from audio

`identify` is an opt-in way to find a recording by sound when its tags are missing or wrong. Audio lookup requires Chromaprint's `fpcalc` and an AcoustID application key. Use the [audio identification guide](docs/audio-identification.md) to install `fpcalc`, create the right key, and set `L1NER_ACOUSTID_KEY` in your shell.

```sh
./bin/l1ner identify music/song.mp3
```

AcoustID receives the fingerprint and duration. The MP3, path, and tags stay local. The command shows recording candidates for review and leaves the file untouched.

### Interactive song preview

```sh
./bin/l1ner preview music/song.mp3
```

Run `preview` in a Linux terminal. It makes the same MusicBrainz search as `lookup` using the file's title and artist. Move through the candidate list with the arrow keys. The focused row expands to show its recording ID and any version description. Durations appear beside the titles when available. Press Enter to select a recording and collapse the list to one confirmation line.

The proposal shows the current and proposed title and artist side by side. When the values differ, choose **Accept proposal**, **Skip song**, **Choose another recording**, or **Quit**. Skip is highlighted by default. Accept prints a decision for the current run, and the file is opened read-only. Ctrl+C cancels the review.

A completed review or a search with no candidates exits `0`. Missing or conflicting tags, a service error, or redirected terminal input or output exits `1`. Invalid arguments exit `2`, and Ctrl+C exits `130`.

### Fix one song

```sh
./bin/l1ner fix music/song.mp3
```

`fix` opens the same MusicBrainz review and uses the MP3's current title and artist for the search. Choose **Apply changes to this MP3** to write the displayed title and artist. **Skip song** is highlighted by default. The command verifies the edited tags, audio and trailing bytes, then saves a byte-for-byte backup as `music/song.mp3.l1ner.bak` before replacing the MP3. The source directory must support hard links, rename, and directory sync.

Fixing requires a regular, writable MP3 you own, with one hard link. Tag-based search also needs an existing title and artist in a supported ID3v2.3.0 or ID3v2.4.0 tag. Fix refuses symlinks, extended file attributes, protected frames, and an existing backup. Close other editors for the file while applying a fix. See the [write constraints](docs/technical-plan.md#single-song-fix).

To restore the original bytes, confirm the overwrite when prompted and then inspect the restored file. This moves the backup into place.

```sh
mv -i -- music/song.mp3.l1ner.bak music/song.mp3
./bin/l1ner inspect music/song.mp3
```

Apply, Skip, Quit, and a review with no changes or candidates exit `0`. Input, service, verification, and write failures exit `1`. Invalid arguments exit `2`, and cancellation before replacement exits `130`. If a failure occurs after the backup is published, the backup path is included in the diagnostic.

### Preview or fix one song from audio

```sh
./bin/l1ner preview --from-audio music/song.mp3
./bin/l1ner fix --from-audio music/song.mp3
```

Use `--from-audio` when the MP3's title or artist is missing or wrong. After [optional AcoustID setup](docs/audio-identification.md#set-up-on-linux), choose a recording in the same keyboard picker. Its details show the AcoustID score. The proposal shows absent fields as **(missing)** and displays the title and artist to add or replace. Preview stays read-only.

In `fix`, **Apply changes to this MP3** writes the approved fields and saves the exact original as `.l1ner.bak`. A tagless MP3 gets an ID3v2.4 tag. An existing supported ID3v2 tag keeps its version and unrelated frames. The same file safety and recovery rules apply. The flag accepts one MP3, and the standard tag-based commands work without AcoustID setup.

### Fix a folder

```sh
./bin/l1ner fix music/
```

Review the regular `.mp3` files directly inside the folder in lexical order. Extensions are case-insensitive, and symlinks are skipped. The wizard shows progress such as **Song 3 of 20**. **Apply** edits and backs up the displayed song immediately. **Skip** advances to the next song. **Quit** or Ctrl+C preserves earlier approved edits and leaves the current song pending unless its edit already committed.

The batch leaves at least a second between MusicBrainz requests and continues after individual file or service failures. The summary counts applied, skipped, unchanged, unresolved, failed, and pending songs. A lookup with zero candidates is unresolved. An existing backup is a per-file failure, so rerunning the folder preserves backups and starts a new review from the beginning.

An empty folder or a completed or quit batch with zero failures exits `0`. A scan, file, or output failure exits `1`. Ctrl+C exits `130`. Use the same per-song recovery command to restore an applied song.

## Development

The initial development target is Linux with Go 1.27.3. Verify your Go installation:

```sh
go version
```

The selected metadata reader is [`github.com/bogem/id3v2/v2`](https://pkg.go.dev/github.com/bogem/id3v2/v2), pinned to `v2.1.4`. Dependencies are pinned in `go.mod` and `go.sum`.

Run checks:

```sh
go test ./...
go vet ./...
```

## Metadata coverage

Frames are displayed in identifier order. Text values are decoded by the metadata library; control characters are escaped for readable terminal output. Repeated frames are retained. Unknown frames and nested chapter data are summarized by identifier and byte size; they are not fully decoded.

ID3v1 and ID3v2.2 are unsupported. Tag-level flags (including extended headers and unsynchronisation), frame format flags (such as compression or encryption), and tags over 64 MiB produce explicit errors. This MVP recognizes MP3/ID3 headers but does not validate or decode the entire audio stream. When a supported ID3v2 tag and a legacy ID3v1 tag coexist, inspection covers the ID3v2 tag only.

Successful inspection, including no metadata, exits with status 0. Inspection errors exit with 1; invalid arguments exit with 2. Text-mode errors go to stderr. With `--json`, inspection and scan errors are included in the JSON report; invalid arguments and output-write failures go to stderr. `go run` may report its own exit status; use the built binary when consuming exit codes.

## Local music files

Keep local MP3 files in `music/`. This directory is ignored by Git, so audio files are not included in the repository.

The starting reference is Tom Lehrer's “When You Are Old and Gray.” Download details, source metadata, and reuse terms are recorded in the [reference file notes](docs/open-questions.md#broader-product--product-validation).

The documented local validation collection contains 20 MP3 samples from one artist and album family; it is not a representative library. See the [validation results](docs/sample-validation.md) for observed parser limitations and the [download manifest](docs/mp3-samples.tsv) for sources and checksums.

## Documentation

- [Next steps](TODO.md): prioritized tasks and future directions.
- [Product vision](docs/vision.md): concept, principles, MVP scope, and success criteria.
- [Open questions](docs/open-questions.md): agreed decisions and questions for later phases.
- [JSON output](docs/json-output.md): report format, examples, and error handling.
- [Audio identification](docs/audio-identification.md) explains AcoustID, application key setup, and troubleshooting.
- [Technical plan](docs/technical-plan.md): library selection, read-only workflow, and validation approach.
- [Sample validation](docs/sample-validation.md): local observations, provenance, and coverage limits.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, our Google Go style conventions, verification commands, and guidance for issues and pull requests. Coding agents should also read [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
