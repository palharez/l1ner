# l1ner

**Give your music its identity back.**

l1ner is an open-source CLI for inspecting local music files, with a longer-term vision of identifying tracks, enriching metadata, and helping users organize portable music collections.

**Status:** Offline MP3 inspection supports files and directories. An opt-in MusicBrainz lookup searches for recording candidates for one MP3.

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
- [Technical plan](docs/technical-plan.md): library selection, read-only workflow, and validation approach.
- [Sample validation](docs/sample-validation.md): local observations, provenance, and coverage limits.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, our Google Go style conventions, verification commands, and guidance for issues and pull requests. Coding agents should also read [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
