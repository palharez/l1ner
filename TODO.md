# Next steps

The implemented CLI inspects local MP3 files and directories on Linux, read-only and offline. The original MVP covered one file; directory inspection is its first extension. Work through the immediate tasks in order; check items off when completed.

## Improve the current MVP

- [x] Add GitHub Actions CI on Linux to check formatting, run `go test ./...` and `go vet ./...`, and build the CLI. Tests must work without the ignored `music/` directory.
- [x] Add `CONTRIBUTING.md` with setup instructions, local checks, the current scope, and guidance for reporting bugs and proposing changes.
- [x] Add dedicated output tests for stable ordering, repeated values, Unicode, terminal control escaping, binary summaries, and output-write failures.
- [x] Expand inspection tests for truncated frame headers, invalid identifiers and sizes, unsupported frame flags, malformed padding, and the tag-size limit. Verify clear failures and unchanged input files.
- [x] Validate against additional legally usable MP3 samples and document observed parser limitations. Keep personal audio in the ignored `music/` directory; use generated fixtures for automated tests. See [20-file validation results](docs/sample-validation.md) and the [download manifest](docs/mp3-samples.tsv).
- [x] Reconcile the question tracker and technical plan with implementation findings, separating completed decisions from unresolved work.

## Decide the next milestone together

- [x] Review the first MVP's terminal output and error messages in everyday use. Accepted on 2026-11-10; batch output can be difficult to review per song, but this does not block the inspection milestone.
- [x] Agree on the next user-facing capability after directory inspection: `--json` writes one report to stdout, including per-file results, errors, and summary counts; no automatic storage.

## Completed extensions

- [x] Add directory scanning and batch inspection: immediate-folder scanning, optional `--recursive`, deterministic order, per-path errors, and summary counts.

- [x] Add optional JSON output for files and directories, document the versioned format, and test error reporting, repeated metadata, binary summaries, and input integrity.

## Future directions — not committed scope

- [ ] Revisit older tag versions and currently unsupported ID3 features.
- [ ] Research identification and metadata services using the questions in `docs/open-questions.md`.
- [ ] Design per-song review and acceptance of proposed changes before adding metadata writing. Clarify skipping/rejection, batch navigation, and integrity/recovery behavior.
- [ ] Consider packaging and other operating systems when local Linux usage is established.
- [ ] Revisit `cmd/l1ner/` and `internal/` packages when additional commands or responsibilities justify them. Keep the current layout for the single-command MVP.

See the [product vision](docs/vision.md), [open questions](docs/open-questions.md), and [technical plan](docs/technical-plan.md) for context. This checklist tracks work; the question tracker records decisions and their rationale.
