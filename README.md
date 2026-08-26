<pre align="center">
 _______  __   __  ___   _______    __   __  _______  __   __  ______  
|       ||  | |  ||   | |       |  |  | |  ||       ||  | |  ||      | 
|_     _||  |_|  ||   | |  _____|  |  |_|  ||   _   ||  | |  ||___   | 
  |   |  |       ||   | | |_____   |       ||  | |  ||  |_|  |  __|  | 
  |   |  |       ||   | |_____  |  |_     _||  |_|  ||       | |_____| 
  |   |  |   _   ||   |  _____| |    |   |  |       ||       |   __    
   |___|  |__| |__||___| |_______|    |___|  |_______||_______|  |__|    
</pre>

<p align="center">
  <strong>Explore a Reddit user's archived posts and comments without leaving your terminal.</strong>
</p>

<p align="center">
  <a href="https://github.com/martintrifunov/thisyou/releases"><img src="https://img.shields.io/github/v/release/martintrifunov/thisyou?style=flat-square&color=ff4b2b" alt="Latest release"></a>
  <a href="https://github.com/martintrifunov/thisyou/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/martintrifunov/thisyou/ci.yml?style=flat-square&label=builds&color=ff4b2b" alt="Cross-platform builds"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.27"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/martintrifunov/thisyou?style=flat-square&color=ff4b2b" alt="MIT license"></a>
</p>

