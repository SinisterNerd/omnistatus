# Contributing to omniStatus

Contributions are welcome! This project is licensed under AGPLv3 (see [`LICENSE`](../LICENSE)) - by submitting a contribution, you agree it's licensed under those same terms, same as any public GitHub repository.

## Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes
4. Push to your branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Developer Certificate of Origin (DCO)

Commits should include a `Signed-off-by` line certifying you wrote the change (or otherwise have the right to submit it under this project's license) - the standard [Developer Certificate of Origin](https://developercertificate.org/) used by the Linux kernel, Docker, GitLab, and many other projects. Add it automatically with:

```bash
git commit -s -m "Your commit message"
```

This appends a line like `Signed-off-by: Your Name <your.email@example.com>` to the commit message, using the name/email from your git config.

**This isn't currently checked automatically** - there's no CI enforcement yet, so a missing sign-off won't block your PR. Please include it anyway; it may become required later, and it costs nothing to add now. If you forget, `git commit --amend -s` re-signs your most recent commit.

## Code Style

- Run `gofmt`/`make fmt` before submitting
- Follow standard [Effective Go](https://go.dev/doc/effective_go) conventions
- See [`docs/dev/DEVELOPMENT.md`](dev/DEVELOPMENT.md) for the fuller developer guide (project layout, adding a new platform, testing, releasing)
