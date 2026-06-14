package main

import (
	"os"
	"path/filepath"
	"regexp"
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

// companion is a co-located subtitle/sidecar file belonging to a video, with
// the optional language infix (".en", ".ja", "") that sits before its
// extension so the tag can be preserved when the file is renamed.
type companion struct {
	path  string
	infix string
}

// langInfix matches a subtitle language tag — e.g. "en", "ja", "jp", "pt-br" —
// that Kodi places between the base name and the extension (Movie.en.srt).
var langInfix = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,4})?$`)

// findCompanions looks in dir for files belonging to the video named stem+ext:
// the plain stem+ext match plus any sidecar carrying a language infix before
// the extension (Kodi convention, e.g. stem.en.srt / stem.ja.ass). Matching is
// case-insensitive and tolerant of NFC/NFD differences, since subtitles often
// differ from their video only in capitalization (Doctor.x vs Doctor.X) or
// normalization form on filesystems like NFS. Returns actual on-disk paths.
func findCompanions(dir, stem, ext string) []companion {
	stemLower := norm.NFC.String(strings.ToLower(stem))
	extLower := strings.ToLower(ext)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []companion
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		nameLower := norm.NFC.String(strings.ToLower(e.Name()))
		if !strings.HasSuffix(nameLower, extLower) {
			continue
		}
		middle := strings.TrimSuffix(nameLower, extLower)
		if middle == stemLower {
			out = append(out, companion{path: filepath.Join(dir, e.Name())})
			continue
		}
		if rest, ok := strings.CutPrefix(middle, stemLower+"."); ok && langInfix.MatchString(rest) {
			out = append(out, companion{path: filepath.Join(dir, e.Name()), infix: "." + rest})
		}
	}
	return out
}
