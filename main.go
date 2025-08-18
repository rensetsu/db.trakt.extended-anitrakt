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

// Override structure
type Override struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title  string `json:"title"`
		ID     int    `json:"id"`
		Type   string `json:"type"`
		Season *struct {
			Number int `json:"number"`
		} `json:"season,omitempty"`
	} `json:"trakt"`
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
		Trakt int    `json:"trakt"`
		Slug  string `json:"slug"`
		TVDB  *int   `json:"tvdb,omitempty"`
		IMDB  *string `json:"imdb,omitempty"`
		TMDB  *int   `json:"tmdb,omitempty"`
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
	TVDB   *int    `json:"tvdb"`
	TMDB   *int    `json:"tmdb"`
	TVRage *int    `json:"tvrage"`
}

type TraktExternalsMovie struct {
	TMDB   *int    `json:"tmdb"`
	IMDB   *string `json:"imdb"`
}

// Output structures
type OutputShow struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title    string  `json:"title"`
		ID       int     `json:"id"`
		Slug     string  `json:"slug"`
		Type     string  `json:"type"`
		Season   *struct {
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
		ID int `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title string `json:"title"`
		ID   int    `json:"id"`
		Slug string `json:"slug"`
		Type string `json:"type"`
	} `json:"trakt"`
	ReleaseYear int                  `json:"release_year"`
	Externals   *TraktExternalsMovie `json:"externals"`
}

type Config struct {
	APIKey      string
	TvFile      string
	MovieFile   string
	OutputFile  string
	Verbose     bool
	NoProgress  bool
	TempDir     string
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

	// Load existing output to skip processed items
	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = strings.TrimSuffix(config.TvFile, ".json") + "_ex.json"
	}

	var existingOutput []OutputShow
	loadJSONOptional(outputFile, &existingOutput)
	
	// Load not exist list
	notExistFile := "not_exist_" + filepath.Base(outputFile)
	var notExist []int
	loadJSONOptional(notExistFile, &notExist)
	notExistMap := make(map[int]bool)
	for _, id := range notExist {
		notExistMap[id] = true
	}

	// Load overrides
	overrideFile := "override_" + filepath.Base(config.TvFile)
	var overrides []Override
	loadJSONOptional(overrideFile, &overrides)
	overrideMap := make(map[int]Override)
	for _, override := range overrides {
		overrideMap[override.Trakt.ID] = override
	}

	existingMap := make(map[int]OutputShow)
	for _, show := range existingOutput {
		existingMap[show.MyAnimeList.ID] = show
	}

	var results []OutputShow
	var newNotExist []int

	// Copy existing results
	for _, show := range existingOutput {
		results = append(results, show)
	}

	var bar *progressbar.ProgressBar
	if !config.NoProgress {
		bar = progressbar.NewOptions(len(shows),
			progressbar.OptionSetDescription("Processing shows"),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionClearOnFinish(),
		)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, show := range shows {
		if !config.NoProgress {
			bar.Add(1)
		}

		if _, exists := existingMap[show.MalID]; exists {
			if config.Verbose {
				fmt.Printf("Skipping already processed show: %s (MAL ID: %d)\n", show.Title, show.MalID)
			}
			continue
		}

		if notExistMap[show.TraktID] {
			if config.Verbose {
				fmt.Printf("Skipping non-existent show: %s (Trakt ID: %d)\n", show.Title, show.TraktID)
			}
			continue
		}

		var traktID int
		var seasonNum int
		var malTitle, traktTitle string

		if override, hasOverride := overrideMap[show.TraktID]; hasOverride {
			traktID = override.Trakt.ID
			malTitle = override.MyAnimeList.Title
			traktTitle = override.Trakt.Title
			if override.Trakt.Season != nil {
				seasonNum = override.Trakt.Season.Number
			} else {
				seasonNum = show.Season
			}
		} else {
			traktID = show.TraktID
			seasonNum = show.Season
			malTitle = show.Title
		}

		if config.Verbose {
			fmt.Printf("\nProcessing show: %s (MAL ID: %d, Trakt ID: %d)", malTitle, show.MalID, traktID)
		}

		traktShow, err := fetchTraktShow(client, config, traktID)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				if config.Verbose {
					fmt.Printf("Show not found on Trakt: %d\n", traktID)
				}
				newNotExist = append(newNotExist, traktID)
				continue
			}
			log.Printf("Error fetching show %d: %v", traktID, err)
			continue
		}

		if traktTitle == "" {
			traktTitle = traktShow.Title
		}

		outputShow := OutputShow{
			MyAnimeList: struct {
				Title string `json:"title"`
				ID    int `json:"id"`
			}{
				Title: malTitle,
				ID: show.MalID,
			},
			Trakt: struct {
				Title    string  `json:"title"`
				ID       int     `json:"id"`
				Slug     string  `json:"slug"`
				Type     string  `json:"type"`
				Season   *struct {
					ID        int             `json:"id"`
					Number    int             `json:"number"`
					Externals *TraktExternalsSeason `json:"externals"`
				} `json:"season"`
				IsSplitCour bool `json:"is_split_cour"`
			}{
				Title: traktTitle,
				ID:    traktShow.IDs.Trakt,
				Slug:  traktShow.IDs.Slug,
				Type:  "shows",
			},
			ReleaseYear: traktShow.Year,
			Externals: &TraktExternalsShow{
				TVDB:   traktShow.IDs.TVDB,
				TMDB:   traktShow.IDs.TMDB,
				IMDB:   traktShow.IDs.IMDB,
				TVRage: nil,
			},
		}

		// Fetch season info
		season, err := fetchTraktSeason(client, config, traktID, seasonNum)
		if err != nil {
			if config.Verbose {
				fmt.Printf("Season %d not found for show %d, marking as split cour\n", seasonNum, traktID)
			}
			outputShow.Trakt.IsSplitCour = true
			outputShow.Trakt.Season = nil
		} else {
			outputShow.Trakt.IsSplitCour = false
			outputShow.Trakt.Season = &struct {
				ID        int             `json:"id"`
				Number    int             `json:"number"`
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

		results = append(results, outputShow)
	}

	// Sort by MAL ID
	sort.Slice(results, func(i, j int) bool {
		return results[i].MyAnimeList.ID < results[j].MyAnimeList.ID
	})

	// Save results
	saveJSON(outputFile, results)

	// Save not exist list
	if len(newNotExist) > 0 {
		allNotExist := append(notExist, newNotExist...)
		saveJSON(notExistFile, allNotExist)
	}

	if config.Verbose {
		fmt.Printf("Processed %d shows, saved to %s\n", len(results), outputFile)
	}
}

func processMovies(config Config) {
	var movies []InputMovie
	loadJSON(config.MovieFile, &movies)

	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = strings.TrimSuffix(config.MovieFile, ".json") + "_ex.json"
	}

	var existingOutput []OutputMovie
	loadJSONOptional(outputFile, &existingOutput)

	// Load not exist list
	notExistFile := "not_exist_" + filepath.Base(outputFile)
	var notExist []int
	loadJSONOptional(notExistFile, &notExist)
	notExistMap := make(map[int]bool)
	for _, id := range notExist {
		notExistMap[id] = true
	}

	// Load overrides
	overrideFile := "override_" + filepath.Base(config.MovieFile)
	var overrides []Override
	loadJSONOptional(overrideFile, &overrides)
	overrideMap := make(map[int]Override)
	for _, override := range overrides {
		overrideMap[override.Trakt.ID] = override
	}

	existingMap := make(map[int]OutputMovie)
	for _, movie := range existingOutput {
		existingMap[movie.MyAnimeList.ID] = movie
	}

	var results []OutputMovie
	var newNotExist []int

	// Copy existing results
	for _, movie := range existingOutput {
		results = append(results, movie)
	}

	var bar *progressbar.ProgressBar
	if !config.NoProgress {
		bar = progressbar.NewOptions(len(movies),
			progressbar.OptionSetDescription("Processing movies"),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionClearOnFinish(),
		)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, movie := range movies {
		if !config.NoProgress {
			bar.Add(1)
		}

		if _, exists := existingMap[movie.MalID]; exists {
			if config.Verbose {
				fmt.Printf("Skipping already processed movie: %s (MAL ID: %d)\n", movie.Title, movie.MalID)
			}
			continue
		}

		if notExistMap[movie.TraktID] {
			if config.Verbose {
				fmt.Printf("Skipping non-existent movie: %s (Trakt ID: %d)\n", movie.Title, movie.TraktID)
			}
			continue
		}

		var traktID int
		var malTitle, traktTitle string

		if override, hasOverride := overrideMap[movie.TraktID]; hasOverride {
			traktID = override.Trakt.ID
			malTitle = override.MyAnimeList.Title
			traktTitle = override.Trakt.Title
		} else {
			traktID = movie.TraktID
			malTitle = movie.Title
		}

		if config.Verbose {
			fmt.Printf("\nProcessing movie: %s (MAL ID: %d, Trakt ID: %d)", malTitle, movie.MalID, traktID)
		}

		traktMovie, err := fetchTraktMovie(client, config, traktID)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				if config.Verbose {
					fmt.Printf("Movie not found on Trakt: %d\n", traktID)
				}
				newNotExist = append(newNotExist, traktID)
				continue
			}
			log.Printf("Error fetching movie %d: %v", traktID, err)
			continue
		}

		if traktTitle == "" {
			traktTitle = traktMovie.Title
		}

		outputMovie := OutputMovie{
			MyAnimeList: struct {
				Title string `json:"title"`
				ID    int `json:"id"`
			}{
				Title: malTitle,
				ID: movie.MalID,
			},
			Trakt: struct {
				Title string `json:"title"`
				ID   int    `json:"id"`
				Slug string `json:"slug"`
				Type string `json:"type"`
			}{
				Title: traktTitle,
				ID:    traktMovie.IDs.Trakt,
				Slug:  traktMovie.IDs.Slug,
				Type:  "movies",
			},
			ReleaseYear: traktMovie.Year,
			Externals: &TraktExternalsMovie{
				TMDB: traktMovie.IDs.TMDB,
				IMDB: traktMovie.IDs.IMDB,
			},
		}

		results = append(results, outputMovie)
	}

	// Sort by MAL ID
	sort.Slice(results, func(i, j int) bool {
		return results[i].MyAnimeList.ID < results[j].MyAnimeList.ID
	})

	// Save results
	saveJSON(outputFile, results)

	// Save not exist list
	if len(newNotExist) > 0 {
		allNotExist := append(notExist, newNotExist...)
		saveJSON(notExistFile, allNotExist)
	}

	if config.Verbose {
		fmt.Printf("Processed %d movies, saved to %s\n", len(results), outputFile)
	}
}

func fetchTraktShow(client *http.Client, config Config, showID int) (*TraktShow, error) {
	cacheFile := filepath.Join(config.TempDir, "shows", fmt.Sprintf("%d.json", showID))
	
	// Check cache first
	if data, err := os.ReadFile(cacheFile); err == nil {
		var show TraktShow
		if json.Unmarshal(data, &show) == nil {
			if config.Verbose {
				fmt.Printf("Using cached data for show %d\n", showID)
			}
			return &show, nil
		}
	}

	if config.Verbose {
		fmt.Printf("Fetching show %d from Trakt API\n", showID)
	}

	// Rate limit: wait 0.5 seconds between requests
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
		return nil, fmt.Errorf("show not found: 404")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var show TraktShow
	if err := json.Unmarshal(body, &show); err != nil {
		return nil, err
	}

	// Cache the result
	os.WriteFile(cacheFile, body, 0644)

	return &show, nil
}

func fetchTraktMovie(client *http.Client, config Config, movieID int) (*TraktMovie, error) {
	cacheFile := filepath.Join(config.TempDir, "movies", fmt.Sprintf("%d.json", movieID))
	
	// Check cache first
	if data, err := os.ReadFile(cacheFile); err == nil {
		var movie TraktMovie
		if json.Unmarshal(data, &movie) == nil {
			if config.Verbose {
				fmt.Printf("Using cached data for movie %d\n", movieID)
			}
			return &movie, nil
		}
	}

	if config.Verbose {
		fmt.Printf("Fetching movie %d from Trakt API\n", movieID)
	}

	// Rate limit: wait 0.5 seconds between requests
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
		return nil, fmt.Errorf("movie not found: 404")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var movie TraktMovie
	if err := json.Unmarshal(body, &movie); err != nil {
		return nil, err
	}

	// Cache the result
	os.WriteFile(cacheFile, body, 0644)

	return &movie, nil
}

func fetchTraktSeason(client *http.Client, config Config, showID, seasonNum int) (*TraktSeason, error) {
	cacheFile := filepath.Join(config.TempDir, "seasons", fmt.Sprintf("%d.json", showID))
	
	// Check cache first
	if data, err := os.ReadFile(cacheFile); err == nil {
		var seasons []TraktSeason
		if json.Unmarshal(data, &seasons) == nil {
			for _, season := range seasons {
				if season.Number == seasonNum {
					if config.Verbose {
						fmt.Printf("Using cached data for show %d season %d\n", showID, seasonNum)
					}
					return &season, nil
				}
			}
		}
	}

	if config.Verbose {
		fmt.Printf("Fetching seasons for show %d from Trakt API\n", showID)
	}

	// Rate limit: wait 0.5 seconds between requests
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
		return nil, fmt.Errorf("seasons not found: 404")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var seasons []TraktSeason
	if err := json.Unmarshal(body, &seasons); err != nil {
		return nil, err
	}

	// Cache the result
	os.WriteFile(cacheFile, body, 0644)

	// Find the requested season
	for _, season := range seasons {
		if season.Number == seasonNum {
			return &season, nil
		}
	}

	return nil, fmt.Errorf("season %d not found", seasonNum)
}

func loadJSON(filename string, v interface{}) {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", filename, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("Failed to parse JSON from %s: %v", filename, err)
	}
}

func loadJSONOptional(filename string, v interface{}) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return // File doesn't exist, that's okay
	}

	json.Unmarshal(data, v) // Ignore errors for optional files
}

func saveJSON(filename string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Fatalf("Failed to write file %s: %v", filename, err)
	}
}
