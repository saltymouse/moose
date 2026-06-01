package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const revertFilename = ".moose-revert.json"

type revertData struct {
	Renames map[string]string `json:"renames"` // origRelPath → newRelPath
	Created []string          `json:"created"` // relPaths of newly created files to delete on revert
}

// WriteRevertFile saves rename ops before any changes are applied.
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
	return writeRevertData(dir, revertData{Renames: m})
}

// AppendCreated adds newly created file paths to the revert file so they
// can be deleted if the user chooses to revert.
func AppendCreated(dir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	rd, err := readRevertData(dir)
	if err != nil {
		return err
	}
	for _, p := range paths {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			continue
		}
		rd.Created = append(rd.Created, rel)
	}
	return writeRevertData(dir, rd)
}

// RevertRenames undoes all renames and deletes any newly created files,
// then removes the revert file.
func RevertRenames(dir string) error {
	rd, err := readRevertData(dir)
	if err != nil {
		return fmt.Errorf("could not read revert file: %w", err)
	}

	var reverted, failed int
	for origRel, newRel := range rd.Renames {
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

	var deleted int
	for _, rel := range rd.Created {
		path := filepath.Join(dir, rel)
		if err := os.Remove(path); err == nil {
			fmt.Printf("  ✓ deleted:  %s\n", rel)
			deleted++
		}
	}

	DeleteRevertFile(dir)
	fmt.Printf("\n%d restored", reverted)
	if deleted > 0 {
		fmt.Printf("  %d deleted", deleted)
	}
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

func readRevertData(dir string) (revertData, error) {
	data, err := os.ReadFile(filepath.Join(dir, revertFilename))
	if err != nil {
		return revertData{}, err
	}
	// Support old format (plain map) by trying new format first.
	var rd revertData
	if err := json.Unmarshal(data, &rd); err != nil {
		return revertData{}, fmt.Errorf("malformed revert file: %w", err)
	}
	// If Renames is nil, the file is in the old flat-map format.
	if rd.Renames == nil {
		var old map[string]string
		if err := json.Unmarshal(data, &old); err != nil {
			return revertData{}, fmt.Errorf("malformed revert file: %w", err)
		}
		rd.Renames = old
	}
	return rd, nil
}

func writeRevertData(dir string, rd revertData) error {
	data, err := json.MarshalIndent(rd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, revertFilename), data, 0644)
}
