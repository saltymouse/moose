package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
)

// dirPattern matches "Show Name (2019)" or "Show Name (2019) extra"
var dirPattern = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)`)

// GuessShow infers a TVmaze show from the directory name.
// It expects the format "Show Name (YYYY)" and uses the year to
// disambiguate when multiple results come back.
func GuessShow(dir string, s Scraper) (*Show, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = filepath.Clean(dir)
	}
	base := filepath.Base(abs)
	m := dirPattern.FindStringSubmatch(base)

	var name, year string
	if m != nil {
		name = strings.TrimSpace(m[1])
		year = m[2]
		fmt.Printf("Guessing from directory: %q\n  → name: %q  year: %s\n\n", base, name, year)
	} else {
		name = base
		fmt.Printf("Guessing from directory: %q\n  → name: %q  (no year found)\n\n", base, name)
	}

	results, err := s.SearchShows(name)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf(
			"no results for %q\n\n"+
				"Moose guessed the show name from the directory.\n"+
				"Run from inside a show folder, or be explicit:\n\n"+
				"  moose --show \"<show name>\"\n"+
				"  moose --id <scraper-id>\n"+
				"  moose --dir \"/path/to/show\"",
			name,
		)
	}

	// Single result — use it without asking.
	if len(results) == 1 {
		show := results[0].Show
		fmt.Printf("Found: %s  (ID %d)\n\n", show.Name, show.ID)
		return &show, nil
	}

	// Year present — try an exact year match before prompting.
	if year != "" {
		for _, r := range results {
			if strings.HasPrefix(r.Show.Premiered, year) {
				show := r.Show
				fmt.Printf("Matched: %s  (ID %d, premiered %s)\n\n",
					show.Name, show.ID, show.Premiered)
				return &show, nil
			}
		}
		fmt.Printf("No results premiered in %s — showing all matches.\n\n", year)
	}

	// Multiple results and no automatic match — let the user pick.
	return pickShow(results)
}

// pickShow presents an interactive huh selector for a slice of search results.
func pickShow(results []SearchResult) (*Show, error) {
	opts := make([]huh.Option[int], 0, len(results))
	for _, r := range results {
		year := r.Show.Premiered
		if len(year) >= 4 {
			year = year[:4]
		}
		label := fmt.Sprintf("%-50s [%s]  ID %d", r.Show.Name, year, r.Show.ID)
		opts = append(opts, huh.NewOption(label, r.Show.ID))
	}

	var selectedID int
	err := huh.NewSelect[int]().
		Title("Multiple results — pick one:").
		Options(opts...).
		Value(&selectedID).
		Run()
	if err != nil {
		return nil, fmt.Errorf("selection cancelled")
	}

	for _, r := range results {
		if r.Show.ID == selectedID {
			show := r.Show
			fmt.Printf("\nUsing: %s  (ID %d)\n\n", show.Name, show.ID)
			return &show, nil
		}
	}
	return nil, fmt.Errorf("no show selected")
}
