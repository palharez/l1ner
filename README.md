# l1ner

**Give your music its identity back.**

l1ner is an open-source CLI for inspecting local music files, with a longer-term vision of identifying tracks, enriching metadata, and helping users organize portable music collections.

**Status:** First MVP implemented: local, read-only MP3 metadata inspection.

## First MVP

Read existing metadata from a single MP3 file and display it in the terminal.

- Built in Go for Linux, running locally from source.
- Supports ID3v2.3 and ID3v2.4 tags.
- Displays all metadata exposed by the reader, including custom fields and repeated values.
- Shows text values in full and describes embedded artwork and other binary data by type and size where available.
- Shows only existing tags, or `No metadata found` when none are present.
- Reports unreadable files and unsupported tag versions clearly.
- Leaves the file unchanged and performs no network lookups.

Identification, external services, metadata editing, directory scanning, and batch processing are deferred.

## Usage

From the repository root, download dependencies once:

```sh
go mod download
```

Inspect a local file:

```sh
go run . inspect music/song.mp3
```

Or build and run the CLI:

```sh
go build -o bin/l1ner .
./bin/l1ner inspect music/song.mp3
```

Use `go run . --help` for usage. File paths containing spaces must be quoted.

Illustrative output:

```text
File: music/song.mp3

Album [TALB]: Example Album
Artwork [APIC]: image/jpeg, 125000 bytes; picture type 3; description: Front cover
Title [TIT2]: Example Song
Artist [TPE1]: Example Artist
```

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

Successful inspection, including no metadata, exits with status 0. Inspection errors exit with 1; invalid arguments exit with 2. Errors go to stderr. `go run` may report its own exit status; use the built binary when consuming exit codes.

## Local music files

Keep local MP3 files in `music/`. This directory is ignored by Git, so audio files are not included in the repository.

The starting reference is Tom Lehrer's “When You Are Old and Gray.” Download details, source metadata, and reuse terms are recorded in the [reference file notes](docs/open-questions.md#broader-product--product-validation).

## Documentation

- [Next steps](TODO.md): prioritized tasks and future directions.
- [Product vision](docs/vision.md): concept, principles, MVP scope, and success criteria.
- [Open questions](docs/open-questions.md): agreed decisions and questions for later phases.
- [Technical plan](docs/technical-plan.md): library selection, read-only workflow, and validation approach.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, our Google Go style conventions, verification commands, and guidance for issues and pull requests. Coding agents should also read [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
