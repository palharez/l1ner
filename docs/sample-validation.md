# MP3 sample validation

## Evidence and provenance

The local collection contains 20 Tom Lehrer MP3 files. The [manifest](mp3-samples.tsv) records source URLs, song pages, file sizes, and SHA-256 digests. These are provenance records; the audio itself stays in the ignored `music/` directory. The [artist's rights statement](https://tomlehrersongs.com/disclaimer/) is the source of reuse information. The repository's MIT license applies to project code, not these recordings.

The manifest was restored on 2026-11-10 from the local `music/sample-manifest.json`, the original sample's documented download, and `music/inspection-results.json`. All 20 saved inspection results reported exit code 0 and empty stderr. Their recorded digests match the current local files. The local JSON reports are ignored and are not required by automated tests.

## Observed behavior

The saved results cover ID3v2.3.0 files with the same general metadata layout: album, title, artist, track number, and a nonstandard `YEAR` frame. The recognized text fields are displayed; `YEAR` appears as `unparsed data, 23 bytes` rather than a decoded date. Successful inspection means the file was processed within the documented coverage; it does not mean every frame was decoded or its content is correct.

The earlier recursive directory check reported `Summary: 20 inspected, 0 failed, 0 scan errors`, with whole-file checksums unchanged. This is a local validation record, not a claim that CI downloaded or tested this collection.

## Limits of this collection

One artist, one album family, and one tag version do not form a representative music library. This collection does not establish coverage for ID3v2.4, artwork, repeated fields, unusual encodings, malformed files, or unsupported flags. Generated tests cover those supported and rejected cases independently of the downloads. Recognition accuracy and service matching have not been evaluated because the CLI performs no identification or lookups.

To repeat local validation, download samples from the manifest into `music/`, build the CLI, and inspect the directory:

```sh
go build -o bin/l1ner .
./bin/l1ner inspect --recursive music/
```

Compare each file's SHA-256 before and after inspection, retain stderr and the built binary's exit status, and compare decoded fields with independently read embedded values. The source links may change or become unavailable.
