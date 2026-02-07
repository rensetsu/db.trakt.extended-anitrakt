package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// Input structures
type InputShow struct {
	Title       string `json:"title"`
	MalID       int    `json:"mal_id"`
	TraktID     int    `json:"trakt_id"`
	GuessedSlug string `json:"guessed_slug"`
	Season      int    `json:"season"`
	Type        string `json:"type"`
}

type InputMovie struct {
	Title       string `json:"title"`
	MalID       int    `json:"mal_id"`
	TraktID     int    `json:"trakt_id"`
	GuessedSlug string `json:"guessed_slug"`
	Type        string `json:"type"`
}

// NotFoundEntry structure for items not found on Trakt
type NotFoundEntry struct {
	MalID int    `json:"mal_id"`
	Title string `json:"title"`
}

// Letterboxd API structure for JSON response
type LetterboxdResponse struct {
	ID   int    `json:"id"`
	LID  string `json:"lid"`
	Slug string `json:"slug"`
}

// Letterboxd structure for our output
type Letterboxd struct {
	Slug *string `json:"slug"`
	UID  *int    `json:"uid"`
	LID  *string `json:"lid"`
}

// Trakt API structures
type TraktExternals struct {
	TVDB   *int    `json:"tvdb,omitempty"`
	TMDB   *int    `json:"tmdb,omitempty"`
	IMDB   *string `json:"imdb,omitempty"`
	TVRage *int    `json:"tvrage,omitempty"`
}

type TraktShow struct {
	Title string `json:"title"`
	IDs   struct {
		Trakt int     `json:"trakt"`
		Slug  string  `json:"slug"`
		TVDB  *int    `json:"tvdb,omitempty"`
		IMDB  *string `json:"imdb,omitempty"`
		TMDB  *int    `json:"tmdb,omitempty"`
	} `json:"ids"`
	Year int `json:"year"`
}

type TraktMovie struct {
	Title string `json:"title"`
	IDs   struct {
		Trakt int     `json:"trakt"`
		Slug  string  `json:"slug"`
		IMDB  *string `json:"imdb,omitempty"`
		TMDB  *int    `json:"tmdb,omitempty"`
	} `json:"ids"`
	Year int `json:"year"`
}

type TraktSeason struct {
	Number int `json:"number"`
	IDs    struct {
		Trakt  int  `json:"trakt"`
		TVDB   *int `json:"tvdb,omitempty"`
		TMDB   *int `json:"tmdb,omitempty"`
		TVRage *int `json:"tvrage,omitempty"`
	} `json:"ids"`
}

type TraktExternalsShow struct {
	TVDB   *int    `json:"tvdb"`
	TMDB   *int    `json:"tmdb"`
	IMDB   *string `json:"imdb"`
	TVRage *int    `json:"tvrage"`
}

type TraktExternalsSeason struct {
	TVDB   *int `json:"tvdb"`
	TMDB   *int `json:"tmdb"`
	TVRage *int `json:"tvrage"`
}

type TraktExternalsMovie struct {
	TMDB       *int        `json:"tmdb"`
	IMDB       *string     `json:"imdb"`
	Letterboxd *Letterboxd `json:"letterboxd"`
}

// Output structures
type OutputShow struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title  string `json:"title"`
		ID     int    `json:"id"`
		Slug   string `json:"slug"`
		Type   string `json:"type"`
		Season *struct {
			ID        int                   `json:"id"`
			Number    int                   `json:"number"`
			Externals *TraktExternalsSeason `json:"externals"`
		} `json:"season"`
		IsSplitCour bool `json:"is_split_cour"`
	} `json:"trakt"`
	ReleaseYear int                 `json:"release_year"`
	Externals   *TraktExternalsShow `json:"externals"`
}

type OutputMovie struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
		Slug  string `json:"slug"`
		Type  string `json:"type"`
	} `json:"trakt"`
	ReleaseYear int                  `json:"release_year"`
	Externals   *TraktExternalsMovie `json:"externals"`
}

type Config struct {
	APIKey     string
	TvFile     string
	MovieFile  string
	OutputFile string
	Verbose    bool
	NoProgress bool
	TempDir    string
	Force      bool
}

