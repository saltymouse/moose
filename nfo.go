package main

import (
	"encoding/xml"
	"fmt"
	"os"
)

type EpisodeNFO struct {
	XMLName   xml.Name `xml:"episodedetails"`
	Title     string   `xml:"title"`
	ShowTitle string   `xml:"showtitle"`
	UniqueID  UniqueID `xml:"uniqueid"`
	Season    int      `xml:"season"`
	Episode   int      `xml:"episode"`
	Plot      string   `xml:"plot"`
	Aired     string   `xml:"aired"`
	Runtime   int      `xml:"runtime,omitempty"`
	Studio    string   `xml:"studio,omitempty"`
	Tags      []string `xml:"tag,omitempty"`
}

type UniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

// TVShowNFO is the show-level tvshow.nfo Kodi/MediaElch expect in the show root.
type TVShowNFO struct {
	XMLName   xml.Name `xml:"tvshow"`
	Title     string   `xml:"title"`
	UniqueID  UniqueID `xml:"uniqueid"`
	Plot      string   `xml:"plot"`
	Premiered string   `xml:"premiered"`
	Status    string   `xml:"status,omitempty"`
	Studio    string   `xml:"studio,omitempty"`
	Genres    []string `xml:"genre,omitempty"`
	Tags      []string `xml:"tag,omitempty"`
}

// WriteNFO writes one <episodedetails> block per episode into a single NFO file.
// Multi-episode files produce multiple concatenated blocks (the Kodi standard).
func WriteNFO(path string, show *Show, eps []*Episode, idType string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	enc := xml.NewEncoder(f)
	enc.Indent("", "    ")
	for _, ep := range eps {
		nfo := EpisodeNFO{
			Title:     ep.Name,
			ShowTitle: show.Name,
			UniqueID:  UniqueID{Type: idType, Default: "true", Value: fmt.Sprintf("%d", ep.ID)},
			Season:    ep.Season,
			Episode:   ep.Number,
			Plot:      stripHTML(ep.Summary),
			Aired:     ep.Airdate,
			Runtime:   ep.Runtime,
			Studio:    show.NetworkName(),
		}
		if show.OriginalLanguage != "" {
			nfo.Tags = []string{LanguageTag(show.OriginalLanguage)}
		}
		if err := enc.Encode(nfo); err != nil {
			return err
		}
	}
	return enc.Close()
}

func WriteShowNFO(path string, show *Show, idType string) error {
	nfo := TVShowNFO{
		Title:     show.Name,
		UniqueID:  UniqueID{Type: idType, Default: "true", Value: fmt.Sprintf("%d", show.ID)},
		Plot:      stripHTML(show.Summary),
		Premiered: show.Premiered,
		Status:    show.Status,
		Studio:    show.NetworkName(),
		Genres:    show.Genres,
	}
	if show.OriginalLanguage != "" {
		nfo.Tags = []string{LanguageTag(show.OriginalLanguage)}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	enc := xml.NewEncoder(f)
	enc.Indent("", "    ")
	if err := enc.Encode(nfo); err != nil {
		return err
	}
	return enc.Close()
}
