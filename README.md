# Moose

Fetches TV show metadata and writes Kodi-compatible NFO sidecar files.
Renames files to a canonical pattern. Downloads poster, fanart, and episode thumbs.

## Install

```bash
go install github.com/saltymouse/moose@latest
```

Requires Go 1.21+.

## Usage

`cd` into a show folder and run `moose`. That's it.

```
cd "/TV Shows/怒れる宇宙人おじさん (2019)"
moose
```

Moose reads the folder name, finds the show, asks two questions (language
and scraper), then handles everything: renames files to a canonical pattern,
writes NFOs, downloads poster/fanart/thumbs. A revert file is saved before
any renames so you can undo if something looks wrong at the final prompt.

You can also pass a path directly:

```
moose "/TV Shows/Biscuits & Betrayal (2023)"
```

### API key

The wizard will prompt for a key if you choose TMDb and one isn't set.
To avoid being asked every time, add it to your shell profile:

```bash
export TMDB_API_KEY=your_key_here
```

Get a free key at [themoviedb.org](https://www.themoviedb.org/signup) →
Settings → API → Developer.

---

## Flags

For scripted or automated use. All flags bypass the relevant wizard prompts.

| Flag | Description |
|------|-------------|
| `--scraper tvmaze\|tmdb` | Metadata source: [TVmaze](https://tvmaze.com) (no key) or [TMDb](https://themoviedb.org) (free key) |
| `--lang ja\|en-GB\|…` | Metadata language, TMDb only, BCP-47 tag |
| `--tmdb-key KEY` | TMDb API key (prefer `TMDB_API_KEY` env var) |
| `--show NAME` | Search by show name instead of guessing from folder |
| `--id ID` | Skip search, use this scraper ID directly |
| `--dir PATH` | Directory to scan (default: current directory) |
| `--rename` | Rename files to canonical pattern |
| `--force` | Overwrite existing NFOs and images |
| `--dry-run` | Preview actions without writing anything |
