package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/rensetsu/db.trakt.extended-anitrakt/internal"
)

func main() {
	config := internal.ParseFlags()

	if err := godotenv.Load(); err != nil && config.Verbose {
		fmt.Println("No .env file found, using environment variables")
	}

	if config.APIKey == "" {
		config.APIKey = os.Getenv("TRAKT_API_KEY")
	}

	if config.APIKey == "" {
		config.APIKey = internal.PromptForAPIKey()
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
		// Clean up temp directories except letterboxd (persisted by GitHub Actions cache)
		os.RemoveAll(filepath.Join(config.TempDir, "shows"))
		os.RemoveAll(filepath.Join(config.TempDir, "movies"))
		os.RemoveAll(filepath.Join(config.TempDir, "seasons"))
		os.Remove(progressFile)
	}()

	if config.TvFile != "" {
		internal.ProcessShows(config)
	}
	if config.MovieFile != "" {
		internal.ProcessMovies(config)
	}
}
