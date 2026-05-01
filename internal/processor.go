package internal

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ProcessShows processes TV shows
func ProcessShows(config Config) {
	var shows []InputShow
	LoadJSON(config.TvFile, &shows)

	// Track duplicates - detect shows with same MAL ID but different Trakt IDs
	malIDTraktMap := make(map[int][]int) // MAL ID -> list of Trakt IDs
	for _, show := range shows {
		malIDTraktMap[show.MalID] = append(malIDTraktMap[show.MalID], show.TraktID)
	}

	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = filepath.Join("json/output", filepath.Base(strings.TrimSuffix(config.TvFile, ".json"))+"_ex.json")
	}

	var existingOutput []OutputShow
	LoadJSONOptional(outputFile, &existingOutput)

	notExistMap := LoadNotFound(outputFile)
	overridesMap := LoadOverrides("tv")

	resultsMap := make(map[int]OutputShow)
	existingMap := make(map[int]OutputShow)
	if !config.Force {
		for _, show := range existingOutput {
			resultsMap[show.MyAnimeList.ID] = show
			existingMap[show.MyAnimeList.ID] = show
		}
	}

	// Track which Trakt IDs succeeded for each MAL ID
	successfulTraktIDs := make(map[int]int) // MAL ID -> Trakt ID that succeeded

	stats := ProcessingStats{
		MediaType:        "tv",
		TotalBefore:      len(existingOutput),
		CreatedDetails:   []ChangeDetail{},
		UpdatedDetails:   []ChangeDetail{},
		ModifiedDetails:  []ChangeDetail{},
		NotFoundDetails:  []ChangeDetail{},
		DuplicateDetails: []ChangeDetail{},
	}

	var newNotExist []NotFoundEntry
	bar := setupProgressBar(len(shows), "Processing shows", config.NoProgress)
	client := &http.Client{Timeout: 30 * time.Second}

	for _, show := range shows {
		bar.Add(1)

		if override, exists := overridesMap[show.MalID]; exists && override.Ignore {
			if config.Verbose {
				fmt.Printf("\nSkipping ignored show: %s (MAL ID: %d) - %s", show.Title, show.MalID, override.Description)
			}
			continue
		}

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

		if _, exists := existingMap[show.MalID]; exists {
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

		if override, exists := overridesMap[show.MalID]; exists && !override.Ignore {
			oldShow := *outputShow
			ApplyShowOverride(outputShow, override)
			if oldShow.Trakt.ID != outputShow.Trakt.ID ||
				oldShow.Trakt.Slug != outputShow.Trakt.Slug ||
				oldShow.Externals != outputShow.Externals {
				stats.ModifiedDetails = append(stats.ModifiedDetails, ChangeDetail{
					MalID:  show.MalID,
					Title:  show.Title,
					Reason: override.Description,
				})
			}
		}

		resultsMap[show.MalID] = *outputShow
		successfulTraktIDs[show.MalID] = show.TraktID
	}

	// Build duplicate report: for each MAL ID with multiple Trakt IDs, report the failed ones
	for malID, traktIDs := range malIDTraktMap {
		if len(traktIDs) > 1 {
			// Find which Trakt ID succeeded
			successfulID := successfulTraktIDs[malID]

			// Find the title (from any entry with this MAL ID)
			var title string
			for _, item := range shows {
				if item.MalID == malID {
					title = item.Title
					break
				}
			}

			// Collect invalid Trakt IDs
			var invalidIDs []int
			for _, traktID := range traktIDs {
				if traktID != successfulID {
					invalidIDs = append(invalidIDs, traktID)
				}
			}

			// Format invalid IDs as [id1, id2, ...]
			invalidStr := ""
			for i, id := range invalidIDs {
				if i > 0 {
					invalidStr += ", "
				}
				invalidStr += fmt.Sprintf("%d", id)
			}

			// Format the reason message
			reason := fmt.Sprintf("Duplicate: valid Trakt ID %d, invalid [%s]", successfulID, invalidStr)
			if successfulID == 0 {
				reason = fmt.Sprintf("Duplicate: no valid Trakt ID, invalid [%s]", invalidStr)
			}

			stats.DuplicateDetails = append(stats.DuplicateDetails, ChangeDetail{
				MalID:  malID,
				Title:  title,
				Reason: reason,
			})
		}
	}

	stats.TotalAfter = len(resultsMap)
	stats.Created = len(stats.CreatedDetails)
	stats.Updated = len(stats.UpdatedDetails)
	stats.Modified = len(stats.ModifiedDetails)
	stats.NotFound = len(stats.NotFoundDetails)

	SaveResults(outputFile, resultsMap)
	SaveNotFound(outputFile, newNotExist, notExistMap)
	OutputStats("tv", stats)

	if config.Verbose {
		fmt.Printf("\nProcessed %d shows, saved to %s\n", len(resultsMap), outputFile)
	}
}

