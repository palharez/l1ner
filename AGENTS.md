# Agent guidance

Read [CONTRIBUTING.md](CONTRIBUTING.md) before changing code. Its Go style and verification rules apply to agent contributions as well as human contributions.

- Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/guide), supported by its decisions and best practices. Prefer clear, focused functions and explicit error handling.
- Keep the current MVP local, Linux-only, and read-only. Preserve CLI output and exit behavior during refactoring unless the task explicitly changes them.
- Keep dependency-specific workarounds documented and covered by behavior tests. Do not remove repeated-frame handling or unknown-data summaries just to shorten functions.
- Keep personal audio in the ignored `music/` directory. Automated tests must use generated fixtures and work without that directory.
- Use `TODO.md` for upcoming work and `docs/open-questions.md` for decisions. Do not add packages, frameworks, or features solely to anticipate future requirements.
- Run the checks in `CONTRIBUTING.md` for code changes and report their results.
