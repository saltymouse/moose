package main

import "testing"

// stubScraper serves canned search results and per-ID episode counts.
type stubScraper struct {
	results []SearchResult
	counts  map[int]int // show ID -> EpisodeCount returned by FetchShow
	fetched []int       // IDs FetchShow was called with, in order
}

func (s *stubScraper) IDType() string { return "stub" }
func (s *stubScraper) SearchShows(string) ([]SearchResult, error) {
	return s.results, nil
}
func (s *stubScraper) FetchShow(id int) (*Show, error) {
	s.fetched = append(s.fetched, id)
	return &Show{ID: id, Premiered: "2013-03-29", EpisodeCount: s.counts[id]}, nil
}
func (s *stubScraper) FetchEpisodes(int) ([]Episode, error) { return nil, nil }

func TestMostPopulatedBreaksTie(t *testing.T) {
	matches := []SearchResult{
		{Show: Show{ID: 322677, Premiered: "2013-03-29"}}, // stub entry, ranked first
		{Show: Show{ID: 64793, Premiered: "2013-03-29"}},  // the populated entry
	}
	s := &stubScraper{counts: map[int]int{322677: 1, 64793: 32}}

	best := mostPopulated(matches, s)
	if best == nil {
		t.Fatal("expected a winner, got nil")
	}
	if best.ID != 64793 {
		t.Errorf("picked ID %d, want 64793 (the entry with more episodes)", best.ID)
	}
}

func TestMostPopulatedTieReturnsNil(t *testing.T) {
	matches := []SearchResult{
		{Show: Show{ID: 1}},
		{Show: Show{ID: 2}},
	}
	// Equal non-zero counts: can't decide, must fall back to prompting.
	if best := mostPopulated(matches, &stubScraper{counts: map[int]int{1: 16, 2: 16}}); best != nil {
		t.Errorf("expected nil on equal counts, got ID %d", best.ID)
	}
	// No episode data at all: also nil.
	if best := mostPopulated(matches, &stubScraper{counts: map[int]int{1: 0, 2: 0}}); best != nil {
		t.Errorf("expected nil when no candidate has episodes, got ID %d", best.ID)
	}
}
