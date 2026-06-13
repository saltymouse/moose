package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const downloadConcurrency = 4

// downloadFile fetches url and writes it to destPath.
// Returns (true, nil) when downloaded, (false, nil) when skipped (already exists).
func downloadFile(url, destPath string, force bool) (downloaded bool, err error) {
	if !force && fileExists(destPath) {
		return false, nil // already exists
	}
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err == nil, err
}

// DownloadShowImages downloads poster.jpg and fanart.jpg into dir.
func DownloadShowImages(dir string, show *Show, force bool) {
	type img struct {
		url  string
		name string
	}
	images := []img{}
	if show.PosterURL != "" {
		images = append(images, img{show.PosterURL, "poster.jpg"})
	}
	if show.FanartURL != "" {
		images = append(images, img{show.FanartURL, "fanart.jpg"})
	}
	for _, im := range images {
		dest := filepath.Join(dir, im.name)
		got, err := downloadFile(im.url, dest, force)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", im.name, err)
		} else if got {
			fmt.Printf("  ✓ %s\n", im.name)
		}
	}
}

// DownloadEpisodeThumbs walks dir, finds video files, and downloads a -thumb.jpg
// alongside each one using the episode's image URL. Runs concurrently.
func DownloadEpisodeThumbs(dir string, show *Show, lookup map[int]map[int]*Episode, force bool) (downloaded, skipped, failed, noStill int) {
	type work struct {
		url      string
		destPath string
	}

	var jobs []work
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		fe, ok := ParseFilename(filepath.Base(path))
		if !ok {
			return nil
		}
		ep := lookup[fe.Season][fe.Episodes[0]]
		if ep == nil || ep.Image == nil || ep.Image.Original == "" {
			noStill++
			return nil
		}
		base := strings.TrimSuffix(path, ext)
		jobs = append(jobs, work{
			url:      ep.Image.Original,
			destPath: base + "-thumb.jpg",
		})
		return nil
	})

	sem := make(chan struct{}, downloadConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j work) {
			defer wg.Done()
			defer func() { <-sem }()

			got, err := downloadFile(j.url, j.destPath, force)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", filepath.Base(j.destPath), err)
				failed++
			} else if got {
				fmt.Printf("  ✓ %s\n", filepath.Base(j.destPath))
				downloaded++
			} else {
				skipped++
			}
		}(j)
	}
	wg.Wait()
	return
}
