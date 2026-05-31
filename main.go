package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true,
	".m4v": true, ".mov": true, ".ts":  true,
}

func main() {
	showName  := flag.String("show", "", "Show name to search")
	showID    := flag.Int("id", 0, "Scraper show ID (skips search)")
	dir       := flag.String("dir", ".", "Directory containing video files")
	dryRun    := flag.Bool("dry-run", false, "Print actions without writing any files")
	force     := flag.Bool("force", false, "Overwrite existing NFO files")
	rename    := flag.Bool("rename", false, "Rename files to canonical pattern before writing NFOs")
	scraperID := flag.String("scraper", "tvmaze", "Scraper to use: tvmaze or tmdb")
	lang      := flag.String("lang", "", "Language for metadata (e.g. ja, en-GB). Defaults: tmdb=en-US, tvmaze=n/a")
	tmdbKey   := flag.String("tmdb-key", "", "TMDb API key (or set TMDB_API_KEY env var)")
	flag.Parse()

	// --- Build scraper ---
	s, err := newScraper(*scraperID, *lang, *tmdbKey)
	if err != nil {
		log.Fatalf("scraper: %v", err)
	}

	// --- Resolve show ---
	var show *Show
	switch {
	case *showID != 0:
		show, err = s.FetchShow(*showID)
	case *showName != "":
		results, serr := s.SearchShows(*showName)
		if serr != nil {
			log.Fatalf("search: %v", serr)
		}
		if len(results) == 0 {
			log.Fatalf("no results for %q", *showName)
		}
		if len(results) == 1 {
			show = &results[0].Show
		} else {
			show, err = pickShow(results)
		}
	default:
		show, err = GuessShow(*dir, s)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Using: %s  (ID %d, scraper: %s)\n\n", show.Name, show.ID, s.IDType())

	// --- Fetch episodes ---
	episodes, err := s.FetchEpisodes(show.ID)
	if err != nil {
		log.Fatalf("episode fetch: %v", err)
	}
	fmt.Printf("Fetched %d episodes\n\n", len(episodes))

	// Build lookup: season → episode number → *Episode
	lookup := make(map[int]map[int]*Episode)
	for i := range episodes {
		ep := &episodes[i]
		if lookup[ep.Season] == nil {
			lookup[ep.Season] = make(map[int]*Episode)
		}
		lookup[ep.Season][ep.Number] = ep
	}

	// --- Rename phase ---
	if *rename {
		fmt.Println("── Rename ──────────────────────────────────────────")
		ops, alreadyOK, noData := planRenames(*dir, show, lookup)
		if err := confirmAndRename(ops, alreadyOK, noData, *dryRun); err != nil {
			log.Fatalf("rename: %v", err)
		}
		fmt.Println()
	}

	// --- NFO phase ---
	fmt.Println("── NFO ─────────────────────────────────────────────")
	var written, skipped, notFound, unparsed int

	err = filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		fe, ok := ParseFilename(filepath.Base(path))
		if !ok {
			fmt.Printf("  ? unparsed:  %s\n", filepath.Base(path))
			unparsed++
			return nil
		}
		if fe.SeasonInferred {
			fmt.Printf("  ~ season inferred as 1: %s\n", filepath.Base(path))
		}
		ep := lookup[fe.Season][fe.Episodes[0]]
		if ep == nil {
			fmt.Printf("  ✗ not found: S%02dE%02d  %s\n", fe.Season, fe.Episodes[0], filepath.Base(path))
			notFound++
			return nil
		}
		nfoPath := path[:len(path)-len(ext)] + ".nfo"
		if !*force {
			if _, statErr := os.Stat(nfoPath); statErr == nil {
				skipped++
				return nil
			}
		}
		if *dryRun {
			fmt.Printf("  ~ would write: %s\n", filepath.Base(nfoPath))
			written++
			return nil
		}
		if err := WriteNFO(nfoPath, show, ep, s.IDType()); err != nil {
			fmt.Printf("  ✗ error: %s: %v\n", filepath.Base(nfoPath), err)
			return nil
		}
		fmt.Printf("  ✓ wrote: %s\n", filepath.Base(nfoPath))
		written++
		return nil
	})
	if err != nil {
		log.Fatalf("walk: %v", err)
	}

	fmt.Printf("\n%d written  %d skipped (existing)  %d not found  %d unparsed\n",
		written, skipped, notFound, unparsed)
}

func newScraper(name, lang, tmdbKey string) (Scraper, error) {
	switch name {
	case "tvmaze":
		return &TVmazeScraper{Lang: lang}, nil
	case "tmdb":
		key := tmdbKey
		if key == "" {
			key = os.Getenv("TMDB_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("TMDb requires an API key: use --tmdb-key or set TMDB_API_KEY")
		}
		if lang == "" {
			lang = "en-US"
		}
		return &TMDbScraper{Key: key, Lang: lang}, nil
	default:
		return nil, fmt.Errorf("unknown scraper %q — choose tvmaze or tmdb", name)
	}
}