// ProcessMovies processes movies
func ProcessMovies(config Config) {
	var movies []InputMovie
	LoadJSON(config.MovieFile, &movies)

	// Track duplicates - detect movies with same MAL ID but different Trakt IDs
	malIDTraktMap := make(map[int][]int) // MAL ID -> list of Trakt IDs
	for _, movie := range movies {
		malIDTraktMap[movie.MalID] = append(malIDTraktMap[movie.MalID], movie.TraktID)
	}

	outputFile := config.OutputFile
	if outputFile == "" {
		outputFile = filepath.Join("json/output", filepath.Base(strings.TrimSuffix(config.MovieFile, ".json"))+"_ex.json")
	}

	var existingOutput []OutputMovie
	LoadJSONOptional(outputFile, &existingOutput)

	notExistMap := LoadNotFound(outputFile)
	overridesMap := LoadOverrides("movies")

	resultsMap := make(map[int]OutputMovie)
	existingMap := make(map[int]OutputMovie)
	if !config.Force {
		for _, movie := range existingOutput {
			resultsMap[movie.MyAnimeList.ID] = movie
			existingMap[movie.MyAnimeList.ID] = movie
		}
	}

	// Track which Trakt IDs succeeded for each MAL ID
	successfulTraktIDs := make(map[int]int) // MAL ID -> Trakt ID that succeeded

	stats := ProcessingStats{
		MediaType:                 "movies",
		TotalBefore:               len(existingOutput),
		CreatedDetails:            []ChangeDetail{},
		UpdatedDetails:            []ChangeDetail{},
		ModifiedDetails:           []ChangeDetail{},
		NotFoundDetails:           []ChangeDetail{},
		DuplicateDetails:          []ChangeDetail{},
		LetterboxdNotFoundDetails: []ChangeDetail{},
	}

	var newNotExist []NotFoundEntry
	bar := setupProgressBar(len(movies), "Processing movies", config.NoProgress)
	client := &http.Client{Timeout: 30 * time.Second}

	for _, movie := range movies {
		bar.Add(1)

		if override, exists := overridesMap[movie.MalID]; exists && override.Ignore {
			if config.Verbose {
				fmt.Printf("\nSkipping ignored movie: %s (MAL ID: %d) - %s", movie.Title, movie.MalID, override.Description)
			}
			continue
		}

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

		if _, exists := existingMap[movie.MalID]; exists {
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

		// Pass existing movie data to preserve Letterboxd info if fetch fails
		var existingMovie *OutputMovie
		if existing, exists := existingMap[movie.MalID]; exists {
			existingMovie = &existing
		}
		letterboxdNotFound := updateLetterboxdInfo(client, config, outputMovie, existingMovie)
		if letterboxdNotFound != nil {
			stats.LetterboxdNotFoundDetails = append(stats.LetterboxdNotFoundDetails, *letterboxdNotFound)
		}

		if override, exists := overridesMap[movie.MalID]; exists && !override.Ignore {
			oldMovie := *outputMovie
			ApplyMovieOverride(outputMovie, override)
			if oldMovie.Trakt.ID != outputMovie.Trakt.ID ||
				oldMovie.Trakt.Slug != outputMovie.Trakt.Slug ||
				oldMovie.Externals != outputMovie.Externals {
				stats.ModifiedDetails = append(stats.ModifiedDetails, ChangeDetail{
					MalID:  movie.MalID,
					Title:  movie.Title,
					Reason: override.Description,
				})
			}
		}

		resultsMap[movie.MalID] = *outputMovie
		successfulTraktIDs[movie.MalID] = movie.TraktID
	}

	// Build duplicate report: for each MAL ID with multiple Trakt IDs, report the failed ones
	for malID, traktIDs := range malIDTraktMap {
		if len(traktIDs) > 1 {
			// Find which Trakt ID succeeded
			successfulID := successfulTraktIDs[malID]

			// Find the title (from any entry with this MAL ID)
			var title string
			for _, item := range movies {
				if item.MalID == malID {
					title = item.Title
					break
				}
			}

			// Collect invalid Trakt IDs
			var invalidIDs []int
			for _, traktID := range traktIDs {
				if traktID != successfulID {
					invalidIDs = append(invalidIDs, traktID)
				}
			}

			// Format invalid IDs as [id1, id2, ...]
			invalidStr := ""
			for i, id := range invalidIDs {
				if i > 0 {
					invalidStr += ", "
				}
				invalidStr += fmt.Sprintf("%d", id)
			}

			// Format the reason message
			reason := fmt.Sprintf("Duplicate: valid Trakt ID %d, invalid [%s]", successfulID, invalidStr)
			if successfulID == 0 {
				reason = fmt.Sprintf("Duplicate: no valid Trakt ID, invalid [%s]", invalidStr)
			}

			stats.DuplicateDetails = append(stats.DuplicateDetails, ChangeDetail{
				MalID:  malID,
				Title:  title,
				Reason: reason,
			})
		}
	}

	stats.TotalAfter = len(resultsMap)
	stats.Created = len(stats.CreatedDetails)
	stats.Updated = len(stats.UpdatedDetails)
	stats.Modified = len(stats.ModifiedDetails)
	stats.NotFound = len(stats.NotFoundDetails)

	SaveMovieResults(outputFile, resultsMap)
	SaveNotFound(outputFile, newNotExist, notExistMap)
	OutputStats("movies", stats)

	if config.Verbose {
		fmt.Printf("\nProcessed %d movies, saved to %s\n", len(resultsMap), outputFile)
	}
}

// getShowData gets data for a show
func getShowData(client *http.Client, config Config, show InputShow) (*OutputShow, error) {
	traktID := show.TraktID
	seasonNum := show.Season
	malTitle := show.Title

	if config.Verbose {
		fmt.Printf("\nProcessing show: %s (MAL ID: %d, Trakt ID: %d)", malTitle, show.MalID, traktID)
	}

	traktShow, err := FetchTraktShow(client, config, traktID)
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

// getMovieData gets data for a movie
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

	traktMovie, err := FetchTraktMovie(client, config, traktID)
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

// updateSeasonInfo updates season information
func updateSeasonInfo(client *http.Client, config Config, outputShow *OutputShow, traktID, seasonNum int) {
	season, err := FetchTraktSeason(client, config, traktID, seasonNum)
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

// updateLetterboxdInfo updates Letterboxd information, preserving existing data if fetch fails
func updateLetterboxdInfo(client *http.Client, config Config, outputMovie *OutputMovie, existingMovie *OutputMovie) *ChangeDetail {
	if outputMovie.Externals != nil && (outputMovie.Externals.Letterboxd == nil || outputMovie.Externals.Letterboxd.Slug == nil) {
		if config.Verbose {
			fmt.Printf("\n    - checking for Letterboxd info...")
		}

		if tmdbID := outputMovie.Externals.TMDB; tmdbID != nil {
			// Get existing Letterboxd data as fallback if it exists
			var existingLetterboxdData *Letterboxd
			if existingMovie != nil && existingMovie.Externals != nil && existingMovie.Externals.Letterboxd != nil {
				existingLetterboxdData = existingMovie.Externals.Letterboxd
			}

			letterboxdInfo, err := FetchLetterboxdInfo(client, config, *tmdbID, existingLetterboxdData)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Film not found on Letterboxd") {
					// Return a ChangeDetail for tracking in summary
					return &ChangeDetail{
						MalID:  outputMovie.MyAnimeList.ID,
						Title:  outputMovie.MyAnimeList.Title,
						Reason: "Film not found on Letterboxd",
					}
				}
				if config.Verbose {
					fmt.Printf("\n    - Could not fetch Letterboxd info for TMDB ID %d: %v", *tmdbID, err)
				}
			} else {
				outputMovie.Externals.Letterboxd = letterboxdInfo
				if config.Verbose && letterboxdInfo != nil {
					fmt.Printf("\n    - success!")
				}
			}
		} else if config.Verbose {
			fmt.Printf("\n    - no TMDB ID available.")
		}
	} else if config.Verbose {
		fmt.Printf("\n    - Letterboxd info already present.")
	}
	return nil
}

// shouldSkipShow checks if a show should be skipped
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

// shouldSkipMovie checks if a movie should be skipped
func shouldSkipMovie(movie InputMovie, resultsMap map[int]OutputMovie, notExistMap map[int]bool, config Config) bool {
	if notExistMap[movie.MalID] {
		if config.Verbose {
			fmt.Printf("\nSkipping non-existent movie: %s (MAL ID: %d)", movie.Title, movie.MalID)
		}
		return true
	}
	return false
}

// setupProgressBar creates a progress bar
func setupProgressBar(total int, description string, noProgress bool) *progressbar.ProgressBar {
	if noProgress {
		return progressbar.New(0)
	}
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
	)
}
