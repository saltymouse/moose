package main

import (
	"reflect"
	"testing"
)

// TestParseFilename is a table of real-world filenames moose has encountered.
// Add a row whenever a new naming scheme shows up so regex changes can't
// silently regress a previously-working format.
func TestParseFilename(t *testing.T) {
	cases := []struct {
		name     string
		season   int
		episodes []int
		inferred bool // SeasonInferred — true when season wasn't explicit in the name
	}{
		// --- standard SxxExx ---
		{"Show Name S01E01.mkv", 1, []int{1}, false},
		{"Show Name S02E07.mkv", 2, []int{7}, false},
		{"Show S01E03E04.mkv", 1, []int{3, 4}, false}, // multi-episode file

		// --- S<season>EP<episode> (e.g. 99.9 Keiji Senmon Bengoshi) ---
		{"99.9.Keiji.Senmon.Bengoshi.S1EP01.720p.HEVC.x265-JJ.mkv", 1, []int{1}, false},
		{"99.9.Keiji.Senmon.Bengoshi.S1EP10.END.720p.HEVC.x265-JJ.mkv", 1, []int{10}, false},
		{"99.9.Keiji.Senmon.Bengoshi.S2EP05.720p.mkv", 2, []int{5}, false},
		{"Show.S01EP12.1080p.mkv", 1, []int{12}, false},

		// --- spelled-out "Episode" word, various separators (BUNGO, Boku) ---
		{"BUNGO -Nihon Bungaku Cinema- Episode 1- Lemon (480p Webrip).mp4", 1, []int{1}, true},
		{"BUNGO -Nihon Bungaku Cinema- Episode 5- The Boat (480p Webrip).mp4", 1, []int{5}, true},
		{"Boku.Unmei.no.Hito.desu.Episode.01.720p.HDTV.x265.AAC-DoA.mkv", 1, []int{1}, true},
		{"Boku.Unmei.no.Hito.desu.Episode.10.END.720p.HDTV.x265.AAC-DoA.mkv", 1, []int{10}, true},

		// --- EP / E prefix without a season ---
		{"Show Name EP05.mkv", 1, []int{5}, true},
		{"Show Name E07.mkv", 1, []int{7}, true},
		{"Kingyo.Club.Ep.01.mkv", 1, []int{1}, true},  // separator between EP and number
		{"Kingyo.Club.Ep.10.mkv", 1, []int{10}, true}, // two-digit
		{"Some Show Ep 03.mkv", 1, []int{3}, true},    // space separator
		{"Hatsukoi.E01[STAY GOLD].mp4", 1, []int{1}, true},  // E-prefix followed by a bracket tag
		{"Hatsukoi.E08[ONLY LOVE].mp4", 1, []int{8}, true},
		{"Show.EP05(720p).mkv", 1, []int{5}, true}, // EP-prefix followed by a paren tag

		// --- separator + number near end, optionally followed by (tags) ---
		{"Some Show - 05 [720p].mkv", 1, []int{5}, true},
		{"Bartender 01 (848x480-Dr Dante).mp4", 1, []int{1}, true}, // space + tag
		{"Bartender 02 (848x480).mp4", 1, []int{2}, true},

		// --- bracketed / trailing bare number ---
		{"Show Name [12].mkv", 1, []int{12}, true},
		{"Show Name 09.mkv", 1, []int{9}, true},

		// --- separator-delimited number mid-name (Ao no Jidai) ---
		{"Ao_No_Jidai_01_hotelpapers.mkv", 1, []int{1}, true},
		{"Ao_no_jidai_11_Final.mkv", 1, []int{11}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fe, ok := ParseFilename(c.name)
			if !ok {
				t.Fatalf("expected to parse, got ok=false")
			}
			if fe.Season != c.season {
				t.Errorf("season = %d, want %d", fe.Season, c.season)
			}
			if !reflect.DeepEqual(fe.Episodes, c.episodes) {
				t.Errorf("episodes = %v, want %v", fe.Episodes, c.episodes)
			}
			if fe.SeasonInferred != c.inferred {
				t.Errorf("SeasonInferred = %v, want %v", fe.SeasonInferred, c.inferred)
			}
		})
	}
}

// TestParseFilename_Rejected covers strings that must NOT be read as an episode
// number — years, resolutions, codecs, and names with no episode at all. These
// guard against over-eager fallback patterns (e.g. grabbing "2011" or "720").
func TestParseFilename_Rejected(t *testing.T) {
	for _, name := range []string{
		"Show_Name_2011_Special.mkv",   // year token
		"Show.Name.720p.x265.mkv",      // resolution + codec, no episode
		"Just A Movie.mkv",             // no number at all
		"Documentary (2019).mkv",       // year in parens only
	} {
		t.Run(name, func(t *testing.T) {
			if fe, ok := ParseFilename(name); ok {
				t.Errorf("expected no parse, got season=%d eps=%v", fe.Season, fe.Episodes)
			}
		})
	}
}
