# JSON inspection output

Use `--json` before the path to write one JSON document to stdout:

```sh
./bin/l1ner inspect --json music/song.mp3
./bin/l1ner inspect --json --recursive music/
./bin/l1ner inspect --json --recursive music/ > inspection.json
```

The CLI does not create a report file or store output in `/tmp`. Shell redirection chooses the destination and overwrites an existing file. Keep the destination outside the files being inspected. Text output remains the default.

## Report format — version 1

Single-file and directory inspection share this structure. Illustrative output:

```json
{
  "schema_version": 1,
  "files": [
    {
      "path": "music/song.mp3",
      "status": "ok",
      "metadata": [
        {
          "id": "TIT2",
          "label": "Title [TIT2]",
          "kind": "text",
          "value": "Example Song"
        }
      ]
    }
  ],
  "scan_errors": [],
  "summary": {
    "inspected": 1,
    "failed": 0,
    "scan_errors": 0
  }
}
```

| Field | Meaning |
| --- | --- |
| `schema_version` | Output format version, currently `1`. |
| `files` | One result per inspected MP3, including failed files. Directory discovery rules match text mode. |
| `files[].path` | Path used for inspection. |
| `files[].status` | `ok` or `error`. A file without metadata is still `ok`. |
| `files[].metadata` | Array of metadata entries; empty for absent metadata or a failed inspection. |
| `files[].error` | Inspection error message, present only on failure. Messages are human-readable; do not parse them as error codes. |
| `scan_errors` | Array of directory-discovery failures, each with `path` and `message`. |
| `summary` | Successful inspections, failed inspections, and directory scan errors. Counts match text mode. |

All collection fields are arrays, including when empty; they are never `null`. A missing input path is represented as a failed file result. An empty directory has empty arrays and zero counts.

## Metadata entries

Entries are sorted by frame identifier, retaining the original order among repeated identifiers. Use `id` and structured fields for processing; `label` is a human-readable display name. Custom fields also include their `description`. Do not convert the array into a map keyed only by `id`, because repeated frames would be lost.

Every entry contains `id`, `label`, and `kind`:

| Kind | Additional fields |
| --- | --- |
| `text` | `value`, including an empty string when present. Comments/lyrics include `language` and `description` when available. Ordinary URL frames are text. |
| `binary` | `size_bytes`; artwork also includes `mime_type`, `picture_type`, and `description` when available. Identifiers include `owner`. Raw payloads are omitted. |
| `unknown` | `size_bytes`; used for unparsed frames and nested chapter data. Raw payloads are omitted. |
| `rating` | `owner` identifies the listener, `rating` is numeric, and `play_count` is a decimal string when available to avoid integer precision loss in consumers. |

Empty optional string fields are omitted. Zero sizes, picture types, and ratings are retained. JSON preserves decoded text values, including control characters via JSON escaping; it does not use text mode's literal escape substitutions. Invalid UTF-8 is replaced by the standard Go JSON encoder. This is an inspection report, not a lossless tag backup or a metadata-write plan.

## Errors and exit status

- `0`: all requested inspections succeeded, including a successful empty scan.
- `1`: at least one inspection or directory scan failed, or the report could not be written.
- `2`: invalid invocation, such as `--recursive` with a file.

Inspection and discovery errors are included in the report, with no duplicate human-readable errors on stderr. Invalid arguments produce stderr diagnostics and no report. Help prints usage rather than JSON. Output-write failures produce stderr diagnostics; stdout may be incomplete, so consumers must handle parse failures.

Use the built binary when consuming exact exit codes; `go run` can wrap the program's failure status. A valid report can accompany exit code 1 and contain successful files alongside failures. Report results are collected before encoding one document, so JSON mode retains the metadata summaries in memory until inspection finishes.
