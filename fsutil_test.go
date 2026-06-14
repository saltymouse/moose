package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFindCompanions(t *testing.T) {
	dir := t.TempDir()
	stem := "Kagi no Kakatta Heya.ep06.480p.x264-[D-Addicts]"
	files := []string{
		stem + ".mp4",         // the video itself (not a companion ext here)
		stem + ".srt",         // plain subtitle
		stem + ".jp.srt",      // language-tagged subtitle
		stem + ".x264.srt",    // not a language tag — must be ignored
		"Unrelated.ep06.srt",  // different stem — must be ignored
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := findCompanions(dir, stem, ".srt")
	var names []string
	for _, c := range got {
		names = append(names, filepath.Base(c.path)+"|"+c.infix)
	}
	sort.Strings(names)

	want := []string{
		stem + ".jp.srt|.jp",
		stem + ".srt|",
	}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("got %v, want %v", names, want)
			break
		}
	}
}
