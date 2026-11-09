# l1ner

**Give your music its identity back.**

l1ner is an open-source CLI for inspecting local music files, with a longer-term vision of identifying tracks, enriching metadata, and helping users organize portable music collections.

**Status:** Technical planning. The CLI has not been implemented yet.

## First MVP

The first release will read existing metadata from a single MP3 file and display it in the terminal.

- Built in Go for Linux, running locally from source.
- Supports ID3v2.3 and ID3v2.4 tags.
- Displays all metadata exposed by the reader, including custom fields and repeated values.
- Shows text values in full and describes embedded artwork and other binary data by type and size where available.
- Shows only existing tags, or `No metadata found` when none are present.
- Reports unreadable files and unsupported tag versions clearly.
- Leaves the file unchanged and performs no network lookups.

Identification, external services, metadata editing, directory scanning, and batch processing are deferred.

## Planned usage

Once implemented, the CLI will accept:

```sh
l1ner inspect path/to/song.mp3
```

For local development, the planned invocation from the repository root is:

```sh
go run . inspect music/song.mp3
```

These commands describe the agreed interface; they are not runnable yet.

Illustrative output:

```text
File: music/song.mp3

Title: Example Song
Artist: Example Artist
Album: Example Album
Artwork: image/jpeg, 125000 bytes
```

## Development

The initial development target is Linux with Go 1.27.3. Verify your Go installation:

```sh
go version
```

The selected metadata reader is [`github.com/bogem/id3v2/v2`](https://pkg.go.dev/github.com/bogem/id3v2/v2), pinned to `v2.1.4`. Dependency setup will be documented when the Go module is created.

## Local music files

Keep local MP3 files in `music/`. This directory is ignored by Git, so audio files are not included in the repository.

The starting reference is Tom Lehrer's “When You Are Old and Gray.” Download details, source metadata, and reuse terms are recorded in the [reference file notes](docs/open-questions.md#broader-product--product-validation).

## Documentation

- [Product vision](docs/vision.md): concept, principles, MVP scope, and success criteria.
- [Open questions](docs/open-questions.md): agreed decisions and questions for later phases.
- [Technical plan](docs/technical-plan.md): library selection, read-only workflow, and validation approach.

## Contributing

The project is currently defining and validating its first MVP. Start with the documentation to understand the agreed scope and remaining questions. Issues and discussions about the plan are welcome.

## License

[MIT](LICENSE).
