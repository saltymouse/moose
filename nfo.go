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
}

type UniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

func WriteNFO(path string, show *Show, ep *Episode, idType string) error {
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
