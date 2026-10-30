# Changelog

All notable changes to this project are documented in this file.

## Unreleased

## 0.3.1 - 2026-10-30

### Fixed

- Archive responses with a missing or null `data` field are reported as errors instead of an empty history that could overwrite the cache.
- Archive responses larger than the size limit are rejected instead of being parsed after truncation.
- Records with non-positive, non-finite, or out-of-range `created_utc` values are skipped instead of ending the crawl early.

### Changed

- Updated dependencies, including Bubbles 1.0.0 and modernc.org/sqlite 1.61.0.

## 0.3.0 - 2026-08-26

### Added

- Cross-platform CI for Linux, macOS, and Windows.
- Automated Homebrew tap updates after new GitHub releases.
- WinGet manifests for x64 and ARM64 Windows releases.
- Persistent configuration for exports, sorting, timeouts, cache behavior, startup health checks, and theme colors.
- Saved profiles and a local-data manager for cached histories and exports.
- Archive availability checks on startup.
- Expanded subreddit and activity-pattern analytics.
- SQLite history exports.

## 0.2.0 - 2026-08-25

### Added

- Windows x64 and ARM64 release archives with a PowerShell installer.
- CSV, JSON, and Markdown history exports.
- Profile summaries, Redditor comparisons, advanced filtering, sorting, bookmarks, diagnostics, offline cache access, and incremental refreshes.
- Full command-reference screen and terminal link actions.

### Changed

- Refined full-screen layouts, resizing behavior, long-record rendering, Unicode selection, and input sizing.

## 0.1.0 - 2026-08-24

### Added

- Initial Reddit archive TUI with concurrent Arctic Shift and PullPush history retrieval.
