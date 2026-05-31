package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

type RenameOp struct {
	OldPath string
	NewPath string
}

// targetName builds the canonical filename for an episode (without directory).
// Multi-episode files get combined tokens: S01E01E02E03.
func targetName(show *Show, ep *Episode, fe FileEpisode, ext string) string {
	token := fmt.Sprintf("S%02dE%02d", ep.Season, ep.Number)
	for _, n := range fe.Episodes[1:] {
		token += fmt.Sprintf("E%02d", n)
	}
	return fmt.Sprintf("%s %s %s%s", show.Name, token, sanitizeTitle(ep.Name), ext)
}

func sanitizeTitle(s string) string {
	r := strings.NewReplacer(
		"/", "-",
		":", " -",
		"\\", "-",
		"*", "",
		"?", "",
		`"`, "'",
		"<", "",
		">", "",
		"|", "-",
	)
	return strings.TrimSpace(r.Replace(s))
}

// companionExts are renamed alongside a video file when they share the same base name.
var companionExts = []string{".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx", ".nfo"}

// planRenames walks dir and returns one RenameOp per file whose name differs
// from the canonical target. Subtitle files with matching base names are
// included automatically. Files that already match are counted as skipped.
func planRenames(dir string, show *Show, lookup map[int]map[int]*Episode) (ops []RenameOp, alreadyOK, noData int) {
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
			return nil
		}
		ep := lookup[fe.Season][fe.Episodes[0]]
		if ep == nil {
			noData++
			return nil
		}
		want := targetName(show, ep, fe, ext)
		if filepath.Base(path) == want {
			alreadyOK++
		} else {
			ops = append(ops, RenameOp{
				OldPath: path,
				NewPath: filepath.Join(filepath.Dir(path), want),
			})
		}
		// Check for co-located subtitle files with the same base name.
		base := strings.TrimSuffix(path, ext)
		wantBase := strings.TrimSuffix(want, ext)
		for _, subExt := range companionExts {
			oldSub := base + subExt
			if _, serr := os.Stat(oldSub); serr != nil {
				continue // subtitle file doesn't exist
			}
			newSub := filepath.Join(filepath.Dir(path), wantBase+subExt)
			if oldSub != newSub {
				ops = append(ops, RenameOp{OldPath: oldSub, NewPath: newSub})
			}
		}
		return nil
	})
	return
}

// previewRenames prints up to maxPreview examples followed by a summary line.
func previewRenames(ops []RenameOp, maxPreview int) {
	shown := min(len(ops), maxPreview)
	for _, op := range ops[:shown] {
		fmt.Printf("  %s\n  → %s\n\n", filepath.Base(op.OldPath), filepath.Base(op.NewPath))
	}
	if len(ops) > shown {
		fmt.Printf("  ... and %d more\n\n", len(ops)-shown)
	}
}

// confirmAndRename shows a preview, asks for confirmation, then applies.
// With dryRun it prints all ops and skips both the prompt and the actual renames.
func confirmAndRename(ops []RenameOp, alreadyOK, noData int, dryRun bool) error {
	if len(ops) == 0 {
		fmt.Printf("Nothing to rename — %d already correct, %d with no TVmaze data.\n", alreadyOK, noData)
		return nil
	}

	fmt.Printf("Rename preview (%d of %d shown):\n\n", min(3, len(ops)), len(ops))
	previewRenames(ops, 3)

	if noData > 0 {
		fmt.Printf("  (%d files skipped — no TVmaze match)\n\n", noData)
	}

	if dryRun {
		fmt.Println("-- dry run: full list --")
		previewRenames(ops, len(ops))
		return nil
	}

	var confirmed bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Rename %d files?", len(ops))).
		Affirmative("Yes, rename").
		Negative("Cancel").
		Value(&confirmed).
		Run()
	if err != nil || !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	fmt.Println()
	var failed int
	for _, op := range ops {
		if err := os.Rename(op.OldPath, op.NewPath); err != nil {
			fmt.Printf("  ✗ %s: %v\n", filepath.Base(op.OldPath), err)
			failed++
		} else {
			fmt.Printf("  ✓ %s\n", filepath.Base(op.NewPath))
		}
	}
	fmt.Printf("\n%d renamed", len(ops)-failed)
	if failed > 0 {
		fmt.Printf("  %d failed", failed)
	}
	fmt.Println()
	return nil
}

