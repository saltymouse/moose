package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

// WizardFlags carries the parsed flag values so the wizard knows which steps to skip.
type WizardFlags struct {
	ShowName string
	ShowID   int
	Dir      string
	DryRun   bool
	Force    bool
	Scraper  string // empty = not set by flag
	Lang     string // empty = not set by flag
	TMDbKey  string
}

func RunWizard(f WizardFlags) error {
	prefs := LoadPrefs()

	// --- Step 1: Language ---
	lang := f.Lang
	if lang == "" {
		lang = prefs.Lang
		if err := askLang(&lang); err != nil {
			return err
		}
	}

	// --- Step 2: Scraper ---
	scraperName := f.Scraper
	if scraperName == "" {
		scraperName = prefs.Scraper
		if scraperName == "" {
			scraperName = "tvmaze"
		}
		if err := askScraper(&scraperName); err != nil {
			return err
		}
	}

	// --- Step 3: TMDb key if needed ---
	tmdbKey := f.TMDbKey
	if scraperName == "tmdb" && tmdbKey == "" {
		tmdbKey = os.Getenv("TMDB_API_KEY")
	}
	if scraperName == "tmdb" && tmdbKey == "" {
		if err := askTMDbKey(&tmdbKey); err != nil {
			return err
		}
		fmt.Println("\nTip: add this to ~/.profile to avoid being asked again:")
		fmt.Printf("  export TMDB_API_KEY=%s\n\n", tmdbKey)
	}

	_ = SavePrefs(Prefs{Lang: lang, Scraper: scraperName})

	s, err := newScraper(scraperName, lang, tmdbKey)
	if err != nil {
		return err
	}

	// --- Remote phase ---
	fmt.Println("── Remote ──────────────────────────────────────────")

	var show *Show
	switch {
	case f.ShowID != 0:
		show, err = s.FetchShow(f.ShowID)
	case f.ShowName != "":
		results, serr := s.SearchShows(f.ShowName)
		if serr != nil {
			return fmt.Errorf("search: %w", serr)
		}
		if len(results) == 0 {
			return fmt.Errorf("no results for %q", f.ShowName)
		}
		if len(results) == 1 {
			show = &results[0].Show
		} else {
			show, err = pickShow(results)
		}
	default:
		show, err = GuessShow(f.Dir, s)
	}
	if err != nil {
		return err
	}

	// Search results omit details (TMDb: genres, status, network), so
	// refetch the full record when the show didn't come from FetchShow.
	if f.ShowID == 0 {
		if full, ferr := s.FetchShow(show.ID); ferr == nil {
			show = full
		}
	}

	episodes, err := s.FetchEpisodes(show.ID)
	if err != nil {
		return fmt.Errorf("episode fetch: %w", err)
	}
	fmt.Printf("  Show:     %s\n", show.Name)
	fmt.Printf("  Scraper:  %s  ·  Language: %s\n", s.IDType(), lang)
	fmt.Printf("  Episodes: %d fetched\n", len(episodes))

	lookup := buildLookup(episodes)

	// --- Local phase ---
	fmt.Println("\n── Local ───────────────────────────────────────────")
	fmt.Printf("  Directory: %s\n\n", f.Dir)

	// Pre-scan: count video files and parse results before doing anything.
	type scanResult struct {
		path   string
		fe     FileEpisode
		parsed bool
	}
	var scanResults []scanResult
	filepath.Walk(f.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		fe, ok := ParseFilename(filepath.Base(path))
		scanResults = append(scanResults, scanResult{path, fe, ok})
		return nil
	})

	if len(scanResults) == 0 {
		fmt.Println("  ✗ No video files found.")
		fmt.Println("\nNothing to do.")
		return nil
	}

	var parsedCount, unparsedCount int
	for _, r := range scanResults {
		if r.parsed {
			parsedCount++
		} else {
			unparsedCount++
		}
	}

	fmt.Printf("  Video files found: %d\n", len(scanResults))
	if parsedCount > 0 {
		fmt.Printf("  ✓ Matched to episode pattern: %d\n", parsedCount)
	}
	if unparsedCount > 0 {
		fmt.Printf("  ✗ Could not parse episode number: %d\n", unparsedCount)
		for _, r := range scanResults {
			if !r.parsed {
				fmt.Printf("      %s\n", filepath.Base(r.path))
			}
		}
	}

	if parsedCount == 0 {
		fmt.Println("\n  No files could be matched to episode data.")
		fmt.Println("  Check that filenames contain an episode number (S01E01, EP01, - 01, etc.)")
		fmt.Println("\nNothing to do.")
		return nil
	}

	// --- Rename preview + confirm ---
	ops, alreadyOK, noData := planRenames(f.Dir, show, lookup)

	if len(ops) == 0 && alreadyOK > 0 {
		fmt.Printf("\n  All %d files already correctly named.\n", alreadyOK)
	} else if len(ops) > 0 {
		fmt.Println()
		if noData > 0 {
			fmt.Printf("  (%d files skipped — no scraper match)\n\n", noData)
		}
		previewRenames(ops, len(ops))
	}

	if f.DryRun {
		return nil
	}

	title := "Write NFOs and download images?"
	if len(ops) > 0 {
		title = fmt.Sprintf("Rename %d files, write NFOs, and download images?", len(ops))
	}
	var confirmed bool
	if err := huh.NewConfirm().
		Title(title).
		Affirmative("Yes, proceed").
		Negative("Cancel").
		Value(&confirmed).
		Run(); err != nil || !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}
	fmt.Println()

	// Apply renames
	for _, op := range ops {
		if err := os.Rename(op.OldPath, op.NewPath); err != nil {
			fmt.Printf("  ✗ rename failed: %s: %v\n", filepath.Base(op.OldPath), err)
		}
	}

	// --- NFO phase ---
	forceNFO := f.Force || len(ops) > 0
	writeShowNFO(f.Dir, show, s.IDType(), f.Force, false)
	writeNFOs(f.Dir, show, lookup, s.IDType(), forceNFO, false)

	// --- Image phase ---
	fmt.Println("\n── Images ──────────────────────────────────────────")
	DownloadShowImages(f.Dir, show, f.Force)
	imgDown, imgSkip, imgFail, imgNone := DownloadEpisodeThumbs(f.Dir, show, lookup, f.Force)
	switch {
	case imgDown == 0 && imgSkip > 0:
		fmt.Printf("  %d thumbs already present\n", imgSkip)
	case imgDown == 0 && imgSkip == 0 && imgFail == 0 && imgNone > 0:
		fmt.Printf("  No episode stills available from %s\n", s.IDType())
	}

	// --- Summary ---
	fmt.Println("\n── Summary ─────────────────────────────────────────")
	if len(ops) > 0 {
		fmt.Printf("  Renamed:  %d files\n", len(ops))
	}
	if imgDown > 0 || imgFail > 0 {
		fmt.Printf("  Images:   %d downloaded", imgDown)
		if imgFail > 0 {
			fmt.Printf(", %d failed", imgFail)
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Done ✓")
	return nil
}

// --- Prompt helpers ---

func askLang(lang *string) error {
	options := []huh.Option[string]{
		huh.NewOption("Japanese (ja)", "ja"),
		huh.NewOption("English — GB (en-GB)", "en-GB"),
		huh.NewOption("English — US (en-US)", "en-US"),
		huh.NewOption("Korean (ko)", "ko"),
		huh.NewOption("Chinese — Simplified (zh-CN)", "zh-CN"),
		huh.NewOption("Other (type below)", "__other__"),
	}
	initial := *lang
	found := false
	for _, o := range options {
		if o.Value == initial {
			found = true
			break
		}
	}
	if !found && initial != "" {
		initial = "__other__"
	}

	var selected string = initial
	if err := huh.NewSelect[string]().
		Title("Metadata language?").
		Options(options...).
		Value(&selected).
		Run(); err != nil {
		return fmt.Errorf("language selection cancelled")
	}

	if selected == "__other__" {
		var custom string
		if *lang != "" && !isBuiltinLang(*lang) {
			custom = *lang
		}
		if err := huh.NewInput().
			Title("Enter language tag (e.g. fr, de, pt-BR):").
			Value(&custom).
			Run(); err != nil {
			return fmt.Errorf("language input cancelled")
		}
		*lang = strings.TrimSpace(custom)
	} else {
		*lang = selected
	}
	return nil
}

func isBuiltinLang(l string) bool {
	for _, v := range []string{"ja", "en-GB", "en-US", "ko", "zh-CN"} {
		if l == v {
			return true
		}
	}
	return false
}

func askScraper(name *string) error {
	options := []huh.Option[string]{
		huh.NewOption("TVmaze  (no API key required)", "tvmaze"),
		huh.NewOption("TMDb    (API key required)", "tmdb"),
	}
	return huh.NewSelect[string]().
		Title("Scraper?").
		Options(options...).
		Value(name).
		Run()
}

func askTMDbKey(key *string) error {
	return huh.NewInput().
		Title("TMDb API key:").
		EchoMode(huh.EchoModePassword).
		Value(key).
		Run()
}

// --- Shared helpers used by both wizard and flag paths ---

func buildLookup(episodes []Episode) map[int]map[int]*Episode {
	lookup := make(map[int]map[int]*Episode)
	for i := range episodes {
		ep := &episodes[i]
		if lookup[ep.Season] == nil {
			lookup[ep.Season] = make(map[int]*Episode)
		}
		lookup[ep.Season][ep.Number] = ep
	}
	return lookup
}

func lookupEpisodes(lookup map[int]map[int]*Episode, season int, nums []int) []*Episode {
	var eps []*Episode
	for _, n := range nums {
		if ep := lookup[season][n]; ep != nil {
			eps = append(eps, ep)
		}
	}
	return eps
}

// writeShowNFO writes the show-level tvshow.nfo into dir, skipping silently
// if one already exists (unless force) so a richer NFO from another tool
// isn't clobbered.
func writeShowNFO(dir string, show *Show, idType string, force, dryRun bool) {
	path := filepath.Join(dir, "tvshow.nfo")
	if !force && fileExists(path) {
		return
	}
	if dryRun {
		fmt.Println("  ~ would write: tvshow.nfo")
		return
	}
	if err := WriteShowNFO(path, show, idType); err != nil {
		fmt.Printf("  ✗ error: tvshow.nfo: %v\n", err)
		return
	}
	fmt.Println("  ✓ wrote: tvshow.nfo")
}

func writeNFOs(dir string, show *Show, lookup map[int]map[int]*Episode,
	idType string, force, dryRun bool) (written, skipped, notFound, unparsed int) {

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		fe, ok := ParseFilename(filepath.Base(path))
		if !ok {
			unparsed++
			return nil
		}
		if fe.SeasonInferred {
			fmt.Printf("  ~ season inferred as 1: %s\n", filepath.Base(path))
		}
		eps := lookupEpisodes(lookup, fe.Season, fe.Episodes)
		if len(eps) == 0 {
			fmt.Printf("  ✗ not found: S%02dE%02d  %s\n", fe.Season, fe.Episodes[0], filepath.Base(path))
			notFound++
			return nil
		}
		nfoPath := path[:len(path)-len(ext)] + ".nfo"
		if !force && fileExists(nfoPath) {
			skipped++
			return nil
		}
		if dryRun {
			fmt.Printf("  ~ would write: %s\n", filepath.Base(nfoPath))
			written++
			return nil
		}
		if err := WriteNFO(nfoPath, show, eps, idType); err != nil {
			fmt.Printf("  ✗ error: %s: %v\n", filepath.Base(nfoPath), err)
			return nil
		}
		fmt.Printf("  ✓ wrote: %s\n", filepath.Base(nfoPath))
		written++
		return nil
	})
	return
}
