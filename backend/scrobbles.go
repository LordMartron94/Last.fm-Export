package backend

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	baseURL               = "http://ws.audioscrobbler.com/2.0/"
	defaultTimeout        = 15 * time.Second
	maxConcurrentRequests = 20
	retryCount            = 3
	retryBackoff          = 500 * time.Millisecond
)

// Scrobble represents a single played track.
type Scrobble struct {
	Timestamp time.Time
	Track     string
	Artist    string
	Album     string
	URL       string
}

// recentTracksResponse models the JSON returned by Last.fm
// Only includes fields we care about.
type recentTracksResponse struct {
	Recenttracks struct {
		Attr struct {
			TotalPages string `json:"totalPages"`
		} `json:"@attr"`
		Track []struct {
			Artist struct {
				Text string `json:"#text"`
			} `json:"artist"`
			Album struct {
				Text string `json:"#text"`
			} `json:"album"`
			Date struct {
				Uts string `json:"uts"`
			} `json:"date"`
			URL  string
			Name string
		} `json:"track"`
	} `json:"recenttracks"`
}

// GetScrobblesCSV fetches all scrobbles, sorts by ascending timestamp,
// and writes them to a tab-delimited file. Logs progress as it runs.
func GetScrobblesCSV(username, apiKey, filename string) error {
	client := newHTTPClient()

	// 1. Determine total pages
	totalPages, err := getTotalPages(client, username, apiKey)
	if err != nil {
		return fmt.Errorf("could not determine total pages: %w", err)
	}
	log.Printf("Total pages to fetch: %d", totalPages)

	// 2. Fetch all scrobbles in parallel into slice
	scrobbles, err := fetchAllScrobbles(client, username, apiKey, totalPages)
	if err != nil {
		return err
	}

	// 3. Sort by timestamp ascending
	sort.Slice(scrobbles, func(i, j int) bool {
		return scrobbles[i].Timestamp.Before(scrobbles[j].Timestamp)
	})
	log.Printf("Total records fetched: %d", len(scrobbles))

	// 4. Write to tab-delimited file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	w.Comma = '\t'
	// header
	w.Write([]string{"timestamp", "track", "artist", "album", "url"})

	for _, s := range scrobbles {
		w.Write([]string{s.Timestamp.UTC().Format(time.RFC3339), s.Track, s.Artist, s.Album, s.URL})
	}
	w.Flush()

	log.Printf("Scrobbles written to %s", filename)
	return nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: maxConcurrentRequests,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

func getTotalPages(client *http.Client, user, key string) (int, error) {
	resp := new(recentTracksResponse)
	url := buildURL(user, key, 1)
	if err := doGetJSON(url, client, resp); err != nil {
		return 0, err
	}
	pages, err := strconv.Atoi(resp.Recenttracks.Attr.TotalPages)
	if err != nil {
		return 0, err
	}
	return pages, nil
}

func fetchAllScrobbles(
	client *http.Client,
	user, key string,
	totalPages int,
) ([]Scrobble, error) {
	ctx := context.Background()
	sem := semaphore.NewWeighted(maxConcurrentRequests)
	var eg errgroup.Group
	var mu sync.Mutex
	sc := make([]Scrobble, 0, totalPages*50)

	for p := 1; p <= totalPages; p++ {
		p := p
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		eg.Go(func() error {
			defer sem.Release(1)

			log.Printf("Fetching page %d/%d", p, totalPages)
			resp := new(recentTracksResponse)
			if err := doGetJSON(buildURL(user, key, p), client, resp); err != nil {
				return fmt.Errorf("page %d: %w", p, err)
			}

			// parse into Scrobble
			var local []Scrobble
			for _, t := range resp.Recenttracks.Track {
				tsInt, _ := strconv.ParseInt(t.Date.Uts, 10, 64)
				local = append(local, Scrobble{
					Timestamp: time.Unix(tsInt, 0).UTC(),
					Track:     t.Name,
					Artist:    t.Artist.Text,
					Album:     t.Album.Text,
					URL:       t.URL,
				})
			}
			log.Printf("Page %d: parsed %d records", p, len(local))

			mu.Lock()
			sc = append(sc, local...)
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return sc, nil
}

// doGetJSON wraps HTTP GET + JSON decode with retries/backoff
func doGetJSON(url string, client *http.Client, target interface{}) error {
	var lastErr error
	for i := 1; i <= retryCount; i++ {
		r, err := client.Get(url)
		if err == nil && r.StatusCode == http.StatusOK {
			defer r.Body.Close()
			return json.NewDecoder(r.Body).Decode(target)
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status %d", r.StatusCode)
		}
		time.Sleep(retryBackoff)
	}
	return fmt.Errorf("GET %s failed after %d attempts: %w", url, retryCount, lastErr)
}

func buildURL(user, key string, page int) string {
	return fmt.Sprintf("%s?method=user.getrecenttracks&user=%s&api_key=%s&format=json&page=%d", baseURL, user, key, page)
}
