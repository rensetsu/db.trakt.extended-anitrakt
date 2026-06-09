package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rensetsu/db.trakt.extended-anitrakt/internal"
)

func main() {
	movieFile := flag.String("movies", "json/output/movies_ex.json", "Path to movies_ex.json file")
	verbose := flag.Bool("verbose", true, "Verbose output")
	force := flag.Bool("force", false, "Force re-fetch Letterboxd data even if already cached")
	flag.Parse()

	// Load existing output
	var existingMovies []internal.OutputMovie
	internal.LoadJSONOptional(*movieFile, &existingMovies)

	if len(existingMovies) == 0 {
		fmt.Printf("No movies found in %s\n", *movieFile)
		os.Exit(0)
	}

	config := internal.Config{
		TempDir:               filepath.Join(os.TempDir(), "trakt_data"),
		Verbose:               *verbose,
		Force:                 *force,
		LetterboxdRateLimiter: internal.NewLetterboxdRateLimiter(),
	}

	// Ensure cache directory exists
	os.MkdirAll(filepath.Join(config.TempDir, "letterboxd"), 0755)

	client := &http.Client{Timeout: 30 * time.Second}
	resultsMap := make(map[int]internal.OutputMovie)

	// Identify missing ones and load others into resultsMap
	var toFetch []internal.OutputMovie
	for _, movie := range existingMovies {
		resultsMap[movie.MyAnimeList.ID] = movie

		needsFetch := false
		if movie.Externals == nil {
			needsFetch = true
		} else if movie.Externals.Letterboxd == nil || movie.Externals.Letterboxd.Slug == nil || *movie.Externals.Letterboxd.Slug == "" {
			needsFetch = true
		}

		if needsFetch && movie.Externals != nil && movie.Externals.TMDB != nil && *movie.Externals.TMDB != 0 {
			toFetch = append(toFetch, movie)
		}
	}

	if len(toFetch) == 0 {
		fmt.Println("All movies already have Letterboxd metadata or are missing TMDB IDs.")
		os.Exit(0)
	}

	fmt.Printf("Found %d movies missing Letterboxd metadata. Starting fetch...\n", len(toFetch))

	successCount := 0
	failCount := 0

	for i, movie := range toFetch {
		fmt.Printf("[%d/%d] Fetching Letterboxd data for: %s (MAL ID: %d, TMDB ID: %d)...", i+1, len(toFetch), movie.MyAnimeList.Title, movie.MyAnimeList.ID, *movie.Externals.TMDB)

		var existingLetterboxd *internal.Letterboxd
		if movie.Externals != nil {
			existingLetterboxd = movie.Externals.Letterboxd
		}

		lbInfo, err := internal.FetchLetterboxdInfo(client, config, *movie.Externals.TMDB, existingLetterboxd)
		if err != nil {
			fmt.Printf(" ERROR: %v\n", err)
			failCount++
			continue
		}

		if lbInfo != nil && lbInfo.Slug != nil && *lbInfo.Slug != "" {
			if movie.Externals == nil {
				movie.Externals = &internal.TraktExternalsMovie{
					TMDB: movie.Externals.TMDB,
					IMDB: movie.Externals.IMDB,
				}
			}
			movie.Externals.Letterboxd = lbInfo
			resultsMap[movie.MyAnimeList.ID] = movie
			successCount++
			fmt.Println(" Success!")
		} else {
			fmt.Println(" Skip (not found or blocked)")
			failCount++
		}
	}

	if successCount > 0 {
		fmt.Printf("\nSaving %d updated movies to %s...\n", successCount, *movieFile)
		internal.SaveMovieResults(*movieFile, resultsMap)
	}

	fmt.Printf("\nFinished: %d successfully fetched, %d failed/skipped.\n", successCount, failCount)
}
