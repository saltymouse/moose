package main

import (
	"fmt"
	"net/url"
	"sync"
)

const tmdbBase      = "https://api.themoviedb.org/3"
const tmdbImageBase = "https://image.tmdb.org/t/p/original"

func tmdbImageURL(path string) string {
	if path == "" {
		return ""
	}
	return tmdbImageBase + path
}

// TMDbScraper implements Scraper against api.themoviedb.org.
// Lang should be a BCP-47 language tag, e.g. "ja", "en-GB", "en-US".
type TMDbScraper struct {
	Key  string
	Lang string
}

func (t *TMDbScraper) IDType() string { return "tmdb" }

func (t *TMDbScraper) endpoint(path string, extra ...string) string {
	u := fmt.Sprintf("%s%s?api_key=%s&language=%s",
		tmdbBase, path, t.Key, url.QueryEscape(t.Lang))
	for i := 0; i+1 < len(extra); i += 2 {
		u += "&" + extra[i] + "=" + url.QueryEscape(extra[i+1])
	}
	return u
}

// --- TMDb response types (internal, not exported) ---

type tmdbSearchResp struct {
	Results []struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		FirstAirDate     string `json:"first_air_date"`
		Overview         string `json:"overview"`
		OriginalLanguage string `json:"original_language"`
		PosterPath       string `json:"poster_path"`
		BackdropPath     string `json:"backdrop_path"`
	} `json:"results"`
}

type tmdbShowResp struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	FirstAirDate     string `json:"first_air_date"`
	Overview         string `json:"overview"`
	OriginalLanguage string `json:"original_language"`
	PosterPath       string `json:"poster_path"`
	BackdropPath     string `json:"backdrop_path"`
	Status           string `json:"status"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Networks []struct {
		Name string `json:"name"`
	} `json:"networks"`
	Seasons []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
}

type tmdbSeasonResp struct {
	Episodes []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
		AirDate       string `json:"air_date"`
		Runtime       int    `json:"runtime"`
		Overview      string `json:"overview"`
		StillPath     string `json:"still_path"`
	} `json:"episodes"`
}

// --- Scraper implementation ---

func (t *TMDbScraper) SearchShows(query string) ([]SearchResult, error) {
	var resp tmdbSearchResp
	if err := get(t.endpoint("/search/tv", "query", query), &resp); err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			Score: float64(len(resp.Results) - i),
			Show: Show{
				ID:               r.ID,
				Name:             r.Name,
				Premiered:        r.FirstAirDate,
				Summary:          r.Overview,
				OriginalLanguage: r.OriginalLanguage,
				PosterURL:        tmdbImageURL(r.PosterPath),
				FanartURL:        tmdbImageURL(r.BackdropPath),
			},
		}
	}
	return results, nil
}

func (t *TMDbScraper) FetchShow(id int) (*Show, error) {
	var resp tmdbShowResp
	if err := get(t.endpoint(fmt.Sprintf("/tv/%d", id)), &resp); err != nil {
		return nil, err
	}
	show := &Show{
		ID:               resp.ID,
		Name:             resp.Name,
		Premiered:        resp.FirstAirDate,
		Summary:          resp.Overview,
		Status:           resp.Status,
		OriginalLanguage: resp.OriginalLanguage,
		PosterURL:        tmdbImageURL(resp.PosterPath),
		FanartURL:        tmdbImageURL(resp.BackdropPath),
	}
	for _, g := range resp.Genres {
		show.Genres = append(show.Genres, g.Name)
	}
	if len(resp.Networks) > 0 {
		show.Network = &Network{Name: resp.Networks[0].Name}
	}
	return show, nil
}

// FetchEpisodes fetches all seasons concurrently then flattens into a single slice.
func (t *TMDbScraper) FetchEpisodes(showID int) ([]Episode, error) {
	var showResp tmdbShowResp
	if err := get(t.endpoint(fmt.Sprintf("/tv/%d", showID)), &showResp); err != nil {
		return nil, err
	}

	type result struct {
		eps []Episode
		err error
	}
	results := make([]result, len(showResp.Seasons))
	var wg sync.WaitGroup

	for i, s := range showResp.Seasons {
		wg.Add(1)
		go func(idx, seasonNum int) {
			defer wg.Done()
			var season tmdbSeasonResp
			err := get(t.endpoint(fmt.Sprintf("/tv/%d/season/%d", showID, seasonNum)), &season)
			if err != nil {
				results[idx] = result{err: err}
				return
			}
			eps := make([]Episode, 0, len(season.Episodes))
			for _, e := range season.Episodes {
				ep := Episode{
					ID:      e.ID,
					Name:    e.Name,
					Season:  e.SeasonNumber,
					Number:  e.EpisodeNumber,
					Airdate: e.AirDate,
					Runtime: e.Runtime,
					Summary: e.Overview,
				}
				if e.StillPath != "" {
					ep.Image = &struct {
						Medium   string `json:"medium"`
						Original string `json:"original"`
					}{Original: tmdbImageBase + e.StillPath}
				}
				eps = append(eps, ep)
			}
			results[idx] = result{eps: eps}
		}(i, s.SeasonNumber)
	}
	wg.Wait()

	var all []Episode
	for _, r := range results {
		if r.err != nil {
			continue // skip failed seasons rather than aborting everything
		}
		all = append(all, r.eps...)
	}
	return all, nil
}
