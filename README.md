# thisyou

`thisyou` is a keyboard-driven terminal UI for viewing a Reddit user's archived
post and comment history. It queries [Arctic Shift](https://github.com/ArthurHeitmann/arctic_shift)
and [PullPush](https://pullpush.io/) concurrently, follows each source's timestamp
cursor to exhaustion, then merges and deduplicates records by Reddit ID.

```text
 _______  __   __  ___   _______    __   __  _______  __   __  ______
|       ||  | |  ||   | |       |  |  | |  ||       ||  | |  ||      |
|_     _||  |_|  ||   | |  _____|  |  |_|  ||   _   ||  | |  ||___   |
  |   |  |       ||   | | |_____   |       ||  | |  ||  |_|  |  __|  |
  |   |  |       ||   | |_____  |  |_     _||  |_|  ||       | |_____|
  |   |  |   _   ||   |  _____| |    |   |  |       ||       |   __
  |___|  |__| |__||___| |_______|    |___|  |_______||_______|  |__|
```

## Requirements

- Go 1.27 or newer
- Network access to the two public archive APIs

## Build and run

```sh
make build
./thisyou
```

Or install it into a binary directory already on your `PATH`:

```sh
make install
thisyou
```

`make install` uses Homebrew's `bin` directory when Homebrew is available and
otherwise uses `/usr/local/bin`. Override the destination when needed:

```sh
make install BINDIR="$HOME/.local/bin"
```

Enter a bare username, `u/username`, `@username`, or a Reddit profile URL. No
Reddit login or API credentials are needed.

Check the installed version without starting the TUI:

```sh
thisyou --version
```

## Keys

| Key | Action |
| --- | --- |
| `j` / `k`, arrows | Move through history |
| `tab`, `shift+tab` | Switch All, Posts, and Comments |
| `enter` | Open the complete archived record |
| `s` | Open the profile summary |
| `b` | Save or remove a bookmark |
| `B` | Open saved bookmarks from results |
| `ctrl+b` | Open saved bookmarks from the username screen |
| `o` | Open the selected record on Reddit |
| `y` | Copy the record URL, text, author, subreddit, or linked URL |
| `/` | Filter the current view |
| `g` / `G` | Jump to first / last result |
| `page up` / `page down` | Move by one screen |
| `r` | Search another username |
| `esc` | Cancel a crawl or close a record, summary, or bookmarks |
| `q` | Quit from results; close a record, summary, or bookmarks |
| `ctrl+c` | Quit from any screen |

After pressing `y`, use `u` for the Reddit URL, `t` for record text, `a` for
the author, `s` for the subreddit, or `l` for the linked URL. Press `esc` to
close the copy menu.

URLs in an inspected record are clickable terminal links. Ctrl-click them to
open a browser tab; some macOS terminals use Cmd-click instead. The `o` key
remains available when terminal hyperlinks are unsupported.

The profile summary covers every fetched record regardless of the active tab or
filter. Scores are archive snapshots and may differ from current Reddit karma.

Bookmarks contain complete archived records and persist between sessions in
`thisyou/bookmarks.json` inside the operating system's user-config directory.
The file is written atomically with permissions restricted to the current user.

## How retrieval works

Four crawlers run concurrently:

- Arctic Shift posts
- Arctic Shift comments
- PullPush posts
- PullPush comments

Each crawler owns its own `before` cursor. This matters because one archive may
reach older records sooner than the other; sharing a cursor could skip results.
Transient network errors, HTTP 429 responses, and server errors are retried once.
If a source remains unavailable, `thisyou` keeps and displays results returned by
the other streams and labels them as partial.

The program can only display records captured by these third-party archives.
Archive coverage and uptime are not guaranteed, so “entire history” means the
complete history currently available from those sources. Use archived public data
responsibly and respect people's privacy and the services' terms.

## Development

```sh
make test
make vet
```
