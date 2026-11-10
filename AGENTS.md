# Agent guidance

Read [CONTRIBUTING.md](CONTRIBUTING.md) before changing code. Its Go style and verification rules apply to agent contributions as well as human contributions.

- Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/guide), supported by its decisions and best practices. Prefer clear, focused functions and explicit error handling.
- Keep `inspect` local and offline. Keep the CLI Linux-only and open music files read-only. Use explicit `lookup` or `preview` invocations for MusicBrainz requests. Preserve CLI output and exit behavior during refactoring unless the task explicitly changes them.
- Keep dependency-specific workarounds documented and covered by behavior tests. Do not remove repeated-frame handling or unknown-data summaries just to shorten functions.
- Keep personal audio in the ignored `music/` directory. Automated tests must use generated fixtures and work without that directory.
- Use `TODO.md` for upcoming work and `docs/open-questions.md` for decisions. Do not add packages, frameworks, or features solely to anticipate future requirements.
- Run the checks in `CONTRIBUTING.md` for code changes and report their results.
- After verifying a milestone, push its branch and use `gh pr create` to open a pull request against `main`. Share the PR link and check results with the user for review and merge. Follow the [feature workflow](CONTRIBUTING.md#feature-workflow).
