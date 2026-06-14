package main

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type FileEpisode struct {
	Season          int
	Episodes        []int // multiple entries for S01E01E02E03 style
	SeasonInferred  bool  // true when season was not explicit in filename
}

// seBlock matches the full SxxExxExx... token, e.g. "S01E03E04E05"
var seBlock = regexp.MustCompile(`(?i)S(\d{1,2})((?:E\d{1,3})+)`)

// singleE extracts individual episode numbers from an E-block
var singleE = regexp.MustCompile(`(?i)E(\d{1,3})`)

// fallback patterns tried in order when seBlock doesn't match.
// Each captures exactly one group: the episode number.
// Season defaults to 1 for all of these.
var fallbacks = []struct {
	re   *regexp.Regexp
	desc string
}{
	// "EP01" or "EP1" — common in Asian web-rips (no season prefix)
	// "Episode 1" / "Episode01" — spelled-out word, number may sit mid-name
	{regexp.MustCompile(`(?i)(?:^|[\s._-])Episode\s*(\d{1,3})(?:\D|$)`), "Episode-word"},
	{regexp.MustCompile(`(?i)(?:^|[\s._-])EP(\d{1,3})(?:[\s._-]|$)`), "EP-prefix"},
	// "E05" without a season prefix, e.g. "Show Name E05"
	{regexp.MustCompile(`(?i)(?:^|[\s._-])E(\d{1,3})(?:[\s._-]|$)`), "E-only"},
	// "- 01"  or  "_01"  optionally followed by bracket tags, e.g. "[720p] [Clean]"
	{regexp.MustCompile(`[-_]\s*(\d{1,3})\s*(?:[\[\(][^\]\)]*[\]\)]\s*)*(?:\.\w+)?$`), "dash/underscore+number"},
	// "[01]" or "(01)"
	{regexp.MustCompile(`[\[\(](\d{1,3})[\]\)]\s*(?:\.\w+)?$`), "bracketed number"},
	// bare number at end: only 1–2 digits to avoid matching years/resolutions
	{regexp.MustCompile(`\s(\d{1,2})\s*(?:\.\w+)?$`), "trailing number"},
}

func ParseFilename(name string) (FileEpisode, bool) {
	// Strip extension for fallback matching so "- 01.mp4" still hits pattern 1.
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	// Primary: standard SxxExx
	if m := seBlock.FindStringSubmatch(stem); m != nil {
		season, _ := strconv.Atoi(m[1])
		epMatches := singleE.FindAllStringSubmatch(m[2], -1)
		eps := make([]int, 0, len(epMatches))
		for _, em := range epMatches {
			n, _ := strconv.Atoi(em[1])
			eps = append(eps, n)
		}
		if len(eps) > 0 {
			return FileEpisode{Season: season, Episodes: eps}, true
		}
	}

	// Fallbacks: assume season 1
	for _, fb := range fallbacks {
		if m := fb.re.FindStringSubmatch(stem); m != nil {
			n, err := strconv.Atoi(m[1])
			if err != nil || n == 0 {
				continue
			}
			return FileEpisode{Season: 1, Episodes: []int{n}, SeasonInferred: true}, true
		}
	}

	return FileEpisode{}, false
}
