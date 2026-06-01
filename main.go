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
	showName := flag.String("show", "", "Show name to search")
	showID   := flag.Int("id", 0, "Scraper show ID (skips search)")
	dirFlag  := flag.String("dir", "", "Directory containing video files (or pass as first arg)")
	dryRun   := flag.Bool("dry-run", false, "Print actions without writing any files")
	force    := flag.Bool("force", false, "Overwrite existing NFO files")
	rename   := flag.Bool("rename", false, "Rename files to canonical pattern (flag-mode only)")
	scraper  := flag.String("scraper", "", "Scraper: tvmaze or tmdb (skips wizard prompt)")
	lang     := flag.String("lang", "", "Metadata language, e.g. ja, en-GB (skips wizard prompt)")
	tmdbKey  := flag.String("tmdb-key", "", "TMDb API key (or set TMDB_API_KEY env var)")
	flag.Parse()

	// Directory: positional arg takes precedence over --dir; both default to ".".
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	} else if *dirFlag != "" {
		dir = *dirFlag
	}

	// Wizard mode: run when no flags that imply scripted/non-interactive use are set.
	// Specifically: if neither --scraper nor --lang nor --dry-run nor --force nor --rename
	// is passed, we go interactive. --show and --id are fine in wizard mode (they skip
	// the show-search step but keep the language/scraper prompts).
	wizardMode := *scraper == "" && *lang == "" && !*dryRun && !*force && !*rename

	if wizardMode {
		err := RunWizard(WizardFlags{
			ShowName: *showName,
			ShowID:   *showID,
			Dir:      dir,
			DryRun:   false,
			Force:    false,
			Scraper:  "",
			Lang:     "",
			TMDbKey:  *tmdbKey,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// --- Flag-driven (non-interactive) path --- preserves original behaviour exactly.

	scraperName := *scraper
	if scraperName == "" {
		scraperName = "tvmaze"
	}

	s, err := newScraper(scraperName, *lang, *tmdbKey)
	if err != nil {
		log.Fatalf("scraper: %v", err)
	}

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
		show, err = GuessShow(dir, s)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Using: %s  (ID %d, scraper: %s)\n\n", show.Name, show.ID, s.IDType())

	episodes, err := s.FetchEpisodes(show.ID)
	if err != nil {
		log.Fatalf("episode fetch: %v", err)
	}
	fmt.Printf("Fetched %d episodes\n\n", len(episodes))

	lookup := buildLookup(episodes)

	if *rename {
		fmt.Println("── Rename ──────────────────────────────────────────")
		ops, alreadyOK, noData := planRenames(dir, show, lookup)
		if err := confirmAndRename(ops, alreadyOK, noData, *dryRun); err != nil {
			log.Fatalf("rename: %v", err)
		}
		fmt.Println()
	}

	fmt.Println("── NFO ─────────────────────────────────────────────")
	written, skipped, notFound, _, _ := writeNFOs(dir, show, lookup, s.IDType(), *force, *dryRun)

	// Also track unparsed files in flag mode (wizard mode skips them silently).
	unparsed := countUnparsed(dir)
	fmt.Printf("\n%d written  %d skipped (existing)  %d not found  %d unparsed\n",
		written, skipped, notFound, unparsed)
}

func countUnparsed(dir string) int {
	var n int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		if _, ok := ParseFilename(filepath.Base(path)); !ok {
			fmt.Printf("  ? unparsed:  %s\n", filepath.Base(path))
			n++
		}
		return nil
	})
	return n
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
