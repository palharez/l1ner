# MP3 sample validation

## Evidence and provenance

The local collection contains 20 Tom Lehrer MP3 files. The [manifest](mp3-samples.tsv) records source URLs, song pages, file sizes, and SHA-256 digests. These are provenance records; the audio itself stays in the ignored `music/` directory. The [artist's rights statement](https://tomlehrersongs.com/disclaimer/) is the source of reuse information. The repository's MIT license applies to project code, not these recordings.

The manifest was restored on 2026-11-10 from the local `music/sample-manifest.json`, the original sample's documented download, and `music/inspection-results.json`. All 20 saved inspection results reported exit code 0 and empty stderr. Their recorded digests match the current local files. The local JSON reports are ignored and are not required by automated tests.

## Observed behavior

The saved results cover ID3v2.3.0 files with the same general metadata layout: album, title, artist, track number, and a nonstandard `YEAR` frame. The recognized text fields are displayed; `YEAR` appears as `unparsed data, 23 bytes` rather than a decoded date. Successful inspection means the file was processed within the documented coverage; it does not mean every frame was decoded or its content is correct.

The earlier recursive directory check reported `Summary: 20 inspected, 0 failed, 0 scan errors`, with whole-file checksums unchanged. This is a local validation record, not a claim that CI downloaded or tested this collection.

## JSON review

The recursive JSON inspection returned 20 successful file results with no scan errors. All files matched the manifest before inspection, and their sizes, permissions, and SHA-256 checksums were unchanged afterward.

Each result included a title, artist, album, and track number. “My Home Town” was tagged as *Revisited*, while the other 19 were tagged as *The Remains of Tom Lehrer*. The title “She’s My Girl” retained its curly apostrophe, which is absent from the filename. All 20 `YEAR` frames appeared as 23-byte unknown entries.

The raw report spanned 731 lines. For per-song review, the tagged title, artist, album, track number, and file path provide a compact starting view. The album field gives context to the track number, and the file path identifies the exact input even when the filename differs from the tagged title.

## MusicBrainz lookup

On 2026-11-10, the built CLI queried MusicBrainz with title and artist tags from four local samples. The input SHA-256 checksums matched before and after each lookup. Expected recording IDs came from the catalog listings for the sample versions in the [manifest](mp3-samples.tsv).

| Sample | Observed result |
| --- | --- |
| My Home Town | The [live recording on *Revisited*, track 9](https://musicbrainz.org/recording/232265e2-7983-461a-918a-be86d0b87a9c) appeared fifth after an initial HTTP 503. |
| When You Are Old and Gray | Five candidates appeared. The [expected live recording](https://musicbrainz.org/recording/9bd8d12b-188c-4b2a-8667-ecc035aa4302), cataloged as “When You Are Old and Grey,” was absent. |
| The Wiener Schnitzel Waltz | Five candidates appeared, including the live version. The [expected studio recording on disc 1, track 12](https://musicbrainz.org/recording/9d769e0b-cb10-4ae8-abe2-a915b71cef74) was absent. |
| It Makes a Fellow Proud to Be a Soldier | MusicBrainz returned HTTP 503 and the CLI exited `1`. |

The results show why recordings with similar credits still need human review. Broader retrieval accuracy and ranking remain open.

## Interactive preview

The built CLI ran in a Linux pseudo-terminal with a generated ID3v2.3 fixture tagged `my home town` and `Tom Lehrer`. MusicBrainz returned recording candidates with durations and version descriptions. After selecting a recording, the preview showed the title changing to `My Home Town` and the artist matching the current tag. Down and Enter accepted the proposal for that run. The command exited `0`, the terminal mode was restored, and the fixture's SHA-256 matched before and after.

## Single-song fix

The built CLI ran in a Linux pseudo-terminal on a disposable copy of “My Home Town” with a lowercase title tag. The wizard showed the MusicBrainz title as the only change, then applied it after explicit approval. Inspection showed `My Home Town`. An independent byte comparison confirmed that all other frames and the audio and trailing bytes were preserved. The backup matched the pre-edit copy byte for byte, and moving it back restored those exact bytes. The reference MP3 in `music/` was unchanged.

## Folder fixes

The built CLI ran in a Linux pseudo-terminal on four disposable copies of “My Home Town” with lowercase title tags. The live review applied the first proposal, skipped the second, and quit on the third. The summary reported one applied, one skipped, and two pending. The applied file retained its audio bytes and had a byte-for-byte backup. The other copies and the reference MP3 in `music/` stayed unchanged, and the terminal mode was restored.

## Limits of this collection

The collection covers one artist, one album family, and one tag version. Generated tests cover ID3v2.4, artwork, repeated fields, unusual encodings, malformed files, and unsupported flags independently of the downloads.

To repeat local validation, download samples from the manifest into `music/`, build the CLI, and inspect the directory:

```sh
go build -o bin/l1ner .
./bin/l1ner inspect --recursive music/
```

Compare each file's SHA-256 before and after inspection, retain stderr and the built binary's exit status, and compare decoded fields with independently read embedded values. The source links may change or become unavailable.
