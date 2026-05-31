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
		path      string
		fe        FileEpisode
		parsed    bool
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
	fmt.Println()

	// --- Rename phase ---
	ops, alreadyOK, noData := planRenames(f.Dir, show, lookup)

	if len(ops) == 0 && alreadyOK > 0 {
		fmt.Printf("  All %d files already correctly named.\n\n", alreadyOK)
	} else if len(ops) > 0 {
		if noData > 0 {
			fmt.Printf("  (%d files skipped — no scraper match)\n\n", noData)
		}

		if f.DryRun {
			fmt.Printf("  Would rename %d files:\n\n", len(ops))
			previewRenames(ops, len(ops))
			return nil
		}

		if err := WriteRevertFile(f.Dir, ops); err != nil {
			return fmt.Errorf("could not write revert file: %w", err)
		}

		var renameCount int
		for _, op := range ops {
			if err := os.Rename(op.OldPath, op.NewPath); err != nil {
				fmt.Printf("  ✗ %s\n    failed: %v\n", filepath.Base(op.OldPath), err)
			} else {
				fmt.Printf("  %s\n  → %s\n\n", filepath.Base(op.OldPath), filepath.Base(op.NewPath))
				renameCount++
			}
		}
		fmt.Printf("  Renamed %d files.\n\n", renameCount)
	}

	// --- NFO phase ---
	nfoCount, nfoSkipped, nfoMissing, _ := writeNFOs(f.Dir, show, lookup, s.IDType(), f.Force, false)

	// --- Final confirmation ---
	fmt.Printf("\n── Summary ─────────────────────────────────────────\n")
	fmt.Printf("  NFOs written:  %d\n", nfoCount)
	if nfoSkipped > 0 {
		fmt.Printf("  Already exist: %d  (use --force to overwrite)\n", nfoSkipped)
	}
	if nfoMissing > 0 {
		fmt.Printf("  Not matched:   %d\n", nfoMissing)
	}
	if unparsedCount > 0 {
		fmt.Printf("  Unparsed:      %d\n", unparsedCount)
	}
	fmt.Println()

	if nfoCount == 0 && nfoSkipped > 0 {
		fmt.Println("  All NFOs already up to date. Run with --force to refresh.")
		DeleteRevertFile(f.Dir)
		return nil
	}

	var confirmed bool
	err = huh.NewConfirm().
		Title("Everything look good?").
		Affirmative("Yes — keep changes").
		Negative("No — revert renames").
		Value(&confirmed).
		Run()
	if err != nil || !confirmed {
		fmt.Println("\nReverting renames...")
		return RevertRenames(f.Dir)
	}

	DeleteRevertFile(f.Dir)
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
		ep := lookup[fe.Season][fe.Episodes[0]]
		if ep == nil {
			fmt.Printf("  ✗ not found: S%02dE%02d  %s\n", fe.Season, fe.Episodes[0], filepath.Base(path))
			notFound++
			return nil
		}
		nfoPath := path[:len(path)-len(ext)] + ".nfo"
		if !force {
			if _, statErr := os.Stat(nfoPath); statErr == nil {
				skipped++
				return nil
			}
		}
		if dryRun {
			fmt.Printf("  ~ would write: %s\n", filepath.Base(nfoPath))
			written++
			return nil
		}
		if err := WriteNFO(nfoPath, show, ep, idType); err != nil {
			fmt.Printf("  ✗ error: %s: %v\n", filepath.Base(nfoPath), err)
			return nil
		}
		fmt.Printf("  ✓ wrote: %s\n", filepath.Base(nfoPath))
		written++
		return nil
	})
	return
}
