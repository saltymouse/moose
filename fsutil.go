package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// namesEqual reports whether two file names are the same modulo Unicode
// normalization (NFC vs NFD). Names cross normalization forms when files
// move between macOS and NFS/Linux filesystems, so byte comparison is not
// enough.
func namesEqual(a, b string) bool {
	return norm.NFC.String(a) == norm.NFC.String(b)
}

// fileExists reports whether path exists, tolerating Unicode normalization
// differences between the constructed name and the on-disk entry.
func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return false
	}
	base := filepath.Base(path)
	for _, e := range entries {
		if namesEqual(e.Name(), base) {
			return true
		}
	}
	return false
}

// findCompanion looks in dir for a file named stem+ext, matching
// case-insensitively and modulo Unicode normalization. Subtitle files often
// differ from their video only in capitalization (Doctor.x... vs Doctor.X...),
// so an exact os.Stat misses them on case-sensitive filesystems like NFS.
// Returns the actual on-disk path when found.
func findCompanion(dir, stem, ext string) (string, bool) {
	want := norm.NFC.String(strings.ToLower(stem + ext))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if norm.NFC.String(strings.ToLower(e.Name())) == want {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}
