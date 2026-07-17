# General Guidelines

Keep solutions simple. Start with the minimal implementation that solves the problem. Don't add extra suggestions or features unless explicitly requested.

## Project Documentation

Read the relevant documentation before changing an established workflow:

- User-facing installation, commands, and behavior: `README.md`
- Build, demo rendering, and releases: `docs/development.md`
- Configuration behavior and product decisions: `docs/config-review.md`
- Terminal layout and copy: `docs/terminal-ui.md`
- Demo and promotional media: `docs/demo-video-plan.md` and `docs/promo-voiceover.md`
- Homebrew publishing: `docs/homebrew.md`
- Current implementation status and version history: `docs/roadmap.md` and `CHANGELOG.md`

Update the corresponding documentation when behavior or workflows change.

## Environment Setup

Go environment: Use the Go version declared in `go.mod` or newer. The project must remain buildable without third-party modules.

## Code Organization

Before creating new utility modules or helper files, check for existing utils in the project and reuse them. Run `find . -name '*util*' -o -name '*helper*'` first.

## Performance & Data Access

When implementing features, prefer using existing cached data files over making API calls. Check for local data sources first before fetching from external APIs.