type ChangeDetail struct {
	MalID  int    `json:"mal_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

type ProcessingStats struct {
	MediaType      string           `json:"media_type"`
	TotalBefore    int              `json:"total_before"`
	TotalAfter     int              `json:"total_after"`
	Created        int              `json:"created"`
	Updated        int              `json:"updated"`
	NotFound       int              `json:"not_found"`
	CreatedDetails []ChangeDetail   `json:"created_details"`
	UpdatedDetails []ChangeDetail   `json:"updated_details"`
	NotFoundDetails []ChangeDetail  `json:"not_found_details"`
}

func main() {
	config := parseFlags()

	if err := godotenv.Load(); err != nil && config.Verbose {
		fmt.Println("No .env file found, using environment variables")
	}

	if config.APIKey == "" {
		config.APIKey = os.Getenv("TRAKT_API_KEY")
	}

	if config.APIKey == "" {
		config.APIKey = promptForAPIKey()
	}

	// Create temp directory structure
	config.TempDir = filepath.Join(os.TempDir(), "trakt_data")
	os.MkdirAll(filepath.Join(config.TempDir, "shows"), 0755)
	os.MkdirAll(filepath.Join(config.TempDir, "movies"), 0755)
	os.MkdirAll(filepath.Join(config.TempDir, "seasons"), 0755)
	os.MkdirAll(filepath.Join(config.TempDir, "letterboxd"), 0755)

	// Create progress marker
	progressFile := filepath.Join(os.TempDir(), ".progress")
	os.WriteFile(progressFile, []byte{}, 0644)

	defer func() {
		os.RemoveAll(config.TempDir)
		os.Remove(progressFile)
	}()

	if config.TvFile != "" {
		processShows(config)
	}
	if config.MovieFile != "" {
		processMovies(config)
	}
}

func parseFlags() Config {
	var config Config
	flag.StringVar(&config.APIKey, "api-key", "", "Trakt API key")
	flag.StringVar(&config.TvFile, "tv", "", "Path to TV shows JSON file")
	flag.StringVar(&config.MovieFile, "movies", "", "Path to movies JSON file")
	flag.StringVar(&config.OutputFile, "output", "", "Output file path")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.NoProgress, "no-progress", false, "Disable progress bar")
	flag.BoolVar(&config.Force, "force", false, "Force update all entries, ignoring cache")
	flag.Parse()
	return config
}

func promptForAPIKey() string {
	fmt.Print("Enter Trakt API key: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal("Failed to read API key:", err)
	}
	fmt.Println()
	return string(bytePassword)
}

func processShows(config Config) {
	var shows []InputShow
	loadJSON(config.TvFile, &shows)

	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = filepath.Join("json/output", filepath.Base(strings.TrimSuffix(config.TvFile, ".json")) + "_ex.json")
	}

	var existingOutput []OutputShow
	loadJSONOptional(outputFile, &existingOutput)

	notExistMap := loadNotFound(outputFile)

	resultsMap := make(map[int]OutputShow)
	existingMap := make(map[int]OutputShow)
	if !config.Force {
		for _, show := range existingOutput {
			resultsMap[show.MyAnimeList.ID] = show
			existingMap[show.MyAnimeList.ID] = show
		}
	}

	stats := ProcessingStats{
		MediaType:       "tv",
		TotalBefore:     len(existingOutput),
		CreatedDetails:  []ChangeDetail{},
		UpdatedDetails:  []ChangeDetail{},
		NotFoundDetails: []ChangeDetail{},
	}

	var newNotExist []NotFoundEntry
	bar := setupProgressBar(len(shows), "Processing shows", config.NoProgress)
	client := &http.Client{Timeout: 30 * time.Second}

	for _, show := range shows {
		bar.Add(1)

		if shouldSkipShow(show, resultsMap, notExistMap, config) {
			continue
		}

		outputShow, err := getShowData(client, config, show)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				newNotExist = append(newNotExist, NotFoundEntry{MalID: show.MalID, Title: show.Title})
				if !notExistMap[show.MalID] {
					stats.NotFoundDetails = append(stats.NotFoundDetails, ChangeDetail{
						MalID:  show.MalID,
						Title:  show.Title,
						Reason: "Not found on Trakt.tv",
					})
				}
			} else {
				log.Printf("Error processing show %d: %v", show.MalID, err)
			}
			continue
		}

		// Track if this is new or updated
		if _, exists := existingMap[show.MalID]; exists {
			// Check if data actually changed
			if outputShow.Trakt.ID != resultsMap[show.MalID].Trakt.ID ||
				outputShow.Trakt.Slug != resultsMap[show.MalID].Trakt.Slug {
				stats.UpdatedDetails = append(stats.UpdatedDetails, ChangeDetail{
					MalID:  show.MalID,
					Title:  show.Title,
					Reason: "Trakt metadata updated",
				})
			}
		} else {
			stats.CreatedDetails = append(stats.CreatedDetails, ChangeDetail{
				MalID:  show.MalID,
				Title:  show.Title,
				Reason: "New entry added",
			})
		}
		resultsMap[show.MalID] = *outputShow
	}

	stats.TotalAfter = len(resultsMap)
	stats.Created = len(stats.CreatedDetails)
	stats.Updated = len(stats.UpdatedDetails)
	stats.NotFound = len(stats.NotFoundDetails)

	saveResults(outputFile, resultsMap)
	saveNotFound(outputFile, newNotExist, notExistMap)
	outputStats("tv", stats)

	if config.Verbose {
		fmt.Printf("\nProcessed %d shows, saved to %s\n", len(resultsMap), outputFile)
	}
}

func processMovies(config Config) {
	var movies []InputMovie
	loadJSON(config.MovieFile, &movies)

	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = filepath.Join("json/output", filepath.Base(strings.TrimSuffix(config.MovieFile, ".json")) + "_ex.json")
	}

	var existingOutput []OutputMovie
	loadJSONOptional(outputFile, &existingOutput)

	notExistMap := loadNotFound(outputFile)

	resultsMap := make(map[int]OutputMovie)
	existingMap := make(map[int]OutputMovie)
	if !config.Force {
		for _, movie := range existingOutput {
			resultsMap[movie.MyAnimeList.ID] = movie
			existingMap[movie.MyAnimeList.ID] = movie
		}
	}

	stats := ProcessingStats{
		MediaType:       "movies",
		TotalBefore:     len(existingOutput),
		CreatedDetails:  []ChangeDetail{},
		UpdatedDetails:  []ChangeDetail{},
		NotFoundDetails: []ChangeDetail{},
	}

	var newNotExist []NotFoundEntry
	bar := setupProgressBar(len(movies), "Processing movies", config.NoProgress)
	client := &http.Client{Timeout: 30 * time.Second}

	for _, movie := range movies {
		bar.Add(1)

		if shouldSkipMovie(movie, resultsMap, notExistMap, config) {
			continue
		}

		outputMovie, err := getMovieData(client, config, movie, resultsMap)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				newNotExist = append(newNotExist, NotFoundEntry{MalID: movie.MalID, Title: movie.Title})
				if !notExistMap[movie.MalID] {
					stats.NotFoundDetails = append(stats.NotFoundDetails, ChangeDetail{
						MalID:  movie.MalID,
						Title:  movie.Title,
						Reason: "Not found on Trakt.tv",
					})
				}
			} else {
				log.Printf("Error processing movie %d: %v", movie.MalID, err)
			}
			continue
		}

		// Track if this is new or updated
		if _, exists := existingMap[movie.MalID]; exists {
			// Check if data actually changed
			if outputMovie.Trakt.ID != resultsMap[movie.MalID].Trakt.ID ||
				outputMovie.Trakt.Slug != resultsMap[movie.MalID].Trakt.Slug {
				stats.UpdatedDetails = append(stats.UpdatedDetails, ChangeDetail{
					MalID:  movie.MalID,
					Title:  movie.Title,
					Reason: "Trakt metadata updated",
				})
			}
		} else {
			stats.CreatedDetails = append(stats.CreatedDetails, ChangeDetail{
				MalID:  movie.MalID,
				Title:  movie.Title,
				Reason: "New entry added",
			})
		}

		updateLetterboxdInfo(client, config, outputMovie)
		resultsMap[movie.MalID] = *outputMovie
	}

	stats.TotalAfter = len(resultsMap)
	stats.Created = len(stats.CreatedDetails)
	stats.Updated = len(stats.UpdatedDetails)
	stats.NotFound = len(stats.NotFoundDetails)

	saveMovieResults(outputFile, resultsMap)
	saveNotFound(outputFile, newNotExist, notExistMap)
	outputStats("movies", stats)

	if config.Verbose {
		fmt.Printf("\nProcessed %d movies, saved to %s\n", len(resultsMap), outputFile)
	}
}

// Helper functions for processing
func setupProgressBar(total int, description string, noProgress bool) *progressbar.ProgressBar {
	if noProgress {
		return progressbar.New(0) // Dummy progress bar
	}
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
	)
}

func loadNotFound(outputFile string) map[int]bool {
	notExistFile := filepath.Join("json/not_found", "not_exist_" + filepath.Base(outputFile))
	var notExist []NotFoundEntry
	loadJSONOptional(notExistFile, &notExist)
	notExistMap := make(map[int]bool)
	for _, entry := range notExist {
		notExistMap[entry.MalID] = true
	}
	return notExistMap
}

func shouldSkipShow(show InputShow, resultsMap map[int]OutputShow, notExistMap map[int]bool, config Config) bool {
	if _, exists := resultsMap[show.MalID]; exists && !config.Force {
		if config.Verbose {
			fmt.Printf("\nSkipping already processed show: %s (MAL ID: %d)", show.Title, show.MalID)
		}
		return true
	}
	if notExistMap[show.MalID] {
		if config.Verbose {
			fmt.Printf("\nSkipping non-existent show: %s (MAL ID: %d)", show.Title, show.MalID)
		}
		return true
	}
	return false
}

func shouldSkipMovie(movie InputMovie, resultsMap map[int]OutputMovie, notExistMap map[int]bool, config Config) bool {
	if notExistMap[movie.MalID] {
		if config.Verbose {
			fmt.Printf("\nSkipping non-existent movie: %s (MAL ID: %d)", movie.Title, movie.MalID)
		}
		return true
	}
	return false
}

func getShowData(client *http.Client, config Config, show InputShow) (*OutputShow, error) {
	traktID := show.TraktID
	seasonNum := show.Season
	malTitle := show.Title

	if config.Verbose {
		fmt.Printf("\nProcessing show: %s (MAL ID: %d, Trakt ID: %d)", malTitle, show.MalID, traktID)
	}

	traktShow, err := fetchTraktShow(client, config, traktID)
	if err != nil {
		return nil, err
	}

	outputShow := &OutputShow{
		MyAnimeList: struct {
			Title string `json:"title"`
			ID    int    `json:"id"`
		}{Title: malTitle, ID: show.MalID},
		Trakt: struct {
			Title  string `json:"title"`
			ID     int    `json:"id"`
			Slug   string `json:"slug"`
			Type   string `json:"type"`
			Season *struct {
				ID        int                   `json:"id"`
				Number    int                   `json:"number"`
				Externals *TraktExternalsSeason `json:"externals"`
			} `json:"season"`
			IsSplitCour bool `json:"is_split_cour"`
		}{Title: traktShow.Title, ID: traktShow.IDs.Trakt, Slug: traktShow.IDs.Slug, Type: "shows"},
		ReleaseYear: traktShow.Year,
		Externals:   &TraktExternalsShow{TVDB: traktShow.IDs.TVDB, TMDB: traktShow.IDs.TMDB, IMDB: traktShow.IDs.IMDB},
	}

	updateSeasonInfo(client, config, outputShow, traktID, seasonNum)
	return outputShow, nil
}

func getMovieData(client *http.Client, config Config, movie InputMovie, resultsMap map[int]OutputMovie) (*OutputMovie, error) {
	if outputMovie, exists := resultsMap[movie.MalID]; exists && !config.Force {
		if config.Verbose {
			fmt.Printf("\nUsing existing data for %s (MAL ID: %d)", movie.Title, movie.MalID)
		}
		return &outputMovie, nil
	}

	traktID := movie.TraktID
	malTitle := movie.Title

	if config.Verbose {
		fmt.Printf("\nProcessing new/forced movie: %s (MAL ID: %d, Trakt ID: %d)", malTitle, movie.MalID, traktID)
	}

	traktMovie, err := fetchTraktMovie(client, config, traktID)
	if err != nil {
		return nil, err
	}

	return &OutputMovie{
		MyAnimeList: struct {
			Title string `json:"title"`
			ID    int    `json:"id"`
		}{Title: malTitle, ID: movie.MalID},
		Trakt: struct {
			Title string `json:"title"`
			ID    int    `json:"id"`
			Slug  string `json:"slug"`
			Type  string `json:"type"`
		}{Title: traktMovie.Title, ID: traktMovie.IDs.Trakt, Slug: traktMovie.IDs.Slug, Type: "movies"},
		ReleaseYear: traktMovie.Year,
		Externals: &TraktExternalsMovie{
			TMDB: traktMovie.IDs.TMDB,
			IMDB: traktMovie.IDs.IMDB,
		},
	}, nil
}

func updateSeasonInfo(client *http.Client, config Config, outputShow *OutputShow, traktID, seasonNum int) {
	season, err := fetchTraktSeason(client, config, traktID, seasonNum)
	if err != nil {
		if config.Verbose {
			fmt.Printf("... season %d not found, marking as split cour", seasonNum)
		}
		outputShow.Trakt.IsSplitCour = true
		outputShow.Trakt.Season = nil
		return
	}

	outputShow.Trakt.IsSplitCour = false
	outputShow.Trakt.Season = &struct {
		ID        int                   `json:"id"`
		Number    int                   `json:"number"`
		Externals *TraktExternalsSeason `json:"externals"`
	}{
		ID:     season.IDs.Trakt,
		Number: season.Number,
		Externals: &TraktExternalsSeason{
			TVDB:   season.IDs.TVDB,
			TMDB:   season.IDs.TMDB,
			TVRage: season.IDs.TVRage,
		},
	}
}

func updateLetterboxdInfo(client *http.Client, config Config, outputMovie *OutputMovie) {
	if outputMovie.Externals != nil && (outputMovie.Externals.Letterboxd == nil || outputMovie.Externals.Letterboxd.Slug == nil) {
		if config.Verbose {
			fmt.Printf("\n    - checking for Letterboxd info...")
		}

		if tmdbID := outputMovie.Externals.TMDB; tmdbID != nil {
			letterboxdInfo, err := fetchLetterboxdInfo(client, config, *tmdbID)
			if err != nil {
				if config.Verbose {
					fmt.Printf("\n    - Could not fetch Letterboxd info for TMDB ID %d: %v", *tmdbID, err)
				}
			} else {
				outputMovie.Externals.Letterboxd = letterboxdInfo
				if config.Verbose {
					fmt.Printf("\n    - success!")
				}
			}
		} else if config.Verbose {
			fmt.Printf("\n    - no TMDB ID available.")
		}
	} else if config.Verbose {
		fmt.Printf("\n    - Letterboxd info already present.")
	}
}

func saveResults(outputFile string, resultsMap map[int]OutputShow) {
	var results []OutputShow
	for _, show := range resultsMap {
		results = append(results, show)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].MyAnimeList.ID < results[j].MyAnimeList.ID
	})
	saveJSON(outputFile, results)
}

func saveMovieResults(outputFile string, resultsMap map[int]OutputMovie) {
	var results []OutputMovie
	for _, movie := range resultsMap {
		results = append(results, movie)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].MyAnimeList.ID < results[j].MyAnimeList.ID
	})
	saveJSON(outputFile, results)
}

func saveNotFound(outputFile string, newNotExist []NotFoundEntry, notExistMap map[int]bool) {
	if len(newNotExist) > 0 {
		notExistFile := filepath.Join("json/not_found", "not_exist_" + filepath.Base(outputFile))
		var existingNotExist []NotFoundEntry
		loadJSONOptional(notExistFile, &existingNotExist)
		for _, entry := range newNotExist {
			if !notExistMap[entry.MalID] {
				existingNotExist = append(existingNotExist, entry)
			}
		}
		saveJSON(notExistFile, existingNotExist)
	}
}

func outputStats(mediaType string, stats ProcessingStats) {
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryFile == "" {
		return
	}

	title := strings.ToTitle(mediaType)
	diff := stats.TotalAfter - stats.TotalBefore
	diffStr := fmt.Sprintf("%+d", diff)
	if diff >= 0 {
		diffStr = "+" + fmt.Sprintf("%d", diff)
	}

	output := fmt.Sprintf("\n## %s - Summary\n\n", title)
	output += "| Metric | Before | After | Diff |\n|--------|--------|-------|------|\n"
	output += fmt.Sprintf("| Total Entries | %d | %d | %s |\n", stats.TotalBefore, stats.TotalAfter, diffStr)
	output += fmt.Sprintf("| Created | - | %d | +%d |\n", stats.Created, stats.Created)
	output += fmt.Sprintf("| Updated | - | %d | +%d |\n", stats.Updated, stats.Updated)
	output += fmt.Sprintf("| Not Found | - | %d | +%d |\n", stats.NotFound, stats.NotFound)

	if len(stats.CreatedDetails) > 0 {
		output += fmt.Sprintf("\n### ✨ Created (%d)\n\n", len(stats.CreatedDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.CreatedDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if len(stats.UpdatedDetails) > 0 {
		output += fmt.Sprintf("\n### 🔄 Updated (%d)\n\n", len(stats.UpdatedDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.UpdatedDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if len(stats.NotFoundDetails) > 0 {
		output += fmt.Sprintf("\n### ❌ Not Found (%d)\n\n", len(stats.NotFoundDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.NotFoundDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: Could not write to GITHUB_STEP_SUMMARY: %v", err)
		return
	}
	defer f.Close()
	f.WriteString(output)
}

func fetchLetterboxdInfo(client *http.Client, config Config, tmdbID int) (*Letterboxd, error) {
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

	req, err := http.NewRequest("GET", redirectURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := noRedirectClient.Do(req)
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
	time.Sleep(500 * time.Millisecond)
	jsonURL := fmt.Sprintf("https://letterboxd.com/film/%s/json/", slug)
	req, err = http.NewRequest("GET", jsonURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err = client.Do(req)
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

	saveJSON(cacheFile, letterboxdInfo)
	time.Sleep(500 * time.Millisecond)

	return letterboxdInfo, nil
}

func fetchTraktShow(client *http.Client, config Config, showID int) (*TraktShow, error) {
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
	time.Sleep(500 * time.Millisecond)

	url := fmt.Sprintf("https://api.trakt.tv/shows/%d", showID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", config.APIKey)

	resp, err := client.Do(req)
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

func fetchTraktMovie(client *http.Client, config Config, movieID int) (*TraktMovie, error) {
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
	time.Sleep(500 * time.Millisecond)

	url := fmt.Sprintf("https://api.trakt.tv/movies/%d", movieID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", config.APIKey)

	resp, err := client.Do(req)
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

func fetchTraktSeason(client *http.Client, config Config, showID, seasonNum int) (*TraktSeason, error) {
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
	time.Sleep(500 * time.Millisecond)

	url := fmt.Sprintf("https://api.trakt.tv/shows/%d/seasons", showID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", config.APIKey)

	resp, err := client.Do(req)
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

func loadJSON(filename string, v interface{}) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", filename, err)
	}

	if err := json.Unmarshal(bytes, v); err != nil {
		log.Fatalf("Failed to unmarshal JSON from %s: %v", filename, err)
	}
}

func loadJSONOptional(filename string, v interface{}) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Warning: Failed to open optional file %s: %v", filename, err)
		return
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Warning: Failed to read optional file %s: %v", filename, err)
		return
	}

	if err := json.Unmarshal(bytes, v); err != nil {
		log.Printf("Warning: Failed to unmarshal JSON from optional file %s: %v", filename, err)
	}
}

func saveJSON(filename string, v interface{}) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal data for %s: %v", filename, err)
	}

	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		log.Fatalf("Failed to write to file %s: %v", filename, err)
	}
}
