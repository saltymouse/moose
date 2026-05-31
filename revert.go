package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const revertFilename = ".moose-revert.json"

// WriteRevertFile saves a size→original-basename map to <dir>/.moose-revert.json
// before any renames are applied, so files can be found by size even after renaming.
func WriteRevertFile(dir string, ops []RenameOp) error {
	m := make(map[string]string, len(ops))
	for _, op := range ops {
		info, err := os.Stat(op.OldPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", op.OldPath, err)
		}
		key := strconv.FormatInt(info.Size(), 10)
		m[key] = filepath.Base(op.OldPath)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, revertFilename), data, 0644)
}

// RevertRenames reads the revert file, locates each original file by its size,
// renames it back, then deletes the revert file.
func RevertRenames(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, revertFilename))
	if err != nil {
		return fmt.Errorf("could not read revert file: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("malformed revert file: %w", err)
	}

	// Build a size→current-path map from the directory.
	sizeToCurrent := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExts[ext] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		key := strconv.FormatInt(info.Size(), 10)
		sizeToCurrent[key] = filepath.Join(dir, e.Name())
	}

	var reverted, failed int
	for sizeKey, origName := range m {
		currentPath, ok := sizeToCurrent[sizeKey]
		if !ok {
			fmt.Printf("  ✗ could not find file with size %s (was: %s)\n", sizeKey, origName)
			failed++
			continue
		}
		target := filepath.Join(dir, origName)
		if currentPath == target {
			reverted++ // already has the original name
			continue
		}
		if err := os.Rename(currentPath, target); err != nil {
			fmt.Printf("  ✗ revert failed %s: %v\n", origName, err)
			failed++
		} else {
			fmt.Printf("  ✓ restored: %s\n", origName)
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
