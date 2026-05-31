package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const tvmazeBase = "https://api.tvmaze.com"

// Network is a named type so both scrapers can construct it easily.
type Network struct {
	Name string `json:"name"`
}

type Show struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Premiered        string   `json:"premiered"`
	Summary          string   `json:"summary"`
	OriginalLanguage string   // ISO 639-1 code; populated by TMDb only
	Network          *Network `json:"network"`
	WebChannel       *Network `json:"webChannel"`
}

func (s *Show) NetworkName() string {
	if s.Network != nil {
		return s.Network.Name
	}
	if s.WebChannel != nil {
		return s.WebChannel.Name
	}
	return ""
}

type SearchResult struct {
	Score float64 `json:"score"`
	Show  Show    `json:"show"`
}

type Episode struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Number  int    `json:"number"`
	Airdate string `json:"airdate"`
	Runtime int    `json:"runtime"`
	Summary string `json:"summary"`
	Image   *struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"image"`
}

// TVmazeScraper implements Scraper against api.tvmaze.com.
// TVmaze doesn't support per-language queries — it returns data in the
// show's original language regardless of the lang setting, so lang is
// accepted but unused.
type TVmazeScraper struct {
	Lang string // stored for symmetry; TVmaze ignores it
}

func (t *TVmazeScraper) IDType() string { return "tvmaze" }

func (t *TVmazeScraper) SearchShows(query string) ([]SearchResult, error) {
	var results []SearchResult
	return results, get(tvmazeBase+"/search/shows?q="+url.QueryEscape(query), &results)
}

func (t *TVmazeScraper) FetchShow(id int) (*Show, error) {
	var show Show
	return &show, get(fmt.Sprintf("%s/shows/%d", tvmazeBase, id), &show)
}

func (t *TVmazeScraper) FetchEpisodes(showID int) ([]Episode, error) {
	var eps []Episode
	return eps, get(fmt.Sprintf("%s/shows/%d/episodes?specials=1", tvmazeBase, showID), &eps)
}

// --- shared HTTP helper ---

func get(endpoint string, v any) error {
	resp, err := http.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// --- HTML stripping (TVmaze summaries are HTML; TMDb are plain text) ---

var htmlTag = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTag.ReplaceAllString(s, "")
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return strings.TrimSpace(r.Replace(s))
}