`thisyou` is a fast, keyboard-driven TUI that searches [Arctic Shift](https://github.com/ArthurHeitmann/arctic_shift)
and [PullPush](https://pullpush.io/) concurrently, then merges and deduplicates the
archived results into one navigable history. No Reddit account, API key, or browser
workflow is required.

## Highlights

- Browse archived posts and comments in a responsive full-screen interface.
- Search both archives concurrently and continue when one source is unavailable.
- Filter by text, subreddit, record type, date, or archived score.
- Sort by newest, oldest, score, or subreddit.
- Inspect full records, open Reddit links, and copy useful fields.
- View profile summaries, compare Redditors, and inspect source coverage.
- Analyze subreddit share, activity patterns, and archived score snapshots.
- Bookmark records, save profiles, and reopen cached histories offline.
- Check archive availability on startup and manage local caches and exports.
- Incrementally refresh an existing history without starting over.
- Export the current result view to CSV, JSON, Markdown, or SQLite.

## Installation

### Homebrew — macOS and Linux

```sh
brew install martintrifunov/tap/thisyou
thisyou
```

Once installed, update to a newer release with:

```sh
brew update
brew upgrade thisyou
```

### PowerShell — Windows

Install the latest x64 or ARM64 release from PowerShell:

```powershell
irm https://raw.githubusercontent.com/martintrifunov/thisyou/main/install.ps1 | iex
```

The installer verifies the release checksum, places `thisyou.exe` in
`%LOCALAPPDATA%\Programs\thisyou`, and adds that directory to your user `PATH`.
Run the same install command again whenever you want to upgrade. Then start the
TUI with:

```powershell
thisyou
```

To review the installer before running it:

```powershell
irm https://raw.githubusercontent.com/martintrifunov/thisyou/main/install.ps1 -OutFile install-thisyou.ps1
Get-Content .\install-thisyou.ps1
.\install-thisyou.ps1
```

The repository also contains a WinGet manifest awaiting inclusion in Microsoft's
community package source. Once accepted, installation will be available as:

```powershell
winget install MartinTrifunov.ThisYou
```

### From source — macOS and Linux

```sh
git clone https://github.com/martintrifunov/thisyou.git
cd thisyou
make install
thisyou
```

`make install` uses Homebrew's `bin` directory when available and otherwise uses
`/usr/local/bin`. You can select a different destination with
`make install BINDIR="$HOME/.local/bin"`.

## Quick start

Run `thisyou`, enter a bare username, `u/username`, `@username`, or a Reddit
profile URL, and press `enter`. Press `?` on any screen for the complete command
reference.

```text
u/example_username
```

Use `tab` to move between all records, posts, and comments. Select a result with
`j` / `k`, inspect it with `enter`, and press `esc` to return.

## Command reference

### Search and navigation

| Key | Action |
| --- | --- |
| `enter` | Search the entered username or inspect the selected record |
| `j` / `k`, arrows | Move through records and scrollable views |
| `tab` / `shift+tab` | Switch between All, Posts, and Comments |
| `page up` / `page down` | Move by one visible page |
| `g` / `G` | Jump to the first or last record |
| `r` | Return to username search |
| `?` | Open the complete in-app command reference |

### Results and profiles

| Key | Action |
| --- | --- |
| `/` | Filter the current result view |
| `S` | Cycle newest, oldest, score, and subreddit sorting |
| `s` | Open the profile summary |
| `C` | Compare the current profile with another Redditor |
| `d` | Open archive stream and source-coverage diagnostics |
| `R` | Fetch and merge records newer than the current history |
| `e` | Export the current view as CSV, JSON, Markdown, or SQLite |
| `b` | Save or remove the selected bookmark |
| `p` | Save or remove the current profile |
| `B` | Open all saved bookmarks |
| `ctrl+b` | Open bookmarks from the username screen |
| `ctrl+o` | Open the entered user's latest cached history offline |
| `ctrl+p` | Open saved profiles |
| `ctrl+l` | Manage cached histories and exports |

### Record actions

| Key | Action |
| --- | --- |
| `o` | Open the selected record on Reddit |
| `y` | Copy a URL, text, author, subreddit, or linked URL |
| `esc` / `backspace` | Close the current secondary view |
| `q` | Quit from results or close the current secondary view |
| `ctrl+c` | Quit immediately from any screen |

After pressing `y`, choose `u` for the Reddit URL, `t` for record text, `a` for
the author, `s` for the subreddit, or `l` for the linked URL. URLs inside an
inspected record are terminal hyperlinks; Ctrl-click to open one, or Cmd-click in
terminals that use the macOS modifier.

## Filtering

The `/` filter combines ordinary search terms with optional query tokens. Every
plain-text term must match, and dates are inclusive.

```text
privacy sub:AskReddit type:comment after:2025-01-01 min:10
```

| Token | Meaning |
| --- | --- |
| `sub:AskReddit` | Match one subreddit |
| `type:post` | Match posts or comments |
| `after:2025-01-01` | Match that date and later |
| `before:2025-12-31` | Match that date and earlier |
| `min:10` / `max:100` | Match an archived score range |

## Exporting

Press `e` from the results screen, then choose `c` for CSV, `j` for JSON, `m`
for Markdown, or `s` for SQLite. The export contains the current tab, filter,
and sort order and is written to the configured export directory.

```text
thisyou-username-20260825-153000.csv
thisyou-username-20260825-153000.json
thisyou-username-20260825-153000.md
thisyou-username-20260825-153000.sqlite
```

SQLite exports contain `metadata`, `records`, and `record_sources` tables, with
indexes on subreddit and creation time.

## Cache and bookmarks

Completed crawls are cached per username. Enter a username and press `ctrl+o` to
reopen the latest saved history without contacting either archive. Press `R` from
results to fetch only newer records, merge them with the current history, and
update the cache.

Bookmarks contain complete archived records and persist between sessions in
`thisyou/bookmarks.json` inside the operating system's user-config directory.
Cache and bookmark files are written atomically with permissions restricted to
the current user.

Press `p` from results to save a profile, then `ctrl+p` to reopen it from any
search or results screen. Press `ctrl+l` to inspect cached-history and export
sizes, open cached profiles, or delete an entry after confirmation.

## Configuration

On first launch, `thisyou` creates `thisyou/config.json` in the operating
system's user-config directory. Available settings are:

```json
{
  "version": 1,
  "export_directory": ".",
  "default_sort": "newest",
  "request_timeout_seconds": 25,
  "cache_enabled": true,
  "startup_health_check": true,
  "theme": {
    "accent": "#ff4500",
    "muted": "#818384",
    "foreground": "#d7dadc",
    "border": "#343536"
  }
}
```

`default_sort` accepts `newest`, `oldest`, `score`, or `subreddit`. Relative
export paths are resolved from the directory where the application starts.

## How retrieval works

Four independent crawlers run concurrently: Arctic Shift posts, Arctic Shift
comments, PullPush posts, and PullPush comments. Each stream owns its timestamp
cursor, so a faster source cannot cause another source to skip records. The final
history is merged and deduplicated by Reddit ID.

Transient network failures, HTTP 429 responses, and server errors are retried. If
a source remains unavailable, `thisyou` retains the records returned by the other
streams and clearly marks the crawl as partial.

> [!IMPORTANT]
> `thisyou` can only display public records captured by third-party archives.
> Archive coverage and uptime are not guaranteed, so "entire history" means the
> complete history currently available from those sources. Use archived data
> responsibly and respect people's privacy and the services' terms.

## Development

macOS and Linux:

```sh
make test
make vet
make build
```

Windows PowerShell:

```powershell
.\build.ps1 -Task Test
.\build.ps1 -Task Build
```

Check an installed build without opening the TUI with `thisyou --version`.

## License

Released under the [MIT License](LICENSE).
