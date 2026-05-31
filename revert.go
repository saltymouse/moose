package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const revertFilename = ".moose-revert.json"

// WriteRevertFile saves an original-rel-path → new-rel-path map so every rename
// can be precisely reversed. Paths are relative to dir.
func WriteRevertFile(dir string, ops []RenameOp) error {
	m := make(map[string]string, len(ops))
	for _, op := range ops {
		origRel, err := filepath.Rel(dir, op.OldPath)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", op.OldPath, err)
		}
		newRel, err := filepath.Rel(dir, op.NewPath)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", op.NewPath, err)
		}
		m[origRel] = newRel
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, revertFilename), data, 0644)
}

// RevertRenames reads the revert file and renames every file back to its
// original path, then deletes the revert file.
func RevertRenames(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, revertFilename))
	if err != nil {
		return fmt.Errorf("could not read revert file: %w", err)
	}
	var m map[string]string // origRelPath → newRelPath
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("malformed revert file: %w", err)
	}

	var reverted, failed int
	for origRel, newRel := range m {
		current := filepath.Join(dir, newRel)
		target := filepath.Join(dir, origRel)
		if current == target {
			reverted++
			continue
		}
		if _, err := os.Stat(current); os.IsNotExist(err) {
			fmt.Printf("  ✗ not found: %s\n", newRel)
			failed++
			continue
		}
		if err := os.Rename(current, target); err != nil {
			fmt.Printf("  ✗ revert failed %s: %v\n", origRel, err)
			failed++
		} else {
			fmt.Printf("  ✓ restored: %s\n", origRel)
			reverted++
		}
	}

	DeleteRevertFile(dir)
	fmt.Printf("\n%d restored", reverted)
	if failed > 0 {
		fmt.Printf("  %d failed", failed)
	}
	fmt.Println()
	return nil
}

// DeleteRevertFile removes the revert file from the show directory.
func DeleteRevertFile(dir string) {
	os.Remove(filepath.Join(dir, revertFilename))
}
