package main

// Scraper is the common interface both TVmaze and TMDb implement.
// IDType is written into the NFO <uniqueid type="..."> attribute.
type Scraper interface {
	IDType() string
	SearchShows(query string) ([]SearchResult, error)
	FetchShow(id int) (*Show, error)
	FetchEpisodes(showID int) ([]Episode, error)
}
