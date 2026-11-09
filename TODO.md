# Next steps

The first MVP implements read-only inspection of one local MP3 file on Linux. Work through the immediate tasks in order; check items off when completed.

## Improve the current MVP

- [x] Add GitHub Actions CI on Linux to check formatting, run `go test ./...` and `go vet ./...`, and build the CLI. Tests must work without the ignored `music/` directory.
- [x] Add `CONTRIBUTING.md` with setup instructions, local checks, the current scope, and guidance for reporting bugs and proposing changes.
- [x] Add dedicated output tests for stable ordering, repeated values, Unicode, terminal control escaping, binary summaries, and output-write failures.
- [x] Expand inspection tests for truncated frame headers, invalid identifiers and sizes, unsupported frame flags, malformed padding, and the tag-size limit. Verify clear failures and unchanged input files.
- [x] Validate against additional legally usable MP3 samples and document observed parser limitations. Keep personal audio in the ignored `music/` directory; use generated fixtures for automated tests. See [20-file validation results](docs/sample-validation.md) and the [download manifest](docs/mp3-samples.tsv).
- [ ] Reconcile the question tracker and technical plan with implementation findings, separating completed decisions from unresolved work.

## Decide the next milestone together

- [ ] Review the first MVP's terminal output and error messages in everyday use.
- [ ] Agree on the next user-facing capability and its success criteria before extending the scope.

## Completed extension

- [x] Add directory scanning and batch inspection: immediate-folder scanning, optional `--recursive`, deterministic order, per-path errors, and summary counts.

## Future directions — not committed scope

- [ ] Revisit older tag versions and currently unsupported ID3 features.
- [ ] Research identification and metadata services using the questions in `docs/open-questions.md`.
- [ ] Define preview, approval, and integrity requirements before adding metadata writing.
- [ ] Consider packaging and other operating systems when local Linux usage is established.
- [ ] Revisit `cmd/l1ner/` and `internal/` packages when additional commands or responsibilities justify them. Keep the current layout for the single-command MVP.

See the [product vision](docs/vision.md), [open questions](docs/open-questions.md), and [technical plan](docs/technical-plan.md) for context. This checklist tracks work; the question tracker records decisions and their rationale.
