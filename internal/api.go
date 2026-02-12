package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchTraktShow fetches show data from Trakt API
func FetchTraktShow(client *http.Client, config Config, showID int) (*TraktShow, error) {
	cacheFile := filepath.Join(config.TempDir, "shows", fmt.Sprintf("%d.json", showID))
	if data, err := os.ReadFile(cacheFile); err == nil && !config.Force {
		var show TraktShow
		if json.Unmarshal(data, &show) == nil {
			if config.Verbose {
				fmt.Printf("\n    - using cached Trakt show data")
			}
			return &show, nil
		}
	}

	if config.Verbose {
		fmt.Printf("\n    - fetching show %d from Trakt API", showID)
	}

	config.RateLimiter.Wait()
	time.Sleep(500 * time.Millisecond)

	retryConfig := DefaultRetryConfig()
	resp, err := RetryWithBackoff(retryConfig, func() (*http.Response, error) {
		url := fmt.Sprintf("https://api.trakt.tv/shows/%d", showID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("trakt-api-version", "2")
		req.Header.Set("trakt-api-key", config.APIKey)

		return client.Do(req)
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("\n    - show not found: 404")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\n    - API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var show TraktShow
	if err := json.Unmarshal(body, &show); err != nil {
		return nil, err
	}

	os.WriteFile(cacheFile, body, 0644)
	return &show, nil
}

// FetchTraktMovie fetches movie data from Trakt API
func FetchTraktMovie(client *http.Client, config Config, movieID int) (*TraktMovie, error) {
	cacheFile := filepath.Join(config.TempDir, "movies", fmt.Sprintf("%d.json", movieID))
	if data, err := os.ReadFile(cacheFile); err == nil && !config.Force {
		var movie TraktMovie
		if json.Unmarshal(data, &movie) == nil {
			if config.Verbose {
				fmt.Printf("\n    - using cached Trakt movie data")
			}
			return &movie, nil
		}
	}

	if config.Verbose {
		fmt.Printf("\n    - fetching movie %d from Trakt API", movieID)
	}

	config.RateLimiter.Wait()
	time.Sleep(500 * time.Millisecond)

	retryConfig := DefaultRetryConfig()
	resp, err := RetryWithBackoff(retryConfig, func() (*http.Response, error) {
		url := fmt.Sprintf("https://api.trakt.tv/movies/%d", movieID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("trakt-api-version", "2")
		req.Header.Set("trakt-api-key", config.APIKey)

		return client.Do(req)
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("\n    - movie not found: 404")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\n    - API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var movie TraktMovie
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, err
	}

	os.WriteFile(cacheFile, body, 0644)
	return &movie, nil
}

// FetchTraktSeason fetches season data from Trakt API
func FetchTraktSeason(client *http.Client, config Config, showID, seasonNum int) (*TraktSeason, error) {
	cacheFile := filepath.Join(config.TempDir, "seasons", fmt.Sprintf("%d.json", showID))
	if data, err := os.ReadFile(cacheFile); err == nil && !config.Force {
		var seasons []TraktSeason
		if json.Unmarshal(data, &seasons) == nil {
			for _, season := range seasons {
				if season.Number == seasonNum {
					if config.Verbose {
						fmt.Printf("\n        - using cached Trakt season data")
					}
					return &season, nil
				}
			}
		}
	}

	if config.Verbose {
		fmt.Printf("\n        - fetching seasons for show %d from Trakt API", showID)
	}

	config.RateLimiter.Wait()
	time.Sleep(500 * time.Millisecond)

	retryConfig := DefaultRetryConfig()
	resp, err := RetryWithBackoff(retryConfig, func() (*http.Response, error) {
		url := fmt.Sprintf("https://api.trakt.tv/shows/%d/seasons", showID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("trakt-api-version", "2")
		req.Header.Set("trakt-api-key", config.APIKey)

		return client.Do(req)
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("\n        - seasons not found: 404")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\n        - API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var seasons []TraktSeason
	if err := json.Unmarshal(body, &seasons); err != nil {
		return nil, err
	}

	os.WriteFile(cacheFile, body, 0644)

	for _, season := range seasons {
		if season.Number == seasonNum {
			return &season, nil
		}
	}

	return nil, fmt.Errorf("\n        - season %d not found", seasonNum)
}

// FetchLetterboxdInfo fetches Letterboxd info from the Letterboxd API
func FetchLetterboxdInfo(client *http.Client, config Config, tmdbID int) (*Letterboxd, error) {
	cacheFile := filepath.Join(config.TempDir, "letterboxd", fmt.Sprintf("%d.json", tmdbID))
	if data, err := os.ReadFile(cacheFile); err == nil && !config.Force {
		var lb Letterboxd
		if json.Unmarshal(data, &lb) == nil {
			if config.Verbose {
				fmt.Printf("\n    - using cached Letterboxd data")
			}
			return &lb, nil
		}
	}

	// Step 1: Get Slug from redirect
	var slug string
	redirectURL := fmt.Sprintf("https://letterboxd.com/tmdb/%d/", tmdbID)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 15 * time.Second,
	}

	config.LetterboxdRateLimiter.Wait()
	retryConfig := DefaultRetryConfig()
	resp, err := RetryWithBackoff(retryConfig, func() (*http.Response, error) {
		req, err := http.NewRequest("GET", redirectURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		return noRedirectClient.Do(req)
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location, err := resp.Location()
		if err != nil {
			return nil, err
		}
		pathParts := strings.Split(strings.Trim(location.Path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "film" {
			slug = pathParts[1]
		} else {
			return nil, fmt.Errorf("\n    - could not parse slug from redirect location: %s", location.Path)
		}
	} else {
		return nil, fmt.Errorf("\n    - expected redirect, but got status %d", resp.StatusCode)
	}

	if slug == "" {
		return nil, fmt.Errorf("failed to extract slug")
	}

	// Step 2: Get JSON data using the slug
	config.LetterboxdRateLimiter.Wait()
	time.Sleep(500 * time.Millisecond)
	jsonURL := fmt.Sprintf("https://letterboxd.com/film/%s/json/", slug)

	resp, err = RetryWithBackoff(retryConfig, func() (*http.Response, error) {
		req, err := http.NewRequest("GET", jsonURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		return client.Do(req)
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\n    - failed to fetch letterboxd json, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var lbResponse LetterboxdResponse
	if err := json.Unmarshal(body, &lbResponse); err != nil {
		return nil, err
	}

	slugPtr := slug
	uidPtr := lbResponse.ID
	lidPtr := lbResponse.LID

	letterboxdInfo := &Letterboxd{
		Slug: &slugPtr,
		UID:  &uidPtr,
		LID:  &lidPtr,
	}

	SaveJSON(cacheFile, letterboxdInfo)
	time.Sleep(500 * time.Millisecond)

	return letterboxdInfo, nil
}
